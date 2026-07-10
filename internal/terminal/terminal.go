// Package terminal manages interactive shell sessions for the User Terminal tab.
package terminal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"glyphdeck/internal/lifecycle"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Terminal represents a running interactive shell session.
type Terminal struct {
	ID        string
	ProjectID string
	Cwd       string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	mu        sync.Mutex
	closed    bool
	createdAt time.Time
}

// Status reports the terminal running state.
type Status struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Running   bool   `json:"running"`
	Cwd       string `json:"cwd"`
	Shell     string `json:"shell"`
	CreatedAt string `json:"createdAt"`
}

// ProjectResolver resolves a project path from its ID.
type ProjectResolver interface {
	GetPath(ctx context.Context, projectID string) (string, error)
}

// Manager creates and tracks terminal sessions.
type Manager struct {
	mu         sync.RWMutex
	terminals  map[string]*Terminal
	nextID     int
	shellPath  string
	shellArgs  []string
	resolver   ProjectResolver
	terminator func(*os.Process) error
}

// NewManager creates a terminal manager with platform-appropriate shell.
func NewManager(resolver ProjectResolver) *Manager {
	shell, args := detectShell()
	return &Manager{
		terminals:  make(map[string]*Terminal),
		shellPath:  shell,
		shellArgs:  args,
		resolver:   resolver,
		terminator: lifecycle.TerminateProcessTree,
	}
}

// detectShell returns the shell path and args for the current platform.
func detectShell() (string, []string) {
	if _, err := exec.LookPath("pwsh.exe"); err == nil {
		return "pwsh.exe", []string{"-NoLogo", "-NoProfile"}
	}
	if _, err := exec.LookPath("powershell.exe"); err == nil {
		return "powershell.exe", []string{"-NoLogo", "-NoProfile"}
	}
	if _, err := exec.LookPath("cmd.exe"); err == nil {
		return "cmd.exe", nil
	}
	return "bash", nil
}

// Start launches an interactive shell in the given working directory.
func (m *Manager) Start(ctx context.Context, projectID, cwd string) (*Status, error) {
	if cwd == "" || cwd == "." {
		if m.resolver != nil {
			resolved, err := m.resolver.GetPath(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("resolve project path: %w", err)
			}
			cwd = resolved
		}
	}
	if cwd == "" {
		return nil, fmt.Errorf("cwd is required")
	}
	if _, err := os.Stat(cwd); err != nil {
		return nil, fmt.Errorf("cwd does not exist: %w", err)
	}

	cmd := exec.Command(m.shellPath, m.shellArgs...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start shell: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("term-%d", m.nextID)

	term := &Terminal{
		ID:        id,
		ProjectID: projectID,
		Cwd:       cwd,
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		createdAt: time.Now(),
	}

	m.terminals[id] = term

	// Monitor process exit.
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if t, ok := m.terminals[id]; ok {
			t.closed = true
		}
		m.mu.Unlock()
	}()

	return &Status{
		ID:        id,
		ProjectID: projectID,
		Running:   true,
		Cwd:       cwd,
		Shell:     m.shellPath,
		CreatedAt: term.createdAt.Format(time.RFC3339),
	}, nil
}

// Write sends input to the terminal.
func (m *Manager) Write(id string, data []byte) error {
	m.mu.RLock()
	term, ok := m.terminals[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}

	term.mu.Lock()
	defer term.mu.Unlock()
	if term.closed {
		return fmt.Errorf("terminal %s is closed", id)
	}

	_, err := term.stdin.Write(data)
	return err
}

// NewReader returns an io.Reader for the terminal's stdout.
func (m *Manager) NewReader(id string) (io.Reader, error) {
	m.mu.RLock()
	term, ok := m.terminals[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("terminal %s not found", id)
	}
	return term.stdout, nil
}

// Resize changes the terminal window size (no-op for pipe-based terminals).
func (m *Manager) Resize(id string, rows, cols uint16) error {
	// Pipe-based terminals don't support resize. Not an error — just a no-op.
	return nil
}

// Close terminates the terminal session.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	term, ok := m.terminals[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("terminal %s not found", id)
	}
	term.closed = true
	m.mu.Unlock()

	if term.stdin != nil {
		_ = term.stdin.Close()
	}
	var process *os.Process
	if term.cmd != nil {
		process = term.cmd.Process
	}
	if err := m.terminateProcess(process); err != nil {
		return fmt.Errorf("terminate terminal %s: %w", id, err)
	}

	m.mu.Lock()
	if m.terminals[id] == term {
		delete(m.terminals, id)
	}
	m.mu.Unlock()
	return nil
}

// Status returns the current status of a terminal.
func (m *Manager) Status(id string) (*Status, error) {
	m.mu.RLock()
	term, ok := m.terminals[id]
	m.mu.RUnlock()
	if !ok {
		return &Status{ID: id, Running: false}, nil
	}

	term.mu.Lock()
	defer term.mu.Unlock()

	return &Status{
		ID:        term.ID,
		ProjectID: term.ProjectID,
		Running:   !term.closed,
		Cwd:       term.Cwd,
		Shell:     m.shellPath,
		CreatedAt: term.createdAt.Format(time.RFC3339),
	}, nil
}

// CloseAll stops all running terminals and their child processes.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for id, term := range m.terminals {
		term.closed = true
		if term.stdin != nil {
			_ = term.stdin.Close()
		}
		var process *os.Process
		if term.cmd != nil {
			process = term.cmd.Process
		}
		if err := m.terminateProcess(process); err != nil {
			errs = append(errs, fmt.Errorf("terminate terminal %s: %w", id, err))
			continue
		}
		delete(m.terminals, id)
	}
	return errors.Join(errs...)
}

func (m *Manager) terminateProcess(process *os.Process) error {
	if m.terminator != nil {
		return m.terminator(process)
	}
	return lifecycle.TerminateProcessTree(process)
}

// copyWithContext copies from reader to writer until context is cancelled or reader closes.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) error {
	reader := bufio.NewReader(src)
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

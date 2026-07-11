// Package terminal manages interactive shell sessions for the User Terminal tab.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"glyphdeck/internal/lifecycle"
)

// Terminal represents a running interactive shell session.
type Terminal struct {
	ID          string
	ProjectID   string
	Cwd         string
	session     termSession
	processTree lifecycle.ProcessTree
	mu          sync.Mutex
	closed      bool
	createdAt   time.Time
}

// Status reports the terminal running state.
type Status struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Running   bool   `json:"running"`
	Cwd       string `json:"cwd"`
	Shell     string `json:"shell"`
	ShellPID  int    `json:"shellPid"`
	ChildPIDs []int  `json:"childPids,omitempty"`
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

func resolveAndValidateCWD(ctx context.Context, resolver ProjectResolver, projectID, cwd string) (string, error) {
	cleanCwd := filepath.Clean(cwd)
	var err error
	cleanCwd, err = filepath.EvalSymlinks(cleanCwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}

	if resolver == nil {
		cleanCwd = filepath.Clean(cleanCwd)
		if _, err := os.Stat(cleanCwd); err != nil {
			return "", fmt.Errorf("cwd does not exist: %w", err)
		}
		return cleanCwd, nil
	}

	projectRoot, err := resolver.GetPath(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}

	cleanRoot, err := filepath.EvalSymlinks(filepath.Clean(projectRoot))
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}

	if !pathIsDescendant(cleanRoot, cleanCwd) {
		return "", fmt.Errorf("cwd is not within the project root")
	}

	cleanCwd = filepath.Clean(cleanCwd)
	if _, err := os.Stat(cleanCwd); err != nil {
		return "", fmt.Errorf("cwd does not exist: %w", err)
	}
	return cleanCwd, nil
}

func pathIsDescendant(root, child string) bool {
	rootSlash := filepath.ToSlash(root)
	childSlash := filepath.ToSlash(child)

	if rootSlash == childSlash {
		return true
	}
	if !strings.HasPrefix(childSlash, rootSlash+"/") {
		return false
	}
	remaining := childSlash[len(rootSlash)+1:]
	return remaining != "" && !strings.Contains(remaining, "..")
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

	cleanCwd, err := resolveAndValidateCWD(ctx, m.resolver, projectID, cwd)
	if err != nil {
		return nil, fmt.Errorf("validate cwd: %w", err)
	}

	session, err := newTermSession(m.shellPath, m.shellArgs, cleanCwd)
	if err != nil {
		return nil, fmt.Errorf("start shell: %w", err)
	}
	processTree, err := lifecycle.AttachProcessTree(session.process())
	if err != nil {
		_ = session.close()
		_ = session.wait()
		return nil, fmt.Errorf("attach shell process tree: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("term-%d", m.nextID)

	term := &Terminal{
		ID:          id,
		ProjectID:   projectID,
		Cwd:         cleanCwd,
		session:     session,
		processTree: processTree,
		createdAt:   time.Now(),
	}

	m.terminals[id] = term

	// Monitor process exit.
	go func() {
		_ = session.wait()
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
		Cwd:       cleanCwd,
		Shell:     m.shellPath,
		ShellPID:  session.process().Pid,
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

	if term.session == nil {
		return fmt.Errorf("terminal %s is closed", id)
	}
	_, err := term.session.stdin().Write(data)
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
	if term.session == nil {
		return nil, fmt.Errorf("terminal %s is closed", id)
	}
	return term.session.stdout(), nil
}

// Resize changes the terminal window size.
func (m *Manager) Resize(id string, rows, cols uint16) error {
	m.mu.RLock()
	term, ok := m.terminals[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}
	if term.session == nil {
		return fmt.Errorf("terminal %s is closed", id)
	}
	return term.session.resize(rows, cols)
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

	if term.session != nil {
		_ = term.session.close()
	}
	if err := m.terminateProcessTree(term.processTree); err != nil {
		return fmt.Errorf("terminate terminal %s: %w", id, err)
	}
	if term.session != nil {
		_ = term.session.wait()
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

	shellPID := 0
	var childPIDs []int
	if term.session != nil {
		shellPID = term.session.process().Pid
	}
	if term.processTree != nil && !term.closed {
		childPIDs = term.processTree.PIDs()
	}
	return &Status{
		ID:        term.ID,
		ProjectID: term.ProjectID,
		Running:   !term.closed,
		Cwd:       term.Cwd,
		Shell:     m.shellPath,
		ShellPID:  shellPID,
		ChildPIDs: childPIDs,
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
		if term.session != nil {
			_ = term.session.close()
		}
		if err := m.terminateProcessTree(term.processTree); err != nil {
			errs = append(errs, fmt.Errorf("terminate terminal %s: %w", id, err))
			continue
		}
		if term.session != nil {
			_ = term.session.wait()
		}
		delete(m.terminals, id)
	}
	return errors.Join(errs...)
}

func (m *Manager) terminateProcessTree(tree lifecycle.ProcessTree) error {
	if tree != nil {
		return tree.Close()
	}
	return nil
}

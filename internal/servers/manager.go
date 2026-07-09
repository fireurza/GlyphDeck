package servers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"glyphdeck/internal/opencode"
)

// ServerManager launches and tracks OpenCode server processes.
type ServerManager struct {
	mu          sync.RWMutex
	detector    opencode.Detector
	resolver    ProjectResolver
	processes   map[string]*managedProcess // keyed by projectID
	eventBridge EventBridgeManager
}

// NewManager creates a ServerManager with the given detector and resolver.
func NewManager(detector opencode.Detector, resolver ProjectResolver) *ServerManager {
	return &ServerManager{
		detector:  detector,
		resolver:  resolver,
		processes: make(map[string]*managedProcess),
	}
}

// SetEventBridgeManager configures an optional event bridge manager.
// When set, the ServerManager will start/stop event bridges as servers come and go.
func (m *ServerManager) SetEventBridgeManager(bridge EventBridgeManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventBridge = bridge
}

// GetBaseURL returns the HTTP base URL for a project's running OpenCode server.
// Returns an error if the server is not ready.
func (m *ServerManager) GetBaseURL(ctx context.Context, projectID string) (string, error) {
	m.mu.RLock()
	mp, exists := m.processes[projectID]
	m.mu.RUnlock()
	if !exists || mp.state != StateReady {
		return "", fmt.Errorf("server not ready for project %s", projectID)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", mp.port), nil
}

// OpencodeStatus reports whether the OpenCode CLI is installed.
func (m *ServerManager) OpencodeStatus() opencode.DetectionResult {
	return m.detector.Detect()
}

// Status returns the current server state for a project.
func (m *ServerManager) Status(ctx context.Context, projectID string) ServerStatus {
	// Resolve project.
	_, err := m.resolver.Get(ctx, projectID)
	if err != nil {
		return ServerStatus{ProjectID: projectID, Status: StateUnknown}
	}

	m.mu.RLock()
	mp, exists := m.processes[projectID]
	m.mu.RUnlock()

	if !exists {
		// No managed process — check if opencode is available.
		detection := m.detector.Detect()
		if !detection.Installed {
			return ServerStatus{ProjectID: projectID, Status: StateNotInstalled}
		}
		return ServerStatus{ProjectID: projectID, Status: StateStopped}
	}

	// On-demand health check for starting/ready states.
	if mp.state == StateStarting || mp.state == StateReady {
		if mp.isHealthy() {
			if mp.state == StateStarting {
				m.mu.Lock()
				mp.state = StateReady
				m.mu.Unlock()
			}
		} else {
			if mp.state == StateReady {
				m.mu.Lock()
				mp.state = StateUnhealthy
				m.mu.Unlock()
			}
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statusFromProcess(mp)
}

// Start launches an OpenCode server for the given project.
func (m *ServerManager) Start(ctx context.Context, projectID string) (ServerStatus, error) {
	// 1. Check opencode installed.
	detection := m.detector.Detect()
	if !detection.Installed {
		return ServerStatus{}, ErrOpenCodeNotInstalled
	}

	// 2. Resolve project.
	project, err := m.resolver.Get(ctx, projectID)
	if err != nil {
		return ServerStatus{}, ErrProjectNotFound
	}

	// 3. Check if already managed.
	m.mu.Lock()
	if mp, exists := m.processes[projectID]; exists {
		if isNonTerminal(mp.state) {
			status := m.statusFromProcess(mp)
			m.mu.Unlock()
			return status, ErrServerAlreadyRunning
		}
		// Terminal state (stopped/failed) — clean up and allow restart.
		delete(m.processes, projectID)
	}

	// 4. Allocate a loopback port.
	port, err := allocatePort()
	if err != nil {
		m.mu.Unlock()
		return ServerStatus{}, fmt.Errorf("allocate port: %w", err)
	}

	// 5. Build the command.
	openCodePath := detection.Executable
	cmd := exec.CommandContext(ctx, openCodePath, "serve", "--port", strconv.Itoa(port), "--hostname", "127.0.0.1")
	cmd.Dir = project.Path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 6. Create child context with cancel for lifecycle management.
	childCtx, cancel := context.WithCancel(context.Background())

	// Replace the command's context with our child context so we control cancellation.
	cmd = exec.CommandContext(childCtx, openCodePath, "serve", "--port", strconv.Itoa(port), "--hostname", "127.0.0.1")
	cmd.Dir = project.Path
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 7. Start the process.
	if err := cmd.Start(); err != nil {
		cancel()
		m.mu.Unlock()
		return ServerStatus{}, fmt.Errorf("start opencode server: %w", err)
	}

	mp := &managedProcess{
		projectID: projectID,
		cmd:       cmd,
		port:      port,
		cancel:    cancel,
		startedAt: time.Now(),
		version:   detection.Version,
		state:     StateStarting,
	}
	m.processes[projectID] = mp
	m.mu.Unlock()

	// 8-9. Health check loop.
	healthOK := m.waitForHealthy(mp)

	m.mu.Lock()
	defer m.mu.Unlock()

	if healthOK {
		mp.state = StateReady
		status := m.statusFromProcess(mp)

		// Start event bridge when server becomes ready.
		if m.eventBridge != nil {
			url := fmt.Sprintf("http://127.0.0.1:%d", mp.port)
			go func() {
				_ = m.eventBridge.StartEventBridge(projectID, url)
			}()
		}

		return status, nil
	}

	// 10. Timeout — kill process, set failed.
	mp.state = StateFailed
	mp.cleanupProcess()
	delete(m.processes, projectID)
	return ServerStatus{}, fmt.Errorf("opencode server health check timed out for project %s", projectID)
}

// Stop terminates the OpenCode server for the given project.
func (m *ServerManager) Stop(ctx context.Context, projectID string) (ServerStatus, error) {
	m.mu.Lock()
	mp, exists := m.processes[projectID]
	if !exists {
		m.mu.Unlock()
		return ServerStatus{ProjectID: projectID, Status: StateStopped}, ErrServerNotRunning
	}

	if mp.state == StateStopped || mp.state == StateFailed {
		delete(m.processes, projectID)
		status := m.statusFromProcess(mp)
		m.mu.Unlock()
		return status, nil
	}

	// Set state to stopping.
	mp.state = StateStopping
	m.mu.Unlock()

	// Cancel the child context.
	mp.cancel()

	// Try graceful shutdown with interrupt signal.
	done := make(chan struct{})
	go func() {
		_ = mp.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited cleanly.
	case <-time.After(5 * time.Second):
		// Graceful shutdown timed out — force kill.
		_ = mp.cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			// On Windows, Kill may not propagate to child processes.
			mp.cleanupProcess()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	mp.state = StateStopped

	// Stop event bridge when server stops.
	if m.eventBridge != nil {
		m.eventBridge.StopEventBridge(projectID)
	}

	delete(m.processes, projectID)
	return ServerStatus{
		ProjectID: projectID,
		Status:    StateStopped,
	}, nil
}

// waitForHealthy polls the health endpoint until success or 10s timeout.
func (m *ServerManager) waitForHealthy(mp *managedProcess) bool {
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/global/health", mp.port)
	client := &http.Client{Timeout: 2 * time.Second}

	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return false
		case <-ticker.C:
			username, password := getOpenCodeCreds()
			req, err := http.NewRequest("GET", healthURL, nil)
			if err != nil {
				continue
			}
			if username != "" && password != "" {
				req.SetBasicAuth(username, password)
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
			// Fallback: if auth failed (401), try TCP dial to confirm port listening.
			if resp.StatusCode == http.StatusUnauthorized {
				conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", mp.port), 2*time.Second)
				if dialErr == nil {
					conn.Close()
					return true
				}
			}
		}
	}
}

// statusFromProcess builds a ServerStatus from a managedProcess.
// Caller must hold at least a read lock.
func (m *ServerManager) statusFromProcess(mp *managedProcess) ServerStatus {
	pid := 0
	if mp.cmd != nil && mp.cmd.Process != nil {
		pid = mp.cmd.Process.Pid
	}
	baseURL := ""
	if mp.port > 0 && (mp.state == StateReady || mp.state == StateStarting) {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", mp.port)
	}
	return ServerStatus{
		ProjectID: mp.projectID,
		Status:    mp.state,
		BaseURL:   baseURL,
		Port:      mp.port,
		PID:       pid,
		Version:   mp.version,
	}
}

// isHealthy returns true if the managed process responds to a health check.
func (mp *managedProcess) isHealthy() bool {
	if mp.port <= 0 {
		return false
	}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/global/health", mp.port)
	client := &http.Client{Timeout: 2 * time.Second}
	username, password := getOpenCodeCreds()
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		return false
	}
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// cleanupProcess forcefully terminates the process tree.
func (mp *managedProcess) cleanupProcess() {
	mp.cancel()
	if mp.cmd == nil || mp.cmd.Process == nil {
		return
	}
	_ = mp.cmd.Process.Kill()
	// On Windows, run taskkill to ensure child processes are terminated.
	if mp.cmd.Process.Pid > 0 {
		pidStr := strconv.Itoa(mp.cmd.Process.Pid)
		killCmd := exec.Command("taskkill", "/PID", pidStr, "/F")
		killCmd.Stdout = nil
		killCmd.Stderr = nil
		_ = killCmd.Run()
	}
	_ = mp.cmd.Wait()
}

// isNonTerminal returns true for states that are not stopped or failed.
func isNonTerminal(state string) bool {
	return state != StateStopped && state != StateFailed
}

// getOpenCodeCreds reads OpenCode server credentials from environment.
// When no password is set, username is cleared so callers can skip BasicAuth entirely.
func getOpenCodeCreds() (username, password string) {
	return opencode.GetServerCreds()
}

// StopAllAppOwned stops all tracked server processes and returns their details.
// Only stops app-owned tracked processes — never kills global opencode.
func (m *ServerManager) StopAllAppOwned(ctx context.Context) ([]StoppedServerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]StoppedServerInfo, 0, len(m.processes))
	for projectID, mp := range m.processes {
		mp.cancel()
		done := make(chan struct{})
		go func() { _ = mp.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = mp.cmd.Process.Kill()
		}

		info := StoppedServerInfo{
			ProjectID: projectID,
			Port:      mp.port,
			Status:    StateStopped,
		}
		if mp.cmd.Process != nil {
			info.PID = mp.cmd.Process.Pid
		}
		result = append(result, info)
	}

	m.processes = make(map[string]*managedProcess)
	return result, nil
}

// allocatePort finds a free loopback TCP port.
func allocatePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

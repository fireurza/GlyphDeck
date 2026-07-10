// Package servers manages OpenCode server lifecycles for registered projects.
package servers

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"glyphdeck/internal/lifecycle"
)

// Server lifecycle states.
const (
	StateNotInstalled = "not-installed"
	StateStopped      = "stopped"
	StateStarting     = "starting"
	StateReady        = "ready"
	StateUnhealthy    = "unhealthy"
	StateStopping     = "stopping"
	StateFailed       = "failed"
	StateUnknown      = "unknown"
)

// Sentinel errors for server operations.
var (
	ErrOpenCodeNotInstalled = errors.New("opencode CLI is not installed")
	ErrServerAlreadyRunning = errors.New("server is already running for this project")
	ErrServerNotRunning     = errors.New("no running server found for this project")
	ErrProjectNotFound      = errors.New("project not found")
)

// ProjectInfo carries the fields required to launch a server.
type ProjectInfo struct {
	ID   string
	Name string
	Path string
}

// ProjectResolver looks up a project by ID for server management.
type ProjectResolver interface {
	Get(ctx context.Context, id string) (ProjectInfo, error)
}

// EventBridgeManager is an optional hook for managing event bridges per project.
// If set, the ServerManager calls it whenever a server transitions to Ready or Stopped.
type EventBridgeManager interface {
	StartEventBridge(projectID, baseURL string) error
	StopEventBridge(projectID string)
}

// ServerStatus reports the current state of a managed server.
type ServerStatus struct {
	ProjectID string `json:"projectId"`
	Status    string `json:"status"`
	BaseURL   string `json:"baseUrl,omitempty"`
	Port      int    `json:"port,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Version   string `json:"version,omitempty"`
}

// StoppedServerInfo reports the result of stopping a tracked server process.
type StoppedServerInfo struct {
	ProjectID string `json:"projectId"`
	Port      int    `json:"port"`
	PID       int    `json:"pid"`
	Status    string `json:"status"`
}

// managedProcess holds the runtime state of a launched server.
type managedProcess struct {
	projectID   string
	cmd         *exec.Cmd
	processTree lifecycle.ProcessTree
	port        int
	cancel      context.CancelFunc
	startedAt   time.Time
	version     string
	state       string
}

// ActiveServer tracks the currently attached OpenCode server target.
type ActiveServer struct {
	ServerID string `json:"serverId"`
	BaseURL  string `json:"baseUrl"`
	Attached bool   `json:"attached"`
}

// Attach sets the active server. The caller is responsible for verifying
// reachability. Pass an empty serverID to detach.
func (m *ServerManager) Attach(serverID, baseURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeServerID = serverID
	m.activeBaseURL = baseURL
}

// Detach clears the active server.
func (m *ServerManager) Detach() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeServerID = ""
	m.activeBaseURL = ""
}

// Active returns the currently attached server.
func (m *ServerManager) Active() ActiveServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeServerID == "" {
		return ActiveServer{}
	}
	return ActiveServer{
		ServerID: m.activeServerID,
		BaseURL:  m.activeBaseURL,
		Attached: true,
	}
}

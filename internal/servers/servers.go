// Package servers manages OpenCode server lifecycles for registered projects.
package servers

import (
	"context"
	"errors"
	"os/exec"
	"time"
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

// ServerStatus reports the current state of a managed server.
type ServerStatus struct {
	ProjectID string `json:"projectId"`
	Status    string `json:"status"`
	BaseURL   string `json:"baseUrl,omitempty"`
	Port      int    `json:"port,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Version   string `json:"version,omitempty"`
}

// managedProcess holds the runtime state of a launched server.
type managedProcess struct {
	projectID string
	cmd       *exec.Cmd
	port      int
	cancel    context.CancelFunc
	startedAt time.Time
	version   string
	state     string
}

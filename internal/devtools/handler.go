// Package devtools provides GLYPHDECK_DEV_TOOLS guarded debug and test endpoints.
package devtools

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"glyphdeck/internal/projects"
	"glyphdeck/internal/servers"
)

// IsEnabled returns true when GLYPHDECK_DEV_TOOLS=1.
func IsEnabled() bool {
	return os.Getenv("GLYPHDECK_DEV_TOOLS") == "1"
}

// RegisterHandlers mounts dev/test endpoints if GLYPHDECK_DEV_TOOLS=1.
// When disabled, no routes are registered.
func RegisterHandlers(mux *http.ServeMux, registry ProjectRegistry, srvManager ServerManager) {
	if !IsEnabled() {
		return
	}
	h := &handler{registry: registry, servers: srvManager}
	mux.HandleFunc("GET /api/dev/state", h.state)
	mux.HandleFunc("POST /api/dev/reset-validation-state", h.resetState)
	mux.HandleFunc("POST /api/dev/stop-all-app-owned-servers", h.stopAllServers)
}

// ProjectRegistry is the subset of projects.Registry used by devtools.
type ProjectRegistry interface {
	List(ctx context.Context) ([]projects.Project, error)
	Remove(ctx context.Context, id string) error
}

// ServerManager is the subset of servers.ServerManager used by devtools.
type ServerManager interface {
	Status(ctx context.Context, projectID string) servers.ServerStatus
	StopAllAppOwned(ctx context.Context) ([]servers.StoppedServerInfo, error)
}

type handler struct {
	registry ProjectRegistry
	servers  ServerManager
}

// stateResponse is the JSON payload for GET /api/dev/state.
type stateResponse struct {
	Projects []projectState `json:"projects"`
	Count    int            `json:"count"`
}

type projectState struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"serverStatus"`
	Port   int    `json:"serverPort,omitempty"`
}

// GET /api/dev/state — returns current projects and their server statuses.
func (h *handler) state(w http.ResponseWriter, r *http.Request) {
	list, err := h.registry.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "registry_error", "Failed to list projects.")
		return
	}

	items := make([]projectState, 0, len(list))
	for _, p := range list {
		status := h.servers.Status(r.Context(), p.ID)
		ps := projectState{
			ID:     p.ID,
			Name:   p.Name,
			Path:   p.Path,
			Status: status.Status,
			Port:   status.Port,
		}
		items = append(items, ps)
	}

	writeJSON(w, http.StatusOK, stateResponse{Projects: items, Count: len(items)})
}

// POST /api/dev/reset-validation-state — removes all projects from the registry.
// Does NOT delete user source files.
func (h *handler) resetState(w http.ResponseWriter, r *http.Request) {
	list, err := h.registry.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "registry_error", "Failed to list projects.")
		return
	}

	removed := 0
	for _, p := range list {
		if err := h.registry.Remove(r.Context(), p.ID); err != nil {
			continue
		}
		removed++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"removed": removed,
		"total":   len(list),
	})
}

// POST /api/dev/stop-all-app-owned-servers — stops all tracked server processes.
// Only stops processes tracked by the server manager — never kills global opencode.
func (h *handler) stopAllServers(w http.ResponseWriter, r *http.Request) {
	stopped, err := h.servers.StopAllAppOwned(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stop_error", "Failed to stop servers.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stopped": stopped,
	})
}

// ---------------------------------------------------------------------------
// Helpers (mirrors internal/projects/http.go pattern)
// ---------------------------------------------------------------------------

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

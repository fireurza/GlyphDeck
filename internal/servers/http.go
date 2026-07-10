package servers

import (
	"encoding/json"
	"errors"
	"glyphdeck/internal/httpapi"
	"net/http"
)

// serverHandler holds the manager dependency for HTTP routes.
type serverHandler struct {
	manager *ServerManager
}

// RegisterHandlers mounts server management routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, manager *ServerManager) {
	h := serverHandler{manager: manager}
	mux.HandleFunc("GET /api/opencode", h.handleOpencodeStatus)
	mux.HandleFunc("GET /api/projects/{projectId}/server", h.handleServerStatus)
	mux.HandleFunc("POST /api/projects/{projectId}/server/start", h.handleServerStart)
	mux.HandleFunc("POST /api/projects/{projectId}/server/stop", h.handleServerStop)
	mux.HandleFunc("POST /api/active-server/attach", h.handleAttach)
	mux.HandleFunc("POST /api/active-server/detach", h.handleDetach)
	mux.HandleFunc("GET /api/active-server", h.handleGetActive)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h serverHandler) handleOpencodeStatus(w http.ResponseWriter, r *http.Request) {
	result := h.manager.OpencodeStatus()
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h serverHandler) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	status := h.manager.Status(r.Context(), projectID)
	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (h serverHandler) handleServerStart(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Server operations must come from the same origin.")
		return
	}
	projectID := r.PathValue("projectId")
	status, err := h.manager.Start(r.Context(), projectID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (h serverHandler) handleServerStop(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Server operations must come from the same origin.")
		return
	}
	projectID := r.PathValue("projectId")
	status, err := h.manager.Stop(r.Context(), projectID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
}

// ---------------------------------------------------------------------------
// Active server attach/detach
// ---------------------------------------------------------------------------

type attachRequest struct {
	ServerID string `json:"serverId"`
	BaseURL  string `json:"baseUrl"`
}

func (h serverHandler) handleAttach(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Must use the same origin.")
		return
	}
	var req attachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}
	if req.ServerID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_fields", "serverId is required.")
		return
	}
	h.manager.Attach(req.ServerID, req.BaseURL)
	httpapi.WriteJSON(w, http.StatusOK, h.manager.Active())
}

func (h serverHandler) handleDetach(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Must use the same origin.")
		return
	}
	h.manager.Detach()
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"attached": false})
}

func (h serverHandler) handleGetActive(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, h.manager.Active())
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func writeServerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOpenCodeNotInstalled):
		httpapi.WriteError(w, http.StatusServiceUnavailable, "opencode_not_installed", "OpenCode CLI is not installed.")
	case errors.Is(err, ErrServerAlreadyRunning):
		httpapi.WriteError(w, http.StatusConflict, "server_already_running", "Server is already running for this project.")
	case errors.Is(err, ErrServerNotRunning):
		httpapi.WriteError(w, http.StatusNotFound, "server_not_running", "No running server found for this project.")
	case errors.Is(err, ErrProjectNotFound):
		httpapi.WriteError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	default:
		httpapi.WriteError(w, http.StatusInternalServerError, "internal_error", "Server operation failed.")
	}
}

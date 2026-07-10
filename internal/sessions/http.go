package sessions

import (
	"encoding/json"
	"errors"
	"glyphdeck/internal/httpapi"
	"net/http"
	"strings"
)

// sessionHandler holds the manager dependency for HTTP routes.
type sessionHandler struct {
	manager *Manager
}

// RegisterHandlers mounts session routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, manager *Manager) {
	h := sessionHandler{manager: manager}
	mux.HandleFunc("GET /api/projects/{projectId}/sessions", h.listSessions)
	mux.HandleFunc("POST /api/projects/{projectId}/sessions", h.createSession)
	mux.HandleFunc("GET /api/projects/{projectId}/sessions/{sessionId}", h.getSession)
	mux.HandleFunc("GET /api/projects/{projectId}/sessions/{sessionId}/messages", h.listMessages)
	mux.HandleFunc("POST /api/projects/{projectId}/sessions/{sessionId}/prompt", h.sendPrompt)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// createSessionRequest is the expected JSON body for POST .../sessions.
type createSessionRequest struct {
	Title string `json:"title"`
}

// sendPromptRequest is the expected JSON body for POST .../prompt.
type sendPromptRequest struct {
	Text string `json:"text"`
}

func (h sessionHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessions, err := h.manager.ListSessions(r.Context(), projectID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (h sessionHandler) createSession(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Session mutations must come from the same origin.")
		return
	}
	if !httpapi.HasJSONContentType(r) {
		httpapi.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return
	}

	projectID := r.PathValue("projectId")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req createSessionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid session JSON.")
		return
	}

	if req.Title == "" {
		req.Title = "Untitled Session"
	}

	session, err := h.manager.CreateSession(r.Context(), projectID, req.Title)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, session)
}

func (h sessionHandler) getSession(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	session, err := h.manager.GetSession(r.Context(), projectID, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, session)
}

func (h sessionHandler) listMessages(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	messages, err := h.manager.ListMessages(r.Context(), projectID, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (h sessionHandler) sendPrompt(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Prompt mutations must come from the same origin.")
		return
	}
	if !httpapi.HasJSONContentType(r) {
		httpapi.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return
	}

	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req sendPromptRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid prompt JSON.")
		return
	}

	if req.Text == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing_text", "Prompt text is required.")
		return
	}

	result, err := h.manager.SendPrompt(r.Context(), projectID, sessionID, req.Text)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func writeSessionError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	switch {
	case errors.Is(err, ErrProjectNotFound):
		httpapi.WriteError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case errors.Is(err, ErrServerNotReady):
		httpapi.WriteError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "server not ready"):
		httpapi.WriteError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "opencode:"):
		httpapi.WriteError(w, http.StatusBadGateway, "opencode_error", "OpenCode server returned an error.")
	default:
		httpapi.WriteError(w, http.StatusInternalServerError, "internal_error", "Session operation failed.")
	}
}

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrServerNotReady  = errors.New("server not ready")
)

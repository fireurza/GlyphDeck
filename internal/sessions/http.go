package sessions

import (
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
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
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (h sessionHandler) createSession(w http.ResponseWriter, r *http.Request) {
	if !sameOriginMutation(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Session mutations must come from the same origin.")
		return
	}
	if !hasJSONContentType(r) {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return
	}

	projectID := r.PathValue("projectId")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req createSessionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid session JSON.")
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
	writeJSON(w, http.StatusCreated, session)
}

func (h sessionHandler) getSession(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	session, err := h.manager.GetSession(r.Context(), projectID, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h sessionHandler) listMessages(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	messages, err := h.manager.ListMessages(r.Context(), projectID, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (h sessionHandler) sendPrompt(w http.ResponseWriter, r *http.Request) {
	if !sameOriginMutation(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Prompt mutations must come from the same origin.")
		return
	}
	if !hasJSONContentType(r) {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
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
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid prompt JSON.")
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "missing_text", "Prompt text is required.")
		return
	}

	result, err := h.manager.SendPrompt(r.Context(), projectID, sessionID, req.Text)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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
		writeError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case errors.Is(err, ErrServerNotReady):
		writeError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "server not ready"):
		writeError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "opencode:"):
		writeError(w, http.StatusBadGateway, "opencode_error", "OpenCode server returned an error.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Session operation failed.")
	}
}

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrServerNotReady  = errors.New("server not ready")
)

// ---------------------------------------------------------------------------
// HTTP helpers (shared pattern with projects and servers packages)
// ---------------------------------------------------------------------------

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func hasJSONContentType(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}

func sameOriginMutation(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Host == r.Host {
		return true
	}
	return isLoopbackHost(hostname(parsed.Host)) && isLoopbackHost(hostname(r.Host))
}

func hostname(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
	}
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return strings.ToLower(host)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

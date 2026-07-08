package servers

import (
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// apiError is the error payload within a JSON error response.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorResponse wraps an apiError for JSON serialization.
type errorResponse struct {
	Error apiError `json:"error"`
}

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
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h serverHandler) handleOpencodeStatus(w http.ResponseWriter, r *http.Request) {
	result := h.manager.OpencodeStatus()
	writeJSON(w, http.StatusOK, result)
}

func (h serverHandler) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	status := h.manager.Status(r.Context(), projectID)
	writeJSON(w, http.StatusOK, status)
}

func (h serverHandler) handleServerStart(w http.ResponseWriter, r *http.Request) {
	if !sameOriginMutation(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Server operations must come from the same origin.")
		return
	}
	projectID := r.PathValue("projectId")
	status, err := h.manager.Start(r.Context(), projectID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h serverHandler) handleServerStop(w http.ResponseWriter, r *http.Request) {
	if !sameOriginMutation(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Server operations must come from the same origin.")
		return
	}
	projectID := r.PathValue("projectId")
	status, err := h.manager.Stop(r.Context(), projectID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func writeServerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOpenCodeNotInstalled):
		writeError(w, http.StatusServiceUnavailable, "opencode_not_installed", "OpenCode CLI is not installed.")
	case errors.Is(err, ErrServerAlreadyRunning):
		writeError(w, http.StatusConflict, "server_already_running", "Server is already running for this project.")
	case errors.Is(err, ErrServerNotRunning):
		writeError(w, http.StatusNotFound, "server_not_running", "No running server found for this project.")
	case errors.Is(err, ErrProjectNotFound):
		writeError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Server operation failed.")
	}
}

// ---------------------------------------------------------------------------
// Helpers (pattern mirrors internal/projects/http.go)
// ---------------------------------------------------------------------------

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

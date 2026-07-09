package usage

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Handler serves usage aggregation HTTP endpoints.
type Handler struct {
	resolver Resolver
}

// NewHandler creates a usage HTTP handler.
func NewHandler(resolver Resolver) *Handler {
	return &Handler{resolver: resolver}
}

// RegisterHandlers mounts usage routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, resolver Resolver) {
	h := NewHandler(resolver)
	mux.HandleFunc("GET /api/projects/{projectId}/sessions/{sessionId}/usage", h.getUsage)
}

func (h *Handler) getUsage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	usage, err := Aggregate(r.Context(), h.resolver, projectID, sessionID)
	if err != nil {
		writeUsageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func writeUsageError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "server not ready"):
		writeError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "project not found"):
		writeError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case strings.Contains(msg, "opencode:"):
		writeError(w, http.StatusBadGateway, "opencode_error", "OpenCode server returned an error.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Usage operation failed.")
	}
}

// ---------------------------------------------------------------------------
// Sentinel errors (reused from sessions package concepts)
// ---------------------------------------------------------------------------

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrServerNotReady  = errors.New("server not ready")
)

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

// hostname extracts the host from a host:port string.
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

// isLoopbackHost returns true if the host is a loopback address.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameOriginMutation checks whether the request origin matches the request host.
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

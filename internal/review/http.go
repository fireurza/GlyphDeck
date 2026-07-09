package review

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler serves review aggregation HTTP endpoints.
type Handler struct {
	resolver Resolver
}

// NewHandler creates a review HTTP handler.
func NewHandler(resolver Resolver) *Handler {
	return &Handler{resolver: resolver}
}

// RegisterHandlers mounts review routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, resolver Resolver) {
	h := NewHandler(resolver)
	mux.HandleFunc("GET /api/projects/{projectId}/sessions/{sessionId}/review", h.getReview)
}

func (h *Handler) getReview(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	sessionID := r.PathValue("sessionId")

	review, err := Aggregate(r.Context(), h.resolver, projectID, sessionID)
	if err != nil {
		writeReviewError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func writeReviewError(w http.ResponseWriter, err error) {
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
		writeError(w, http.StatusInternalServerError, "internal_error", "Review operation failed.")
	}
}

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

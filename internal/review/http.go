package review

import (
	"glyphdeck/internal/httpapi"
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
	httpapi.WriteJSON(w, http.StatusOK, review)
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
		httpapi.WriteError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "project not found"):
		httpapi.WriteError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case strings.Contains(msg, "opencode:"):
		httpapi.WriteError(w, http.StatusBadGateway, "opencode_error", "OpenCode server returned an error.")
	default:
		httpapi.WriteError(w, http.StatusInternalServerError, "internal_error", "Review operation failed.")
	}
}

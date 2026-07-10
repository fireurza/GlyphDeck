package usage

import (
	"errors"
	"glyphdeck/internal/httpapi"
	"net/http"
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
	httpapi.WriteJSON(w, http.StatusOK, usage)
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
		httpapi.WriteError(w, http.StatusConflict, "server_not_ready", "Server is not ready for this project.")
	case strings.Contains(msg, "project not found"):
		httpapi.WriteError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case strings.Contains(msg, "opencode:"):
		httpapi.WriteError(w, http.StatusBadGateway, "opencode_error", "OpenCode server returned an error.")
	default:
		httpapi.WriteError(w, http.StatusInternalServerError, "internal_error", "Usage operation failed.")
	}
}

// ---------------------------------------------------------------------------
// Sentinel errors (reused from sessions package concepts)
// ---------------------------------------------------------------------------

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrServerNotReady  = errors.New("server not ready")
)

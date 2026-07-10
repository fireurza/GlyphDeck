package problems

import (
	"glyphdeck/internal/httpapi"
	"net/http"
)

// Handler serves problems HTTP endpoints.
type Handler struct {
	manager *Manager
}

// NewHandler creates a problems HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterHandlers mounts problems routes.
func RegisterHandlers(mux *http.ServeMux, manager *Manager) {
	h := NewHandler(manager)
	mux.HandleFunc("GET /api/problems", h.listProblems)
	mux.HandleFunc("POST /api/problems/clear", h.clearProblems)
}

func (h *Handler) listProblems(w http.ResponseWriter, r *http.Request) {
	problems := h.manager.List()
	httpapi.WriteJSON(w, http.StatusOK, problems)
}

func (h *Handler) clearProblems(w http.ResponseWriter, r *http.Request) {
	h.manager.Clear()
	w.WriteHeader(http.StatusNoContent)
}

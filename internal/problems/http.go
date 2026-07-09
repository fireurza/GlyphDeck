package problems

import (
	"encoding/json"
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
	writeJSON(w, http.StatusOK, problems)
}

func (h *Handler) clearProblems(w http.ResponseWriter, r *http.Request) {
	h.manager.Clear()
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

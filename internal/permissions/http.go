package permissions

import (
	"encoding/json"
	"glyphdeck/internal/httpapi"
	"net/http"

	"glyphdeck/internal/opencode"
)

// RegisterHandlers mounts permission routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, resolver Resolver) {
	h := NewHandler(resolver)
	mux.HandleFunc("GET /api/permissions", h.listPermissions)
	mux.HandleFunc("POST /api/permissions/{requestId}/reply", h.replyPermission)
}

func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing_project", "Query parameter ?projectId= is required.")
		return
	}

	requests, err := h.ListPending(r.Context(), projectID)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "opencode_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, requests)
}

func (h *Handler) replyPermission(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("requestId")
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing_project", "Query parameter ?projectId= is required.")
		return
	}

	var reply struct {
		Reply string `json:"reply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reply); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON with { reply: \"once\" | \"always\" | \"reject\" }.")
		return
	}

	if err := h.Reply(r.Context(), projectID, requestID, opencode.PermissionReply{Reply: reply.Reply}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "opencode_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

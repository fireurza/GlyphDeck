package permissions

import (
	"encoding/json"
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
		writeError(w, http.StatusBadRequest, "missing_project", "Query parameter ?projectId= is required.")
		return
	}

	requests, err := h.ListPending(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "opencode_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (h *Handler) replyPermission(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("requestId")
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "missing_project", "Query parameter ?projectId= is required.")
		return
	}

	var reply struct {
		Reply string `json:"reply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reply); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON with { reply: \"once\" | \"always\" | \"reject\" }.")
		return
	}

	if err := h.Reply(r.Context(), projectID, requestID, opencode.PermissionReply{Reply: reply.Reply}); err != nil {
		writeError(w, http.StatusInternalServerError, "opencode_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

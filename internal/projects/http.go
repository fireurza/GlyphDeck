package projects

import (
	"encoding/json"
	"errors"
	"glyphdeck/internal/httpapi"
	"net/http"
)

func (h httpHandler) list(w http.ResponseWriter, r *http.Request) {
	projects, err := h.registry.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal_error", "Project registry could not be read.")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string][]Project{"projects": projects})
}

func (h httpHandler) add(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Project registry changes must come from the same origin.")
		return
	}
	if !httpapi.HasJSONContentType(r) {
		httpapi.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req AddRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid project JSON.")
		return
	}

	project, err := h.registry.Add(r.Context(), req)
	if err != nil {
		writeProjectError(w, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusCreated, project)
}

func (h httpHandler) get(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	project, err := h.registry.Get(r.Context(), projectID)
	if err != nil {
		writeProjectError(w, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, project)
}

func (h httpHandler) remove(w http.ResponseWriter, r *http.Request) {
	if !httpapi.SameOriginMutation(r) {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Project registry changes must come from the same origin.")
		return
	}

	projectID := r.PathValue("projectId")
	if err := h.registry.Remove(r.Context(), projectID); err != nil {
		writeProjectError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeProjectError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrMissingPath):
		httpapi.WriteError(w, http.StatusBadRequest, "missing_path", "Project path is required.")
	case errors.Is(err, ErrPathNotFound):
		httpapi.WriteError(w, http.StatusBadRequest, "path_not_found", "Project path does not exist.")
	case errors.Is(err, ErrPathNotDirectory):
		httpapi.WriteError(w, http.StatusBadRequest, "path_not_directory", "Project path must be a directory.")
	case errors.Is(err, ErrDuplicatePath):
		httpapi.WriteError(w, http.StatusConflict, "duplicate_path", "Project path is already registered.")
	case errors.Is(err, ErrProjectNotFound):
		httpapi.WriteError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case errors.Is(err, ErrUnsupportedPath):
		httpapi.WriteError(w, http.StatusBadRequest, "unsupported_path", "Project path must be a local directory.")
	default:
		httpapi.WriteError(w, http.StatusInternalServerError, "internal_error", "Project registry could not be updated.")
	}
}

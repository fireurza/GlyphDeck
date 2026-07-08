package projects

import (
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h httpHandler) list(w http.ResponseWriter, r *http.Request) {
	projects, err := h.registry.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Project registry could not be read.")
		return
	}

	writeJSON(w, http.StatusOK, map[string][]Project{"projects": projects})
}

func (h httpHandler) add(w http.ResponseWriter, r *http.Request) {
	if !sameOriginMutation(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Project registry changes must come from the same origin.")
		return
	}
	if !hasJSONContentType(r) {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req AddRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid project JSON.")
		return
	}

	project, err := h.registry.Add(r.Context(), req)
	if err != nil {
		writeProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func (h httpHandler) get(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	project, err := h.registry.Get(r.Context(), projectID)
	if err != nil {
		writeProjectError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (h httpHandler) remove(w http.ResponseWriter, r *http.Request) {
	if !sameOriginMutation(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Project registry changes must come from the same origin.")
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
		writeError(w, http.StatusBadRequest, "missing_path", "Project path is required.")
	case errors.Is(err, ErrPathNotFound):
		writeError(w, http.StatusBadRequest, "path_not_found", "Project path does not exist.")
	case errors.Is(err, ErrPathNotDirectory):
		writeError(w, http.StatusBadRequest, "path_not_directory", "Project path must be a directory.")
	case errors.Is(err, ErrDuplicatePath):
		writeError(w, http.StatusConflict, "duplicate_path", "Project path is already registered.")
	case errors.Is(err, ErrProjectNotFound):
		writeError(w, http.StatusNotFound, "project_not_found", "Project was not found.")
	case errors.Is(err, ErrUnsupportedPath):
		writeError(w, http.StatusBadRequest, "unsupported_path", "Project path must be a local directory.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Project registry could not be updated.")
	}
}

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

package preferences

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"glyphdeck/internal/httpapi"
)

// Handler serves preferences API endpoints.
type Handler struct {
	store *Store
}

// NewHandler creates a preferences API handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterHandlers mounts preference routes.
func RegisterHandlers(mux *http.ServeMux, store *Store) {
	h := NewHandler(store)
	mux.HandleFunc("GET /api/preferences", h.getSettings)
	mux.HandleFunc("POST /api/preferences/preview", h.preview)
	mux.HandleFunc("PUT /api/preferences", h.update)
	mux.HandleFunc("GET /api/preferences/backups", h.listBackups)
	mux.HandleFunc("POST /api/preferences/backups/{id}/restore", h.restore)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	doc, err := h.store.Load()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "load_error", "Failed to load settings.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, doc)
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	body, err := readLimitedBody(r, 64*1024)
	if err != nil {
		httpapi.WriteError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body too large.")
		return
	}

	var prefs *Prefs
	contentType := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)

	switch {
	case mediaType == "application/json":
		prefs, err = ParsePrefsJSON(body)
	case mediaType == "application/yaml" || mediaType == "text/yaml":
		prefs, err = ParsePrefsYAML(body)
	default:
		// Fall back to JSON.
		prefs, err = ParsePrefsJSON(body)
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "parse_error", err.Error())
		return
	}

	// Normalize and validate.
	prefs.Normalize()
	valErrs := prefs.Validate()

	// Load current to compute diff.
	current, err := h.store.Load()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "load_error", "Failed to load current settings.")
		return
	}

	result := PreviewResult{
		Normalized: PrefsDocument{
			Data:     *prefs,
			Revision: current.Revision,
		},
		Changes: Diff(current.Data, *prefs),
		Errors:  valErrs,
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	body, err := readLimitedBody(r, 64*1024)
	if err != nil {
		httpapi.WriteError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body too large.")
		return
	}

	var req UpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON.")
		return
	}

	doc, err := h.store.Save(req)
	if err != nil {
		if errors.Is(err, ErrRevisionConflict) {
			httpapi.WriteError(w, http.StatusConflict, "revision_conflict", "Settings were modified by another client. Reload and try again.")
			return
		}
		if errors.Is(err, ErrValidationFailed) {
			httpapi.WriteError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "save_error", "Failed to save settings.")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, doc)
}

func (h *Handler) listBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.store.ListBackups()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "load_error", "Failed to list backups.")
		return
	}
	if backups == nil {
		backups = []BackupEntry{}
	}
	httpapi.WriteJSON(w, http.StatusOK, backups)
}

func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid backup ID.")
		return
	}

	doc, err := h.store.Restore(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "Backup not found.")
			return
		}
		if errors.Is(err, ErrRevisionConflict) {
			httpapi.WriteError(w, http.StatusConflict, "revision_conflict", "Settings were modified. Reload and try again.")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "restore_error", "Failed to restore backup.")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, doc)
}

func readLimitedBody(r *http.Request, maxBytes int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	return io.ReadAll(r.Body)
}

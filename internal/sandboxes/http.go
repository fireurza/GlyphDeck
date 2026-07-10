package sandboxes

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"glyphdeck/internal/httpapi"
)

// Handler serves server config HTTP endpoints.
type Handler struct {
	registry *Registry
}

// NewHandler creates a Handler.
func NewHandler(registry *Registry) *Handler {
	return &Handler{registry: registry}
}

// RegisterHandlers mounts server config routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, registry *Registry) {
	h := NewHandler(registry)
	mux.HandleFunc("GET /api/server-configs", h.listConfigs)
	mux.HandleFunc("POST /api/server-configs", h.addConfig)
	mux.HandleFunc("DELETE /api/server-configs/{id}", h.deleteConfig)
	mux.HandleFunc("POST /api/server-configs/{id}/check", h.checkConfig)
}

func (h *Handler) listConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.registry.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "server_configs_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"configs": configs})
}

type addConfigRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	SSHAlias string `json:"sshAlias"`
}

func (h *Handler) addConfig(w http.ResponseWriter, r *http.Request) {
	var req addConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}
	if req.ID == "" || req.Name == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_fields", "id and name are required.")
		return
	}

	cfg := ServerConfig{
		ID:       req.ID,
		Name:     req.Name,
		Type:     ServerType(req.Type),
		URL:      req.URL,
		SSHAlias: req.SSHAlias,
	}
	if cfg.Type == "" {
		cfg.Type = TypeLocal
	}

	if err := h.registry.Add(r.Context(), cfg); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "server_configs_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, cfg)
}

func (h *Handler) deleteConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.registry.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "Server config not found.")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "server_configs_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) checkConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := h.registry.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "Server config not found.")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "server_configs_error", err.Error())
		return
	}

	status := "unknown"
	if cfg.Type == TypeSSHAlias && cfg.URL == "" {
		status = "unknown" // SSH-only targets not checkable yet
	} else {
		targetURL := cfg.URL
		if targetURL == "" && cfg.Type == TypeLocal {
			targetURL = "http://127.0.0.1:4096"
		}
		if targetURL != "" {
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(targetURL + "/health")
			if err == nil && resp.StatusCode < 500 {
				status = "online"
				resp.Body.Close()
			} else {
				status = "offline"
			}
		}
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{
		"id":     cfg.ID,
		"status": status,
	})
}

package sandboxes

import (
	"encoding/json"
	"errors"
	"fmt"
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
	mux.HandleFunc("PUT /api/server-configs/{id}", h.updateConfig)
	mux.HandleFunc("DELETE /api/server-configs/{id}", h.deleteConfig)
	mux.HandleFunc("POST /api/server-configs/{id}/check", h.checkConfig)
	mux.HandleFunc("POST /api/server-configs/{id}/test-ssh", h.testSSH)
	mux.HandleFunc("POST /api/server-configs/{id}/detect", h.detect)
	mux.HandleFunc("POST /api/server-configs/{id}/start-remote", h.startRemote)
	mux.HandleFunc("POST /api/server-configs/{id}/stop-remote", h.stopRemote)
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
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	SSHAlias     string `json:"sshAlias"`
	WorkingDir   string `json:"workingDir"`
	StartCommand string `json:"startCommand"`
	StopCommand  string `json:"stopCommand"`
	StatusCmd    string `json:"statusCommand"`
}

func (h *Handler) addConfig(w http.ResponseWriter, r *http.Request) {
	var req addConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}
	if err := validateServerConfig(req.Name, ServerType(req.Type), req.SSHAlias); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}

	cfg := ServerConfig{
		ID:           req.ID,
		Name:         req.Name,
		Type:         ServerType(req.Type),
		URL:          req.URL,
		SSHAlias:     req.SSHAlias,
		WorkingDir:   req.WorkingDir,
		StartCommand: req.StartCommand,
		StopCommand:  req.StopCommand,
		StatusCmd:    req.StatusCmd,
	}
	if cfg.Type == "" {
		cfg.Type = TypeLocal
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("srv-%d", time.Now().UnixNano())
	}

	if err := h.registry.Add(r.Context(), cfg); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "server_configs_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, cfg)
}

func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req addConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}
	if err := validateServerConfig(req.Name, ServerType(req.Type), req.SSHAlias); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_fields", err.Error())
		return
	}

	cfg := ServerConfig{
		ID:           id,
		Name:         req.Name,
		Type:         ServerType(req.Type),
		URL:          req.URL,
		SSHAlias:     req.SSHAlias,
		WorkingDir:   req.WorkingDir,
		StartCommand: req.StartCommand,
		StopCommand:  req.StopCommand,
		StatusCmd:    req.StatusCmd,
	}

	if err := h.registry.Update(r.Context(), cfg); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "Server config not found.")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "server_configs_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, cfg)
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

func (h *Handler) testSSH(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.registry.TestSSH(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpapi.WriteError(w, http.StatusNotFound, "not_found", "Server config not found.")
			return
		}
		httpapi.WriteError(w, http.StatusBadRequest, "ssh_error", err.Error())
		return
	}
	success := result.Err == nil
	msg := "SSH connection OK"
	if !success {
		msg = result.Err.Error()
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"success": success,
		"message": msg,
	})
}

func (h *Handler) detect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, err := h.registry.Detect(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "detect_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (h *Handler) startRemote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.registry.StartRemote(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "start_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) stopRemote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.registry.StopRemote(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "stop_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func validateServerConfig(name string, typ ServerType, sshAlias string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	switch typ {
	case TypeLocal, TypeManualURL, TypeSSHAlias:
		// valid
	case "":
		// default: let the registry apply the default
	default:
		return fmt.Errorf("unsupported server type: %s", string(typ))
	}
	if typ == TypeSSHAlias && sshAlias == "" {
		return fmt.Errorf("SSH alias is required for SSH targets")
	}
	return nil
}

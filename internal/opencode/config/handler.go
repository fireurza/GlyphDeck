package config

import (
	"context"
	"errors"
	"glyphdeck/internal/httpapi"
	"glyphdeck/internal/projects"
	"net/http"
)

// ProjectResolver resolves a project ID to its path.
type ProjectResolver interface {
	Get(ctx context.Context, id string) (ProjectInfo, error)
}

// ProjectInfo carries the minimal project fields needed for config inspection.
type ProjectInfo struct {
	ID      string
	Name    string
	Path    string
	Trusted bool
}

// projectAdapter adapts projects.Registry to ProjectResolver.
type projectAdapter struct {
	reg *projects.Registry
}

func (a *projectAdapter) Get(ctx context.Context, id string) (ProjectInfo, error) {
	project, err := a.reg.Get(ctx, id)
	if err != nil {
		return ProjectInfo{}, err
	}
	return ProjectInfo{
		ID:      project.ID,
		Name:    project.Name,
		Path:    project.Path,
		Trusted: project.Trusted,
	}, nil
}

// Handler serves OpenCode config inventory requests.
type Handler struct {
	scanner  *Scanner
	resolver ProjectResolver
}

// NewHandler creates a config inventory handler.
func NewHandler(scanner *Scanner, resolver ProjectResolver) *Handler {
	return &Handler{scanner: scanner, resolver: resolver}
}

// RegisterHandlers mounts config routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, scanner *Scanner, reg *projects.Registry) {
	h := NewHandler(scanner, &projectAdapter{reg: reg})
	mux.HandleFunc("GET /api/opencode/config/inventory", h.handleInventory)
}

func (h *Handler) handleInventory(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")

	// Always scan global config.
	inv := h.scanner.ScanGlobal()

	// If a project ID is provided, scan project config too.
	if projectID != "" {
		project, err := h.resolver.Get(r.Context(), projectID)
		if err != nil {
			if errors.Is(err, projects.ErrProjectNotFound) {
				httpapi.WriteError(w, http.StatusNotFound, "project_not_found", "Project not found.")
				return
			}
			httpapi.WriteError(w, http.StatusInternalServerError, "project_lookup_error", "Failed to look up project.")
			return
		}

		// Only inspect config for trusted projects.
		if !project.Trusted {
			inv.Warnings = append(inv.Warnings, ConfigWarning{
				Source:  projectID,
				Message: "Project is not trusted. Project config inspection skipped.",
			})
		} else {
			h.scanner.ScanProject(inv, project.Path)
		}
	}

	httpapi.WriteJSON(w, http.StatusOK, inv)
}

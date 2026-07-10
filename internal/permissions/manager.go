package permissions

import (
	"context"
	"fmt"

	"glyphdeck/internal/opencode"
)

// Manager resolves OpenCode clients for permission operations.
type Manager struct {
	servers  opencode.ServerResolver
	projects ProjectResolver
}

// ProjectResolver resolves project details.
type ProjectResolver interface {
	Get(ctx context.Context, id string) (opencode.ProjectPaths, error)
}

// NewManager creates a permissions Manager.
func NewManager(servers opencode.ServerResolver, projects ProjectResolver) *Manager {
	return &Manager{servers: servers, projects: projects}
}

// ResolveClient returns an OpenCode client for the given project.
func (m *Manager) ResolveClient(ctx context.Context, projectID string) (*opencode.Client, error) {
	_, err := m.projects.Get(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve project: %w", err)
	}

	baseURL, err := m.servers.GetBaseURL(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("server not ready for project %s: %w", projectID, err)
	}

	username, password := opencode.GetServerCreds()
	return opencode.NewClient(baseURL, username, password), nil
}

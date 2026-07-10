package usage

import (
	"context"
	"fmt"

	"glyphdeck/internal/opencode"
)

// Manager resolves OpenCode clients for usage aggregation.
type Manager struct {
	servers  opencode.ServerResolver
	projects ProjectResolver
	clientFn func(baseURL, username, password string) opencode.SessionClient
}

// ProjectResolver resolves project details for server management.
type ProjectResolver interface {
	Get(ctx context.Context, id string) (opencode.ProjectPaths, error)
}

// NewManager creates a usage Manager with the given resolvers.
func NewManager(servers opencode.ServerResolver, projects ProjectResolver) *Manager {
	return &Manager{
		servers:  servers,
		projects: projects,
		clientFn: func(baseURL, username, password string) opencode.SessionClient {
			return opencode.NewClient(baseURL, username, password)
		},
	}
}

// NewManagerWithClient creates a usage Manager with a custom client factory.
func NewManagerWithClient(servers opencode.ServerResolver, projects ProjectResolver, clientFn func(baseURL, username, password string) opencode.SessionClient) *Manager {
	return &Manager{
		servers:  servers,
		projects: projects,
		clientFn: clientFn,
	}
}

// Resolve resolves an OpenCode SessionClient for the given GlyphDeck project.
func (m *Manager) Resolve(ctx context.Context, projectID string) (opencode.SessionClient, string, error) {
	project, err := m.projects.Get(ctx, projectID)
	if err != nil {
		return nil, "", fmt.Errorf("resolve project: %w", err)
	}

	baseURL, err := m.servers.GetBaseURL(ctx, projectID)
	if err != nil {
		return nil, "", fmt.Errorf("server not ready for project %s: %w", projectID, err)
	}

	username, password := opencode.GetServerCreds()
	client := m.clientFn(baseURL, username, password)

	return client, project.Path, nil
}

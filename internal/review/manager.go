package review

import (
	"context"
	"fmt"

	"glyphdeck/internal/opencode"
	"glyphdeck/internal/projects"
)

// Manager resolves OpenCode clients and project info for review aggregation.
type Manager struct {
	servers  opencode.ServerResolver
	projects ProjectResolver
	clientFn func(baseURL, username, password string) opencode.SessionClient
}

// ProjectResolver resolves project details.
type ProjectResolver interface {
	Get(ctx context.Context, id string) (*projects.Project, error)
}

// NewManager creates a review Manager with the given resolvers.
func NewManager(servers opencode.ServerResolver, projects ProjectResolver) *Manager {
	return &Manager{
		servers:  servers,
		projects: projects,
		clientFn: func(baseURL, username, password string) opencode.SessionClient {
			return opencode.NewClient(baseURL, username, password)
		},
	}
}

// NewManagerWithClient creates a review Manager with a custom client factory.
func NewManagerWithClient(servers opencode.ServerResolver, projects ProjectResolver, clientFn func(baseURL, username, password string) opencode.SessionClient) *Manager {
	return &Manager{
		servers:  servers,
		projects: projects,
		clientFn: clientFn,
	}
}

// Resolve resolves an OpenCode SessionClient and project for the given GlyphDeck project ID.
func (m *Manager) Resolve(ctx context.Context, projectID string) (opencode.SessionClient, *projects.Project, error) {
	project, err := m.projects.Get(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve project: %w", err)
	}

	baseURL, err := m.servers.GetBaseURL(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("server not ready for project %s: %w", projectID, err)
	}

	username, password := opencode.GetServerCreds()
	client := m.clientFn(baseURL, username, password)

	return client, project, nil
}

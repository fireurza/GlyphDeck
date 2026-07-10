package sessions

import (
	"context"
	"fmt"

	"glyphdeck/internal/opencode"
)

// Manager coordinates sessions across OpenCode servers.
type Manager struct {
	servers  opencode.ServerResolver
	projects ProjectResolver
	clientFn func(baseURL, username, password string) opencode.SessionClient
}

// ProjectResolver resolves project details for server management.
type ProjectResolver interface {
	Get(ctx context.Context, id string) (opencode.ProjectPaths, error)
}

// NewManager creates a sessions Manager with the given resolvers.
// Uses opencode.NewClient as the default client factory.
func NewManager(servers opencode.ServerResolver, projects ProjectResolver) *Manager {
	return NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return opencode.NewClient(baseURL, username, password)
	})
}

// NewManagerWithClient creates a sessions Manager with a custom client factory.
// Useful for injecting mock clients in tests.
func NewManagerWithClient(servers opencode.ServerResolver, projects ProjectResolver, clientFn func(baseURL, username, password string) opencode.SessionClient) *Manager {
	return &Manager{
		servers:  servers,
		projects: projects,
		clientFn: clientFn,
	}
}

// resolveClient resolves a SessionClient and directory path for the given GlyphDeck project.
func (m *Manager) resolveClient(ctx context.Context, projectID string) (opencode.SessionClient, string, error) {
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

// ListSessions returns all sessions for a project.
func (m *Manager) ListSessions(ctx context.Context, projectID string) ([]opencode.Session, error) {
	client, directory, err := m.resolveClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return client.ListSessions(ctx, directory)
}

// CreateSession creates a new session for a project.
func (m *Manager) CreateSession(ctx context.Context, projectID, title string) (opencode.Session, error) {
	client, directory, err := m.resolveClient(ctx, projectID)
	if err != nil {
		return opencode.Session{}, err
	}
	return client.CreateSession(ctx, title, directory)
}

// GetSession returns a single session's details.
func (m *Manager) GetSession(ctx context.Context, projectID, sessionID string) (opencode.Session, error) {
	client, _, err := m.resolveClient(ctx, projectID)
	if err != nil {
		return opencode.Session{}, err
	}
	return client.GetSession(ctx, sessionID)
}

// ListMessages returns all messages for a session.
func (m *Manager) ListMessages(ctx context.Context, projectID, sessionID string) ([]opencode.Message, error) {
	client, _, err := m.resolveClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return client.ListMessages(ctx, sessionID)
}

// SendPrompt sends a user message and returns the assistant response.
func (m *Manager) SendPrompt(ctx context.Context, projectID, sessionID, text string) (opencode.PromptResult, error) {
	client, _, err := m.resolveClient(ctx, projectID)
	if err != nil {
		return opencode.PromptResult{}, err
	}
	return client.SendPrompt(ctx, sessionID, text)
}

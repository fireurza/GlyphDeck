package sessions

import (
	"context"
	"errors"
	"testing"

	"glyphdeck/internal/opencode"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockServerResolver struct {
	baseURL string
	err     error
}

func (m mockServerResolver) GetBaseURL(_ context.Context, projectID string) (string, error) {
	return m.baseURL, m.err
}

type mockProjectResolver struct {
	projects map[string]ProjectInfo
	err      error
}

func (m mockProjectResolver) Get(_ context.Context, id string) (ProjectInfo, error) {
	if m.err != nil {
		return ProjectInfo{}, m.err
	}
	p, ok := m.projects[id]
	if !ok {
		return ProjectInfo{}, ErrProjectNotFound
	}
	return p, nil
}

type mockSessionClient struct {
	sessions []opencode.Session
	messages []opencode.Message
	result   opencode.PromptResult
	err      error
}

func (m *mockSessionClient) ListSessions(_ context.Context, _ string) ([]opencode.Session, error) {
	return m.sessions, m.err
}

func (m *mockSessionClient) CreateSession(_ context.Context, _, _ string) (opencode.Session, error) {
	if len(m.sessions) > 0 {
		return m.sessions[0], m.err
	}
	return opencode.Session{}, m.err
}

func (m *mockSessionClient) GetSession(_ context.Context, _ string) (opencode.Session, error) {
	if len(m.sessions) > 0 {
		return m.sessions[0], m.err
	}
	return opencode.Session{}, m.err
}

func (m *mockSessionClient) ListMessages(_ context.Context, _ string) ([]opencode.Message, error) {
	return m.messages, m.err
}

func (m *mockSessionClient) SendPrompt(_ context.Context, _, _ string) (opencode.PromptResult, error) {
	return m.result, m.err
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newTestManager(servers ServerResolver, projects ProjectResolver) *Manager {
	return NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{}
	})
}

func TestManager_ProjectNotFound(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{projects: map[string]ProjectInfo{}}
	mgr := newTestManager(servers, projects)

	_, err := mgr.ListSessions(context.Background(), "nonexistent")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestManager_ServerNotReady(t *testing.T) {
	servers := mockServerResolver{err: ErrServerNotReady}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}
	mgr := newTestManager(servers, projects)

	_, err := mgr.ListSessions(context.Background(), "proj-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !stringsContains(err.Error(), "server not ready") {
		t.Fatalf("err = %v, want server not ready", err)
	}
}

func TestManager_ListSessions(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			sessions: []opencode.Session{
				{ID: "sess-1", Title: "Session One"},
				{ID: "sess-2", Title: "Session Two"},
			},
		}
	})

	sessions, err := mgr.ListSessions(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Fatalf("sessions[0].ID = %q, want sess-1", sessions[0].ID)
	}
}

func TestManager_CreateSession(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			sessions: []opencode.Session{
				{ID: "sess-new", Title: "New Session"},
			},
		}
	})

	session, err := mgr.CreateSession(context.Background(), "proj-1", "New Session")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.ID != "sess-new" {
		t.Fatalf("session.ID = %q, want sess-new", session.ID)
	}
}

func TestManager_GetSession(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			sessions: []opencode.Session{
				{ID: "sess-1", Title: "My Session"},
			},
		}
	})

	session, err := mgr.GetSession(context.Background(), "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session.ID != "sess-1" {
		t.Fatalf("session.ID = %q, want sess-1", session.ID)
	}
}

func TestManager_ListMessages(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			messages: []opencode.Message{
				{Info: opencode.MessageInfo{ID: "msg-1", Role: "user"}},
				{Info: opencode.MessageInfo{ID: "msg-2", Role: "assistant"}},
			},
		}
	})

	messages, err := mgr.ListMessages(context.Background(), "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
}

func TestManager_SendPrompt(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			result: opencode.PromptResult{
				MessageID: "msg-3",
				Role:      "assistant",
				Text:      "Hello back!",
			},
		}
	})

	result, err := mgr.SendPrompt(context.Background(), "proj-1", "sess-1", "Hello")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	if result.Text != "Hello back!" {
		t.Fatalf("result.Text = %q, want Hello back!", result.Text)
	}
	if result.Role != "assistant" {
		t.Fatalf("result.Role = %q, want assistant", result.Role)
	}
}

func TestManager_OpencodeError(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			err: errors.New("opencode: GET /session returned 500: boom"),
		}
	})

	_, err := mgr.ListSessions(context.Background(), "proj-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !stringsContains(err.Error(), "opencode:") {
		t.Fatalf("err = %v, want opencode error", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package sessions

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"glyphdeck/internal/opencode"
)

// ---------------------------------------------------------------------------
// HTTP tests
// ---------------------------------------------------------------------------

func postSameOrigin(t *testing.T, ts *httptest.Server, path, contentType string, body io.Reader) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, body)
	if err != nil {
		t.Fatalf("create POST request: %v", err)
	}
	req.Header.Set("Origin", ts.URL)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return ts.Client().Do(req)
}

func TestListSessions_ProjectNotFound(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{projects: map[string]ProjectInfo{}}
	mgr := newTestManager(servers, projects)

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/nonexistent/sessions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListSessions_ServerNotReady(t *testing.T) {
	servers := mockServerResolver{err: ErrServerNotReady}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}
	mgr := newTestManager(servers, projects)

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/proj-1/sessions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestListSessions_Success(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			sessions: []opencode.Session{
				{ID: "sess-1", Title: "First"},
				{ID: "sess-2", Title: "Second"},
			},
		}
	})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/proj-1/sessions")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Sessions []opencode.Session `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(body.Sessions))
	}
}

func TestCreateSession_Success(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			sessions: []opencode.Session{
				{ID: "sess-new", Title: "My Title"},
			},
		}
	})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := strings.NewReader(`{"title":"My Title"}`)
	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/sessions", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var session opencode.Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if session.ID != "sess-new" {
		t.Fatalf("session.ID = %q, want sess-new", session.ID)
	}
}

func TestCreateSession_DefaultTitle(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			sessions: []opencode.Session{
				{ID: "sess-default", Title: "Untitled Session"},
			},
		}
	})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// No title field in JSON.
	body := strings.NewReader(`{}`)
	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/sessions", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

func TestCreateSession_SameOriginRejection(t *testing.T) {
	mgr := newTestManager(mockServerResolver{}, mockProjectResolver{})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/proj-1/sessions", strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.com")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestCreateSession_MissingJSONContentType(t *testing.T) {
	mgr := newTestManager(mockServerResolver{}, mockProjectResolver{})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := strings.NewReader(`{"title":"X"}`)
	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/sessions", "", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnsupportedMediaType)
	}
}

func TestGetSession_Success(t *testing.T) {
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

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/proj-1/sessions/sess-1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestListMessages_Success(t *testing.T) {
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
			},
		}
	})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/proj-1/sessions/sess-1/messages")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Messages []opencode.Message `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(body.Messages))
	}
}

func TestSendPrompt_Success(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			result: opencode.PromptResult{
				MessageID: "msg-2",
				Role:      "assistant",
				Text:      "Response text",
			},
		}
	})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := strings.NewReader(`{"text":"Hello"}`)
	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/sessions/sess-1/prompt", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result opencode.PromptResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Text != "Response text" {
		t.Fatalf("result.Text = %q, want Response text", result.Text)
	}
}

func TestSendPrompt_MissingText(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}
	mgr := newTestManager(servers, projects)

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := strings.NewReader(`{}`)
	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/sessions/sess-1/prompt", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSendPrompt_SameOriginRejection(t *testing.T) {
	mgr := newTestManager(mockServerResolver{}, mockProjectResolver{})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/proj-1/sessions/sess-1/prompt", strings.NewReader(`{"text":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.com")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestSendPrompt_OpencodeError(t *testing.T) {
	servers := mockServerResolver{baseURL: "http://127.0.0.1:4096"}
	projects := mockProjectResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Path: "/test"},
		},
	}

	mgr := NewManagerWithClient(servers, projects, func(baseURL, username, password string) opencode.SessionClient {
		return &mockSessionClient{
			err: errors.New("opencode: POST /session/sess-1/message returned 502: bad gateway"),
		}
	})

	mux := http.NewServeMux()
	RegisterHandlers(mux, mgr)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := strings.NewReader(`{"text":"Hello"}`)
	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/sessions/sess-1/prompt", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}

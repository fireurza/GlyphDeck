package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Fake OpenCode server for client tests
// ---------------------------------------------------------------------------

func newFakeOpenCodeServer(t *testing.T) (*httptest.Server, *fakeOpenCodeHandler) {
	t.Helper()
	h := &fakeOpenCodeHandler{
		sessions: make(map[string][]Session), // keyed by directory
		messages: make(map[string][]Message), // keyed by sessionID
	}
	ts := httptest.NewServer(h.router())
	t.Cleanup(ts.Close)
	return ts, h
}

type fakeOpenCodeHandler struct {
	sessions   map[string][]Session // keyed by directory
	messages   map[string][]Message // keyed by sessionID
	lastAuth   string               // capture auth header for verification
	nextSessID int
	nextMsgID  int
}

func (h *fakeOpenCodeHandler) router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /session", h.handleListSessions)
	mux.HandleFunc("POST /session", h.handleCreateSession)
	mux.HandleFunc("GET /session/{sessionID}", h.handleGetSession)
	mux.HandleFunc("GET /session/{sessionID}/message", h.handleListMessages)
	mux.HandleFunc("POST /session/{sessionID}/message", h.handleSendPrompt)
	return mux
}

func (h *fakeOpenCodeHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	directory := r.URL.Query().Get("directory")
	sessions := h.sessions[directory]
	if sessions == nil {
		sessions = []Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *fakeOpenCodeHandler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.nextSessID++
	session := Session{
		ID:        "sess-" + itoa(h.nextSessID),
		Title:     req.Title,
		Directory: req.Directory,
	}

	h.sessions[req.Directory] = append(h.sessions[req.Directory], session)
	writeJSON(w, http.StatusCreated, session)
}

func (h *fakeOpenCodeHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	for _, ss := range h.sessions {
		for _, s := range ss {
			if s.ID == sessionID {
				writeJSON(w, http.StatusOK, s)
				return
			}
		}
	}
	http.Error(w, "session not found", http.StatusNotFound)
}

func (h *fakeOpenCodeHandler) handleListMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	msgs := h.messages[sessionID]
	if msgs == nil {
		msgs = []Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (h *fakeOpenCodeHandler) handleSendPrompt(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	var req sendPromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Capture auth header.
	h.lastAuth = r.Header.Get("Authorization")

	// Store the user message.
	var userText string
	for _, p := range req.Parts {
		if p.Type == "text" {
			userText = p.Text
			break
		}
	}

	h.nextMsgID++
	userMsg := Message{
		Info:  MessageInfo{ID: "msg-" + itoa(h.nextMsgID), Role: "user"},
		Parts: []Part{{Type: "text", Text: userText}},
	}

	h.nextMsgID++
	assistantMsg := Message{
		Info: MessageInfo{ID: "msg-" + itoa(h.nextMsgID), Role: "assistant"},
		Parts: []Part{
			{Type: "text", Text: "Response to: " + userText},
			{Type: "text", Text: "Second line of response."},
		},
	}

	h.messages[sessionID] = append(h.messages[sessionID], userMsg, assistantMsg)

	// Return the assistant message (OpenCode convention: last posted message).
	writeJSON(w, http.StatusOK, assistantMsg)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestClient_ListSessions_Empty(t *testing.T) {
	ts, _ := newFakeOpenCodeServer(t)
	client := NewClient(ts.URL, "", "")

	sessions, err := client.ListSessions(context.Background(), "/dir")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("len(sessions) = %d, want 0", len(sessions))
	}
}

func TestClient_CreateAndGetSession(t *testing.T) {
	ts, _ := newFakeOpenCodeServer(t)
	client := NewClient(ts.URL, "", "")

	session, err := client.CreateSession(context.Background(), "My Session", "/dir")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.ID == "" {
		t.Fatal("session.ID is empty")
	}
	if session.Title != "My Session" {
		t.Fatalf("session.Title = %q, want My Session", session.Title)
	}

	got, err := client.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != session.ID {
		t.Fatalf("GetSession ID = %q, want %q", got.ID, session.ID)
	}
}

func TestClient_ListMessages_Empty(t *testing.T) {
	ts, _ := newFakeOpenCodeServer(t)
	client := NewClient(ts.URL, "", "")

	session, err := client.CreateSession(context.Background(), "Test", "/dir")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	msgs, err := client.ListMessages(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0", len(msgs))
	}
}

func TestClient_SendPrompt(t *testing.T) {
	ts, h := newFakeOpenCodeServer(t)
	client := NewClient(ts.URL, "user", "pass")

	session, err := client.CreateSession(context.Background(), "Prompt Test", "/dir")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	result, err := client.SendPrompt(context.Background(), session.ID, "Hello, world!")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	if result.Role != "assistant" {
		t.Fatalf("Role = %q, want assistant", result.Role)
	}
	if result.MessageID == "" {
		t.Fatal("MessageID is empty")
	}
	if result.Text == "" {
		t.Fatal("Text is empty")
	}

	// Verify auth header was sent.
	if h.lastAuth == "" {
		t.Error("Authorization header was not sent")
	}

	// After sending, ListMessages should return 2 messages.
	msgs, err := client.ListMessages(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
}

func TestClient_AuthHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "opencode" || pass != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="opencode"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, []Session{})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "opencode", "secret")
	sessions, err := client.ListSessions(context.Background(), "/dir")
	if err != nil {
		t.Fatalf("ListSessions with auth: %v", err)
	}
	if sessions == nil {
		t.Fatal("sessions is nil, want empty slice")
	}
}

func TestClient_NoAuthWhenEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := r.BasicAuth()
		if ok {
			http.Error(w, "unexpected auth", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, []Session{})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", "")
	sessions, err := client.ListSessions(context.Background(), "/dir")
	if err != nil {
		t.Fatalf("ListSessions without auth: %v", err)
	}
	if sessions == nil {
		t.Fatal("sessions is nil, want empty slice")
	}
}

func TestClient_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", "")
	_, err := client.ListSessions(context.Background(), "/dir")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	ts, _ := newFakeOpenCodeServer(t)
	client := NewClient(ts.URL, "", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.ListSessions(ctx, "/dir")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestClient_SendPrompt_NonTextParts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, Message{
			Info: MessageInfo{ID: "msg-1", Role: "assistant"},
			Parts: []Part{
				{Type: "tool_use", Text: ""},
				{Type: "text", Text: "Only text part."},
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", "")
	result, err := client.SendPrompt(context.Background(), "s", "test")
	if err != nil {
		t.Fatalf("SendPrompt: %v", err)
	}
	if result.Text != "Only text part." {
		t.Fatalf("Text = %q, want Only text part.", result.Text)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func itoa(n int) string {
	// Minimal integer-to-string for test IDs. Only called with small positive ints.
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if len(buf) == 0 {
		return "0"
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

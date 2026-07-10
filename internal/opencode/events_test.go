package opencode

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fake SSE server helpers
// ---------------------------------------------------------------------------

// fakeSSEServer returns an httptest.Server that emits SSE events from the given lines.
// Each call to sseHandler is a separate SSE stream.
type fakeSSEServer struct {
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc // path -> handler
}

func newFakeSSEServer() *fakeSSEServer {
	return &fakeSSEServer{
		handlers: make(map[string]http.HandlerFunc),
	}
}

func (f *fakeSSEServer) setHandler(path string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[path] = h
}

func (f *fakeSSEServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		h, ok := f.handlers[r.URL.Path]
		f.mu.Unlock()
		if ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// writeSSELines writes SSE-formatted lines to the response writer and flushes.
func writeSSELines(t *testing.T, w http.ResponseWriter, lines ...string) {
	t.Helper()
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("response writer does not implement http.Flusher")
	}
	for _, line := range lines {
		fmt.Fprint(w, line, "\n")
	}
	flusher.Flush()
}

// realEvent formats a single OpenCode SSE event exactly as OpenCode 1.17.x
// emits it: one `data:` line carrying {"id","type","properties"} JSON,
// terminated by a blank line. There is NO `event:` type line.
func realEvent(id, typ, propsJSON string) []string {
	return []string{
		fmt.Sprintf(`data: {"id":%q,"type":%q,"properties":%s}`, id, typ, propsJSON),
		"",
	}
}

// collectUntil drains events into a slice until pred is satisfied or the
// context/channel closes. Connected/disconnected control events are included.
func collectUntil(ctx context.Context, events <-chan NormalizedEvent, want int) []NormalizedEvent {
	var got []NormalizedEvent
	for len(got) < want {
		select {
		case ev, ok := <-events:
			if !ok {
				return got
			}
			got = append(got, ev)
		case <-ctx.Done():
			return got
		}
	}
	return got
}

// findByType returns the first event with the given type, or nil.
func findByType(got []NormalizedEvent, typ string) *NormalizedEvent {
	for i := range got {
		if got[i].Type == typ {
			return &got[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests: SSE parsing against the REAL OpenCode event format
// ---------------------------------------------------------------------------

func TestStreamEvents_RealFormat_MessageEvents(t *testing.T) {
	fake := newFakeSSEServer()
	var callCount int
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			http.Error(w, "gone", http.StatusGone)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		var lines []string
		lines = append(lines, realEvent("evt_1", "session.updated", `{"sessionID":"ses_1","info":{"id":"ses_1","title":"Test"}}`)...)
		lines = append(lines, realEvent("evt_2", "message.updated", `{"sessionID":"ses_1","info":{"id":"msg_1","role":"assistant"}}`)...)
		lines = append(lines, realEvent("evt_3", "message.part.updated", `{"sessionID":"ses_1","part":{"messageID":"msg_1","type":"text","text":"hello"},"time":1}`)...)
		writeSSELines(t, w, lines...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 4) // connected + 3 events

	t.Run("connected", func(t *testing.T) {
		if findByType(got, "glyphdeck.eventstream.connected") == nil {
			t.Errorf("expected glyphdeck.eventstream.connected; got %+v", got)
		}
	})

	t.Run("session.updated", func(t *testing.T) {
		sess := findByType(got, "opencode.session.updated")
		if sess == nil {
			t.Fatalf("missing opencode.session.updated; got %+v", got)
		}
		if sess.SessionID != "ses_1" {
			t.Errorf("session.updated SessionID = %q, want ses_1", sess.SessionID)
		}
	})

	t.Run("message.updated", func(t *testing.T) {
		msg := findByType(got, "opencode.message.updated")
		if msg == nil {
			t.Fatalf("missing opencode.message.updated")
		}
		if msg.SessionID != "ses_1" || msg.MessageID != "msg_1" {
			t.Errorf("message.updated ids = (%q,%q), want (ses_1,msg_1)", msg.SessionID, msg.MessageID)
		}
	})

	t.Run("message.part.updated", func(t *testing.T) {
		part := findByType(got, "opencode.message.part.updated")
		if part == nil {
			t.Fatalf("missing opencode.message.part.updated")
		}
		if part.SessionID != "ses_1" || part.MessageID != "msg_1" {
			t.Errorf("part.updated ids = (%q,%q), want (ses_1,msg_1)", part.SessionID, part.MessageID)
		}
		if m, ok := part.Data.(map[string]any); !ok {
			t.Errorf("part.updated Data is not properties map: %T", part.Data)
		} else if _, ok := m["part"]; !ok {
			t.Errorf("part.updated Data missing 'part' key: %v", m)
		}
	})
}

func TestStreamEvents_RealFormat_Delta(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, realEvent("evt_1", "message.part.delta",
			`{"sessionID":"ses_1","messageID":"msg_1","partID":"prt_1","field":"text","delta":"he"}`)...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2) // connected + delta

	d := findByType(got, "opencode.message.part.delta")
	if d == nil {
		t.Fatalf("missing opencode.message.part.delta; got %+v", got)
	}
	if d.SessionID != "ses_1" || d.MessageID != "msg_1" {
		t.Errorf("delta ids = (%q,%q), want (ses_1,msg_1)", d.SessionID, d.MessageID)
	}
	m, ok := d.Data.(map[string]any)
	if !ok {
		t.Fatalf("delta Data not a map: %T", d.Data)
	}
	if m["delta"] != "he" || m["field"] != "text" {
		t.Errorf("delta payload = %v, want field=text delta=he", m)
	}
}

func TestStreamEvents_ServerConnectedMapped(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, realEvent("evt_1", "server.connected", `{}`)...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2)

	// Both the handshake signal and the server.connected event normalize to
	// the same connected type — at least one must be present.
	if findByType(got, "glyphdeck.eventstream.connected") == nil {
		t.Errorf("server.connected not mapped to glyphdeck.eventstream.connected; got %+v", got)
	}
}

func TestStreamEvents_UnknownEventPassthrough(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, realEvent("evt_1", "custom.plugin.action", `{"key":"value"}`)...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2)

	if findByType(got, "opencode.custom.plugin.action") == nil {
		t.Errorf("unknown type not prefixed to opencode.custom.plugin.action; got %+v", got)
	}
}

func TestStreamEvents_MultiLineData(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		// A JSON object split across two data: lines.
		writeSSELines(t, w,
			`data: {"id":"evt_1","type":"session.updated",`,
			`data:  "properties":{"sessionID":"ses_1","info":{"id":"ses_1"}}}`,
			"",
		)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2)

	s := findByType(got, "opencode.session.updated")
	if s == nil || s.SessionID != "ses_1" {
		t.Errorf("multi-line data not reassembled correctly; got %+v", got)
	}
}

func TestStreamEvents_JSONDecodeFailure(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, "data: not-valid-json!!!", "")
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2)

	u := findByType(got, "opencode.unknown")
	if u == nil {
		t.Fatalf("expected opencode.unknown for bad JSON; got %+v", got)
	}
	if s, ok := u.Data.(string); !ok || s != "not-valid-json!!!" {
		t.Errorf("unknown Data = %v, want raw string", u.Data)
	}
}

func TestStreamEvents_EmptyType(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, `data: {"id":"evt_1","properties":{"k":"v"}}`, "")
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2)

	if findByType(got, "opencode.unknown") == nil {
		t.Errorf("empty type should normalize to opencode.unknown; got %+v", got)
	}
}

func TestStreamEvents_ContextCancellation(t *testing.T) {
	fake := newFakeSSEServer()
	connOpen := make(chan struct{})
	connClosed := make(chan struct{})

	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(connOpen)
		<-r.Context().Done()
		close(connClosed)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	events, _ := client.StreamEvents(ctx)

	select {
	case <-connOpen:
	case <-time.After(time.Second):
		t.Fatal("SSE connection did not open")
	}
	cancel()

	select {
	case <-connClosed:
	case <-time.After(time.Second):
		t.Fatal("server did not detect client disconnect")
	}
	for range events {
	}
}

func TestStreamEvents_Reconnect(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		count := callCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		if count == 1 {
			writeSSELines(t, w, realEvent("evt_1", "session.updated", `{"sessionID":"ses_1","info":{"id":"ses_1"}}`)...)
			return // close connection
		}
		if count == 2 {
			writeSSELines(t, w, realEvent("evt_2", "message.updated", `{"sessionID":"ses_1","info":{"id":"msg_2"}}`)...)
			return
		}
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 6)

	if findByType(got, "glyphdeck.eventstream.disconnected") == nil {
		t.Error("expected glyphdeck.eventstream.disconnected event")
	}
	if findByType(got, "glyphdeck.eventstream.connected") == nil {
		t.Error("expected glyphdeck.eventstream.connected event")
	}

	mu.Lock()
	finalCalls := callCount
	mu.Unlock()
	if finalCalls < 2 {
		t.Errorf("callCount = %d, want at least 2 (reconnect happened)", finalCalls)
	}
}

func TestStreamEvents_HTTPError(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	events, errs := client.StreamEvents(ctx)

	var lastErr error
	for {
		select {
		case _, ok := <-events:
			if !ok {
				events = nil
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
			} else if err != nil && lastErr == nil {
				lastErr = err
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for error")
		}
		if events == nil && errs == nil {
			break
		}
	}

	if lastErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(lastErr.Error(), "500") && !strings.Contains(lastErr.Error(), "SSE returned") {
		t.Errorf("unexpected error: %v", lastErr)
	}
}

func TestStreamEvents_AuthHeader(t *testing.T) {
	var capturedAuth string
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, realEvent("evt_1", "session.updated", `{"sessionID":"ses_1"}`)...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "opencode", "secret")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	for range events {
		cancel()
	}

	if capturedAuth == "" {
		t.Error("Authorization header not sent for SSE request")
	}
}

func TestStreamEvents_NoAuthWhenEmpty(t *testing.T) {
	var capturedAuth string
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w, realEvent("evt_1", "session.updated", `{"sessionID":"ses_1"}`)...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	for range events {
		cancel()
	}

	if capturedAuth != "" {
		t.Error("Authorization header sent but credentials were empty")
	}
}

func TestStreamEvents_PermissionEvents(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		var lines []string
		lines = append(lines, realEvent("evt_1", "permission.asked", `{"sessionID":"ses_1","permission":"bash"}`)...)
		lines = append(lines, realEvent("evt_2", "permission.replied", `{"sessionID":"ses_1","permission":"bash"}`)...)
		writeSSELines(t, w, lines...)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 3)

	if findByType(got, "opencode.permission.asked") == nil {
		t.Errorf("missing opencode.permission.asked; got %+v", got)
	}
	if findByType(got, "opencode.permission.replied") == nil {
		t.Errorf("missing opencode.permission.replied; got %+v", got)
	}
}

func TestStreamEvents_CommentLinesIgnored(t *testing.T) {
	fake := newFakeSSEServer()
	fake.setHandler("/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		writeSSELines(t, w,
			": this is a comment",
			`data: {"id":"evt_1","type":"session.updated","properties":{"sessionID":"ses_1"}}`,
			"",
		)
	})
	ts := fake.start(t)

	client := NewClient(ts.URL, "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, _ := client.StreamEvents(ctx)
	got := collectUntil(ctx, events, 2)

	if findByType(got, "opencode.session.updated") == nil {
		t.Errorf("comment line broke parsing; got %+v", got)
	}
}

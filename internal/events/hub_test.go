package events

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"glyphdeck/internal/opencode"
)

// ---------------------------------------------------------------------------
// Mock EventStream for hub tests
// ---------------------------------------------------------------------------

type mockEventStream struct {
	events chan opencode.NormalizedEvent
	errs   chan error
}

func newMockEventStream() *mockEventStream {
	return &mockEventStream{
		events: make(chan opencode.NormalizedEvent, 256),
		errs:   make(chan error, 1),
	}
}

func (m *mockEventStream) StreamEvents(ctx context.Context) (<-chan opencode.NormalizedEvent, <-chan error) {
	events := make(chan opencode.NormalizedEvent, 256)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-m.events:
				if !ok {
					return
				}
				select {
				case events <- ev:
				case <-ctx.Done():
					return
				}
			case err, ok := <-m.errs:
				if !ok {
					return
				}
				select {
				case errs <- err:
				default:
				}
			}
		}
	}()

	return events, errs
}

func (m *mockEventStream) send(ev opencode.NormalizedEvent) {
	m.events <- ev
}

func (m *mockEventStream) close() {
	close(m.events)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// bridgeEventFromSSE parses an SSE "data:" line into a bridgeEvent.
func parseBridgeEvent(t *testing.T, sseLine string) bridgeEvent {
	t.Helper()
	// Strip "data: " prefix.
	data := strings.TrimPrefix(sseLine, "data: ")
	var be bridgeEvent
	if err := json.Unmarshal([]byte(data), &be); err != nil {
		t.Fatalf("parse bridge event: %v\ndata: %s", err, data)
	}
	return be
}

// readSSELine reads one SSE line from the response body.
func readSSELines(t *testing.T, resp *http.Response) []string {
	t.Helper()
	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHub_SubscribeReceivesEvents(t *testing.T) {
	hub := NewHub()
	mockStream := newMockEventStream()

	// Start SSE server that the hub will serve.
	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start bridge with mock stream.
	hub.startBridgeWithStream(ctx, "proj-1", mockStream)

	// Connect a browser client.
	var wg sync.WaitGroup
	var received []bridgeEvent
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get(ts.URL + "?projectId=proj-1")
		if err != nil {
			t.Errorf("browser SSE request: %v", err)
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				be := parseBridgeEvent(t, line)
				// Ignore the connected/disconnected control events replayed to
				// late subscribers; this test asserts real event fanout.
				if strings.HasPrefix(be.Type, "glyphdeck.") {
					continue
				}
				mu.Lock()
				received = append(received, be)
				mu.Unlock()
				if len(received) >= 2 {
					cancel() // stop the client
					return
				}
			}
		}
	}()

	// Wait briefly for subscriber to connect.
	time.Sleep(50 * time.Millisecond)

	// Send events.
	mockStream.send(opencode.NormalizedEvent{
		Type:      "opencode.session.updated",
		SessionID: "ses_1",
		Data:      map[string]any{"sessionId": "ses_1"},
	})
	mockStream.send(opencode.NormalizedEvent{
		Type:      "opencode.message.updated",
		SessionID: "ses_1",
		MessageID: "msg_1",
		Data:      map[string]any{"sessionId": "ses_1", "messageId": "msg_1"},
	})

	// Give time for events to propagate, then force client disconnect.
	time.Sleep(100 * time.Millisecond)
	mockStream.close()

	wg.Wait()
	ts.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(received) < 2 {
		t.Fatalf("received %d events, want 2", len(received))
	}
	if received[0].Type != "opencode.session.updated" {
		t.Errorf("event[0].Type = %q, want opencode.session.updated", received[0].Type)
	}
	if received[1].Type != "opencode.message.updated" {
		t.Errorf("event[1].Type = %q, want opencode.message.updated", received[1].Type)
	}
}

func TestHub_ProjectFiltering(t *testing.T) {
	hub := NewHub()
	mockStream := newMockEventStream()

	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	hub.startBridgeWithStream(ctx, "proj-1", mockStream)

	var received []bridgeEvent
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Client subscribes to proj-2 (should NOT receive proj-1 events).
	wg.Add(1)
	go func() {
		defer wg.Done()
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer reqCancel()
		req, err := http.NewRequestWithContext(reqCtx, "GET", ts.URL+"?projectId=proj-2", nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return // expected on timeout or disconnect
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				be := parseBridgeEvent(t, line)
				mu.Lock()
				received = append(received, be)
				mu.Unlock()
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Send event for proj-1.
	mockStream.send(opencode.NormalizedEvent{
		Type: "opencode.session.updated",
		Data: map[string]any{"sessionId": "ses_1"},
	})

	// Wait enough time for event to propagate (or not).
	time.Sleep(200 * time.Millisecond)

	// Kill the bridge by closing mock stream.
	cancel()
	mockStream.close()

	wg.Wait()
	ts.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(received) > 0 {
		t.Errorf("received %d events for wrong project, want 0", len(received))
	}
}

func TestHub_WildcardSubscriber(t *testing.T) {
	hub := NewHub()
	mockStream := newMockEventStream()

	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.startBridgeWithStream(ctx, "proj-1", mockStream)

	var received []bridgeEvent
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Client without projectId (wildcard) should receive all.
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Errorf("browser SSE request: %v", err)
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				be := parseBridgeEvent(t, line)
				mu.Lock()
				received = append(received, be)
				mu.Unlock()
				if len(received) >= 1 {
					cancel()
					return
				}
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	mockStream.send(opencode.NormalizedEvent{
		Type: "opencode.session.updated",
		Data: map[string]any{"sessionId": "ses_1"},
	})

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(received) < 1 {
		t.Fatal("wildcard subscriber received no events")
	}
	if received[0].ProjectID != "proj-1" {
		t.Errorf("event.ProjectID = %q, want proj-1", received[0].ProjectID)
	}
}

func TestHub_MultipleSubscribers(t *testing.T) {
	hub := NewHub()
	mockStream := newMockEventStream()

	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.startBridgeWithStream(ctx, "proj-1", mockStream)

	var mu sync.Mutex
	received1 := make([]bridgeEvent, 0)
	received2 := make([]bridgeEvent, 0)
	var wg sync.WaitGroup

	// Two subscribers.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := http.Get(ts.URL + "?projectId=proj-1")
			if err != nil {
				t.Errorf("browser SSE request %d: %v", idx, err)
				return
			}
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					be := parseBridgeEvent(t, line)
					mu.Lock()
					if idx == 0 {
						received1 = append(received1, be)
					} else {
						received2 = append(received2, be)
					}
					total := len(received1) + len(received2)
					mu.Unlock()
					if total >= 2 {
						return
					}
				}
			}
		}(i)
	}

	time.Sleep(50 * time.Millisecond)

	mockStream.send(opencode.NormalizedEvent{
		Type: "opencode.session.updated",
		Data: map[string]any{"sessionId": "ses_1"},
	})

	time.Sleep(200 * time.Millisecond)
	mockStream.close()

	wg.Wait()
	ts.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(received1) == 0 {
		t.Error("subscriber 1 received no events")
	}
	if len(received2) == 0 {
		t.Error("subscriber 2 received no events")
	}
}

func TestHub_SubscriberCleanupOnDisconnect(t *testing.T) {
	hub := NewHub()
	mockStream := newMockEventStream()

	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	hub.startBridgeWithStream(ctx, "proj-1", mockStream)

	// Override transport to disconnect after one read.
	client := &http.Client{
		Transport: &http.Transport{},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, err := http.NewRequest("GET", ts.URL+"?projectId=proj-1", nil)
		if err != nil {
			t.Errorf("create request: %v", err)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			// Expected on disconnect.
			return
		}
		defer resp.Body.Close()

		// Read one line then disconnect.
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			_ = scanner.Text()
			cancel() // kill the bridge, which causes client to disconnect
			return
		}
	}()

	time.Sleep(50 * time.Millisecond)

	mockStream.send(opencode.NormalizedEvent{
		Type: "opencode.session.updated",
		Data: map[string]any{"sessionId": "ses_1"},
	})

	wg.Wait()

	// Stop the bridge and allow subscriber cleanup to propagate.
	mockStream.close()
	time.Sleep(50 * time.Millisecond)

	// Check subscriber count is 0.
	hub.mu.RLock()
	count := len(hub.subscribers)
	hub.mu.RUnlock()

	if count != 0 {
		t.Errorf("subscriber count = %d, want 0 after disconnect", count)
	}
}

func TestHub_StopBridge(t *testing.T) {
	hub := NewHub()
	mockStream := newMockEventStream()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.startBridgeWithStream(ctx, "proj-1", mockStream)

	// Verify bridge exists.
	hub.mu.RLock()
	_, exists := hub.bridges["proj-1"]
	hub.mu.RUnlock()
	if !exists {
		t.Fatal("bridge not found after start")
	}

	// Stop the bridge.
	hub.StopEventBridge("proj-1")

	// Wait briefly for goroutine cleanup.
	time.Sleep(50 * time.Millisecond)

	// Verify bridge removed.
	hub.mu.RLock()
	_, exists = hub.bridges["proj-1"]
	hub.mu.RUnlock()
	if exists {
		t.Error("bridge still exists after stop")
	}
}

func TestHub_StopAll(t *testing.T) {
	hub := NewHub()

	stream1 := newMockEventStream()
	stream2 := newMockEventStream()

	ctx := context.Background()

	hub.startBridgeWithStream(ctx, "proj-1", stream1)
	hub.startBridgeWithStream(ctx, "proj-2", stream2)

	hub.mu.RLock()
	count := len(hub.bridges)
	hub.mu.RUnlock()
	if count != 2 {
		t.Fatalf("bridge count = %d, want 2", count)
	}

	hub.StopAll()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	count = len(hub.bridges)
	hub.mu.RUnlock()
	if count != 0 {
		t.Errorf("bridge count = %d, want 0 after StopAll", count)
	}
}

func TestHub_DuplicateBridgeIgnored(t *testing.T) {
	hub := NewHub()
	stream1 := newMockEventStream()
	stream2 := newMockEventStream()

	ctx := context.Background()

	hub.startBridgeWithStream(ctx, "proj-1", stream1)
	// Second start should be a no-op.
	hub.startBridgeWithStream(ctx, "proj-1", stream2)

	hub.mu.RLock()
	count := len(hub.bridges)
	hub.mu.RUnlock()
	if count != 1 {
		t.Errorf("bridge count = %d, want 1 (duplicate should be ignored)", count)
	}
}

func TestHub_BrowserSSEContentType(t *testing.T) {
	hub := NewHub()

	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("SSE request: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	cc := resp.Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}

	conn := resp.Header.Get("Connection")
	if conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
}

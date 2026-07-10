// Package events provides the event bus and bridge between backend subsystems.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"glyphdeck/internal/opencode"
)

// subscriber represents a connected browser SSE client.
type subscriber struct {
	id        string
	ch        chan []byte // buffered channel for SSE data (JSON bytes)
	projectID string      // empty means receive all projects
	closeOnce sync.Once
}

// close safely closes the subscriber channel once.
func (s *subscriber) close() {
	s.closeOnce.Do(func() {
		close(s.ch)
	})
}

// Hub manages browser SSE subscribers and fans out events from OpenCode bridges.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]*subscriber        // keyed by subscriber ID
	bridges     map[string]context.CancelFunc // projectID -> cancel function
	nextSubID   int
}

// NewHub creates an empty event hub.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]*subscriber),
		bridges:     make(map[string]context.CancelFunc),
	}
}

// StartEventBridge creates a new OpenCode SSE connection for the given project
// and fans out all received events to matching subscribers.
func (h *Hub) StartEventBridge(projectID, baseURL string) error {
	h.mu.Lock()
	if _, exists := h.bridges[projectID]; exists {
		h.mu.Unlock()
		return fmt.Errorf("events: bridge already running for project %s", projectID)
	}

	username, password := opencode.GetServerCreds()
	client := opencode.NewClient(baseURL, username, password)

	bridgeCtx, cancel := context.WithCancel(context.Background())
	h.bridges[projectID] = cancel
	h.mu.Unlock()

	go h.runBridgeWithCleanup(bridgeCtx, projectID, client)
	return nil
}

// startBridgeWithStream is a test-only entry point that accepts an EventStream
// directly instead of creating a Client internally.
func (h *Hub) startBridgeWithStream(ctx context.Context, projectID string, stream opencode.EventStream) {
	h.mu.Lock()
	if _, exists := h.bridges[projectID]; exists {
		h.mu.Unlock()
		return
	}
	bridgeCtx, cancel := context.WithCancel(context.Background())
	h.bridges[projectID] = cancel
	h.mu.Unlock()

	go h.runBridgeWithCleanup(bridgeCtx, projectID, stream)
}

// StopEventBridge stops the OpenCode SSE bridge for a project and cleans up.
func (h *Hub) StopEventBridge(projectID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cancel, exists := h.bridges[projectID]
	if !exists {
		return
	}
	cancel()
	delete(h.bridges, projectID)
}

// StopAll stops all running bridges and removes all subscribers.
func (h *Hub) StopAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for projectID, cancel := range h.bridges {
		cancel()
		delete(h.bridges, projectID)
	}
}

// nextSubscriberID returns a unique subscriber identifier.
func (h *Hub) nextSubscriberID() string {
	h.nextSubID++
	return fmt.Sprintf("sub-%d", h.nextSubID)
}

// addSubscriber registers a new browser SSE client.
func (h *Hub) addSubscriber(projectID string) *subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()

	sub := &subscriber{
		id:        h.nextSubscriberID(),
		ch:        make(chan []byte, 256),
		projectID: projectID,
	}
	h.subscribers[sub.id] = sub

	// The bridge emits glyphdeck.eventstream.connected once, at the OpenCode
	// handshake. A browser that subscribes AFTER that would never learn the
	// stream is live. If a matching bridge is already running, replay a
	// connected signal to this subscriber so late joiners see real status.
	if bridgeProjectID, ok := h.matchingLiveBridge(projectID); ok {
		if data, err := json.Marshal(bridgeEvent{
			Type:      "glyphdeck.eventstream.connected",
			ProjectID: bridgeProjectID,
		}); err == nil {
			select {
			case sub.ch <- data:
			default:
			}
		}
	}

	return sub
}

// matchingLiveBridge reports whether a running bridge matches the subscriber's
// project filter, and returns the bridge's projectID. Caller holds h.mu.
func (h *Hub) matchingLiveBridge(projectID string) (string, bool) {
	if projectID != "" {
		if _, ok := h.bridges[projectID]; ok {
			return projectID, true
		}
		return "", false
	}
	// Wildcard subscriber: any running bridge counts.
	for pid := range h.bridges {
		return pid, true
	}
	return "", false
}

// removeSubscriber removes a browser SSE client.
func (h *Hub) removeSubscriber(sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers, sub.id)
}

// closeProjectSubscribers closes the channels of all subscribers filtered to a project.
// This causes any blocked ServeHTTP goroutines to exit cleanly.
func (h *Hub) closeProjectSubscribers(projectID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sub := range h.subscribers {
		if sub.projectID == projectID || sub.projectID == "" {
			sub.close()
		}
	}
}

// bridgeEvent is the JSON envelope sent to browser SSE clients.
type bridgeEvent struct {
	Type      string `json:"type"`
	ProjectID string `json:"projectId"`
	SessionID string `json:"sessionId,omitempty"`
	Payload   any    `json:"payload"`
}

// runBridgeWithCleanup reads events from the OpenCode client and fans them out,
// wrapping the fanout loop with bridge cleanup on exit.
func (h *Hub) runBridgeWithCleanup(ctx context.Context, projectID string, stream opencode.EventStream) {
	defer func() {
		h.mu.Lock()
		delete(h.bridges, projectID)
		h.mu.Unlock()
		// Close subscriber channels so ServeHTTP goroutines exit cleanly.
		h.closeProjectSubscribers(projectID)
	}()

	h.runBridgeWithStream(ctx, projectID, stream)
}

// runBridgeWithStream is the actual fanout loop shared by both bridge entry points.
func (h *Hub) runBridgeWithStream(ctx context.Context, projectID string, stream opencode.EventStream) {
	events, errs := stream.StreamEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errs:
			if !ok {
				return
			}
			log.Printf("events: bridge error for project %s: %v", projectID, err)
			return
		case ne, ok := <-events:
			if !ok {
				return
			}
			h.fanout(projectID, ne)
		}
	}
}

// fanout marshals a NormalizedEvent into the browser envelope and sends it
// to all subscribers whose project filter matches.
func (h *Hub) fanout(projectID string, ne opencode.NormalizedEvent) {
	envelope := bridgeEvent{
		Type:      ne.Type,
		ProjectID: projectID,
		SessionID: ne.SessionID,
		Payload:   ne.Data,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		// Filter: empty projectID means subscribe to all; otherwise must match.
		if sub.projectID != "" && sub.projectID != projectID {
			continue
		}

		select {
		case sub.ch <- data:
		default:
			// Channel full; drop event to avoid blocking.
		}
	}
}

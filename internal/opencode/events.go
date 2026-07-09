package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// errNonRetryable is returned when the SSE endpoint returns a non-2xx status.
type errNonRetryable struct {
	code int
	body string
}

func (e *errNonRetryable) Error() string {
	return fmt.Sprintf("opencode: SSE returned %d: %s", e.code, e.body)
}

// StreamEvents connects to the OpenCode SSE endpoint and streams normalized events.
// Reconnects with bounded backoff on unexpected disconnects.
// The returned channels are closed when ctx is cancelled or max retries are exhausted.
func (c *Client) StreamEvents(ctx context.Context) (<-chan NormalizedEvent, <-chan error) {
	events := make(chan NormalizedEvent, 256)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		const maxRetries = 10
		backoff := 500 * time.Millisecond

		for attempt := 0; attempt <= maxRetries; attempt++ {
			// readSSEStream emits glyphdeck.eventstream.connected after a
			// successful HTTP 200 handshake, so we do not emit it here.
			err := c.readSSEStream(ctx, events)
			if ctx.Err() != nil {
				return // normal shutdown via context cancellation
			}

			if err != nil {
				// Non-retryable errors (HTTP status errors) are fatal immediately.
				var nr *errNonRetryable
				if errors.As(err, &nr) || attempt >= maxRetries {
					select {
					case events <- NormalizedEvent{Type: "glyphdeck.eventstream.error", Data: err.Error()}:
					default:
					}
					select {
					case errs <- err:
					default:
					}
					return
				}
			}

			select {
			case events <- NormalizedEvent{Type: "glyphdeck.eventstream.disconnected"}:
			default:
			}

			// Backoff before reconnecting.
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}()

	return events, errs
}

// readSSEStream makes a single GET /event connection and reads SSE events.
// Returns on connection close, context cancellation, or read error.
func (c *Client) readSSEStream(ctx context.Context, events chan<- NormalizedEvent) error {
	url := c.baseURL + "/event"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("opencode: create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode: SSE request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &errNonRetryable{code: resp.StatusCode, body: string(body)}
	}

	// A successful 200 handshake means the OpenCode SSE stream is live.
	// Emit a connected signal so the browser can show real stream status.
	select {
	case events <- NormalizedEvent{Type: "glyphdeck.eventstream.connected"}:
	case <-ctx.Done():
		return nil
	}

	// OpenCode's /event stream does NOT use SSE `event:` type lines. Each event
	// is a single JSON object on one or more `data:` lines, with the event type
	// carried in the JSON `type` field and the payload under `properties`.
	scanner := bufio.NewScanner(resp.Body)
	// Allow large single-line events (tool output, big parts) up to 1 MiB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line terminates an event.
			if len(dataLines) > 0 {
				c.emitSSEEvent(events, strings.Join(dataLines, "\n"))
			}
			dataLines = nil
			continue
		}

		// Skip comment lines (start with colon).
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Ignore `event:` lines if a proxy ever adds them; type comes from JSON.
		if strings.HasPrefix(line, "event:") {
			continue
		}
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			dataLines = append(dataLines, strings.TrimSpace(after))
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("opencode: SSE read error: %w", err)
	}
	return nil
}

// emitSSEEvent decodes an OpenCode SSE event JSON, normalizes the event type,
// extracts session/message identifiers, and sends a NormalizedEvent on the
// events channel (non-blocking drop on full).
//
// OpenCode event envelope shape (verified against OpenCode 1.17.x /event):
//
//	{"id":"evt_...","type":"message.part.updated","properties":{...}}
//
// The `type` field carries the event type; `properties` carries the payload.
func (c *Client) emitSSEEvent(events chan<- NormalizedEvent, data string) {
	var envelope struct {
		Type       string          `json:"type"`
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		// Non-JSON payload — pass through as a raw unknown event.
		select {
		case events <- NormalizedEvent{Type: "opencode.unknown", Data: data}:
		default:
		}
		return
	}

	var props map[string]any
	if len(envelope.Properties) > 0 {
		_ = json.Unmarshal(envelope.Properties, &props)
	}

	ev := NormalizedEvent{
		Type: normalizeEventType(envelope.Type),
		Data: props,
	}
	ev.SessionID, ev.MessageID = extractIDs(envelope.Type, props)

	select {
	case events <- ev:
	default:
		// Channel full; drop event to avoid blocking the SSE reader.
	}
}

// extractIDs pulls the sessionID and messageID out of an OpenCode event's
// properties based on the concrete event type. OpenCode uses capital-ID keys
// (sessionID, messageID) and nests IDs differently per event type.
func extractIDs(rawType string, props map[string]any) (sessionID, messageID string) {
	if props == nil {
		return "", ""
	}
	sessionID, _ = props["sessionID"].(string)

	switch rawType {
	case "message.part.delta":
		messageID, _ = props["messageID"].(string)
	case "message.part.updated":
		if part, ok := props["part"].(map[string]any); ok {
			messageID, _ = part["messageID"].(string)
			if sessionID == "" {
				sessionID, _ = part["sessionID"].(string)
			}
		}
	case "message.updated", "message.removed":
		if info, ok := props["info"].(map[string]any); ok {
			messageID, _ = info["id"].(string)
			if sessionID == "" {
				sessionID, _ = info["sessionID"].(string)
			}
		}
	case "session.updated", "session.created", "session.deleted":
		if sessionID == "" {
			if info, ok := props["info"].(map[string]any); ok {
				sessionID, _ = info["id"].(string)
			}
		}
	}
	return sessionID, messageID
}

// normalizeEventType maps raw OpenCode event types to GlyphDeck-internal names.
// server.connected is mapped to the GlyphDeck connected signal so the browser
// can reflect real OpenCode stream connectivity.
func normalizeEventType(raw string) string {
	switch raw {
	case "":
		return "opencode.unknown"
	case "server.connected":
		return "glyphdeck.eventstream.connected"
	default:
		// Already-namespaced GlyphDeck/OpenCode types pass through unchanged.
		if strings.HasPrefix(raw, "opencode.") || strings.HasPrefix(raw, "glyphdeck.") {
			return raw
		}
		return "opencode." + raw
	}
}

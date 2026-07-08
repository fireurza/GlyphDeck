package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client makes authenticated HTTP calls to an OpenCode serve instance.
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
}

// NewClient creates an OpenCode API client.
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
		username:   username,
		password:   password,
	}
}

// createSessionRequest is the body for POST /session.
type createSessionRequest struct {
	Title     string `json:"title,omitempty"`
	Directory string `json:"directory"`
}

// sendPromptRequest is the body for POST /session/{id}/message.
type sendPromptRequest struct {
	Parts   []Part `json:"parts"`
	NoReply bool   `json:"noReply"`
}

// ListSessions returns all sessions for a directory.
func (c *Client) ListSessions(ctx context.Context, directory string) ([]Session, error) {
	var sessions []Session
	path := fmt.Sprintf("/session?directory=%s", url.QueryEscape(directory))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// CreateSession creates a new OpenCode session.
func (c *Client) CreateSession(ctx context.Context, title, directory string) (Session, error) {
	body := createSessionRequest{Title: title, Directory: directory}
	data, err := json.Marshal(body)
	if err != nil {
		return Session{}, fmt.Errorf("marshal create session request: %w", err)
	}

	var session Session
	if err := c.doJSON(ctx, http.MethodPost, "/session", data, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// GetSession returns a single session's details.
func (c *Client) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var session Session
	path := fmt.Sprintf("/session/%s", sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// ListMessages returns all messages for a session.
func (c *Client) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	var messages []Message
	path := fmt.Sprintf("/session/%s/message", sessionID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// SendPrompt sends a user message and returns the assistant response.
func (c *Client) SendPrompt(ctx context.Context, sessionID, text string) (PromptResult, error) {
	req := sendPromptRequest{
		Parts:   []Part{{Type: "text", Text: text}},
		NoReply: false,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return PromptResult{}, fmt.Errorf("marshal prompt request: %w", err)
	}

	var msg Message
	path := fmt.Sprintf("/session/%s/message", sessionID)
	if err := c.doJSON(ctx, http.MethodPost, path, data, &msg); err != nil {
		return PromptResult{}, err
	}

	return msg.toPromptResult(), nil
}

// doJSON performs an HTTP request with JSON request/response bodies.
func (c *Client) doJSON(ctx context.Context, method, path string, reqBody []byte, result any) error {
	url := c.baseURL + path

	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("opencode: create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode: request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("opencode: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("opencode: %s %s returned %d: %s", method, path, resp.StatusCode, string(respData))
	}

	if result == nil {
		return nil
	}

	if err := json.Unmarshal(respData, result); err != nil {
		return fmt.Errorf("opencode: decode response: %w\nbody: %s", err, string(respData))
	}

	return nil
}

// toPromptResult extracts a PromptResult from an assistant Message.
func (m Message) toPromptResult() PromptResult {
	var textBuilder strings.Builder
	for _, p := range m.Parts {
		if p.Type == "text" && p.Text != "" {
			if textBuilder.Len() > 0 {
				textBuilder.WriteByte('\n')
			}
			textBuilder.WriteString(p.Text)
		}
	}
	return PromptResult{
		MessageID: m.Info.ID,
		Role:      m.Info.Role,
		Text:      textBuilder.String(),
		Parts:     m.Parts,
	}
}

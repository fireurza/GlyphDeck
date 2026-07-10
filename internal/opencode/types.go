package opencode

import "context"

// SessionClient abstracts OpenCode session API operations for testability.
type SessionClient interface {
	ListSessions(ctx context.Context, directory string) ([]Session, error)
	CreateSession(ctx context.Context, title, directory string) (Session, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	ListMessages(ctx context.Context, sessionID string) ([]Message, error)
	SendPrompt(ctx context.Context, sessionID, text string) (PromptResult, error)
	StreamEvents(ctx context.Context) (<-chan NormalizedEvent, <-chan error)
}

// NormalizedEvent is the GlyphDeck-internal event representation.
type NormalizedEvent struct {
	Type      string
	SessionID string
	MessageID string
	Data      any // raw decoded JSON payload
}

// EventStream defines the interface for streaming OpenCode events.
type EventStream interface {
	StreamEvents(ctx context.Context) (<-chan NormalizedEvent, <-chan error)
}

// Session represents an OpenCode session.
type Session struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Directory string `json:"directory"`
	ProjectID string `json:"projectID"`
	Time      any    `json:"time,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Model     any    `json:"model,omitempty"`
}

// Message represents a message within an OpenCode session.
type Message struct {
	Info  MessageInfo `json:"info"`
	Parts []Part      `json:"parts"`
}

// MessageInfo carries metadata about a message.
type MessageInfo struct {
	ID         string      `json:"id"`
	Role       string      `json:"role"`
	ProviderID string      `json:"providerID,omitempty"`
	ModelID    string      `json:"modelID,omitempty"`
	Agent      string      `json:"agent,omitempty"`
	Mode       string      `json:"mode,omitempty"`
	Cost       float64     `json:"cost,omitempty"`
	Tokens     TokenUsage  `json:"tokens,omitempty"`
}

// TokenUsage captures token breakdown from assistant messages.
type TokenUsage struct {
	Total     int        `json:"total"`
	Input     int        `json:"input"`
	Output    int        `json:"output"`
	Reasoning int        `json:"reasoning"`
	Cache     CacheUsage `json:"cache,omitempty"`
}

// CacheUsage captures cache read/write token counts.
type CacheUsage struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

// Part holds a piece of message content.
type Part struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// PromptResult captures the assistant response from SendPrompt.
type PromptResult struct {
	MessageID string
	Role      string
	Text      string
	Parts     []Part
}

// PermissionRequest represents a pending permission request from OpenCode.
type PermissionRequest struct {
	ID         string            `json:"id"`
	SessionID  string            `json:"sessionID"`
	Permission string            `json:"permission"`
	Patterns   []string          `json:"patterns"`
	Metadata   PermissionMeta    `json:"metadata"`
	Always     []string          `json:"always"`
	Tool       PermissionToolRef `json:"tool"`
}

// PermissionMeta carries the command/binary context of a permission request.
type PermissionMeta struct {
	Command string `json:"command"`
}

// PermissionToolRef identifies the tool call that triggered the permission.
type PermissionToolRef struct {
	MessageID string `json:"messageID"`
	CallID    string `json:"callID"`
}

// PermissionReply is the body sent to reply to a permission request.
type PermissionReply struct {
	Reply string `json:"reply"`
}

// ServerResolver resolves a ready OpenCode server's base URL for a given project.
type ServerResolver interface {
	GetBaseURL(ctx context.Context, projectID string) (string, error)
}

// ProjectPaths carries the minimal project fields needed to interact with an OpenCode server.
type ProjectPaths struct {
	ID   string
	Path string
}

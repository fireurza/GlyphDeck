package usage

import (
	"context"
	"errors"
	"testing"

	"glyphdeck/internal/opencode"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockResolver struct {
	client opencode.SessionClient
	err    error
}

func (m mockResolver) Resolve(_ context.Context, _ string) (opencode.SessionClient, string, error) {
	return m.client, "/test", m.err
}

type mockSessionClient struct {
	messages []opencode.Message
	err      error
}

func (m *mockSessionClient) ListSessions(_ context.Context, _ string) ([]opencode.Session, error) {
	return nil, nil
}
func (m *mockSessionClient) CreateSession(_ context.Context, _, _ string) (opencode.Session, error) {
	return opencode.Session{}, nil
}
func (m *mockSessionClient) GetSession(_ context.Context, _ string) (opencode.Session, error) {
	return opencode.Session{}, nil
}
func (m *mockSessionClient) ListMessages(_ context.Context, _ string) ([]opencode.Message, error) {
	return m.messages, m.err
}
func (m *mockSessionClient) SendPrompt(_ context.Context, _, _ string) (opencode.PromptResult, error) {
	return opencode.PromptResult{}, nil
}
func (m *mockSessionClient) StreamEvents(_ context.Context) (<-chan opencode.NormalizedEvent, <-chan error) {
	events := make(chan opencode.NormalizedEvent)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAggregate_ResolveError(t *testing.T) {
	r := mockResolver{err: errors.New("server not ready")}
	_, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAggregate_ListMessagesError(t *testing.T) {
	r := mockResolver{
		client: &mockSessionClient{err: errors.New("opencode: GET /session returned 500")},
	}
	_, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAggregate_EmptyMessages(t *testing.T) {
	r := mockResolver{
		client: &mockSessionClient{messages: []opencode.Message{}},
	}
	resp, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Available {
		t.Fatal("Available = true, want false for empty messages")
	}
	if resp.Reason == "" {
		t.Fatal("Reason is empty, want non-empty reason")
	}
	if resp.MessageCount != 0 {
		t.Fatalf("MessageCount = %d, want 0", resp.MessageCount)
	}
}

func TestAggregate_NoAssistantMessages(t *testing.T) {
	r := mockResolver{
		client: &mockSessionClient{
			messages: []opencode.Message{
				{Info: opencode.MessageInfo{ID: "m1", Role: "user"}},
			},
		},
	}
	resp, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Available {
		t.Fatal("Available = true, want false when no assistant messages")
	}
	if resp.Reason == "" {
		t.Fatal("Reason is empty, want non-empty reason")
	}
	if resp.MessageCount != 1 {
		t.Fatalf("MessageCount = %d, want 1", resp.MessageCount)
	}
}

func TestAggregate_LastAssistantWithTokens(t *testing.T) {
	r := mockResolver{
		client: &mockSessionClient{
			messages: []opencode.Message{
				{Info: opencode.MessageInfo{ID: "m1", Role: "user"}},
				{
					Info: opencode.MessageInfo{
						ID:         "m2",
						Role:       "assistant",
						ProviderID: "deepseek",
						ModelID:    "deepseek-v4-pro",
						Agent:      "build",
						Mode:       "build",
						Cost:       0.009337275,
						Tokens: opencode.TokenUsage{
							Total:     23329,
							Input:     21369,
							Output:    8,
							Reasoning: 32,
							Cache:     opencode.CacheUsage{Read: 1920, Write: 0},
						},
					},
				},
			},
		},
	}
	resp, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Available {
		t.Fatal("Available = false, want true when tokens present")
	}
	if resp.ProviderID != "deepseek" {
		t.Fatalf("ProviderID = %q, want deepseek", resp.ProviderID)
	}
	if resp.ModelID != "deepseek-v4-pro" {
		t.Fatalf("ModelID = %q, want deepseek-v4-pro", resp.ModelID)
	}
	if resp.Agent != "build" {
		t.Fatalf("Agent = %q, want build", resp.Agent)
	}
	if resp.Mode != "build" {
		t.Fatalf("Mode = %q, want build", resp.Mode)
	}
	if resp.Cost != 0.009337275 {
		t.Fatalf("Cost = %f, want 0.009337275", resp.Cost)
	}
	if resp.Tokens.Total != 23329 {
		t.Fatalf("Tokens.Total = %d, want 23329", resp.Tokens.Total)
	}
	if resp.Tokens.Input != 21369 {
		t.Fatalf("Tokens.Input = %d, want 21369", resp.Tokens.Input)
	}
	if resp.Tokens.Output != 8 {
		t.Fatalf("Tokens.Output = %d, want 8", resp.Tokens.Output)
	}
	if resp.Tokens.Reasoning != 32 {
		t.Fatalf("Tokens.Reasoning = %d, want 32", resp.Tokens.Reasoning)
	}
	if resp.Tokens.Cache.Read != 1920 {
		t.Fatalf("Tokens.Cache.Read = %d, want 1920", resp.Tokens.Cache.Read)
	}
	if resp.Tokens.Cache.Write != 0 {
		t.Fatalf("Tokens.Cache.Write = %d, want 0", resp.Tokens.Cache.Write)
	}
	if resp.MessageCount != 2 {
		t.Fatalf("MessageCount = %d, want 2", resp.MessageCount)
	}
}

func TestAggregate_SkipsAssistantWithoutTokens(t *testing.T) {
	r := mockResolver{
		client: &mockSessionClient{
			messages: []opencode.Message{
				{Info: opencode.MessageInfo{ID: "m1", Role: "user"}},
				{
					Info: opencode.MessageInfo{
						ID:         "m2",
						Role:       "assistant",
						ProviderID: "deepseek",
						ModelID:    "deepseek-v4-pro",
						// No tokens in this one.
					},
				},
				{Info: opencode.MessageInfo{ID: "m3", Role: "user"}},
				{
					Info: opencode.MessageInfo{
						ID:         "m4",
						Role:       "assistant",
						ProviderID: "deepseek",
						ModelID:    "deepseek-v4-pro",
						Cost:       0.001,
						Tokens: opencode.TokenUsage{
							Total:  100,
							Input:  80,
							Output: 20,
						},
					},
				},
			},
		},
	}
	resp, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Available {
		t.Fatal("Available = false, want true when tokens found in later message")
	}
	// Should pick m4 (last assistant with tokens).
	if resp.Tokens.Total != 100 {
		t.Fatalf("Tokens.Total = %d, want 100", resp.Tokens.Total)
	}
	if resp.MessageCount != 4 {
		t.Fatalf("MessageCount = %d, want 4", resp.MessageCount)
	}
}

func TestAggregate_AssistantWithoutTokens_Unavailable(t *testing.T) {
	r := mockResolver{
		client: &mockSessionClient{
			messages: []opencode.Message{
				{Info: opencode.MessageInfo{ID: "m1", Role: "user"}},
				{
					Info: opencode.MessageInfo{
						ID:         "m2",
						Role:       "assistant",
						ProviderID: "deepseek",
						ModelID:    "deepseek-v4-pro",
						// No tokens.
					},
				},
			},
		},
	}
	resp, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Available {
		t.Fatal("Available = true, want false when no token data")
	}
	if resp.Reason == "" {
		t.Fatal("Reason is empty, want non-empty reason")
	}
	if resp.ProviderID != "deepseek" {
		t.Fatalf("ProviderID = %q, want deepseek (metadata present)", resp.ProviderID)
	}
	if resp.ModelID != "deepseek-v4-pro" {
		t.Fatalf("ModelID = %q, want deepseek-v4-pro", resp.ModelID)
	}
	if resp.MessageCount != 2 {
		t.Fatalf("MessageCount = %d, want 2", resp.MessageCount)
	}
}

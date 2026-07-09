// Package usage aggregates token usage and cost data from OpenCode session messages.
package usage

import (
	"context"

	"glyphdeck/internal/opencode"
)

// UsageResponse is the JSON shape returned by the usage endpoint.
type UsageResponse struct {
	ProviderID   string              `json:"providerID"`
	ModelID      string              `json:"modelID"`
	Agent        string              `json:"agent"`
	Mode         string              `json:"mode"`
	Cost         float64             `json:"cost"`
	Tokens       opencode.TokenUsage `json:"tokens"`
	MessageCount int                 `json:"messageCount"`
	UpdatedAt    string              `json:"updatedAt,omitempty"`
}

// Resolver resolves a client and project path for a project ID.
type Resolver interface {
	Resolve(ctx context.Context, projectID string) (opencode.SessionClient, string, error)
}

// Aggregate computes usage from the latest assistant message in a session.
func Aggregate(ctx context.Context, resolver Resolver, projectID, sessionID string) (*UsageResponse, error) {
	client, _, err := resolver.Resolve(ctx, projectID)
	if err != nil {
		return nil, err
	}

	messages, err := client.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	resp := &UsageResponse{
		MessageCount: len(messages),
	}

	// Walk messages in reverse to find the last assistant message with usage data.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Info.Role != "assistant" {
			continue
		}
		resp.ProviderID = msg.Info.ProviderID
		resp.ModelID = msg.Info.ModelID
		resp.Agent = msg.Info.Agent
		resp.Mode = msg.Info.Mode
		resp.Cost = msg.Info.Cost
		resp.Tokens = msg.Info.Tokens

		// Return as soon as we have a non-empty token total.
		if resp.Tokens.Total > 0 {
			return resp, nil
		}

		// If this assistant message has provider/model but no tokens,
		// keep it but continue searching for token data.
		if resp.ProviderID != "" || resp.ModelID != "" {
			continue
		}
	}

	// Return what we found even without token data.
	// The caller can distinguish zero tokens from missing data.
	return resp, nil
}

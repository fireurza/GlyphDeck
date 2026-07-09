// Package usage aggregates token usage and cost data from OpenCode session messages.
package usage

import (
	"context"

	"glyphdeck/internal/opencode"
)

// UsageResponse is the JSON shape returned by the usage endpoint.
type UsageResponse struct {
	Available    bool                `json:"available"`
	Reason       string              `json:"reason,omitempty"`
	ProviderID   string              `json:"providerID"`
	ModelID      string              `json:"modelID"`
	Agent        string              `json:"agent"`
	Mode         string              `json:"mode"`
	Cost         float64             `json:"cost"`
	Tokens       opencode.TokenUsage `json:"tokens"`
	MessageCount int                 `json:"messageCount"`
	UpdatedAt    string              `json:"updatedAt,omitempty"`
}

// aggregateResult is an internal type for building the response.
type aggregateResult struct {
	providerID string
	modelID    string
	agent      string
	mode       string
	cost       float64
	tokens     opencode.TokenUsage
	hasData    bool
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

	ar := aggregateResult{}

	// Walk messages in reverse to find the last assistant message with usage data.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Info.Role != "assistant" {
			continue
		}
		ar.providerID = msg.Info.ProviderID
		ar.modelID = msg.Info.ModelID
		ar.agent = msg.Info.Agent
		ar.mode = msg.Info.Mode
		ar.cost = msg.Info.Cost
		ar.tokens = msg.Info.Tokens

		// Return as soon as we have a non-empty token total.
		if ar.tokens.Total > 0 {
			return &UsageResponse{
				Available:    true,
				ProviderID:   ar.providerID,
				ModelID:      ar.modelID,
				Agent:        ar.agent,
				Mode:         ar.mode,
				Cost:         ar.cost,
				Tokens:       ar.tokens,
				MessageCount: len(messages),
			}, nil
		}

		// If this assistant message has provider/model but no tokens,
		// keep it but continue searching for token data.
		if ar.providerID != "" || ar.modelID != "" {
			continue
		}
	}

	// No assistant message with token data found.
	// Return what we found even without token data.
	resp := &UsageResponse{
		MessageCount: len(messages),
	}
	if ar.providerID != "" || ar.modelID != "" {
		resp.Available = false
		resp.Reason = "OpenCode returned assistant metadata but no token usage data for this session yet."
		resp.ProviderID = ar.providerID
		resp.ModelID = ar.modelID
		resp.Agent = ar.agent
		resp.Mode = ar.mode
		resp.Cost = ar.cost
		resp.Tokens = ar.tokens
	} else {
		resp.Available = false
		resp.Reason = "OpenCode did not provide usage fields for this session yet."
	}
	return resp, nil
}

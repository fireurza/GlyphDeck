// Package permissions manages OpenCode permission approval requests.
package permissions

import (
	"context"
	"fmt"

	"glyphdeck/internal/opencode"
)

// Handler serves permission HTTP endpoints.
type Handler struct {
	resolver Resolver
}

// Resolver resolves an OpenCode client for a project.
type Resolver interface {
	ResolveClient(ctx context.Context, projectID string) (*opencode.Client, error)
}

// NewHandler creates a permissions HTTP handler.
func NewHandler(resolver Resolver) *Handler {
	return &Handler{resolver: resolver}
}

// ListPending returns pending permission requests for a project.
func (h *Handler) ListPending(ctx context.Context, projectID string) ([]opencode.PermissionRequest, error) {
	client, err := h.resolver.ResolveClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve client: %w", err)
	}
	return client.ListPermissions(ctx)
}

// Reply sends an approval/denial for a permission request.
func (h *Handler) Reply(ctx context.Context, projectID, requestID string, reply opencode.PermissionReply) error {
	client, err := h.resolver.ResolveClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("resolve client: %w", err)
	}
	return client.ReplyPermission(ctx, requestID, reply)
}

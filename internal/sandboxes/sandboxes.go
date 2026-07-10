// Package sandboxes manages OpenCode server targets and the active server selection.
package sandboxes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ServerType classifies how GlyphDeck reaches an OpenCode server.
type ServerType string

const (
	TypeLocal     ServerType = "local"
	TypeManualURL ServerType = "manual_url"
	TypeSSHAlias  ServerType = "ssh_alias"
)

// ServerConfig is a persisted OpenCode server target.
type ServerConfig struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      ServerType `json:"type"`
	URL       string     `json:"url"`
	SSHAlias  string     `json:"sshAlias"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
}

// ErrNotFound is returned when a server config is not found.
var ErrNotFound = errors.New("server config not found")

// Registry persists and retrieves OpenCode server configurations.
type Registry struct {
	db *sql.DB
}

// NewRegistry creates a Registry backed by the given database connection.
func NewRegistry(db *sql.DB) (*Registry, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	return &Registry{db: db}, nil
}

// Add inserts a new server configuration.
func (r *Registry) Add(ctx context.Context, cfg ServerConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO server_configs (id, name, type, url, ssh_alias, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cfg.ID, cfg.Name, string(cfg.Type), cfg.URL, cfg.SSHAlias, cfg.CreatedAt, cfg.UpdatedAt,
	)
	return err
}

// Update modifies an existing server configuration.
func (r *Registry) Update(ctx context.Context, cfg ServerConfig) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE server_configs SET name=?, type=?, url=?, ssh_alias=?, updated_at=? WHERE id=?`,
		cfg.Name, string(cfg.Type), cfg.URL, cfg.SSHAlias, time.Now().UTC().Format(time.RFC3339), cfg.ID,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get retrieves a single server configuration by ID.
func (r *Registry) Get(ctx context.Context, id string) (ServerConfig, error) {
	var cfg ServerConfig
	var typ string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, type, url, ssh_alias, created_at, updated_at
		 FROM server_configs WHERE id=?`, id,
	).Scan(&cfg.ID, &cfg.Name, &typ, &cfg.URL, &cfg.SSHAlias, &cfg.CreatedAt, &cfg.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServerConfig{}, ErrNotFound
	}
	if err != nil {
		return ServerConfig{}, fmt.Errorf("query server config: %w", err)
	}
	cfg.Type = ServerType(typ)
	return cfg, nil
}

// List returns all configured server targets.
func (r *Registry) List(ctx context.Context) ([]ServerConfig, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, url, ssh_alias, created_at, updated_at
		 FROM server_configs ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query server configs: %w", err)
	}
	defer rows.Close()

	var configs []ServerConfig
	for rows.Next() {
		var cfg ServerConfig
		var typ string
		if err := rows.Scan(&cfg.ID, &cfg.Name, &typ, &cfg.URL, &cfg.SSHAlias, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan server config: %w", err)
		}
		cfg.Type = ServerType(typ)
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// Delete removes a server configuration by ID.
func (r *Registry) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM server_configs WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

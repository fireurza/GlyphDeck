package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Store persists admin credentials and sessions in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by the given database connection.
func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate auth tables: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS auth_config (
		key TEXT PRIMARY KEY,
		value BLOB NOT NULL
	);
	CREATE TABLE IF NOT EXISTS auth_sessions (
		token TEXT PRIMARY KEY,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// HasAdmin returns true if an admin credential has been set up.
func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_config WHERE key='admin_hash'`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check admin: %w", err)
	}
	return count > 0, nil
}

// SetAdminHash stores the bcrypt hash for the admin password.
// Returns ErrAdminExists if an admin hash already exists.
func (s *Store) SetAdminHash(ctx context.Context, hash AdminHash) error {
	hasAdmin, err := s.HasAdmin(ctx)
	if err != nil {
		return err
	}
	if hasAdmin {
		return ErrAdminExists
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO auth_config (key, value) VALUES ('admin_hash', ?)`, []byte(hash),
	)
	return err
}

// GetAdminHash returns the stored admin password hash.
func (s *Store) GetAdminHash(ctx context.Context) (AdminHash, error) {
	var hash []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM auth_config WHERE key='admin_hash'`,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoAdmin
	}
	if err != nil {
		return nil, fmt.Errorf("get admin hash: %w", err)
	}
	return AdminHash(hash), nil
}

// CreateSession stores a session token and returns it.
func (s *Store) CreateSession(ctx context.Context) (string, error) {
	token, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO auth_sessions (token, created_at) VALUES (?, ?)`,
		token, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

// ValidateSession checks if a session token exists and is valid.
func (s *Store) ValidateSession(ctx context.Context, token string) error {
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT created_at FROM auth_sessions WHERE token=?`, token,
	).Scan(&createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("validate session: %w", err)
	}
	// Sessions are valid indefinitely (until deleted).
	// Future: add expiration if needed.
	return nil
}

// DeleteSession removes a session token (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token=?`, token)
	return err
}

package preferences

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const defaultMaxBackups = 20

// Store persists typed preferences in SQLite with revision tracking and backups.
type Store struct {
	db         *sql.DB
	maxBackups int
}

// NewStore creates a preferences store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, maxBackups: defaultMaxBackups}
}

// NewStoreWithOptions creates a store with custom backup retention.
func NewStoreWithOptions(db *sql.DB, maxBackups int) *Store {
	if maxBackups < 1 {
		maxBackups = 1
	}
	return &Store{db: db, maxBackups: maxBackups}
}

// MigrateSchema ensures preferences and backups tables exist.
func MigrateSchema(db *sql.DB) error {
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS preferences (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			data TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS preferences_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			revision INTEGER NOT NULL,
			data TEXT NOT NULL,
			summary TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_backups_created ON preferences_backups(created_at DESC)`,
	}
	for _, d := range ddl {
		if _, err := db.Exec(d); err != nil {
			return fmt.Errorf("preferences migration: %w", err)
		}
	}
	return nil
}

// Load retrieves the current preferences document.
// If no preferences exist, returns defaults with revision 0.
func (s *Store) Load() (*PrefsDocument, error) {
	var dataStr string
	var revision int
	var updatedAt string
	err := s.db.QueryRow("SELECT data, revision, updated_at FROM preferences WHERE id = 1").Scan(&dataStr, &revision, &updatedAt)
	if err == sql.ErrNoRows {
		def := Defaults()
		return &PrefsDocument{Data: def, Revision: 0}, nil
	}
	if err != nil {
		return nil, err
	}

	var prefs Prefs
	if err := json.Unmarshal([]byte(dataStr), &prefs); err != nil {
		return nil, fmt.Errorf("corrupt preferences: %w", err)
	}
	return &PrefsDocument{Data: prefs, Revision: revision, UpdatedAt: updatedAt}, nil
}

// Save atomically stores preferences with revision check, creates backup.
func (s *Store) Save(req UpdateRequest) (*PrefsDocument, error) {
	// Validate.
	if errs := req.Data.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("%w: %v", ErrValidationFailed, errs)
	}

	// Normalize.
	req.Data.Normalize()

	// Load current to check revision.
	current, err := s.Load()
	if err != nil {
		return nil, err
	}

	if current.Revision != req.ExpectedRevision {
		return nil, ErrRevisionConflict
	}

	// Compute diff for backup summary.
	diff := Diff(current.Data, req.Data)

	// Serialize.
	ser, err := json.Marshal(req.Data)
	if err != nil {
		return nil, fmt.Errorf("serialization error: %w", err)
	}

	newRev := current.Revision + 1
	now := timeStr()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Backup current state (skip if revision 0 — no existing data).
	if current.Revision > 0 {
		curSer, _ := json.Marshal(current.Data)
		curSummary, _ := json.Marshal(ChangeSummary{}) // backing up current, not changed.

		// If a change occurred, store the diff summary; if not, store empty summary.
		if len(diff.Fields) > 0 {
			curSummary, _ = json.Marshal(diff)
		}

		_, err = tx.Exec(
			"INSERT INTO preferences_backups (revision, data, summary, created_at) VALUES (?, ?, ?, ?)",
			current.Revision, string(curSer), string(curSummary), now,
		)
		if err != nil {
			return nil, fmt.Errorf("backup failed: %w", err)
		}
	}

	// Upsert preferences.
	_, err = tx.Exec(
		`INSERT INTO preferences (id, data, revision, updated_at) VALUES (1, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, revision = excluded.revision, updated_at = excluded.updated_at`,
		string(ser), newRev, now,
	)
	if err != nil {
		return nil, fmt.Errorf("save failed: %w", err)
	}

	// Prune old backups.
	if s.maxBackups > 0 {
		_, _ = tx.Exec(
			`DELETE FROM preferences_backups WHERE id NOT IN (
				SELECT id FROM preferences_backups ORDER BY created_at DESC LIMIT ?
			)`, s.maxBackups,
		)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &PrefsDocument{Data: req.Data, Revision: newRev, UpdatedAt: now}, nil
}

// ListBackups returns all stored backups ordered newest-first.
func (s *Store) ListBackups() ([]BackupEntry, error) {
	rows, err := s.db.Query(
		"SELECT id, revision, summary, created_at FROM preferences_backups ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BackupEntry
	for rows.Next() {
		var id, revision int
		var summaryJSON, createdAt string
		if err := rows.Scan(&id, &revision, &summaryJSON, &createdAt); err != nil {
			return nil, err
		}
		var cs ChangeSummary
		_ = json.Unmarshal([]byte(summaryJSON), &cs)
		out = append(out, BackupEntry{
			ID:        id,
			Revision:  revision,
			CreatedAt: createdAt,
			Summary:   cs,
		})
	}
	return out, rows.Err()
}

// Restore reverts preferences to a specific backup.
// The current state is backed up before restoration.
func (s *Store) Restore(backupID int) (*PrefsDocument, error) {
	var dataStr, createdAt string
	var revision int
	err := s.db.QueryRow(
		"SELECT data, revision, created_at FROM preferences_backups WHERE id = ?",
		backupID,
	).Scan(&dataStr, &revision, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var prefs Prefs
	if err := json.Unmarshal([]byte(dataStr), &prefs); err != nil {
		return nil, fmt.Errorf("corrupt backup: %w", err)
	}

	// Restore via Save to ensure backup-before-update and revision tracking.
	current, err := s.Load()
	if err != nil {
		return nil, err
	}

	req := UpdateRequest{
		Data:             prefs,
		ExpectedRevision: current.Revision,
	}
	return s.Save(req)
}

// dropAllTables is used only in tests. NOT exported to production API.
func dropAllTables(db *sql.DB) {
	for _, t := range []string{"preferences_backups", "preferences"} {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + t)
	}
}

func extErr(err error) error {
	if err == nil {
		return nil
	}
	// Wrap known sentinel errors.
	if err == ErrRevisionConflict || err == ErrValidationFailed || err == ErrNotFound {
		return err
	}
	// Do not expose SQLite internals in messages.
	if strings.Contains(err.Error(), "SQLITE") || strings.Contains(err.Error(), "sqlite") {
		return fmt.Errorf("storage error")
	}
	return err
}

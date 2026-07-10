// Package storage provides persistence for GlyphDeck app data.
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database handle.
type DB struct {
	conn *sql.DB
	path string
}

// DataDir returns the directory used for GlyphDeck-owned persistent data.
// GLYPHDECK_DATA_DIR is intended for controlled validation and isolated runs;
// normal application runs continue to use the repo-local .glyphdeck directory.
func DataDir() string {
	if path := os.Getenv("GLYPHDECK_DATA_DIR"); path != "" {
		return path
	}
	return ".glyphdeck"
}

// DefaultDBPath returns the default SQLite database path.
func DefaultDBPath() string {
	return filepath.Join(DataDir(), "glyphdeck.db")
}

// Open initializes the SQLite database, creating it if needed.
func Open(path string) (*DB, error) {
	if path == "" {
		path = DefaultDBPath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Verify connection.
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	db := &DB{conn: conn, path: path}

	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Close shuts down the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying *sql.DB for use by other packages.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// migrate applies the current schema if tables don't exist.
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		trusted INTEGER NOT NULL DEFAULT 0,
		tags_json TEXT NOT NULL DEFAULT '[]',
		git_is_repo INTEGER NOT NULL DEFAULT 0,
		git_branch TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS server_configs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL DEFAULT 'local',
		url TEXT NOT NULL DEFAULT '',
		ssh_alias TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// MigrateFromJSON imports projects from the legacy JSON file if the projects
// table is empty and the JSON file exists. Returns the number of projects imported.
func (db *DB) MigrateFromJSON(jsonPath string) (int, error) {
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		return 0, fmt.Errorf("check projects count: %w", err)
	}
	if count > 0 {
		return 0, nil
	}

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return 0, nil
	}

	data, err := loadJSONProjects(jsonPath)
	if err != nil {
		return 0, fmt.Errorf("load legacy json: %w", err)
	}
	if len(data) == 0 {
		return 0, nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	inserted := 0
	for _, p := range data {
		tagsStr := tagsToJSON(p.Tags)
		gitRepo := 0
		if p.Git.IsRepo {
			gitRepo = 1
		}
		trusted := 0
		if p.Trusted {
			trusted = 1
		}
		_, err := tx.Exec(
			`INSERT OR IGNORE INTO projects (id, name, path, trusted, tags_json, git_is_repo, git_branch)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.Path, trusted, tagsStr, gitRepo, p.Git.Branch,
		)
		if err != nil {
			return 0, fmt.Errorf("insert project %s: %w", p.ID, err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	backupPath := jsonPath + ".bak"
	_ = os.Rename(jsonPath, backupPath)

	return inserted, nil
}

type jsonProject struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Trusted bool     `json:"trusted"`
	Tags    []string `json:"tags"`
	Git     struct {
		IsRepo bool   `json:"isRepo"`
		Branch string `json:"branch"`
	} `json:"git"`
}

type jsonProjectFile struct {
	Projects []jsonProject `json:"projects"`
}

func loadJSONProjects(path string) ([]jsonProject, error) {
	var data jsonProjectFile
	if err := LoadJSON(path, &data); err != nil {
		return nil, err
	}
	return data.Projects, nil
}

func tagsToJSON(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	s := "["
	for i, t := range tags {
		if i > 0 {
			s += ","
		}
		s += `"` + t + `"`
	}
	s += "]"
	return s
}

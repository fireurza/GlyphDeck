// Package settings manages GlyphDeck app settings backed by SQLite.
package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
)

// Manager stores settings in SQLite via a key-value table.
type Manager struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewManager creates a settings manager backed by a SQLite connection.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Get returns a single setting value.
func (m *Manager) Get(key string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var value string
	err := m.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// Set stores a setting value. For new keys, it inserts; for existing, it updates.
func (m *Manager) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// GetAll returns all settings as a map.
func (m *Manager) GetAll() (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query("SELECT key, value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// MigrateSchema ensures the settings table exists.
func MigrateSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	return err
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

// RegisterHandlers mounts settings routes.
func RegisterHandlers(mux *http.ServeMux, mgr *Manager) {
	h := &handler{mgr: mgr}
	mux.HandleFunc("GET /api/settings", h.list)
	mux.HandleFunc("PUT /api/settings", h.bulkUpdate)
}

type handler struct {
	mgr *Manager
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	all, err := h.mgr.GetAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if all == nil {
		all = make(map[string]string)
	}
	writeJSON(w, http.StatusOK, all)
}

func (h *handler) bulkUpdate(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	for k, v := range updates {
		if k == "" {
			continue
		}
		if err := h.mgr.Set(k, v); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

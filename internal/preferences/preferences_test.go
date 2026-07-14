package preferences

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := MigrateSchema(db); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	return db
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.Appearance != "system" {
		t.Fatalf("appearance=%s", d.Appearance)
	}
	if d.TerminalFontSize != 14 {
		t.Fatalf("terminalFontSize=%d", d.TerminalFontSize)
	}
	if !d.TranscriptAutoScroll {
		t.Fatal("transcriptAutoScroll should be true")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		prefs   Prefs
		wantErr bool
		field   string
	}{
		{"valid defaults", Defaults(), false, ""},
		{"invalid appearance", Prefs{Appearance: "neon"}, true, "appearance"},
		{"invalid density", Prefs{InterfaceDensity: "sparse"}, true, "interfaceDensity"},
		{"font too small", Prefs{TerminalFontSize: 5}, true, "terminalFontSize"},
		{"font too large", Prefs{TerminalFontSize: 99}, true, "terminalFontSize"},
		{"valid font 11", Prefs{TerminalFontSize: 11}, false, ""},
		{"valid font 24", Prefs{TerminalFontSize: 24}, false, ""},
		{"invalid tab", Prefs{DefaultRightPanelTab: "projects"}, true, "defaultRightPanelTab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't normalize before validation — test raw validation.
			errs := tt.prefs.Validate()
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Fatalf("validate: hasErr=%v want=%v errs=%v", hasErr, tt.wantErr, errs)
			}
			if tt.field != "" {
				found := false
				for _, e := range errs {
					if e.Field == tt.field {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected error on field %s, got %v", tt.field, errs)
				}
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStore(db)
	doc, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Revision != 0 {
		t.Fatalf("revision=%d, want 0", doc.Revision)
	}
	if doc.Data.Appearance != "system" {
		t.Fatalf("appearance=%s", doc.Data.Appearance)
	}
}

func TestSaveAndLoad(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStore(db)

	prefs := Defaults()
	prefs.Appearance = "dark"
	prefs.TerminalFontSize = 18

	doc, err := s.Save(UpdateRequest{Data: prefs, ExpectedRevision: 0})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if doc.Revision != 1 {
		t.Fatalf("revision=%d, want 1", doc.Revision)
	}
	if doc.Data.Appearance != "dark" {
		t.Fatalf("appearance=%s", doc.Data.Appearance)
	}

	// Reload.
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Revision != 1 {
		t.Fatalf("loaded revision=%d", loaded.Revision)
	}
}

func TestRevisionConflict(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStore(db)
	_, _ = s.Save(UpdateRequest{Data: Defaults(), ExpectedRevision: 0})

	_, err := s.Save(UpdateRequest{Data: Defaults(), ExpectedRevision: 0}) // Stale
	if err != ErrRevisionConflict {
		t.Fatalf("expected conflict, got: %v", err)
	}
}

func TestBackupOnSave(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStoreWithOptions(db, 5)

	// Save revision 1.
	p1 := Defaults()
	p1.Appearance = "dark"
	_, _ = s.Save(UpdateRequest{Data: p1, ExpectedRevision: 0})

	// Save revision 2.
	p2 := Defaults()
	p2.Appearance = "light"
	_, _ = s.Save(UpdateRequest{Data: p2, ExpectedRevision: 1})

	backups, err := s.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	// Should have at least 1 backup (revision 1 backed up when saving revision 2).
	if len(backups) == 0 {
		t.Fatal("expected at least 1 backup")
	}
}

func TestBackupRetention(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStoreWithOptions(db, 3)

	p := Defaults()
	for i := 0; i < 10; i++ {
		p.TerminalFontSize = 11 + i
		_, err := s.Save(UpdateRequest{Data: p, ExpectedRevision: i})
		if err != nil {
			t.Fatalf("Save rev %d: %v", i, err)
		}
	}

	backups, err := s.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) > 3 {
		t.Fatalf("backup count %d exceeds limit 3", len(backups))
	}
}

func TestNoBackupInitialSave(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStore(db)
	_, _ = s.Save(UpdateRequest{Data: Defaults(), ExpectedRevision: 0})

	backups, _ := s.ListBackups()
	if len(backups) != 0 {
		t.Fatalf("expected 0 backups after initial save, got %d", len(backups))
	}
}

func TestRestore(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStore(db)

	// Save rev 1: dark.
	p1 := Defaults()
	p1.Appearance = "dark"
	_, _ = s.Save(UpdateRequest{Data: p1, ExpectedRevision: 0})

	// Save rev 2: light.
	p2 := Defaults()
	p2.Appearance = "light"
	_, _ = s.Save(UpdateRequest{Data: p2, ExpectedRevision: 1})

	backups, _ := s.ListBackups()
	if len(backups) == 0 {
		t.Fatal("no backups")
	}

	// Restore the first backup (rev 1, dark).
	restored, err := s.Restore(backups[0].ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.Data.Appearance != "dark" {
		t.Fatalf("restored appearance=%s, want dark", restored.Data.Appearance)
	}
}

func TestRestoreNotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	s := NewStore(db)
	_, err := s.Restore(999)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestParseJSONRejectUnknown(t *testing.T) {
	_, err := ParsePrefsJSON([]byte(`{"appearance":"dark","bogusField":true}`))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestParseYAMLRejectUnknown(t *testing.T) {
	_, err := ParsePrefsYAML([]byte("appearance: dark\nbogusField: true\n"))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestParseJSON(t *testing.T) {
	p, err := ParsePrefsJSON([]byte(`{"appearance":"dark","terminalFontSize":20}`))
	if err != nil {
		t.Fatalf("ParsePrefsJSON: %v", err)
	}
	if p.Appearance != "dark" || p.TerminalFontSize != 20 {
		t.Fatalf("unexpected values: %+v", p)
	}
}

func TestParseYAML(t *testing.T) {
	p, err := ParsePrefsYAML([]byte("appearance: dark\nterminalFontSize: 20\n"))
	if err != nil {
		t.Fatalf("ParsePrefsYAML: %v", err)
	}
	if p.Appearance != "dark" || p.TerminalFontSize != 20 {
		t.Fatalf("unexpected values: %+v", p)
	}
}

func TestDiff(t *testing.T) {
	old := Defaults()
	neu := Defaults()
	neu.Appearance = "dark"
	neu.TerminalFontSize = 20

	cs := Diff(old, neu)
	if len(cs.Fields) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(cs.Fields))
	}
}

func TestNormalize(t *testing.T) {
	p := Prefs{}
	p.Normalize()
	if p.Appearance != "system" {
		t.Fatalf("appearance not default")
	}
	if p.TerminalFontSize != 14 {
		t.Fatalf("font size not default")
	}
}

// --- HTTP handler tests ---

func newTestHandler(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()
	db := openTestDB(t)
	s := NewStore(db)
	return NewHandler(s), db
}

func setupTestMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings", h.getSettings)
	mux.HandleFunc("POST /api/settings/preview", h.preview)
	mux.HandleFunc("PUT /api/settings", h.update)
	mux.HandleFunc("GET /api/settings/backups", h.listBackups)
	mux.HandleFunc("POST /api/settings/backups/{id}/restore", h.restore)
	return mux
}

func TestHandlerGetDefaults(t *testing.T) {
	h, db := newTestHandler(t)
	defer db.Close()
	mux := setupTestMux(h)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/settings")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var doc PrefsDocument
	json.NewDecoder(resp.Body).Decode(&doc)
	if doc.Revision != 0 {
		t.Fatalf("revision=%d", doc.Revision)
	}
}

func TestHandlerPreview(t *testing.T) {
	h, db := newTestHandler(t)
	defer db.Close()
	mux := setupTestMux(h)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/api/settings/preview", "application/json",
		strings.NewReader(`{"appearance":"dark"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var pr PreviewResult
	json.NewDecoder(resp.Body).Decode(&pr)
	if len(pr.Changes.Fields) == 0 {
		t.Fatal("expected changes")
	}
}

func TestHandlerUpdateConflict(t *testing.T) {
	h, db := newTestHandler(t)
	defer db.Close()
	mux := setupTestMux(h)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Save rev 1 first.
	h.store.Save(UpdateRequest{Data: Defaults(), ExpectedRevision: 0})

	// Try with stale revision.
	resp, err := ts.Client().Post(ts.URL+"/api/settings", "application/json",
		strings.NewReader(`{"data":{"appearance":"dark"},"expectedRevision":0}`))
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	// Note: PUT is registered but we POST because Go mux matches method.
	// Use PUT directly.
	req, _ := http.NewRequest("PUT", ts.URL+"/api/settings", strings.NewReader(`{"data":{"appearance":"dark"},"expectedRevision":0}`))
	req.Header.Set("Content-Type", "application/json")
	resp2, _ := ts.Client().Do(req)
	defer resp2.Body.Close()

	if resp2.StatusCode != 409 {
		t.Fatalf("expected 409 conflict, got %d", resp2.StatusCode)
	}
}

func TestHandlerBackups(t *testing.T) {
	h, db := newTestHandler(t)
	defer db.Close()
	mux := setupTestMux(h)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	h.store.Save(UpdateRequest{Data: Defaults(), ExpectedRevision: 0})
	p := Defaults()
	p.Appearance = "dark"
	h.store.Save(UpdateRequest{Data: p, ExpectedRevision: 1})

	resp, err := ts.Client().Get(ts.URL + "/api/settings/backups")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var backups []BackupEntry
	json.NewDecoder(resp.Body).Decode(&backups)
	if len(backups) == 0 {
		t.Fatal("expected backups")
	}
}

func TestHandlerRestore(t *testing.T) {
	h, db := newTestHandler(t)
	defer db.Close()
	mux := setupTestMux(h)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	h.store.Save(UpdateRequest{Data: Defaults(), ExpectedRevision: 0})
	p := Defaults()
	p.Appearance = "dark"
	h.store.Save(UpdateRequest{Data: p, ExpectedRevision: 1})

	backups, _ := h.store.ListBackups()
	if len(backups) == 0 {
		t.Fatal("no backups to restore")
	}

	req, _ := http.NewRequest("POST", ts.URL+"/api/settings/backups/"+strconv.Itoa(backups[0].ID)+"/restore", nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
}

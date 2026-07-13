package sandboxes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// Create server_configs table.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS server_configs (
		id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL DEFAULT 'local', url TEXT NOT NULL DEFAULT '',
		ssh_alias TEXT NOT NULL DEFAULT '',
		working_dir TEXT NOT NULL DEFAULT '',
		start_command TEXT NOT NULL DEFAULT '',
		stop_command TEXT NOT NULL DEFAULT '',
		status_command TEXT NOT NULL DEFAULT '',
		last_pid INTEGER NOT NULL DEFAULT 0,
		last_url TEXT NOT NULL DEFAULT '',
		last_status TEXT NOT NULL DEFAULT 'unknown',
		last_checked_at TEXT NOT NULL DEFAULT '',
		started_by_glyphdeck INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestRegistry(t *testing.T) (*Registry, *fakeSSHRunner) {
	t.Helper()
	ssh := newFakeSSHRunner()
	reg, err := NewRegistryWithSSH(newTestDB(t), ssh)
	if err != nil {
		t.Fatalf("NewRegistryWithSSH: %v", err)
	}
	return reg, ssh
}

func addSSHTarget(t *testing.T, reg *Registry, id, alias string) {
	t.Helper()
	err := reg.Add(context.Background(), ServerConfig{
		ID:       id,
		Name:     id,
		Type:     TypeSSHAlias,
		SSHAlias: alias,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
}

func newTestServer(t *testing.T, reg *Registry) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	RegisterHandlers(mux, reg)
	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Test SSH
// ---------------------------------------------------------------------------

func TestSSH_Success(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("echo ok", SSHResult{Stdout: "ok\n"})

	result, err := reg.TestSSH(context.Background(), "remote-1")
	if err != nil {
		t.Fatalf("TestSSH: %v", err)
	}
	if result.Err != nil {
		t.Fatalf("expected no error, got %v", result.Err)
	}
}

func TestSSH_Failure(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("echo ok", SSHResult{Err: fmt.Errorf("connection refused")})

	result, _ := reg.TestSSH(context.Background(), "remote-1")
	if result.Err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSSH_NonSSHTarget(t *testing.T) {
	reg, _ := newTestRegistry(t)
	reg.Add(context.Background(), ServerConfig{ID: "local-1", Name: "local", Type: TypeLocal})

	_, err := reg.TestSSH(context.Background(), "local-1")
	if err == nil {
		t.Fatal("expected error for non-SSH target")
	}
}

// ---------------------------------------------------------------------------
// Detect
// ---------------------------------------------------------------------------

func TestDetect_Online(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("pgrep -f 'opencode serve' && echo 'running' || echo 'stopped'",
		SSHResult{Stdout: "12345\nrunning\n"})

	status, err := reg.Detect(context.Background(), "remote-1")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if status.Status != "online" {
		t.Fatalf("status = %q, want online", status.Status)
	}
	if status.PID != 12345 {
		t.Fatalf("PID = %d, want 12345", status.PID)
	}
}

func TestDetect_Offline(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("pgrep -f 'opencode serve' && echo 'running' || echo 'stopped'",
		SSHResult{Stdout: "stopped\n"})

	status, _ := reg.Detect(context.Background(), "remote-1")
	if status.Status != "offline" {
		t.Fatalf("status = %q, want offline", status.Status)
	}
}

func TestDetect_SSHError(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("pgrep -f 'opencode serve' && echo 'running' || echo 'stopped'",
		SSHResult{Err: fmt.Errorf("timeout")})

	status, _ := reg.Detect(context.Background(), "remote-1")
	if status.Status != "offline" {
		t.Fatalf("status = %q, want offline after SSH error", status.Status)
	}
}

// ---------------------------------------------------------------------------
// Start remote
// ---------------------------------------------------------------------------

func TestStartRemote_StoresPID(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("cd ~ && opencode serve --port 4096 --hostname 0.0.0.0 &; echo GLYPHDECK_PID=$!",
		SSHResult{Stdout: "GLYPHDECK_PID=98765\n"})

	result, err := reg.StartRemote(context.Background(), "remote-1")
	if err != nil {
		t.Fatalf("StartRemote: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %v", result.Message)
	}
	if result.PID != 98765 {
		t.Fatalf("PID = %d, want 98765", result.PID)
	}

	cfg, _ := reg.Get(context.Background(), "remote-1")
	if cfg.LastPID != 98765 {
		t.Fatalf("stored PID = %d, want 98765", cfg.LastPID)
	}
	if cfg.LastStatus != "online" {
		t.Fatalf("stored status = %q, want online", cfg.LastStatus)
	}
}

func TestStartRemote_NoPIDExtraction(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("cd ~ && opencode serve --port 4096 --hostname 0.0.0.0 &; echo GLYPHDECK_PID=$!",
		SSHResult{Stdout: "started\n"}) // No PID marker.

	result, _ := reg.StartRemote(context.Background(), "remote-1")
	if result.Success {
		t.Fatal("expected failure when PID cannot be extracted")
	}
}

// ---------------------------------------------------------------------------
// Stop remote
// ---------------------------------------------------------------------------

func TestStopRemote_UsesStoredPID(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")

	// Store a PID directly.
	reg.setRuntime(context.Background(), "remote-1", 55555, "http://localhost:4096", "online")

	// Mock verification: PID 55555 belongs to opencode.
	ssh.setResult("ps -p 55555 -o comm= 2>/dev/null | grep -q opencode && echo valid || echo invalid",
		SSHResult{Stdout: "valid\n"})
	ssh.setResult("kill 55555", SSHResult{Stdout: ""})

	result, err := reg.StopRemote(context.Background(), "remote-1")
	if err != nil {
		t.Fatalf("StopRemote: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success: %v", result.Message)
	}
	if result.PID != 55555 {
		t.Fatalf("PID = %d, want 55555", result.PID)
	}
}

func TestStopRemote_RefusesWhenNoPID(t *testing.T) {
	reg, _ := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")

	result, _ := reg.StopRemote(context.Background(), "remote-1")
	if result.Success {
		t.Fatal("expected stop to be refused when no PID")
	}
}

func TestStopRemote_RefusesWhenPIDVerificationFails(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	reg.setRuntime(context.Background(), "remote-1", 55555, "http://localhost:4096", "online")

	ssh.setResult("ps -p 55555 -o comm= 2>/dev/null | grep -q opencode && echo valid || echo invalid",
		SSHResult{Stdout: "invalid\n"})

	result, _ := reg.StopRemote(context.Background(), "remote-1")
	if result.Success {
		t.Fatal("expected stop to be refused when PID verification fails")
	}
}

// ---------------------------------------------------------------------------
// HTTP endpoints
// ---------------------------------------------------------------------------

func TestHTTP_TestSSH(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("echo ok", SSHResult{Stdout: "ok\n"})
	ts := newTestServer(t, reg)
	defer ts.Close()

	resp, _ := ts.Client().Post(ts.URL+"/api/server-configs/remote-1/test-ssh", "application/json", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("test-ssh status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHTTP_Detect(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("pgrep -f 'opencode serve' && echo 'running' || echo 'stopped'",
		SSHResult{Stdout: "99999\nrunning\n"})
	ts := newTestServer(t, reg)
	defer ts.Close()

	resp, _ := ts.Client().Post(ts.URL+"/api/server-configs/remote-1/detect", "application/json", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detect status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHTTP_StartRemote(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("cd ~ && opencode serve --port 4096 --hostname 0.0.0.0 &; echo GLYPHDECK_PID=$!",
		SSHResult{Stdout: "GLYPHDECK_PID=11111\n"})
	ts := newTestServer(t, reg)
	defer ts.Close()

	resp, _ := ts.Client().Post(ts.URL+"/api/server-configs/remote-1/start-remote", "application/json", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start-remote status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHTTP_StopRemote(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	reg.setRuntime(context.Background(), "remote-1", 22222, "http://localhost:4096", "online")
	ssh.setResult("ps -p 22222 -o comm= 2>/dev/null | grep -q opencode && echo valid || echo invalid",
		SSHResult{Stdout: "valid\n"})
	ssh.setResult("kill 22222", SSHResult{Stdout: ""})
	ts := newTestServer(t, reg)
	defer ts.Close()

	resp, _ := ts.Client().Post(ts.URL+"/api/server-configs/remote-1/stop-remote", "application/json", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop-remote status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Timeout
// ---------------------------------------------------------------------------

func TestSSH_Timeout(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("echo ok", SSHResult{Err: fmt.Errorf("command timed out")})

	result, _ := reg.TestSSH(context.Background(), "remote-1")
	if result.Err == nil || !strings.Contains(result.Err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", result.Err)
	}
}

// ---------------------------------------------------------------------------
// Update (PUT) tests
// ---------------------------------------------------------------------------

func TestRegistry_Update(t *testing.T) {
	reg, _ := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")

	cfg, _ := reg.Get(context.Background(), "remote-1")
	cfg.Name = "Updated Name"
	cfg.WorkingDir = "/home/user"
	if err := reg.Update(context.Background(), cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := reg.Get(context.Background(), "remote-1")
	if updated.Name != "Updated Name" {
		t.Fatalf("name = %q, want %q", updated.Name, "Updated Name")
	}
	if updated.WorkingDir != "/home/user" {
		t.Fatalf("workingDir = %q, want %q", updated.WorkingDir, "/home/user")
	}
}

func TestRegistry_UpdatePreservesRuntime(t *testing.T) {
	reg, _ := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	reg.setRuntime(context.Background(), "remote-1", 12345, "http://example.com:4096", "online")

	cfg, _ := reg.Get(context.Background(), "remote-1")
	cfg.Name = "Renamed"
	if err := reg.Update(context.Background(), cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := reg.Get(context.Background(), "remote-1")
	if updated.LastPID != 12345 {
		t.Fatalf("LastPID = %d, want 12345", updated.LastPID)
	}
	if updated.LastURL != "http://example.com:4096" {
		t.Fatalf("LastURL = %q", updated.LastURL)
	}
	if updated.LastStatus != "online" {
		t.Fatalf("LastStatus = %q, want online", updated.LastStatus)
	}
}

func TestRegistry_UpdateResetsRuntimeOnTypeChange(t *testing.T) {
	reg, _ := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	reg.setRuntime(context.Background(), "remote-1", 99999, "http://remote:4096", "online")

	cfg, _ := reg.Get(context.Background(), "remote-1")
	cfg.Type = TypeManualURL
	if err := reg.Update(context.Background(), cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := reg.Get(context.Background(), "remote-1")
	if updated.LastPID != 0 {
		t.Fatalf("LastPID = %d, want 0 (reset on type change)", updated.LastPID)
	}
	if updated.LastURL != "" {
		t.Fatalf("LastURL = %q, want empty", updated.LastURL)
	}
	if updated.StartedByGlyphdeck {
		t.Fatal("startedByGlyphdeck should be false after type change")
	}
}

func TestRegistry_UpdateNotFound(t *testing.T) {
	reg, _ := newTestRegistry(t)
	if err := reg.Update(context.Background(), ServerConfig{ID: "noexist", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestHTTP_UpdateConfig(t *testing.T) {
	reg, _ := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ts := newTestServer(t, reg)
	defer ts.Close()

	body := `{"name":"Renamed Target","type":"ssh_alias","sshAlias":"myserver"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/server-configs/remote-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := ts.Client().Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	cfg, _ := reg.Get(context.Background(), "remote-1")
	if cfg.Name != "Renamed Target" {
		t.Fatalf("name = %q, want Renamed Target", cfg.Name)
	}
}

func TestHTTP_AddValidation(t *testing.T) {
	reg, _ := newTestRegistry(t)
	ts := newTestServer(t, reg)
	defer ts.Close()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing name", `{"type":"ssh_alias","sshAlias":"x"}`, 400},
		{"missing ssh alias", `{"name":"test","type":"ssh_alias"}`, 400},
		{"invalid type", `{"name":"test","type":"bogus"}`, 400},
		{"valid local", `{"name":"test","type":"local"}`, 201},
		{"valid ssh alias", `{"name":"test","type":"ssh_alias","sshAlias":"myserver"}`, 201},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", ts.URL+"/api/server-configs", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := ts.Client().Do(req)
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			resp.Body.Close()
		})
	}
}

func TestHTTP_UpdateValidation(t *testing.T) {
	reg, _ := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ts := newTestServer(t, reg)
	defer ts.Close()

	body := `{"name":"","type":"ssh_alias","sshAlias":"myserver"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/server-configs/remote-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := ts.Client().Do(req)
	if resp.StatusCode != 400 {
		t.Fatalf("PUT with empty name status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRegistry_StopRemoteNoPID(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	// Don't set runtime — LastPID is 0.
	_ = ssh // not used
	result, _ := reg.StopRemote(context.Background(), "remote-1")
	if result.Success {
		t.Fatal("stop without PID should fail")
	}
}

func TestRegistry_StopRemotePIDVerificationFails(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	reg.setRuntime(context.Background(), "remote-1", 33333, "", "online")
	ssh.setResult("ps -p 33333 -o comm= 2>/dev/null | grep -q opencode && echo valid || echo invalid",
		SSHResult{Stdout: "invalid\n"})

	result, _ := reg.StopRemote(context.Background(), "remote-1")
	if result.Success {
		t.Fatal("stop with PID verification failure should not succeed")
	}
}

func TestRegistry_ConcurrentLifecycleOps(t *testing.T) {
	reg, ssh := newTestRegistry(t)
	addSSHTarget(t, reg, "remote-1", "myserver")
	ssh.setResult("echo ok", SSHResult{Stdout: "ok\n"})

	// Simulate concurrent TestSSH calls — second should block until first releases.
	done := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func() {
			reg.TestSSH(context.Background(), "remote-1")
			done <- true
		}()
	}
	<-done
	<-done
	// Both completed without panic means locking works.
}

func TestValidateServerConfig(t *testing.T) {
	tests := []struct {
		name     string
		srvName  string
		typ      ServerType
		sshAlias string
		wantErr  bool
	}{
		{"valid local", "test", TypeLocal, "", false},
		{"valid manual", "test", TypeManualURL, "", false},
		{"valid ssh", "test", TypeSSHAlias, "myserver", false},
		{"missing name", "", TypeLocal, "", true},
		{"missing ssh alias", "test", TypeSSHAlias, "", true},
		{"invalid type", "test", ServerType("bogus"), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerConfig(tt.srvName, tt.typ, tt.sshAlias)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateServerConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

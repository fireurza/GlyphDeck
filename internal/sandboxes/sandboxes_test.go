package sandboxes

import (
	"context"
	"database/sql"
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

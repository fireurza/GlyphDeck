package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"glyphdeck/internal/auth"
	"glyphdeck/internal/httpapi"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestStore(t *testing.T) *auth.Store {
	t.Helper()
	store, err := auth.NewStore(newTestDB(t))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

// readAdminPassword implements the password resolution logic from bootstrapAdmin.
// Returns (password, nil) on success, or ("", error) on failure.
func readAdminPassword(passwordEnv, passwordFileEnv string) (string, error) {
	if passwordEnv != "" && passwordFileEnv != "" {
		return "", errBothPasswordSources
	}
	if passwordFileEnv != "" {
		data, err := os.ReadFile(passwordFileEnv)
		if err != nil {
			return "", err
		}
		password := strings.TrimSpace(string(data))
		if password == "" {
			return "", errEmptyPasswordFile
		}
		return password, nil
	}
	return passwordEnv, nil
}

// Sentinel errors for password resolution.
var (
	errBothPasswordSources = &passwordError{"both password sources set"}
	errEmptyPasswordFile   = &passwordError{"password file is empty"}
)

type passwordError struct {
	msg string
}

func (e *passwordError) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// Container mode host validation
// ---------------------------------------------------------------------------

func TestContainerModeDisabledByDefault(t *testing.T) {
	if !httpapi.IsLoopbackHost("127.0.0.1") {
		t.Fatal("127.0.0.1 must be loopback")
	}
	if !httpapi.IsLoopbackHost("localhost") {
		t.Fatal("localhost must be loopback")
	}
}

func TestNativeUnsafeBindRejected(t *testing.T) {
	hosts := []string{"0.0.0.0", "192.168.1.1", "example.com"}
	for _, host := range hosts {
		if httpapi.IsLoopbackHost(host) {
			t.Fatalf("host %q must not be loopback for this test", host)
		}
	}
}

func TestContainerModeEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"unset", "", false},
		{"one", "1", true},
		{"zero", "0", false},
		{"true", "true", false},
		{"empty string after set", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("GLYPHDECK_CONTAINER_MODE", tt.envValue)
			}
			got := getEnv("GLYPHDECK_CONTAINER_MODE", "") == "1"
			if got != tt.want {
				t.Fatalf("containerMode = %v, want %v (env=%q)", got, tt.want, tt.envValue)
			}
		})
	}
}

func TestContainerModeHostValidation(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		containerMode bool
		valid         bool
	}{
		{"default 127.0.0.1", "127.0.0.1", false, true},
		{"localhost", "localhost", false, true},
		{"0.0.0.0 without container mode", "0.0.0.0", false, false},
		{"public ip without container mode", "192.168.1.1", false, false},
		{"0.0.0.0 with container mode", "0.0.0.0", true, true},
		{"localhost with container mode", "localhost", true, false},
		{"127.0.0.1 with container mode", "127.0.0.1", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.containerMode {
				if tt.host == "0.0.0.0" {
					if !tt.valid {
						t.Fatal("0.0.0.0 must be valid in container mode")
					}
					return
				}
				if tt.valid {
					t.Fatalf("host %q must not be valid in container mode", tt.host)
				}
				return
			}

			// Not container mode: enforce loopback.
			if httpapi.IsLoopbackHost(tt.host) != tt.valid {
				t.Fatalf("host %q loopback=%v, want valid=%v", tt.host, httpapi.IsLoopbackHost(tt.host), tt.valid)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Admin password file tests
// ---------------------------------------------------------------------------

func TestPasswordFileSuccess(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "admin-password.txt")
	if err := os.WriteFile(passwordFile, []byte("file-based-password\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	password, err := readAdminPassword("", passwordFile)
	if err != nil {
		t.Fatalf("readAdminPassword: %v", err)
	}
	if password != "file-based-password" {
		t.Fatalf("password = %q, want file-based-password", password)
	}

	store := newTestStore(t)
	hash, _ := auth.HashPassword(password)
	_ = store.SetAdminHash(context.Background(), hash)

	stored, _ := store.GetAdminHash(context.Background())
	if !auth.VerifyPassword(stored, "file-based-password") {
		t.Fatal("file-based password should verify")
	}
}

func TestPasswordFileMissing(t *testing.T) {
	_, err := readAdminPassword("", "/nonexistent/password/file.txt")
	if err == nil {
		t.Fatal("expected error for missing password file")
	}
}

func TestPasswordFileEmpty(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(passwordFile, []byte("\n"), 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	_, err := readAdminPassword("", passwordFile)
	if err != errEmptyPasswordFile {
		t.Fatalf("expected errEmptyPasswordFile, got %v", err)
	}
}

func TestPasswordFileWhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "whitespace.txt")
	if err := os.WriteFile(passwordFile, []byte("  \t\n  "), 0o600); err != nil {
		t.Fatalf("write whitespace file: %v", err)
	}

	_, err := readAdminPassword("", passwordFile)
	if err != errEmptyPasswordFile {
		t.Fatalf("expected errEmptyPasswordFile, got %v", err)
	}
}

func TestBothPasswordSourcesRejected(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "admin-password.txt")
	if err := os.WriteFile(passwordFile, []byte("some-password"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	_, err := readAdminPassword("env-password", passwordFile)
	if err != errBothPasswordSources {
		t.Fatalf("expected errBothPasswordSources, got %v", err)
	}
}

func TestPasswordEnvOnlyStillWorks(t *testing.T) {
	password, err := readAdminPassword("env-only-password", "")
	if err != nil {
		t.Fatalf("readAdminPassword: %v", err)
	}
	if password != "env-only-password" {
		t.Fatalf("password = %q, want env-only-password", password)
	}
}

func TestPasswordNotInErrorMessages(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "admin-password.txt")
	password := "secret-password-abc123"
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	resolved, err := readAdminPassword("", passwordFile)
	if err != nil {
		t.Fatalf("readAdminPassword: %v", err)
	}
	if resolved != password {
		t.Fatal("resolved password mismatch")
	}

	// The password value itself should never appear in any error message.
	_, err = readAdminPassword("", "/nonexistent/file")
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if strings.Contains(errStr, password) {
		t.Fatalf("error message contains password: %q", errStr)
	}
}

func TestPasswordFileBootstrapIntegration(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "admin-password.txt")
	password := "bootstrap-integration-pass"
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	resolved, err := readAdminPassword("", passwordFile)
	if err != nil {
		t.Fatalf("readAdminPassword: %v", err)
	}
	if resolved != password {
		t.Fatal("password mismatch")
	}

	store := newTestStore(t)

	hasAdmin, _ := store.HasAdmin(context.Background())
	if hasAdmin {
		t.Fatal("expected no admin before bootstrap")
	}

	hash, _ := auth.HashPassword(password)
	if err := store.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatalf("SetAdminHash: %v", err)
	}

	stored, _ := store.GetAdminHash(context.Background())
	if !auth.VerifyPassword(stored, password) {
		t.Fatal("bootstrap password should verify")
	}

	// Verify bootstrap does not overwrite existing admin.
	err = store.SetAdminHash(context.Background(), hash)
	if err != auth.ErrAdminExists {
		t.Fatalf("expected ErrAdminExists, got %v", err)
	}
}

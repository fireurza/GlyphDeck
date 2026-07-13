package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func newTestScanner(t *testing.T, root string) *Scanner {
	t.Helper()
	s, err := NewScanner(root)
	if err != nil {
		t.Fatalf("NewScanner: %v", err)
	}
	return s
}

// ---------------------------------------------------------------------------
// Missing global config
// ---------------------------------------------------------------------------

func TestScanGlobal_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if !inv.Available {
		t.Fatal("inventory should be available even with missing config")
	}
	if len(inv.Sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(inv.Sources))
	}
}

// ---------------------------------------------------------------------------
// Global config discovery
// ---------------------------------------------------------------------------

func TestScanGlobal_JSONCConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		"shell": "pwsh",
		"model": "deepseek/deepseek-v4-pro"
	}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if !inv.Available {
		t.Fatal("expected available")
	}
	if len(inv.Sources) == 0 {
		t.Fatal("expected at least 1 source")
	}
	if len(inv.ShellProfiles) == 0 {
		t.Fatal("expected shell profile from config")
	}
	if inv.ShellProfiles[0].Name != "pwsh" {
		t.Fatalf("expected pwsh shell, got %s", inv.ShellProfiles[0].Name)
	}
}

func TestScanGlobal_JSONConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.json"), `{"shell": "bash"}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.ShellProfiles) == 0 || inv.ShellProfiles[0].Name != "bash" {
		t.Fatal("expected bash shell from .json config")
	}
}

func TestScanGlobal_AgentsDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "agents", "test-agent.md"), "# Test Agent")
	writeFile(t, filepath.Join(dir, "agents", "builder.md"), "# Builder Agent")
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.Agents) < 2 {
		t.Fatalf("expected 2 agents from directory, got %d", len(inv.Agents))
	}
	names := make(map[string]bool)
	for _, a := range inv.Agents {
		names[a.Name] = true
	}
	if !names["test-agent"] || !names["builder"] {
		t.Fatalf("missing agent names: %v", names)
	}
}

func TestScanGlobal_SkillsDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skills", "test-skill", "SKILL.md"), "# Test Skill\nDescription here.")
	writeFile(t, filepath.Join(dir, "skills", "builder", "SKILL.md"), "# Builder Skill")
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.Skills) < 2 {
		t.Fatalf("expected 2 skills, got %d", len(inv.Skills))
	}
}

func TestScanGlobal_PluginsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "plugins", "test-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	found := false
	for _, p := range inv.Plugins {
		if p.ID == "test-plugin" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected test-plugin from plugins directory")
	}
}

// ---------------------------------------------------------------------------
// Project config (trusted)
// ---------------------------------------------------------------------------

func TestScanProject_Trusted(t *testing.T) {
	dir := t.TempDir()
	s := newTestScanner(t, dir)

	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, ".opencode", "opencode.json"), `{"shell": "zsh"}`)

	inv := s.ScanGlobal()
	s.ScanProject(inv, projDir)

	hasProjectSource := false
	for _, src := range inv.Sources {
		if src.Scope == "project" {
			hasProjectSource = true
		}
	}
	if !hasProjectSource {
		t.Fatal("expected project source in inventory")
	}

	hasProjectShell := false
	for _, sh := range inv.ShellProfiles {
		if sh.Scope == "project" && sh.Name == "zsh" {
			hasProjectShell = true
		}
	}
	if !hasProjectShell {
		t.Fatal("expected zsh shell from project config")
	}
}

// ---------------------------------------------------------------------------
// Malformed and oversized files
// ---------------------------------------------------------------------------

func TestScanGlobal_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.json"), `{not valid json!!!`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.Warnings) == 0 {
		t.Fatal("expected parse warning for malformed JSON")
	}
	if !strings.Contains(inv.Warnings[0].Message, "parse error") {
		t.Fatalf("expected parse error warning, got: %s", inv.Warnings[0].Message)
	}
}

func TestScanGlobal_OversizedFile(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than maxConfigFileSize.
	bigContent := strings.Repeat("x", maxConfigFileSize+1)
	writeFile(t, filepath.Join(dir, "opencode.json"), bigContent)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.Warnings) == 0 {
		t.Fatal("expected warning for oversized file")
	}
	if !strings.Contains(inv.Warnings[0].Message, "too large") {
		t.Fatalf("expected size warning, got: %s", inv.Warnings[0].Message)
	}
}

// ---------------------------------------------------------------------------
// Traversal rejection
// ---------------------------------------------------------------------------

func TestScanGlobal_TraversalRejected(t *testing.T) {
	dir := t.TempDir()
	s := newTestScanner(t, dir)

	// Create a file outside the config root entirely.
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")
	writeFile(t, outsidePath, "secret")

	_, err := s.safeReadFile(outsidePath)
	if err == nil {
		t.Fatal("expected traversal rejection for file outside config root")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("expected traversal error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Symlink escape rejection
// ---------------------------------------------------------------------------

func TestScanGlobal_SymlinkEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	_ = newTestScanner(t, dir) // scanner for containment

	outsidePath := filepath.Join(t.TempDir(), "escaped.txt")
	writeFile(t, outsidePath, "should-not-read")

	symlinkPath := filepath.Join(dir, "escape-link")
	if err := os.Symlink(outsidePath, symlinkPath); err != nil {
		t.Skip("symlink not supported on this platform")
	}

	contained, err := isContained(symlinkPath, dir)
	if err != nil || contained {
		t.Fatalf("symlink outside root should not be contained: err=%v, contained=%v", err, contained)
	}
}

// ---------------------------------------------------------------------------
// Stable ordering
// ---------------------------------------------------------------------------

func TestScanGlobal_StableOrdering(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "agents", "c.md"), "# C")
	writeFile(t, filepath.Join(dir, "agents", "a.md"), "# A")
	writeFile(t, filepath.Join(dir, "agents", "b.md"), "# B")
	s := newTestScanner(t, dir)

	inv1 := s.ScanGlobal()
	inv2 := s.ScanGlobal()

	if len(inv1.Agents) != len(inv2.Agents) {
		t.Fatal("agent count differs between scans")
	}
	for i := range inv1.Agents {
		if inv1.Agents[i].Name != inv2.Agents[i].Name {
			t.Fatalf("ordering differs: %s vs %s at index %d", inv1.Agents[i].Name, inv2.Agents[i].Name, i)
		}
	}
}

// ---------------------------------------------------------------------------
// Precedence (global vs project)
// ---------------------------------------------------------------------------

func TestScanProject_Precedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.json"), `{"shell": "bash"}`)
	s := newTestScanner(t, dir)

	projDir := t.TempDir()
	writeFile(t, filepath.Join(projDir, ".opencode", "opencode.json"), `{"shell": "pwsh"}`)

	inv := s.ScanGlobal()
	s.ScanProject(inv, projDir)

	globalShell := false
	projectShell := false
	for _, sh := range inv.ShellProfiles {
		if sh.Scope == "global" && sh.Name == "bash" {
			globalShell = true
		}
		if sh.Scope == "project" && sh.Name == "pwsh" {
			projectShell = true
		}
	}
	if !globalShell || !projectShell {
		t.Fatalf("expected both global and project shells: global=%v project=%v", globalShell, projectShell)
	}
}

// ---------------------------------------------------------------------------
// Sensitive-key redaction
// ---------------------------------------------------------------------------

func TestScanGlobal_SensitiveKeyRedaction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		"mcp": {
			"test-server": {
				"type": "remote",
				"url": "https://mcp.example.com",
				"headers": {
					"API_KEY": "secret-abc-123"
				},
				"env": {
					"SECRET": "value"
				}
			}
		}
	}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.MCPServers) == 0 {
		t.Fatal("expected MCP server entry")
	}
	entry := inv.MCPServers[0]
	if entry.Name != "test-server" {
		t.Fatalf("expected test-server, got %s", entry.Name)
	}
	// URL should be sanitized (no credentials).
	if strings.Contains(entry.URL, "secret") {
		t.Fatal("URL should not contain secret")
	}
	// Command should not contain redacted headers env.
	if strings.Contains(entry.Command, "API_KEY") || strings.Contains(entry.Command, "SECRET") {
		t.Fatal("command should not contain sensitive keys")
	}
}

func TestScanGlobal_CredentialURL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		"mcp": {
			"cred-server": {
				"type": "remote",
				"url": "https://user:pass123@mcp.example.com/v1"
			}
		}
	}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.MCPServers) == 0 {
		t.Fatal("expected MCP server")
	}
	url := inv.MCPServers[0].URL
	if strings.Contains(url, "pass123") {
		t.Fatalf("URL should be redacted, got: %s", url)
	}
	if !strings.Contains(url, "<redacted>") {
		t.Fatalf("URL should contain redacted marker, got: %s", url)
	}
}

func TestScanGlobal_CommandSanitization(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		"mcp": {
			"cmd-server": {
				"type": "local",
				"command": "server --api-key abc123 --token xyz --port 8080"
			}
		}
	}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.MCPServers) == 0 {
		t.Fatal("expected MCP server")
	}
	cmd := inv.MCPServers[0].Command
	if strings.Contains(cmd, "abc123") || strings.Contains(cmd, "xyz") {
		t.Fatalf("command should be sanitized, got: %s", cmd)
	}
	if !strings.Contains(cmd, "<redacted>") {
		t.Fatalf("command should contain redacted marker, got: %s", cmd)
	}
}

// ---------------------------------------------------------------------------
// Environment references not resolved
// ---------------------------------------------------------------------------

func TestScanGlobal_EnvNotResolved(t *testing.T) {
	dir := t.TempDir()
	// Set an env var and verify it does NOT appear in the output.
	os.Setenv("GLYPHDECK_TEST_ENV", "should-not-appear")
	defer os.Unsetenv("GLYPHDECK_TEST_ENV")

	writeFile(t, filepath.Join(dir, "opencode.json"), `{"shell": "bash"}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	// Serialize to JSON and verify the env value is not present.
	data, _ := json.Marshal(inv)
	if strings.Contains(string(data), "should-not-appear") {
		t.Fatal("env variable value should not appear in inventory")
	}
}

// ---------------------------------------------------------------------------
// Inventory categories populated
// ---------------------------------------------------------------------------

func TestScanGlobal_AllCategoriesPopulated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		"shell": "pwsh",
		"agent": {
			"test-agent": {
				"description": "Test agent",
				"mode": "primary"
			}
		},
		"provider": {
			"test-provider": {
				"name": "Test Provider",
				"baseUrl": "https://api.test.com",
				"models": {
					"test-model": {
						"name": "Test Model"
					}
				}
			}
		},
		"mcp": {
			"test-mcp": {
				"type": "remote",
				"url": "https://mcp.test.com"
			}
		},
		"plugin": [
			"test-plugin"
		]
	}`)
	s := newTestScanner(t, dir)
	inv := s.ScanGlobal()

	if len(inv.Agents) == 0 {
		t.Fatal("expected agents")
	}
	if len(inv.Providers) == 0 {
		t.Fatal("expected providers")
	}
	if len(inv.Models) == 0 {
		t.Fatal("expected models")
	}
	if len(inv.MCPServers) == 0 {
		t.Fatal("expected MCP servers")
	}
	if len(inv.ShellProfiles) == 0 {
		t.Fatal("expected shell profiles")
	}
}

// ---------------------------------------------------------------------------
// Handler tests
// ---------------------------------------------------------------------------

func TestHandler_InventorySuccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.json"), `{"shell":"bash"}`)
	s := newTestScanner(t, dir)

	mux := http.NewServeMux()
	// Register without project registry (just global).
	h := NewHandler(s, nil)
	mux.HandleFunc("GET /api/opencode/config/inventory", h.handleInventory)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/opencode/config/inventory")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var inv Inventory
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !inv.Available {
		t.Fatal("expected available")
	}
	if len(inv.ShellProfiles) == 0 {
		t.Fatal("expected shell profile")
	}
}

// ---------------------------------------------------------------------------
// Directory symlink escape
// ---------------------------------------------------------------------------

func TestScanGlobal_DirSymlinkEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	s := newTestScanner(t, dir)

	outsideDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outsideDir, "escape-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	symlinkPath := filepath.Join(dir, "skills", "escape-link")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, "escape-dir"), symlinkPath); err != nil {
		t.Skip("symlink not supported")
	}

	inv := s.ScanGlobal()
	// The symlinked directory should not be followed.
	for _, sk := range inv.Skills {
		if strings.Contains(sk.SourceFile, "escape") {
			t.Fatal("symlink escape directory should not be included")
		}
	}
}

// ---------------------------------------------------------------------------
// JSONC parsing with comments
// ---------------------------------------------------------------------------

func TestScanGlobal_JSONCComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		// This is a comment
		"shell": "pwsh" /* inline comment */
	}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.ShellProfiles) == 0 || inv.ShellProfiles[0].Name != "pwsh" {
		t.Fatal("JSONC comments should be stripped")
	}
}

func TestScanGlobal_JSONCBlockComment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "opencode.jsonc"), `{
		/*
		 * Multi-line block comment
		 */
		"shell": "bash"
	}`)
	s := newTestScanner(t, dir)

	inv := s.ScanGlobal()
	if len(inv.ShellProfiles) == 0 || inv.ShellProfiles[0].Name != "bash" {
		t.Fatal("block comments should be stripped")
	}
}

// ---------------------------------------------------------------------------
// sanitizeURL edge cases
// ---------------------------------------------------------------------------

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		input    string
		contains string
		absent   string
	}{
		{"", "", ""},
		{"https://example.com", "example.com", "redacted"},
		{"https://user:pass@example.com", "<redacted>", "pass"},
		{"https://example.com/path?query=1", "example.com", "redacted"},
	}

	for _, tt := range tests {
		result := sanitizeURL(tt.input)
		if tt.contains != "" && !strings.Contains(result, tt.contains) {
			t.Errorf("sanitizeURL(%q) = %q, want contains %q", tt.input, result, tt.contains)
		}
		if tt.absent != "" && strings.Contains(result, tt.absent) {
			t.Errorf("sanitizeURL(%q) = %q, must not contain %q", tt.input, result, tt.absent)
		}
	}
}

// ---------------------------------------------------------------------------
// stripCredentialFields
// ---------------------------------------------------------------------------

func TestStripCredentialFields(t *testing.T) {
	input := map[string]any{
		"type": "remote",
		"url":  "https://example.com",
		"headers": map[string]any{
			"API_KEY": "secret-value",
		},
		"apiKey": "another-secret",
	}

	result := stripCredentialFields(input)
	if result["type"] != "remote" {
		t.Fatal("type should be preserved")
	}
	if result["apiKey"] != "<redacted>" {
		t.Fatal("apiKey should be redacted")
	}
	headers, ok := result["headers"].(map[string]any)
	if !ok {
		t.Fatal("headers should be a map")
	}
	if headers["API_KEY"] != "<redacted>" {
		t.Fatal("API_KEY in headers should be redacted")
	}
}

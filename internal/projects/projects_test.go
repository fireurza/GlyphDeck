package projects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRegistryAddValidProject(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	projectDir := t.TempDir()

	project, err := registry.Add(context.Background(), AddRequest{
		Name:    "GlyphDeck",
		Path:    projectDir,
		Trusted: true,
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if project.ID != "glyphdeck" {
		t.Fatalf("ID = %q, want glyphdeck", project.ID)
	}
	if project.Name != "GlyphDeck" {
		t.Fatalf("Name = %q, want GlyphDeck", project.Name)
	}
	if project.Path != filepath.Clean(projectDir) {
		t.Fatalf("Path = %q, want %q", project.Path, filepath.Clean(projectDir))
	}
	if !project.Trusted {
		t.Fatal("Trusted = false, want true")
	}
	if project.Tags == nil || len(project.Tags) != 0 {
		t.Fatalf("Tags = %#v, want empty slice", project.Tags)
	}
	if project.Git.IsRepo || project.Git.Branch != "" {
		t.Fatalf("Git = %#v, want non-repo", project.Git)
	}
}

func TestRegistryRejectsMissingPath(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()

	_, err := registry.Add(context.Background(), AddRequest{Name: "Missing"})
	if !errors.Is(err, ErrMissingPath) {
		t.Fatalf("Add error = %v, want ErrMissingPath", err)
	}
}

func TestRegistryRejectsNonDirectoryPath(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	filePath := filepath.Join(t.TempDir(), "project.txt")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := registry.Add(context.Background(), AddRequest{Name: "File", Path: filePath})
	if !errors.Is(err, ErrPathNotDirectory) {
		t.Fatalf("Add error = %v, want ErrPathNotDirectory", err)
	}
}

func TestRegistryRejectsDuplicatePath(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	projectDir := t.TempDir()

	if _, err := registry.Add(context.Background(), AddRequest{Name: "First", Path: projectDir}); err != nil {
		t.Fatalf("first Add returned error: %v", err)
	}

	duplicatePath := filepath.Join(projectDir, ".")
	_, err := registry.Add(context.Background(), AddRequest{Name: "Second", Path: duplicatePath})
	if !errors.Is(err, ErrDuplicatePath) {
		t.Fatalf("Add error = %v, want ErrDuplicatePath", err)
	}
}

func TestRegistryRejectsUnsupportedWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path safety check")
	}
	registry := newTestRegistry(t)
	defer registry.Close()

	for _, networkPath := range []string{
		`\\server\share\project`,
		`//server/share/project`,
		`\/server/share/project`,
		`/\\server/share/project`,
	} {
		_, err := registry.Add(context.Background(), AddRequest{Name: "Network", Path: networkPath})
		if !errors.Is(err, ErrUnsupportedPath) {
			t.Fatalf("Add(%q) error = %v, want ErrUnsupportedPath", networkPath, err)
		}
	}
}

func TestLegacyStoragePathUsesDataDirOverride(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "glyphdeck-data")
	t.Setenv("GLYPHDECK_DATA_DIR", dataDir)

	if got, want := LegacyStoragePath(), filepath.Join(dataDir, "projects.json"); got != want {
		t.Fatalf("LegacyStoragePath() = %q, want %q", got, want)
	}
}

func TestRegistryDetectsGitBranch(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	projectDir := t.TempDir()
	gitDir := filepath.Join(projectDir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	project, err := registry.Add(context.Background(), AddRequest{Name: "Repo", Path: projectDir})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if !project.Git.IsRepo {
		t.Fatal("Git.IsRepo = false, want true")
	}
	if project.Git.Branch != "main" {
		t.Fatalf("Git.Branch = %q, want main", project.Git.Branch)
	}
}

func TestRegistryPersistsAndReloadsProjects(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "glyphdeck.db")
	registry, err := NewRegistryFromPath(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryFromPath returned error: %v", err)
	}
	projectDir := t.TempDir()

	created, err := registry.Add(context.Background(), AddRequest{Name: "Persisted", Path: projectDir, Trusted: true})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	registry.Close()

	// Reload from same DB file.
	reloaded, err := NewRegistryFromPath(dbPath)
	if err != nil {
		t.Fatalf("reload NewRegistryFromPath returned error: %v", err)
	}
	defer reloaded.Close()

	projects, err := reloaded.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1", len(projects))
	}
	if projects[0].ID != created.ID || projects[0].Path != created.Path || projects[0].Trusted != created.Trusted {
		t.Fatalf("reloaded project = %#v, want %#v", projects[0], created)
	}
}

func TestRegistryRemovesProject(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	projectDir := t.TempDir()
	project, err := registry.Add(context.Background(), AddRequest{Name: "Remove Me", Path: projectDir})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if err := registry.Remove(context.Background(), project.ID); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	_, err = registry.Get(context.Background(), project.ID)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Get error = %v, want ErrProjectNotFound", err)
	}
}

func TestRegistryGeneratesUniqueIDs(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	firstDir := t.TempDir()
	secondDir := t.TempDir()

	first, err := registry.Add(context.Background(), AddRequest{Name: "GlyphDeck", Path: firstDir})
	if err != nil {
		t.Fatalf("first Add returned error: %v", err)
	}
	second, err := registry.Add(context.Background(), AddRequest{Name: "GlyphDeck", Path: secondDir})
	if err != nil {
		t.Fatalf("second Add returned error: %v", err)
	}

	if first.ID != "glyphdeck" {
		t.Fatalf("first ID = %q, want glyphdeck", first.ID)
	}
	if second.ID != "glyphdeck-2" {
		t.Fatalf("second ID = %q, want glyphdeck-2", second.ID)
	}
}

func TestProjectHTTPRejectsMissingJSONContentType(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	mux := http.NewServeMux()
	RegisterHandlers(mux, registry)
	body := strings.NewReader(`{"name":"Test","path":"` + t.TempDir() + `","trusted":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/projects", body)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnsupportedMediaType)
	}
}

func TestProjectHTTPRejectsCrossOriginMutation(t *testing.T) {
	registry := newTestRegistry(t)
	defer registry.Close()
	mux := http.NewServeMux()
	RegisterHandlers(mux, registry)
	body := strings.NewReader(`{"name":"Test","path":"` + t.TempDir() + `","trusted":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/projects", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestSameOriginMutation(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{
			name: "empty origin",
			host: "127.0.0.1:8756",
			want: true,
		},
		{
			name:   "same origin",
			host:   "127.0.0.1:8756",
			origin: "http://127.0.0.1:8756",
			want:   true,
		},
		{
			name:   "vite loopback proxy",
			host:   "127.0.0.1:8756",
			origin: "http://127.0.0.1:5173",
			want:   true,
		},
		{
			name:   "external origin",
			host:   "127.0.0.1:8756",
			origin: "http://evil.example",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://"+tt.host+"/api/projects", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := sameOriginMutation(req)
			if got != tt.want {
				t.Fatalf("sameOriginMutation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := NewRegistryFromPath(filepath.Join(t.TempDir(), "glyphdeck.db"))
	if err != nil {
		t.Fatalf("NewRegistryFromPath returned error: %v", err)
	}
	return registry
}

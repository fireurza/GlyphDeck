// Package projects manages the local project registry backed by SQLite.
package projects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"glyphdeck/internal/storage"
)

var (
	ErrMissingPath      = errors.New("missing project path")
	ErrPathNotFound     = errors.New("project path not found")
	ErrPathNotDirectory = errors.New("project path is not a directory")
	ErrDuplicatePath    = errors.New("project path is already registered")
	ErrProjectNotFound  = errors.New("project not found")
	ErrUnsupportedPath  = errors.New("project path is not supported")
)

type GitInfo struct {
	IsRepo bool   `json:"isRepo"`
	Branch string `json:"branch"`
}

type Project struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Trusted   bool     `json:"trusted"`
	Tags      []string `json:"tags"`
	Git       GitInfo  `json:"git"`
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

type AddRequest struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Trusted bool   `json:"trusted"`
}

// Registry manages projects with SQLite persistence.
type Registry struct {
	mu   sync.Mutex
	db   *sql.DB
	path string // physical DB file path (for info only)
}

// NewRegistry creates a Registry backed by a SQLite database handle.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{db: db}
}

// NewRegistryFromPath opens a SQLite database at the given path and creates a Registry.
// Use this for testing or standalone operation.
func NewRegistryFromPath(dbPath string) (*Registry, error) {
	sdb, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open registry db: %w", err)
	}
	return &Registry{db: sdb.Conn(), path: dbPath}, nil
}

// Close shuts down the underlying database.
func (r *Registry) Close() error {
	return r.db.Close()
}

func DefaultStoragePath() string {
	return filepath.Join(".glyphdeck", "glyphdeck.db")
}

// LegacyStoragePath returns the path to the old JSON projects file.
func LegacyStoragePath() string {
	return filepath.Join(".glyphdeck", "projects.json")
}

// List returns all registered projects.
func (r *Registry) List(ctx context.Context) ([]Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, path, trusted, tags_json, git_is_repo, git_branch, created_at, updated_at
		 FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []Project{}
	}
	return projects, rows.Err()
}

// Add registers a new project.
func (r *Registry) Add(ctx context.Context, req AddRequest) (Project, error) {
	if err := ctx.Err(); err != nil {
		return Project{}, err
	}

	normalizedPath, err := normalizePath(req.Path)
	if err != nil {
		return Project{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = filepath.Base(normalizedPath)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check duplicate path.
	normalizedKey := pathKey(normalizedPath)
	var existingCount int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM projects WHERE path = ?", normalizedPath).Scan(&existingCount); err != nil {
		return Project{}, fmt.Errorf("check duplicate: %w", err)
	}
	// Also check case-insensitive.
	all, err := r.listLocked(ctx)
	if err != nil {
		return Project{}, err
	}
	for _, p := range all {
		if pathKey(p.Path) == normalizedKey {
			return Project{}, ErrDuplicatePath
		}
	}

	id := nextID(name, normalizedPath, all)
	git := DetectGit(normalizedPath)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, path, trusted, tags_json, git_is_repo, git_branch, created_at, updated_at)
		 VALUES (?, ?, ?, ?, '[]', ?, ?, ?, ?)`,
		id, name, normalizedPath, boolToInt(req.Trusted), boolToInt(git.IsRepo), git.Branch, now, now)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}

	return Project{
		ID:        id,
		Name:      name,
		Path:      normalizedPath,
		Trusted:   req.Trusted,
		Tags:      []string{},
		Git:       git,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Get returns a single project by ID.
func (r *Registry) Get(ctx context.Context, id string) (Project, error) {
	if err := ctx.Err(); err != nil {
		return Project{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, path, trusted, tags_json, git_is_repo, git_branch, created_at, updated_at
		 FROM projects WHERE id = ?`, id)

	return scanProject(row)
}

// Remove deletes a project by ID.
func (r *Registry) Remove(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// listLocked returns all projects without locking (caller holds mu).
func (r *Registry) listLocked(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, path, trusted, tags_json, git_is_repo, git_branch, created_at, updated_at
		 FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// scanProject reads a single project from a row scanner.
func scanProject(row interface{ Scan(...interface{}) error }) (Project, error) {
	var (
		id, name, path, tagsJSON, gitBranch, createdAt, updatedAt string
		trusted, gitIsRepo                                         int
	)
	err := row.Scan(&id, &name, &path, &trusted, &tagsJSON, &gitIsRepo, &gitBranch, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrProjectNotFound
		}
		return Project{}, fmt.Errorf("scan project: %w", err)
	}

	tags := parseTags(tagsJSON)
	return Project{
		ID:        id,
		Name:      name,
		Path:      path,
		Trusted:   trusted != 0,
		Tags:      tags,
		Git:       GitInfo{IsRepo: gitIsRepo != 0, Branch: gitBranch},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func parseTags(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return []string{}
	}
	// Simple parser for ["tag1","tag2"].
	s := strings.TrimPrefix(jsonStr, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func DetectGit(projectPath string) GitInfo {
	headPath := filepath.Join(projectPath, ".git", "HEAD")
	content, err := os.ReadFile(headPath)
	if errors.Is(err, os.ErrNotExist) {
		return GitInfo{IsRepo: false, Branch: ""}
	}
	if err != nil {
		return GitInfo{IsRepo: true, Branch: ""}
	}

	head := strings.TrimSpace(string(content))
	branch := ""
	if strings.HasPrefix(head, "ref: refs/heads/") {
		branch = strings.TrimPrefix(head, "ref: refs/heads/")
	}

	return GitInfo{IsRepo: true, Branch: branch}
}

// normalizePath validates and normalizes a project directory path.
func normalizePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrMissingPath
	}

	evaluated, err := filepath.EvalSymlinks(trimmed)
	if err != nil {
		return "", ErrPathNotFound
	}

	abs, err := filepath.Abs(evaluated)
	if err != nil {
		return "", ErrUnsupportedPath
	}

	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrPathNotFound
		}
		return "", ErrUnsupportedPath
	}
	if !info.IsDir() {
		return "", ErrPathNotDirectory
	}

	return abs, nil
}

// nextID generates a stable project ID from name and path.
func nextID(name, path string, existing []Project) string {
	slug := slugify(name)
	if slug == "" {
		slug = slugify(filepath.Base(path))
	}
	if slug == "" {
		slug = "project"
	}

	base := slug
	n := 1
	for {
		id := base
		if n > 1 {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		if !idExists(id, existing) {
			return id
		}
		n++
	}
}

func idExists(id string, existing []Project) bool {
	for _, p := range existing {
		if p.ID == id {
			return true
		}
	}
	return false
}

func slugify(name string) string {
	name = strings.ToLower(name)
	var builder strings.Builder
	lastHyphen := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func pathKey(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
}

// httpHandler is the HTTP handler for project routes.
type httpHandler struct {
	registry *Registry
}

// RegisterHandlers mounts project routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, registry *Registry) {
	handler := httpHandler{registry: registry}
	mux.HandleFunc("GET /api/projects", handler.list)
	mux.HandleFunc("POST /api/projects", handler.add)
	mux.HandleFunc("GET /api/projects/{projectId}", handler.get)
	mux.HandleFunc("DELETE /api/projects/{projectId}", handler.remove)
}

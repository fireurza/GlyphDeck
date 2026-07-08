// Package projects manages the local project registry.
package projects

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Trusted bool     `json:"trusted"`
	Tags    []string `json:"tags"`
	Git     GitInfo  `json:"git"`
}

type AddRequest struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Trusted bool   `json:"trusted"`
}

type Registry struct {
	mu          sync.Mutex
	storagePath string
	projects    []Project
}

type registryFile struct {
	Projects []Project `json:"projects"`
}

func DefaultStoragePath() string {
	return filepath.Join(".glyphdeck", "projects.json")
}

func NewRegistry(storagePath string) (*Registry, error) {
	if storagePath == "" {
		storagePath = DefaultStoragePath()
	}

	var data registryFile
	if err := storage.LoadJSON(storagePath, &data); err != nil {
		return nil, fmt.Errorf("load project registry: %w", err)
	}

	for i := range data.Projects {
		if data.Projects[i].Tags == nil {
			data.Projects[i].Tags = []string{}
		}
	}

	return &Registry{storagePath: storagePath, projects: data.Projects}, nil
}

func (r *Registry) List(ctx context.Context) ([]Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	return cloneProjects(r.projects), nil
}

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

	normalizedKey := pathKey(normalizedPath)
	for _, project := range r.projects {
		if pathKey(project.Path) == normalizedKey {
			return Project{}, ErrDuplicatePath
		}
	}

	project := Project{
		ID:      nextID(name, normalizedPath, r.projects),
		Name:    name,
		Path:    normalizedPath,
		Trusted: req.Trusted,
		Tags:    []string{},
		Git:     DetectGit(normalizedPath),
	}

	r.projects = append(r.projects, project)
	if err := r.saveLocked(); err != nil {
		r.projects = r.projects[:len(r.projects)-1]
		return Project{}, err
	}

	return project, nil
}

func (r *Registry) Get(ctx context.Context, id string) (Project, error) {
	if err := ctx.Err(); err != nil {
		return Project{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, project := range r.projects {
		if project.ID == id {
			return cloneProject(project), nil
		}
	}

	return Project{}, ErrProjectNotFound
}

func (r *Registry) Remove(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i, project := range r.projects {
		if project.ID == id {
			removed := project
			r.projects = append(r.projects[:i], r.projects[i+1:]...)
			if err := r.saveLocked(); err != nil {
				r.projects = append(r.projects, Project{})
				copy(r.projects[i+1:], r.projects[i:])
				r.projects[i] = removed
				return err
			}
			return nil
		}
	}

	return ErrProjectNotFound
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

func (r *Registry) saveLocked() error {
	data := registryFile{Projects: r.projects}
	if err := storage.SaveJSON(r.storagePath, data); err != nil {
		return fmt.Errorf("save project registry: %w", err)
	}
	return nil
}

func normalizePath(value string) (string, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		return "", ErrMissingPath
	}
	if isUnsafeWindowsPath(path) {
		return "", ErrUnsupportedPath
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("normalize project path: %w", err)
	}
	cleanPath := filepath.Clean(absPath)

	info, err := os.Stat(cleanPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrPathNotFound
	}
	if err != nil {
		return "", fmt.Errorf("stat project path: %w", err)
	}
	if !info.IsDir() {
		return "", ErrPathNotDirectory
	}

	return cleanPath, nil
}

func isUnsafeWindowsPath(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`)
}

func pathKey(projectPath string) string {
	key := filepath.Clean(projectPath)
	if runtime.GOOS == "windows" {
		return strings.ToLower(key)
	}
	return key
}

func nextID(name, projectPath string, projects []Project) string {
	base := slugify(name)
	if base == "" {
		base = slugify(filepath.Base(projectPath))
	}
	if base == "" {
		base = "project"
	}

	used := map[string]struct{}{}
	for _, project := range projects {
		used[project.ID] = struct{}{}
	}

	if _, ok := used[base]; !ok {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func slugify(value string) string {
	var builder strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(value) {
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

func cloneProjects(projects []Project) []Project {
	cloned := make([]Project, len(projects))
	for i, project := range projects {
		cloned[i] = cloneProject(project)
	}
	return cloned
}

func cloneProject(project Project) Project {
	if project.Tags == nil {
		project.Tags = []string{}
		return project
	}
	project.Tags = append([]string{}, project.Tags...)
	return project
}

type httpHandler struct {
	registry *Registry
}

func RegisterHandlers(mux *http.ServeMux, registry *Registry) {
	handler := httpHandler{registry: registry}
	mux.HandleFunc("GET /api/projects", handler.list)
	mux.HandleFunc("POST /api/projects", handler.add)
	mux.HandleFunc("GET /api/projects/{projectId}", handler.get)
	mux.HandleFunc("DELETE /api/projects/{projectId}", handler.remove)
}

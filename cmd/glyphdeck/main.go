package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"glyphdeck/internal/devtools"
	"glyphdeck/internal/events"
	"glyphdeck/internal/opencode"
	"glyphdeck/internal/permissions"
	"glyphdeck/internal/projects"
	"glyphdeck/internal/review"
	"glyphdeck/internal/servers"
	"glyphdeck/internal/sessions"
	"glyphdeck/internal/terminal"
	"glyphdeck/internal/usage"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	host := getEnv("GLYPHDECK_HOST", "127.0.0.1")
	port := getEnv("GLYPHDECK_PORT", "8756")
	if !isLoopbackHost(host) {
		log.Fatalf("server host must be loopback-only; set GLYPHDECK_HOST to 127.0.0.1 or localhost")
	}
	addr := net.JoinHostPort(host, port)
	registry, err := projects.NewRegistry(projects.DefaultStoragePath())
	if err != nil {
		log.Fatalf("project registry error: %v", err)
	}

	// OpenCode detection and server manager.
	detector := opencode.NewDetector()
	adapter := &projectResolverAdapter{registry: registry}
	manager := servers.NewManager(detector, adapter)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	projects.RegisterHandlers(mux, registry)
	servers.RegisterHandlers(mux, manager)

	// Sessions.
	sessionProjectAdapter := &sessionProjectResolverAdapter{registry: registry}
	sessionsMgr := sessions.NewManager(manager, sessionProjectAdapter)
	sessions.RegisterHandlers(mux, sessionsMgr)

	// Usage.
	usageProjectAdapter := &usageProjectResolverAdapter{registry: registry}
	usageMgr := usage.NewManager(manager, usageProjectAdapter)
	usage.RegisterHandlers(mux, usageMgr)

	// Review.
	reviewProjectAdapter := &reviewProjectResolverAdapter{registry: registry}
	reviewMgr := review.NewManager(manager, reviewProjectAdapter)
	review.RegisterHandlers(mux, reviewMgr)

	// Permissions.
	permissionsProjectAdapter := &permissionsProjectResolverAdapter{registry: registry}
	permissionsMgr := permissions.NewManager(manager, permissionsProjectAdapter)
	permissions.RegisterHandlers(mux, permissionsMgr)

	// User Terminal.
	terminalProjectAdapter := &terminalProjectResolverAdapter{registry: registry}
	terminalMgr := terminal.NewManager(terminalProjectAdapter)
	terminal.RegisterHandlers(mux, terminalMgr)
	defer terminalMgr.CloseAll()

	// Events hub — bridges OpenCode SSE to browser clients.
	eventsHub := events.NewHub()
	manager.SetEventBridgeManager(eventsHub)
	mux.HandleFunc("GET /api/events", eventsHub.ServeHTTP)

	// Dev tools — only registered when GLYPHDECK_DEV_TOOLS=1.
	devtools.RegisterHandlers(mux, registry, manager)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine.
	go func() {
		log.Printf("GlyphDeck server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("GlyphDeck server shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}
	log.Println("GlyphDeck server stopped")
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// projectResolverAdapter adapts the projects.Registry to servers.ProjectResolver.
type projectResolverAdapter struct {
	registry *projects.Registry
}

func (a *projectResolverAdapter) Get(ctx context.Context, id string) (servers.ProjectInfo, error) {
	project, err := a.registry.Get(ctx, id)
	if errors.Is(err, projects.ErrProjectNotFound) {
		return servers.ProjectInfo{}, servers.ErrProjectNotFound
	}
	if err != nil {
		return servers.ProjectInfo{}, err
	}
	return servers.ProjectInfo{ID: project.ID, Name: project.Name, Path: project.Path}, nil
}

// sessionProjectResolverAdapter adapts the projects.Registry to sessions.ProjectResolver.
type sessionProjectResolverAdapter struct {
	registry *projects.Registry
}

func (a *sessionProjectResolverAdapter) Get(ctx context.Context, id string) (sessions.ProjectInfo, error) {
	project, err := a.registry.Get(ctx, id)
	if errors.Is(err, projects.ErrProjectNotFound) {
		return sessions.ProjectInfo{}, sessions.ErrProjectNotFound
	}
	if err != nil {
		return sessions.ProjectInfo{}, err
	}
	return sessions.ProjectInfo{ID: project.ID, Path: project.Path}, nil
}

// usageProjectResolverAdapter adapts the projects.Registry to usage.ProjectResolver.
type usageProjectResolverAdapter struct {
	registry *projects.Registry
}

func (a *usageProjectResolverAdapter) Get(ctx context.Context, id string) (usage.ProjectInfo, error) {
	project, err := a.registry.Get(ctx, id)
	if errors.Is(err, projects.ErrProjectNotFound) {
		return usage.ProjectInfo{}, usage.ErrProjectNotFound
	}
	if err != nil {
		return usage.ProjectInfo{}, err
	}
	return usage.ProjectInfo{ID: project.ID, Path: project.Path}, nil
}

// reviewProjectResolverAdapter adapts the projects.Registry to review.ProjectResolver.
type reviewProjectResolverAdapter struct {
	registry *projects.Registry
}

func (a *reviewProjectResolverAdapter) Get(ctx context.Context, id string) (*projects.Project, error) {
	project, err := a.registry.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// permissionsProjectResolverAdapter adapts the projects.Registry to permissions.ProjectResolver.
type permissionsProjectResolverAdapter struct {
	registry *projects.Registry
}

func (a *permissionsProjectResolverAdapter) Get(ctx context.Context, id string) (permissions.ProjectInfo, error) {
	project, err := a.registry.Get(ctx, id)
	if errors.Is(err, projects.ErrProjectNotFound) {
		return permissions.ProjectInfo{}, fmt.Errorf("project not found")
	}
	if err != nil {
		return permissions.ProjectInfo{}, err
	}
	return permissions.ProjectInfo{ID: project.ID, Path: project.Path}, nil
}

// terminalProjectResolverAdapter adapts the projects.Registry to terminal.ProjectResolver.
type terminalProjectResolverAdapter struct {
	registry *projects.Registry
}

func (a *terminalProjectResolverAdapter) GetPath(ctx context.Context, id string) (string, error) {
	project, err := a.registry.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return project.Path, nil
}

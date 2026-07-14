package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"glyphdeck/internal/auth"
	"glyphdeck/internal/devtools"
	"glyphdeck/internal/events"
	"glyphdeck/internal/httpapi"
	"glyphdeck/internal/opencode"
	"glyphdeck/internal/opencode/config"
	"glyphdeck/internal/permissions"
	"glyphdeck/internal/preferences"
	"glyphdeck/internal/problems"
	"glyphdeck/internal/projects"
	"glyphdeck/internal/review"
	"glyphdeck/internal/sandboxes"
	"glyphdeck/internal/servers"
	"glyphdeck/internal/sessions"
	"glyphdeck/internal/storage"
	"glyphdeck/internal/terminal"
	"glyphdeck/internal/usage"
	"glyphdeck/web"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	host := getEnv("GLYPHDECK_HOST", "127.0.0.1")
	port := getEnv("GLYPHDECK_PORT", "8756")
	containerMode := getEnv("GLYPHDECK_CONTAINER_MODE", "") == "1"

	if containerMode {
		// Container mode allows binding to 0.0.0.0 so the container port
		// is reachable from the Docker network. The host publication is
		// still restricted to 127.0.0.1 by the compose port mapping.
		if host != "0.0.0.0" {
			log.Fatalf("container mode requires GLYPHDECK_HOST=0.0.0.0")
		}
	} else {
		if !httpapi.IsLoopbackHost(host) {
			log.Fatalf("server host must be loopback-only; set GLYPHDECK_HOST to 127.0.0.1 or localhost")
		}
	}
	addr := net.JoinHostPort(host, port)

	// Open SQLite database.
	db, err := storage.Open(storage.DefaultDBPath())
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer db.Close()

	// Migrate legacy JSON projects if present.
	jsonPath := projects.LegacyStoragePath()
	if imported, err := db.MigrateFromJSON(jsonPath); err != nil {
		log.Printf("migration warning: %v", err)
	} else if imported > 0 {
		log.Printf("migrated %d projects from %s to SQLite", imported, jsonPath)
	}

	// Create project registry backed by SQLite.
	registry := projects.NewRegistry(db.Conn())
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

	// Server/sandbox configuration registry.
	sandboxReg, err := sandboxes.NewRegistry(db.Conn())
	if err != nil {
		log.Fatalf("sandbox registry error: %v", err)
	}
	sandboxes.RegisterHandlers(mux, sandboxReg)

	// Auth store — credentials and sessions.
	authStore, err := auth.NewStore(db.Conn())
	if err != nil {
		log.Fatalf("auth store error: %v", err)
	}
	auth.RegisterHandlers(mux, authStore)

	// Bootstrap admin from environment if no admin exists.
	bootstrapAdmin(authStore)

	// Sessions.
	sessionsProjectAdapter := &projectPathsResolverAdapter{registry: registry, notFoundErr: sessions.ErrProjectNotFound}
	sessionsMgr := sessions.NewManager(manager, sessionsProjectAdapter)
	sessions.RegisterHandlers(mux, sessionsMgr)

	// Usage.
	usageProjectAdapter := &projectPathsResolverAdapter{registry: registry, notFoundErr: usage.ErrProjectNotFound}
	usageMgr := usage.NewManager(manager, usageProjectAdapter)
	usage.RegisterHandlers(mux, usageMgr)

	// Review.
	reviewProjectAdapter := &reviewProjectResolverAdapter{registry: registry}
	reviewMgr := review.NewManager(manager, reviewProjectAdapter)
	review.RegisterHandlers(mux, reviewMgr)

	// Permissions.
	permissionsProjectAdapter := &projectPathsResolverAdapter{registry: registry, notFoundErr: fmt.Errorf("project not found")}
	permissionsMgr := permissions.NewManager(manager, permissionsProjectAdapter)
	permissions.RegisterHandlers(mux, permissionsMgr)

	// User Terminal.
	terminalProjectAdapter := &terminalProjectResolverAdapter{registry: registry}
	terminalMgr := terminal.NewManager(terminalProjectAdapter)
	terminal.RegisterHandlers(mux, terminalMgr)
	defer func() {
		if err := terminalMgr.CloseAll(); err != nil {
			log.Printf("shutdown: error closing terminals: %v", err)
		}
	}()

	// Problems.
	problemsMgr := problems.NewManager(100)
	problems.RegisterHandlers(mux, problemsMgr)

	// Settings / Preferences (typed, with revision tracking and backups).
	if err := preferences.MigrateSchema(db.Conn()); err != nil {
		log.Fatalf("preferences migration error: %v", err)
	}
	prefsStore := preferences.NewStore(db.Conn())
	preferences.RegisterHandlers(mux, prefsStore)

	// Events hub — bridges OpenCode SSE to browser clients.
	eventsHub := events.NewHub()
	manager.SetEventBridgeManager(eventsHub)
	mux.HandleFunc("GET /api/events", eventsHub.ServeHTTP)

	// Dev tools — only registered when GLYPHDECK_DEV_TOOLS=1.
	devtools.RegisterHandlers(mux, registry, manager)

	// OpenCode config inspection (read-only).
	configRoot := opencodeConfigRoot()
	configScanner, err := config.NewScanner(configRoot)
	if err != nil {
		log.Printf("config scanner init warning: %v", err)
	} else {
		config.RegisterHandlers(mux, configScanner, registry)
	}

	// Frontend — serve the compiled React assets embedded in this binary.
	mux.HandleFunc("/", serveFrontend)

	srv := &http.Server{
		Addr:         addr,
		Handler:      auth.Middleware(authStore)(localMutationGuard(mux)),
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

	// Stop all app-owned OpenCode servers.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	stopped, err := manager.StopAllAppOwned(shutdownCtx)
	if err != nil {
		log.Printf("shutdown: error stopping OpenCode servers: %v", err)
	} else {
		log.Printf("shutdown: stopped %d app-owned OpenCode server(s)", len(stopped))
	}

	// Stop all app-owned terminals.
	if err := terminalMgr.CloseAll(); err != nil {
		log.Printf("shutdown: error closing terminals: %v", err)
	} else {
		log.Println("shutdown: terminals closed")
	}

	// Stop event hub bridges.
	eventsHub.StopAll()
	log.Println("shutdown: event bridges stopped")

	if err := srv.Shutdown(shutdownCtx); err != nil {
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

// localMutationGuard blocks cross-origin and non-loopback mutation requests.
func localMutationGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpapi.IsMutationMethod(r.Method) && !httpapi.AllowLocalMutation(r) {
			httpapi.WriteError(w, http.StatusForbidden, "forbidden_origin", "Mutation requests must use the same local origin.")
			return
		}
		next.ServeHTTP(w, r)
	})
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

// projectPathsResolverAdapter adapts projects.Registry to ProjectResolver for
// sessions, usage, and permissions packages. notFoundErr provides the
// package-specific sentinel so HTTP error mappers can use errors.Is.
type projectPathsResolverAdapter struct {
	registry    *projects.Registry
	notFoundErr error
}

func (a *projectPathsResolverAdapter) Get(ctx context.Context, id string) (opencode.ProjectPaths, error) {
	project, err := a.registry.Get(ctx, id)
	if errors.Is(err, projects.ErrProjectNotFound) {
		return opencode.ProjectPaths{}, a.notFoundErr
	}
	if err != nil {
		return opencode.ProjectPaths{}, err
	}
	return opencode.ProjectPaths{ID: project.ID, Path: project.Path}, nil
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

// bootstrapAdmin creates an admin from GLYPHDECK_ADMIN_PASSWORD or
// GLYPHDECK_ADMIN_PASSWORD_FILE if no admin exists.
func bootstrapAdmin(store *auth.Store) {
	password := getEnv("GLYPHDECK_ADMIN_PASSWORD", "")
	passwordFile := getEnv("GLYPHDECK_ADMIN_PASSWORD_FILE", "")

	if password != "" && passwordFile != "" {
		log.Fatalf("GLYPHDECK_ADMIN_PASSWORD and GLYPHDECK_ADMIN_PASSWORD_FILE are both set; use only one")
	}

	if passwordFile != "" {
		data, err := os.ReadFile(passwordFile)
		if err != nil {
			log.Fatalf("auth bootstrap: cannot read password file: %v", err)
		}
		password = strings.TrimSpace(string(data))
		if password == "" {
			log.Fatalf("auth bootstrap: password file is empty")
		}
	}

	if password == "" {
		return
	}

	ctx := context.Background()
	hasAdmin, err := store.HasAdmin(ctx)
	if err != nil {
		log.Printf("auth bootstrap: error checking admin: %v", err)
		return
	}
	if hasAdmin {
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("auth bootstrap: error hashing password: %v", err)
		return
	}
	if err := store.SetAdminHash(ctx, hash); err != nil {
		log.Printf("auth bootstrap: error creating admin: %v", err)
		return
	}
	log.Println("auth bootstrap: admin created")
}

// serveFrontend serves the React frontend embedded in the release binary.
// Vite still serves the frontend directly during development.
func serveFrontend(w http.ResponseWriter, r *http.Request) {
	assets := web.Assets()
	server := http.FileServer(http.FS(assets))

	// Serve compiled files directly, retaining the SPA fallback for client routes.
	assetPath := strings.TrimPrefix(r.URL.Path, "/")
	if assetPath == "" {
		server.ServeHTTP(w, r)
		return
	}
	if _, err := fs.Stat(assets, assetPath); err == nil {
		server.ServeHTTP(w, r)
		return
	}

	spaRequest := r.Clone(r.Context())
	spaURL := *r.URL
	spaURL.Path = "/"
	spaRequest.URL = &spaURL
	server.ServeHTTP(w, spaRequest)
}

// opencodeConfigRoot returns the global OpenCode configuration directory.
func opencodeConfigRoot() string {
	// GLYPHDECK_OPCODE_CONFIG_ROOT override for testing.
	if root := os.Getenv("GLYPHDECK_OPENCODE_CONFIG_ROOT"); root != "" {
		return root
	}

	// OpenCode follows XDG conventions: ~/.config/opencode on all platforms.
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode")
}

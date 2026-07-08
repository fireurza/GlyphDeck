package main

import (
	"context"
	"encoding/json"
	"errors"
	"glyphdeck/internal/opencode"
	"glyphdeck/internal/projects"
	"glyphdeck/internal/servers"
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

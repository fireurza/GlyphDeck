package servers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"glyphdeck/internal/opencode"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockDetector struct {
	installed  bool
	executable string
	version    string
}

func (m mockDetector) Detect() opencode.DetectionResult {
	status := "not-installed"
	if m.installed {
		status = "ready"
	}
	return opencode.DetectionResult{
		Installed:  m.installed,
		Executable: m.executable,
		Version:    m.version,
		Status:     status,
	}
}

type mockResolver struct {
	projects map[string]ProjectInfo
	err      error
}

func (m mockResolver) Get(_ context.Context, id string) (ProjectInfo, error) {
	if m.err != nil {
		return ProjectInfo{}, m.err
	}
	p, ok := m.projects[id]
	if !ok {
		return ProjectInfo{}, ErrProjectNotFound
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestManager(detector opencode.Detector, resolver ProjectResolver) *ServerManager {
	return NewManager(detector, resolver)
}

func mustTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// buildFakeOpenCodeBinary compiles a fake opencode.exe that supports
// "version" and "serve" subcommands. Returns the directory containing the binary.
func buildFakeOpenCodeBinary(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "main.go")

	code := `package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("opencode v2.5.0-test")
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		port := fs.Int("port", 0, "listen port")
		hostname := fs.String("hostname", "127.0.0.1", "hostname")
		_ = fs.Parse(os.Args[2:])

		mux := http.NewServeMux()
		mux.HandleFunc("/global/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(` + "`" + `{"status":"ok"}` + "`" + `))
		})
		addr := fmt.Sprintf("%s:%d", *hostname, *port)
		_ = http.ListenAndServe(addr, mux)
	default:
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(src, []byte(code), 0644); err != nil {
		t.Fatalf("write fake opencode main.go: %v", err)
	}

	exe := filepath.Join(tmpDir, "opencode.exe")
	build := exec.Command("go", "build", "-o", exe, src)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake opencode: %v\n%s", err, out)
	}

	return tmpDir
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestOpencodeStatus_NotInstalled(t *testing.T) {
	detector := mockDetector{installed: false}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	mgr := newTestManager(detector, resolver)

	result := mgr.OpencodeStatus()
	if result.Installed {
		t.Error("Installed = true, want false")
	}
	if result.Status != "not-installed" {
		t.Errorf("Status = %q, want %q", result.Status, "not-installed")
	}
}

func TestOpencodeStatus_Installed(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	mgr := newTestManager(detector, resolver)

	result := mgr.OpencodeStatus()
	if !result.Installed {
		t.Error("Installed = false, want true")
	}
	if result.Status != "ready" {
		t.Errorf("Status = %q, want %q", result.Status, "ready")
	}
	if result.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", result.Version, "v1.0.0")
	}
}

func TestStatus_NotInstalled(t *testing.T) {
	detector := mockDetector{installed: false}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: mustTempDir(t)},
		},
	}
	mgr := newTestManager(detector, resolver)

	status := mgr.Status(context.Background(), "proj-1")
	if status.Status != StateNotInstalled {
		t.Errorf("Status = %q, want %q", status.Status, StateNotInstalled)
	}
}

func TestStatus_StoppedWhenInstalledButNotRunning(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: mustTempDir(t)},
		},
	}
	mgr := newTestManager(detector, resolver)

	status := mgr.Status(context.Background(), "proj-1")
	if status.Status != StateStopped {
		t.Errorf("Status = %q, want %q", status.Status, StateStopped)
	}
}

func TestStatus_UnknownProject(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{},
	}
	mgr := newTestManager(detector, resolver)

	status := mgr.Status(context.Background(), "nonexistent")
	if status.Status != StateUnknown {
		t.Errorf("Status = %q, want %q", status.Status, StateUnknown)
	}
}

func TestStart_NotInstalled(t *testing.T) {
	detector := mockDetector{installed: false}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: mustTempDir(t)},
		},
	}
	mgr := newTestManager(detector, resolver)

	_, err := mgr.Start(context.Background(), "proj-1")
	if !errors.Is(err, ErrOpenCodeNotInstalled) {
		t.Errorf("err = %v, want ErrOpenCodeNotInstalled", err)
	}
}

func TestStart_ProjectNotFound(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{},
	}
	mgr := newTestManager(detector, resolver)

	_, err := mgr.Start(context.Background(), "nonexistent")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestStop_NotRunning(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: mustTempDir(t)},
		},
	}
	mgr := newTestManager(detector, resolver)

	_, err := mgr.Stop(context.Background(), "proj-1")
	if !errors.Is(err, ErrServerNotRunning) {
		t.Errorf("err = %v, want ErrServerNotRunning", err)
	}
}

func TestStartStop_Lifecycle(t *testing.T) {
	// Build fake opencode binary that serves a real health endpoint.
	fakeDir := buildFakeOpenCodeBinary(t)
	fakeExe := filepath.Join(fakeDir, "opencode.exe")

	projectDir := mustTempDir(t)
	detector := mockDetector{installed: true, executable: fakeExe, version: "v2.5.0-test"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: projectDir},
		},
	}
	mgr := newTestManager(detector, resolver)

	ctx := context.Background()

	// Start the server.
	status, err := mgr.Start(ctx, "proj-1")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if status.Status != StateReady {
		t.Errorf("Start status = %q, want %q", status.Status, StateReady)
	}
	if status.Port <= 0 {
		t.Error("Start did not assign a port")
	}
	if status.PID <= 0 {
		t.Error("Start did not capture a PID")
	}

	// Status should reflect ready.
	status = mgr.Status(ctx, "proj-1")
	if status.Status != StateReady {
		t.Errorf("Status after start = %q, want %q", status.Status, StateReady)
	}

	// Start again should fail with ErrServerAlreadyRunning.
	_, err = mgr.Start(ctx, "proj-1")
	if !errors.Is(err, ErrServerAlreadyRunning) {
		t.Errorf("second Start err = %v, want ErrServerAlreadyRunning", err)
	}

	// Stop the server.
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, err = mgr.Stop(stopCtx, "proj-1")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if status.Status != StateStopped {
		t.Errorf("Stop status = %q, want %q", status.Status, StateStopped)
	}

	// Status after stop should be stopped.
	status = mgr.Status(ctx, "proj-1")
	if status.Status != StateStopped {
		t.Errorf("Status after stop = %q, want %q", status.Status, StateStopped)
	}
}

func TestStart_FailedHealthCheck(t *testing.T) {
	// Use a fake opencode that exits immediately instead of serving.
	fakeDir := t.TempDir()
	src := filepath.Join(fakeDir, "main.go")
	code := `package main

import "fmt"

func main() {
	fmt.Println("opencode v2.5.0-test")
}
`
	if err := os.WriteFile(src, []byte(code), 0644); err != nil {
		t.Fatalf("write fake main.go: %v", err)
	}

	fakeExe := filepath.Join(fakeDir, "opencode.exe")
	build := exec.Command("go", "build", "-o", fakeExe, src)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake opencode: %v\n%s", err, out)
	}

	projectDir := mustTempDir(t)
	detector := mockDetector{installed: true, executable: fakeExe, version: "v2.5.0-test"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: projectDir},
		},
	}
	mgr := newTestManager(detector, resolver)

	_, err := mgr.Start(context.Background(), "proj-1")
	if err == nil {
		t.Fatal("Start should have failed with health check timeout")
	}

	// Status after failed start should be stopped.
	status := mgr.Status(context.Background(), "proj-1")
	if status.Status != StateStopped {
		t.Errorf("Status after failed start = %q, want %q", status.Status, StateStopped)
	}
}

func TestAllocatePort(t *testing.T) {
	port, err := allocatePort()
	if err != nil {
		t.Fatalf("allocatePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("port = %d, want valid port (1-65535)", port)
	}
}

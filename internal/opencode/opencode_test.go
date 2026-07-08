package opencode

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultDetector_NotInstalled(t *testing.T) {
	// Use an empty temp directory as PATH so opencode cannot be found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	d := NewDetector()
	result := d.Detect()

	if result.Installed {
		t.Errorf("Installed = true, want false")
	}
	if result.Status != "not-installed" {
		t.Errorf("Status = %q, want %q", result.Status, "not-installed")
	}
	if result.Executable != "" {
		t.Errorf("Executable = %q, want empty", result.Executable)
	}
}

func TestDefaultDetector_ReturnsStruct(t *testing.T) {
	// If opencode is on PATH, verify we get valid struct fields.
	// If not on PATH, this is still a valid code-path test.
	d := NewDetector()
	result := d.Detect()

	if result.Installed {
		if result.Status != "ready" {
			t.Errorf("Status = %q, want %q", result.Status, "ready")
		}
		if result.Executable == "" {
			t.Error("Executable is empty for installed opencode")
		}
	} else {
		if result.Status != "not-installed" {
			t.Errorf("Status = %q, want %q", result.Status, "not-installed")
		}
	}
}

// buildFakeOpenCode builds a minimal fake opencode binary that echoes a version.
func buildFakeOpenCode(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create a minimal Go main that acts like opencode.
	src := filepath.Join(tmpDir, "main.go")
	code := `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	switch os.Args[1] {
	case "--version":
		fmt.Println("opencode v2.5.0-test")
	case "version":
		fmt.Println("opencode v2.5.0-test")
	default:
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(src, []byte(code), 0644); err != nil {
		t.Fatalf("write fake main.go: %v", err)
	}

	exeName := "opencode"
	if runtime.GOOS == "windows" {
		exeName = "opencode.exe"
	}
	exe := filepath.Join(tmpDir, exeName)
	build := exec.Command("go", "build", "-o", exe, src)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake opencode: %v\n%s", err, out)
	}

	return tmpDir
}

func TestDefaultDetector_FakeOpenCodeOnPath(t *testing.T) {
	fakeDir := buildFakeOpenCode(t)
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+origPath)

	d := NewDetector()
	result := d.Detect()

	if !result.Installed {
		t.Fatal("Installed = false, want true")
	}
	if result.Status != "ready" {
		t.Errorf("Status = %q, want %q", result.Status, "ready")
	}
	if result.Version != "opencode v2.5.0-test" {
		t.Errorf("Version = %q, want %q", result.Version, "opencode v2.5.0-test")
	}
}

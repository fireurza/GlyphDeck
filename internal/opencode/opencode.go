// Package opencode provides OpenCode CLI detection.
package opencode

import (
	"bytes"
	"os/exec"
	"strings"
)

// DetectionResult describes the detected OpenCode CLI.
type DetectionResult struct {
	Installed  bool   `json:"installed"`
	Executable string `json:"executable"`
	Version    string `json:"version"`
	Status     string `json:"status"` // "ready" or "not-installed"
}

// Detector checks for the OpenCode CLI.
type Detector interface {
	Detect() DetectionResult
}

// DefaultDetector uses exec.LookPath and exec.Command.
type DefaultDetector struct{}

// NewDetector returns a default OpenCode CLI detector.
func NewDetector() Detector {
	return DefaultDetector{}
}

// Detect checks whether opencode is on PATH and returns its version.
func (d DefaultDetector) Detect() DetectionResult {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return DetectionResult{Installed: false, Status: "not-installed"}
	}

	cmd := exec.Command(path, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	version := ""
	if err := cmd.Run(); err == nil {
		version = strings.TrimSpace(stdout.String())
	}
	if version == "" && stderr.Len() > 0 {
		version = strings.TrimSpace(stderr.String())
	}

	return DetectionResult{
		Installed:  true,
		Executable: path,
		Version:    version,
		Status:     "ready",
	}
}

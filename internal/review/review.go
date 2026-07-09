// Package review aggregates project, Git, session, and activity data for the Review tab.
package review

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"glyphdeck/internal/opencode"
	"glyphdeck/internal/projects"
)

// Response is the JSON shape returned by the review endpoint.
type Response struct {
	Project   ProjectSummary  `json:"project"`
	Git       GitSummary      `json:"git"`
	Session   SessionSummary  `json:"session"`
	Activity  ActivitySummary `json:"activity"`
	UpdatedAt string          `json:"updatedAt"`
}

// ProjectSummary exposes project-level metadata.
type ProjectSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Trusted bool   `json:"trusted"`
}

// GitSummary reports the project's Git repository status.
type GitSummary struct {
	Available    bool     `json:"available"`
	Branch       string   `json:"branch"`
	Dirty        bool     `json:"dirty"`
	ChangedFiles []string `json:"changedFiles"`
}

// SessionSummary provides a high-level view of the active session.
type SessionSummary struct {
	ID                   string `json:"id"`
	MessageCount         int    `json:"messageCount"`
	LastAssistantSummary string `json:"lastAssistantSummary"`
}

// ActivitySummary reports aggregate event counts for the session.
// Live event counts are not persisted by the hub; this summary is derived
// from the session message count and available metadata.
type ActivitySummary struct {
	MessageCount int    `json:"messageCount"`
	ToolEvents   int    `json:"toolEvents"`
	Note         string `json:"note,omitempty"`
}

// Resolver resolves a client and project info for a project ID.
type Resolver interface {
	Resolve(ctx context.Context, projectID string) (opencode.SessionClient, *projects.Project, error)
}

// Aggregate computes review data from project, Git, and session state.
func Aggregate(ctx context.Context, resolver Resolver, projectID, sessionID string) (*Response, error) {
	client, project, err := resolver.Resolve(ctx, projectID)
	if err != nil {
		return nil, err
	}

	resp := &Response{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Project
	resp.Project = ProjectSummary{
		ID:      project.ID,
		Name:    project.Name,
		Path:    project.Path,
		Trusted: project.Trusted,
	}

	// Git
	resp.Git = gitSummary(project.Path)

	// Session
	messages, err := client.ListMessages(ctx, sessionID)
	if err != nil {
		// Session unavailable — return project+git only, no error.
		resp.Activity = ActivitySummary{Note: "Session data unavailable — server may not be ready."}
		return resp, nil
	}
	resp.Session.ID = sessionID
	resp.Session.MessageCount = len(messages)
	resp.Session.LastAssistantSummary = lastAssistantExcerpt(messages)

	// Activity
	resp.Activity = ActivitySummary{
		MessageCount: len(messages),
	}

	return resp, nil
}

// lastAssistantExcerpt returns the first 200 characters of text content
// from the most recent assistant message.
func lastAssistantExcerpt(messages []opencode.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Info.Role != "assistant" {
			continue
		}
		var buf bytes.Buffer
		for _, part := range messages[i].Parts {
			if part.Text != "" {
				buf.WriteString(part.Text)
				buf.WriteString(" ")
			}
		}
		text := strings.TrimSpace(buf.String())
		if text == "" {
			continue
		}
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		return text
	}
	return ""
}

// gitSummary returns the Git status for the given project directory.
func gitSummary(projectPath string) GitSummary {
	gs := GitSummary{}

	// Branch
	branch, err := gitExec(projectPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return gs
	}
	gs.Available = true
	gs.Branch = strings.TrimSpace(branch)

	// Dirty check + changed files
	status, err := gitExec(projectPath, "status", "--porcelain")
	if err != nil {
		return gs
	}
	lines := strings.Split(strings.TrimSpace(status), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		gs.Dirty = true
		// Extract filename (skip the XY status prefix).
		if len(line) > 3 {
			file := strings.TrimSpace(line[3:])
			if file != "" {
				gs.ChangedFiles = append(gs.ChangedFiles, file)
			}
		}
	}
	return gs
}

// gitExec runs a git command in the given directory and returns stdout.
func gitExec(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.String(), err
}

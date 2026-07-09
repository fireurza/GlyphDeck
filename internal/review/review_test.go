package review

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"glyphdeck/internal/opencode"
	"glyphdeck/internal/projects"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockResolver struct {
	client  opencode.SessionClient
	project *projects.Project
	err     error
}

func (m mockResolver) Resolve(_ context.Context, _ string) (opencode.SessionClient, *projects.Project, error) {
	return m.client, m.project, m.err
}

type mockSessionClient struct {
	messages []opencode.Message
	err      error
}

func (m *mockSessionClient) ListSessions(_ context.Context, _ string) ([]opencode.Session, error) {
	return nil, nil
}
func (m *mockSessionClient) CreateSession(_ context.Context, _, _ string) (opencode.Session, error) {
	return opencode.Session{}, nil
}
func (m *mockSessionClient) GetSession(_ context.Context, _ string) (opencode.Session, error) {
	return opencode.Session{}, nil
}
func (m *mockSessionClient) ListMessages(_ context.Context, _ string) ([]opencode.Message, error) {
	return m.messages, m.err
}
func (m *mockSessionClient) SendPrompt(_ context.Context, _, _ string) (opencode.PromptResult, error) {
	return opencode.PromptResult{}, nil
}
func (m *mockSessionClient) StreamEvents(_ context.Context) (<-chan opencode.NormalizedEvent, <-chan error) {
	events := make(chan opencode.NormalizedEvent)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs
}

func testProject(t *testing.T) *projects.Project {
	t.Helper()
	dir := t.TempDir()
	return &projects.Project{
		ID:      "proj-1",
		Name:    "Test Project",
		Path:    dir,
		Trusted: true,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAggregate_ResolveError(t *testing.T) {
	r := mockResolver{err: errors.New("server not ready")}
	_, err := Aggregate(context.Background(), r, "proj-1", "sess-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAggregate_ProjectInfo(t *testing.T) {
	dir := t.TempDir()
	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: true}
	r := mockResolver{
		client:  &mockSessionClient{messages: []opencode.Message{}},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Project.ID != "p1" {
		t.Fatalf("Project.ID = %q, want p1", resp.Project.ID)
	}
	if resp.Project.Name != "Test" {
		t.Fatalf("Project.Name = %q, want Test", resp.Project.Name)
	}
	if !resp.Project.Trusted {
		t.Fatal("Project.Trusted = false, want true")
	}
	if resp.Session.ID != "s1" {
		t.Fatalf("Session.ID = %q, want s1", resp.Session.ID)
	}
	if resp.Session.MessageCount != 0 {
		t.Fatalf("MessageCount = %d, want 0", resp.Session.MessageCount)
	}
	if resp.Activity.MessageCount != 0 {
		t.Fatalf("Activity.MessageCount = %d, want 0", resp.Activity.MessageCount)
	}
}

func TestAggregate_ListMessagesError(t *testing.T) {
	dir := t.TempDir()
	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: false}
	r := mockResolver{
		client:  &mockSessionClient{err: errors.New("opencode: GET /session returned 500")},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v (Aggregate should not fail when session data is unavailable)", err)
	}
	// Project info should still be populated.
	if resp.Project.ID != "p1" {
		t.Fatalf("Project.ID = %q, want p1", resp.Project.ID)
	}
	// Activity note should indicate session unavailable.
	if resp.Activity.Note == "" {
		t.Fatal("Activity.Note is empty, want session-unavailable note")
	}
}

func TestAggregate_SessionWithMessages(t *testing.T) {
	dir := t.TempDir()
	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: true}
	msg1 := opencode.Message{
		Info: opencode.MessageInfo{ID: "m1", Role: "user"},
	}
	textContent := "This is the assistant response"
	msg2 := opencode.Message{
		Info:  opencode.MessageInfo{ID: "m2", Role: "assistant"},
		Parts: []opencode.Part{{Text: "Hello "}, {Text: textContent}},
	}
	r := mockResolver{
		client:  &mockSessionClient{messages: []opencode.Message{msg1, msg2}},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Session.MessageCount != 2 {
		t.Fatalf("MessageCount = %d, want 2", resp.Session.MessageCount)
	}
	if resp.Session.LastAssistantSummary == "" {
		t.Fatal("LastAssistantSummary is empty")
	}
	if resp.Activity.MessageCount != 2 {
		t.Fatalf("Activity.MessageCount = %d, want 2", resp.Activity.MessageCount)
	}
}

func TestAggregate_LastAssistantExcerpt(t *testing.T) {
	dir := t.TempDir()
	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: true}
	longText := ""
	for i := 0; i < 300; i++ {
		longText += "x"
	}
	msg := opencode.Message{
		Info:  opencode.MessageInfo{ID: "m1", Role: "assistant"},
		Parts: []opencode.Part{{Text: longText}},
	}
	r := mockResolver{
		client:  &mockSessionClient{messages: []opencode.Message{msg}},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Session.LastAssistantSummary) > 203 {
		t.Fatalf("LastAssistantSummary too long: %d chars, want ≤203", len(resp.Session.LastAssistantSummary))
	}
}

func TestAggregate_GitAvailable(t *testing.T) {
	// Create a temp dir with a git repo.
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	// Create a file to make it dirty.
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	runGit(t, dir, "add", "test.txt")
	runGit(t, dir, "commit", "-m", "init")

	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: true}
	r := mockResolver{
		client:  &mockSessionClient{messages: []opencode.Message{}},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Git.Available {
		t.Fatal("Git.Available = false, want true for git repo")
	}
	if resp.Git.Branch == "" {
		t.Fatal("Git.Branch is empty")
	}
}

func TestAggregate_GitDirty(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	// Make a dirty change.
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified"), 0644)

	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: true}
	r := mockResolver{
		client:  &mockSessionClient{messages: []opencode.Message{}},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Git.Dirty {
		t.Fatal("Git.Dirty = false, want true after file modification")
	}
	if len(resp.Git.ChangedFiles) == 0 {
		t.Fatal("ChangedFiles is empty, want at least one changed file")
	}
}

func TestAggregate_GitNotAvailable(t *testing.T) {
	dir := t.TempDir()
	proj := &projects.Project{ID: "p1", Name: "Test", Path: dir, Trusted: true}
	r := mockResolver{
		client:  &mockSessionClient{messages: []opencode.Message{}},
		project: proj,
	}
	resp, err := Aggregate(context.Background(), r, "p1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Git.Available {
		t.Fatal("Git.Available = true, want false for non-git directory")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

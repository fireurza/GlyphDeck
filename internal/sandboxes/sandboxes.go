// Package sandboxes manages OpenCode server targets, remote SSH lifecycle, and active server selection.
package sandboxes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ServerType classifies how GlyphDeck reaches an OpenCode server.
type ServerType string

const (
	TypeLocal     ServerType = "local"
	TypeManualURL ServerType = "manual_url"
	TypeSSHAlias  ServerType = "ssh_alias"
)

// ServerConfig is a persisted OpenCode server target.
type ServerConfig struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     ServerType `json:"type"`
	URL      string     `json:"url"`
	SSHAlias string     `json:"sshAlias"`

	// Remote lifecycle fields (SSH targets).
	WorkingDir   string `json:"workingDir"`
	StartCommand string `json:"startCommand"`
	StopCommand  string `json:"stopCommand"`
	StatusCmd    string `json:"statusCommand"`

	// Runtime metadata (updated after lifecycle operations).
	LastPID            int    `json:"lastPid"`
	LastURL            string `json:"lastUrl"`
	LastStatus         string `json:"lastStatus"`
	LastCheckedAt      string `json:"lastCheckedAt"`
	StartedByGlyphdeck bool   `json:"startedByGlyphdeck"`

	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// RemoteStatus describes the result of a detect/status operation.
type RemoteStatus struct {
	Status  string `json:"status"` // online|offline|unknown
	PID     int    `json:"pid,omitempty"`
	URL     string `json:"url,omitempty"`
	Message string `json:"message,omitempty"`
}

// RemoteResult describes the result of a start or stop operation.
type RemoteResult struct {
	Success bool   `json:"success"`
	PID     int    `json:"pid,omitempty"`
	URL     string `json:"url,omitempty"`
	Message string `json:"message"`
}

// SSHResult is the raw output of an SSH command execution.
type SSHResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// SSHRunner executes commands over SSH for a given alias.
type SSHRunner interface {
	Run(ctx context.Context, sshAlias, command string, timeout time.Duration) SSHResult
}

// ErrNotFound is returned when a server config is not found.
var ErrNotFound = errors.New("server config not found")

// ErrNoPID is returned when a stop is attempted without a recorded PID.
var ErrNoPID = errors.New("no recorded PID for this server")

// ErrPIDMismatch is returned when the recorded PID does not belong to an OpenCode process.
var ErrPIDMismatch = errors.New("PID does not belong to an OpenCode process")

// Registry persists and retrieves OpenCode server configurations and remote lifecycle state.
type Registry struct {
	db        *sql.DB
	sshRunner SSHRunner
	mu        sync.Mutex
	ops       map[string]*sync.Mutex // per-target operation locks
}

// NewRegistry creates a Registry backed by the given database connection.
func NewRegistry(db *sql.DB) (*Registry, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	return &Registry{db: db, sshRunner: &realSSHRunner{}, ops: make(map[string]*sync.Mutex)}, nil
}

// NewRegistryWithSSH creates a Registry with a custom SSH runner (for tests).
func NewRegistryWithSSH(db *sql.DB, runner SSHRunner) (*Registry, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	return &Registry{db: db, sshRunner: runner, ops: make(map[string]*sync.Mutex)}, nil
}

func scanConfig(row interface{ Scan(dest ...any) error }) (ServerConfig, error) {
	var cfg ServerConfig
	var typ string
	var startedBy int
	err := row.Scan(&cfg.ID, &cfg.Name, &typ, &cfg.URL, &cfg.SSHAlias,
		&cfg.WorkingDir, &cfg.StartCommand, &cfg.StopCommand, &cfg.StatusCmd,
		&cfg.LastPID, &cfg.LastURL, &cfg.LastStatus, &cfg.LastCheckedAt,
		&startedBy, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return cfg, err
	}
	cfg.Type = ServerType(typ)
	cfg.StartedByGlyphdeck = startedBy == 1
	return cfg, nil
}

// lockOp acquires the per-target operation lock, preventing concurrent lifecycle ops.
func (r *Registry) lockOp(id string) func() {
	r.mu.Lock()
	mu, ok := r.ops[id]
	if !ok {
		mu = &sync.Mutex{}
		r.ops[id] = mu
	}
	r.mu.Unlock()
	mu.Lock()
	return func() { mu.Unlock() }
}

// Add inserts a new server configuration.
func (r *Registry) Add(ctx context.Context, cfg ServerConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	startedBy := 0
	if cfg.StartedByGlyphdeck {
		startedBy = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO server_configs (id, name, type, url, ssh_alias,
		 working_dir, start_command, stop_command, status_command,
		 last_pid, last_url, last_status, last_checked_at, started_by_glyphdeck,
		 created_at, updated_at)
		 VALUES (?,?,?,?,?, ?,?,?,?, ?,?,?,?,?, ?,?)`,
		cfg.ID, cfg.Name, string(cfg.Type), cfg.URL, cfg.SSHAlias,
		cfg.WorkingDir, cfg.StartCommand, cfg.StopCommand, cfg.StatusCmd,
		cfg.LastPID, cfg.LastURL, cfg.LastStatus, cfg.LastCheckedAt,
		startedBy, cfg.CreatedAt, cfg.UpdatedAt,
	)
	return err
}

// Update modifies an existing server configuration. Preserves runtime metadata
// unless the type or SSH alias changes.
func (r *Registry) Update(ctx context.Context, cfg ServerConfig) error {
	existing, err := r.Get(ctx, cfg.ID)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cfg.UpdatedAt = now

	// Preserve runtime metadata unless identity changes.
	cfg.LastPID = existing.LastPID
	cfg.LastURL = existing.LastURL
	cfg.LastStatus = existing.LastStatus
	cfg.LastCheckedAt = existing.LastCheckedAt
	cfg.StartedByGlyphdeck = existing.StartedByGlyphdeck
	cfg.CreatedAt = existing.CreatedAt

	// Reset runtime metadata when type or SSH alias changes.
	if cfg.Type != existing.Type || cfg.SSHAlias != existing.SSHAlias {
		cfg.LastPID = 0
		cfg.LastURL = ""
		cfg.LastStatus = ""
		cfg.LastCheckedAt = ""
		cfg.StartedByGlyphdeck = false
	}

	startedBy := 0
	if cfg.StartedByGlyphdeck {
		startedBy = 1
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE server_configs SET name=?, type=?, url=?, ssh_alias=?,
		 working_dir=?, start_command=?, stop_command=?, status_command=?,
		 last_pid=?, last_url=?, last_status=?, last_checked_at=?, started_by_glyphdeck=?,
		 updated_at=?
		 WHERE id=?`,
		cfg.Name, string(cfg.Type), cfg.URL, cfg.SSHAlias,
		cfg.WorkingDir, cfg.StartCommand, cfg.StopCommand, cfg.StatusCmd,
		cfg.LastPID, cfg.LastURL, cfg.LastStatus, cfg.LastCheckedAt, startedBy,
		cfg.UpdatedAt, cfg.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get retrieves a single server configuration by ID.
func (r *Registry) Get(ctx context.Context, id string) (ServerConfig, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, type, url, ssh_alias,
		 working_dir, start_command, stop_command, status_command,
		 last_pid, last_url, last_status, last_checked_at, started_by_glyphdeck,
		 created_at, updated_at
		 FROM server_configs WHERE id=?`, id)
	cfg, err := scanConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ServerConfig{}, ErrNotFound
	}
	return cfg, err
}

// List returns all configured server targets.
func (r *Registry) List(ctx context.Context) ([]ServerConfig, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, url, ssh_alias,
		 working_dir, start_command, stop_command, status_command,
		 last_pid, last_url, last_status, last_checked_at, started_by_glyphdeck,
		 created_at, updated_at
		 FROM server_configs ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []ServerConfig
	for rows.Next() {
		cfg, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// Delete removes a server configuration by ID.
func (r *Registry) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM server_configs WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// setRuntime updates the runtime metadata fields.
func (r *Registry) setRuntime(ctx context.Context, id string, pid int, url, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	startedBy := 0
	if pid > 0 {
		startedBy = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE server_configs SET last_pid=?, last_url=?, last_status=?,
		 last_checked_at=?, started_by_glyphdeck=?, updated_at=?
		 WHERE id=?`,
		pid, url, status, now, startedBy, now, id)
	return err
}

// ---------------------------------------------------------------------------
// Remote lifecycle operations
// ---------------------------------------------------------------------------

// TestSSH verifies that an SSH connection can be established to the given alias.
func (r *Registry) TestSSH(ctx context.Context, id string) (SSHResult, error) {
	unlock := r.lockOp(id)
	defer unlock()

	cfg, err := r.Get(ctx, id)
	if err != nil {
		return SSHResult{}, err
	}
	if cfg.Type != TypeSSHAlias || cfg.SSHAlias == "" {
		return SSHResult{}, fmt.Errorf("server is not an SSH target")
	}
	result := r.sshRunner.Run(ctx, cfg.SSHAlias, "echo ok", 10*time.Second)
	r.setRuntime(ctx, id, cfg.LastPID, cfg.LastURL, "unknown")
	return result, nil
}

// Detect runs the configured status command (or basic detect) on a remote server.
func (r *Registry) Detect(ctx context.Context, id string) (RemoteStatus, error) {
	unlock := r.lockOp(id)
	defer unlock()

	cfg, err := r.Get(ctx, id)
	if err != nil {
		return RemoteStatus{}, err
	}
	if cfg.Type != TypeSSHAlias || cfg.SSHAlias == "" {
		return RemoteStatus{}, fmt.Errorf("server is not an SSH target")
	}

	statusCmd := cfg.StatusCmd
	if statusCmd == "" {
		statusCmd = "pgrep -f 'opencode serve' && echo 'running' || echo 'stopped'"
	}

	result := r.sshRunner.Run(ctx, cfg.SSHAlias, statusCmd, 10*time.Second)
	if result.Err != nil {
		r.setRuntime(ctx, id, 0, "", "offline")
		return RemoteStatus{Status: "offline", Message: result.Err.Error()}, nil
	}

	status := "offline"
	if result.Stdout != "" && !containsString(result.Stdout, "stopped") {
		status = "online"
	}

	// Try to extract PID and URL from status output.
	pid := extractPID(result.Stdout)
	url := cfg.LastURL
	if pid == 0 && status == "online" {
		// Run pgrep to get PID.
		pidResult := r.sshRunner.Run(ctx, cfg.SSHAlias, "pgrep -f 'opencode serve' | head -1", 5*time.Second)
		if pidResult.Err == nil && pidResult.Stdout != "" {
			pid = extractPID(pidResult.Stdout)
		}
	}

	r.setRuntime(ctx, id, pid, url, status)
	return RemoteStatus{Status: status, PID: pid, URL: url, Message: "detect completed"}, nil
}

// StartRemote starts an OpenCode server on a remote host via SSH.
func (r *Registry) StartRemote(ctx context.Context, id string) (RemoteResult, error) {
	unlock := r.lockOp(id)
	defer unlock()

	cfg, err := r.Get(ctx, id)
	if err != nil {
		return RemoteResult{}, err
	}
	if cfg.Type != TypeSSHAlias || cfg.SSHAlias == "" {
		return RemoteResult{}, fmt.Errorf("server is not an SSH target")
	}

	startCmd := cfg.StartCommand
	if startCmd == "" {
		startCmd = "cd ~ && opencode serve --port 4096 --hostname 0.0.0.0 &"
	}

	// Start the server in background and capture PID.
	fullCmd := fmt.Sprintf("%s; echo GLYPHDECK_PID=$!", startCmd)
	result := r.sshRunner.Run(ctx, cfg.SSHAlias, fullCmd, 15*time.Second)
	if result.Err != nil {
		return RemoteResult{Success: false, Message: fmt.Sprintf("SSH start failed: %v", result.Err)}, nil
	}

	pid := extractPIDFromMarker(result.Stdout, "GLYPHDECK_PID=")
	if pid == 0 {
		return RemoteResult{Success: false, Message: "could not extract PID from start output"}, nil
	}

	// Default URL for remote OpenCode.
	url := cfg.URL
	if url == "" {
		url = "http://localhost:4096"
	}

	r.setRuntime(ctx, id, pid, url, "online")
	return RemoteResult{Success: true, PID: pid, URL: url, Message: "started"}, nil
}

// StopRemote stops a remote OpenCode server by its recorded PID.
func (r *Registry) StopRemote(ctx context.Context, id string) (RemoteResult, error) {
	unlock := r.lockOp(id)
	defer unlock()

	cfg, err := r.Get(ctx, id)
	if err != nil {
		return RemoteResult{}, err
	}
	if cfg.Type != TypeSSHAlias || cfg.SSHAlias == "" {
		return RemoteResult{}, fmt.Errorf("server is not an SSH target")
	}
	if cfg.LastPID <= 0 {
		return RemoteResult{Success: false, Message: "no recorded PID — cannot stop safely"}, nil
	}

	// Verify PID belongs to OpenCode.
	verifyCmd := fmt.Sprintf("ps -p %d -o comm= 2>/dev/null | grep -q opencode && echo valid || echo invalid", cfg.LastPID)
	verify := r.sshRunner.Run(ctx, cfg.SSHAlias, verifyCmd, 5*time.Second)
	if verify.Err != nil || containsString(verify.Stdout, "invalid") {
		return RemoteResult{Success: false, Message: "PID verification failed — refusing to stop"}, nil
	}

	// Stop only the exact PID.
	stopCmd := cfg.StopCommand
	if stopCmd == "" {
		stopCmd = fmt.Sprintf("kill %d", cfg.LastPID)
	}
	result := r.sshRunner.Run(ctx, cfg.SSHAlias, stopCmd, 10*time.Second)

	r.setRuntime(ctx, id, 0, cfg.LastURL, "offline")
	return RemoteResult{Success: result.Err == nil, Message: "stopped", PID: cfg.LastPID}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractPID(output string) int {
	// Find the first numeric line or value.
	fields := splitFields(output)
	for _, f := range fields {
		if pid, err := strconv.Atoi(f); err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func extractPIDFromMarker(output, marker string) int {
	idx := findIndex(output, marker)
	if idx < 0 {
		return 0
	}
	start := idx + len(marker)
	if start >= len(output) {
		return 0
	}
	rest := output[start:]
	return extractPID(rest)
}

func splitFields(s string) []string {
	var fields []string
	current := ""
	for _, ch := range s {
		if ch == ' ' || ch == '\n' || ch == '\t' || ch == '\r' {
			if current != "" {
				fields = append(fields, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

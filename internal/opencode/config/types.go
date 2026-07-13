// Package config provides read-only inspection of OpenCode configuration files.
// It never executes scripts, reads .env, or resolves environment variable values.
package config

// Inventory is the complete read-only snapshot of OpenCode configuration.
type Inventory struct {
	Available     bool              `json:"available"`
	Reason        string            `json:"reason"`
	Sources       []ConfigSource    `json:"sources"`
	Agents        []AgentEntry      `json:"agents"`
	Providers     []ProviderEntry   `json:"providers"`
	Models        []ModelEntry      `json:"models"`
	MCPServers    []MCPServerEntry  `json:"mcpServers"`
	Skills        []SkillEntry      `json:"skills"`
	Plugins       []PluginEntry     `json:"plugins"`
	ShellProfiles []ShellProfile    `json:"shellProfiles"`
	Warnings      []ConfigWarning   `json:"warnings"`
}

// ConfigSource identifies a configuration file that was inspected.
type ConfigSource struct {
	Path    string `json:"path"`
	Scope   string `json:"scope"` // "global" or "project"
	Format  string `json:"format"` // "json", "jsonc", "markdown", "directory"
	Loaded  bool   `json:"loaded"`
	Warning string `json:"warning,omitempty"`
}

// AgentEntry describes a discovered OpenCode agent.
type AgentEntry struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"` // "global" or "project"
	Source      string `json:"source"` // "opencode.json", "agents/", or "AGENTS.md"
	SourceFile  string `json:"sourceFile,omitempty"`
	Role        string `json:"role,omitempty"` // "primary" or "subagent"
	Model       string `json:"model,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// ProviderEntry describes a discovered AI provider configuration.
type ProviderEntry struct {
	ID      string `json:"id"`
	Scope   string `json:"scope"`
	Name    string `json:"name,omitempty"`
	BaseURL string `json:"baseUrl,omitempty"`
	Enabled bool   `json:"enabled"`
	// Never includes API keys, tokens, or headers.
}

// ModelEntry describes a discovered model configuration.
type ModelEntry struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Scope    string `json:"scope"`
	Name     string `json:"name,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// MCPServerEntry describes a discovered MCP server.
type MCPServerEntry struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // "remote", "local", "stdio", "sse"
	Scope      string `json:"scope"`
	SourceFile string `json:"sourceFile,omitempty"`
	URL        string `json:"url,omitempty"`
	Command    string `json:"command,omitempty"`
	Enabled    bool   `json:"enabled"`
	// Never includes headers, environment variables, API keys, or tokens.
}

// SkillEntry describes a discovered skill or prompt.
type SkillEntry struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	SourceFile  string `json:"sourceFile,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// PluginEntry describes a discovered plugin.
type PluginEntry struct {
	ID         string `json:"id"`
	Scope      string `json:"scope"`
	SourceFile string `json:"sourceFile,omitempty"`
	Type       string `json:"type,omitempty"` // "local" or "npm"
	Enabled    bool   `json:"enabled"`
}

// ShellProfile describes a configured shell preference.
type ShellProfile struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

// ConfigWarning describes a non-fatal issue discovered during scanning.
type ConfigWarning struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

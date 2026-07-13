package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxConfigFileSize = 2 * 1024 * 1024 // 2 MiB per config file

// OpenCode global config root (platform-specific, resolved by caller).
// Scanner stores resolved paths for containment validation.
type Scanner struct {
	globalConfigRoot string // e.g., ~/.config/opencode
}

// NewScanner creates a Scanner for the given global OpenCode config directory.
func NewScanner(globalConfigRoot string) *Scanner {
	return &Scanner{globalConfigRoot: globalConfigRoot}
}

// ScanGlobal scans the global OpenCode configuration directory.
func (s *Scanner) ScanGlobal() *Inventory {
	inv := &Inventory{Available: true}

	// Read the main config file (prefer .jsonc, fall back to .json).
	configPath, configData, format, err := s.readConfigFile(s.globalConfigRoot)
	if err != nil {
		inv.Available = false
		inv.Reason = fmt.Sprintf("cannot read global config: %v", err)
		return inv
	}

	inv.Sources = append(inv.Sources, ConfigSource{
		Path:   configPath,
		Scope:  "global",
		Format: format,
		Loaded: true,
	})

	s.parseConfig(inv, configData, "global", configPath)

	// Scan subdirectories.
	s.scanAgentsDir(inv, s.globalConfigRoot, "global")
	s.scanSkillsDir(inv, s.globalConfigRoot, "global")
	s.scanPluginsDir(inv, s.globalConfigRoot, "global")

	s.sortAll(inv)
	return inv
}

// ScanProject scans a registered project's .opencode directory.
// The project root must be validated before calling this method.
func (s *Scanner) ScanProject(inv *Inventory, projectRoot string) {
	projectConfigDir := filepath.Join(projectRoot, ".opencode")

	// Read project config if it exists.
	configPath, configData, format, err := s.readConfigFile(projectConfigDir)
	if err == nil {
		inv.Sources = append(inv.Sources, ConfigSource{
			Path:   configPath,
			Scope:  "project",
			Format: format,
			Loaded: true,
		})
		s.parseConfig(inv, configData, "project", configPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		inv.Warnings = append(inv.Warnings, ConfigWarning{
			Source:  configPath,
			Message: fmt.Sprintf("cannot read project config: %v", err),
		})
	}

	// Scan subdirectories under .opencode.
	s.scanAgentsDir(inv, projectConfigDir, "project")
	s.scanSkillsDir(inv, projectConfigDir, "project")
	s.scanPluginsDir(inv, projectConfigDir, "project")

	s.sortAll(inv)
}

// ---------------------------------------------------------------------------
// Config file reading
// ---------------------------------------------------------------------------

func (s *Scanner) readConfigFile(configDir string) (path string, data []byte, format string, err error) {
	// Prefer .jsonc over .json.
	for _, name := range []string{"opencode.jsonc", "opencode.json"} {
		p := filepath.Join(configDir, name)
		d, fErr := s.safeReadFile(p)
		if fErr == nil {
			format := "json"
			if strings.HasSuffix(name, ".jsonc") {
				format = "jsonc"
			}
			return p, d, format, nil
		}
		if !errors.Is(fErr, fs.ErrNotExist) {
			return p, nil, "", fErr
		}
	}
	return filepath.Join(configDir, "opencode.jsonc"), nil, "", fs.ErrNotExist
}

// safeReadFile reads a file with a size limit.
func (s *Scanner) safeReadFile(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > maxConfigFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", fi.Size(), maxConfigFileSize)
	}
	return os.ReadFile(path)
}

// ---------------------------------------------------------------------------
// JSON/JSONC parsing
// ---------------------------------------------------------------------------

// parseConfig parses a raw config file and populates the inventory.
func (s *Scanner) parseConfig(inv *Inventory, data []byte, scope, sourcePath string) {
	if data == nil || len(data) == 0 {
		return
	}

	// Strip JSONC comments (// and /* */) before parsing.
	cleaned := stripJSONCComments(data)

	var raw map[string]any
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		inv.Warnings = append(inv.Warnings, ConfigWarning{
			Source:  sourcePath,
			Message: fmt.Sprintf("parse error: %v", err),
		})
		return
	}

	// Extract agents.
	s.parseAgents(inv, raw, scope, sourcePath)

	// Extract providers and models.
	s.parseProviders(inv, raw, scope)

	// Extract MCP servers.
	s.parseMCP(inv, raw, scope, sourcePath)

	// Extract plugins.
	s.parsePlugins(inv, raw, scope, sourcePath)

	// Extract shell.
	s.parseShell(inv, raw, scope)
}

func (s *Scanner) parseAgents(inv *Inventory, raw map[string]any, scope, sourcePath string) {
	agents, ok := raw["agent"].(map[string]any)
	if !ok {
		// "agents" (plural) is an alternative key some configs use.
		agents, _ = raw["agents"].(map[string]any)
	}
	if agents == nil {
		return
	}

	for name, val := range agents {
		cfg, ok := val.(map[string]any)
		if !ok {
			continue
		}

		entry := AgentEntry{
			Name:       name,
			Scope:      scope,
			Source:     "opencode.json",
			SourceFile: sourcePath,
			Enabled:    true,
		}

		if desc, ok := cfg["description"].(string); ok {
			entry.Description = desc
		}
		if model, ok := cfg["model"].(string); ok {
			entry.Model = model
		}
		if mode, ok := cfg["mode"].(string); ok {
			entry.Role = mode
		}

		inv.Agents = append(inv.Agents, entry)
	}
}

func (s *Scanner) parseProviders(inv *Inventory, raw map[string]any, scope string) {
	providers, ok := raw["provider"].(map[string]any)
	if !ok {
		providers, _ = raw["providers"].(map[string]any)
	}
	if providers == nil {
		return
	}

	for id, val := range providers {
		cfg, ok := val.(map[string]any)
		if !ok {
			continue
		}

		entry := ProviderEntry{
			ID:      id,
			Scope:   scope,
			Enabled: true,
		}

		if name, ok := cfg["name"].(string); ok {
			entry.Name = name
		}
		if baseURL, ok := cfg["baseUrl"].(string); ok {
			entry.BaseURL = baseURL
		}
		if baseURL, ok := cfg["base_url"].(string); ok {
			entry.BaseURL = baseURL
		}

		inv.Providers = append(inv.Providers, entry)

		// Extract models from this provider.
		if models, ok := cfg["models"].(map[string]any); ok {
			for modelID, modelVal := range models {
				modelCfg, _ := modelVal.(map[string]any)
				modelEntry := ModelEntry{
					ID:       modelID,
					Provider: id,
					Scope:    scope,
					Enabled:  true,
				}
				if modelCfg != nil {
					if name, ok := modelCfg["name"].(string); ok {
						modelEntry.Name = name
					}
				}
				inv.Models = append(inv.Models, modelEntry)
			}
		}
	}
}

func (s *Scanner) parseMCP(inv *Inventory, raw map[string]any, scope, sourcePath string) {
	// MCP servers can be under "mcp" or "mcpServers".
	mcp, ok := raw["mcp"].(map[string]any)
	if !ok {
		mcp, _ = raw["mcpServers"].(map[string]any)
	}
	if mcp == nil {
		return
	}

	for name, val := range mcp {
		cfg, ok := val.(map[string]any)
		if !ok {
			continue
		}

		entry := MCPServerEntry{
			Name:       name,
			Scope:      scope,
			SourceFile: sourcePath,
		}

		if t, ok := cfg["type"].(string); ok {
			entry.Type = t
		}
		if url, ok := cfg["url"].(string); ok {
			entry.URL = url
		}
		if cmd, ok := cfg["command"].(string); ok {
			entry.Command = cmd
		}
		if cmds, ok := cfg["command"].([]any); ok {
			parts := make([]string, 0, len(cmds))
			for _, c := range cmds {
				if s, ok := c.(string); ok {
					parts = append(parts, s)
				}
			}
			entry.Command = strings.Join(parts, " ")
		}

		// Enabled defaults to true unless explicitly false.
		entry.Enabled = true
		if enabled, ok := cfg["enabled"].(bool); ok {
			entry.Enabled = enabled
		}

		// Redact: never include headers, env, or apiKey fields.
		inv.MCPServers = append(inv.MCPServers, entry)
	}
}

func (s *Scanner) parsePlugins(inv *Inventory, raw map[string]any, scope, sourcePath string) {
	plugins, ok := raw["plugin"].([]any)
	if !ok {
		return
	}

	for _, p := range plugins {
		var id string
		pluginType := "npm"

		switch v := p.(type) {
		case string:
			id = v
			if strings.HasPrefix(id, "./") || strings.HasPrefix(id, ".\\") || filepath.IsAbs(id) {
				pluginType = "local"
			}
		case map[string]any:
			if name, ok := v["name"].(string); ok {
				id = name
			}
			if t, ok := v["type"].(string); ok {
				pluginType = t
			}
		default:
			continue
		}

		inv.Plugins = append(inv.Plugins, PluginEntry{
			ID:         id,
			Scope:      scope,
			SourceFile: sourcePath,
			Type:       pluginType,
			Enabled:    true,
		})
	}
}

func (s *Scanner) parseShell(inv *Inventory, raw map[string]any, scope string) {
	shell, ok := raw["shell"].(string)
	if !ok || shell == "" {
		return
	}

	inv.ShellProfiles = append(inv.ShellProfiles, ShellProfile{
		Name:  shell,
		Scope: scope,
	})
}

// ---------------------------------------------------------------------------
// Directory scanners
// ---------------------------------------------------------------------------

func (s *Scanner) scanAgentsDir(inv *Inventory, configDir, scope string) {
	// Scan agents/ directory for markdown agent definitions.
	agentsDir := filepath.Join(configDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return
	}

	inv.Sources = append(inv.Sources, ConfigSource{
		Path:   agentsDir,
		Scope:  scope,
		Format: "directory",
		Loaded: true,
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		agentName := strings.TrimSuffix(name, ".md")
		// Don't duplicate agents already parsed from config.
		if s.hasAgent(inv, agentName) {
			continue
		}

		inv.Agents = append(inv.Agents, AgentEntry{
			Name:       agentName,
			Scope:      scope,
			Source:     "agents/",
			SourceFile: filepath.Join(agentsDir, name),
			Enabled:    true,
		})
	}
}

func (s *Scanner) scanSkillsDir(inv *Inventory, configDir, scope string) {
	skillsDir := filepath.Join(configDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return
	}

	inv.Sources = append(inv.Sources, ConfigSource{
		Path:   skillsDir,
		Scope:  scope,
		Format: "directory",
		Loaded: true,
	})

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillDir := filepath.Join(skillsDir, skillName)
		skillFile := filepath.Join(skillDir, "SKILL.md")

		desc := ""
		if data, err := s.safeReadFile(skillFile); err == nil {
			// Extract first meaningful line from SKILL.md for description.
			desc = extractFirstLine(string(data), 120)
		}

		inv.Skills = append(inv.Skills, SkillEntry{
			Name:        skillName,
			Scope:       scope,
			SourceFile:  skillDir,
			Description: desc,
			Enabled:     true,
		})
	}
}

func (s *Scanner) scanPluginsDir(inv *Inventory, configDir, scope string) {
	pluginsDir := filepath.Join(configDir, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			// Skip loose files in plugins/.
			continue
		}

		pluginName := entry.Name()
		// Don't duplicate plugins already listed in plugin array.
		if s.hasPlugin(inv, pluginName) {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, pluginName)

		inv.Plugins = append(inv.Plugins, PluginEntry{
			ID:         pluginName,
			Scope:      scope,
			SourceFile: pluginDir,
			Type:       "local",
			Enabled:    true,
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *Scanner) hasAgent(inv *Inventory, name string) bool {
	for _, a := range inv.Agents {
		if a.Name == name {
			return true
		}
	}
	return false
}

func (s *Scanner) hasPlugin(inv *Inventory, name string) bool {
	for _, p := range inv.Plugins {
		if p.ID == name {
			return true
		}
	}
	return false
}

func (s *Scanner) sortAll(inv *Inventory) {
	sort.Slice(inv.Sources, func(i, j int) bool { return inv.Sources[i].Path < inv.Sources[j].Path })
	sort.Slice(inv.Agents, func(i, j int) bool { return inv.Agents[i].Name < inv.Agents[j].Name })
	sort.Slice(inv.Providers, func(i, j int) bool { return inv.Providers[i].ID < inv.Providers[j].ID })
	sort.Slice(inv.Models, func(i, j int) bool { return inv.Models[i].ID < inv.Models[j].ID })
	sort.Slice(inv.MCPServers, func(i, j int) bool { return inv.MCPServers[i].Name < inv.MCPServers[j].Name })
	sort.Slice(inv.Skills, func(i, j int) bool { return inv.Skills[i].Name < inv.Skills[j].Name })
	sort.Slice(inv.Plugins, func(i, j int) bool { return inv.Plugins[i].ID < inv.Plugins[j].ID })
	sort.Slice(inv.ShellProfiles, func(i, j int) bool { return inv.ShellProfiles[i].Name < inv.ShellProfiles[j].Name })
}

// extractFirstLine returns the first non-empty line of text, truncated.
func extractFirstLine(text string, maxLen int) string {
	lines := strings.SplitN(text, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if len(first) > maxLen {
		first = first[:maxLen] + "..."
	}
	return first
}

// stripJSONCComments removes // and /* */ comments from JSONC content.
func stripJSONCComments(data []byte) []byte {
	// Simple state-machine comment stripper for JSONC.
	// This handles line comments (//) and block comments (/* */).
	var result []byte
	i := 0
	n := len(data)

	for i < n {
		// Check for string literals (skip them).
		if data[i] == '"' {
			result = append(result, data[i])
			i++
			for i < n {
				if data[i] == '\\' && i+1 < n {
					result = append(result, data[i], data[i+1])
					i += 2
					continue
				}
				result = append(result, data[i])
				if data[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}

		// Check for line comment.
		if i+1 < n && data[i] == '/' && data[i+1] == '/' {
			// Skip until end of line.
			for i < n && data[i] != '\n' {
				i++
			}
			if i < n {
				result = append(result, '\n')
				i++
			}
			continue
		}

		// Check for block comment.
		if i+1 < n && data[i] == '/' && data[i+1] == '*' {
			i += 2
			for i+1 < n {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			// Replace block comment with a space to maintain position context.
			result = append(result, ' ')
			continue
		}

		result = append(result, data[i])
		i++
	}

	return result
}

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

// Scanner holds resolved paths for containment validation.
type Scanner struct {
	globalConfigRoot string // canonicalized ~/.config/opencode
}

// NewScanner creates a Scanner for the given global OpenCode config directory.
// The root is canonicalized via filepath.EvalSymlinks.
func NewScanner(globalConfigRoot string) (*Scanner, error) {
	if globalConfigRoot == "" {
		return nil, fmt.Errorf("global config root is required")
	}
	canon, err := filepath.EvalSymlinks(globalConfigRoot)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve config root %s: %w", globalConfigRoot, err)
	}
	return &Scanner{globalConfigRoot: canon}, nil
}

// ScanGlobal scans the global OpenCode configuration directory.
// Missing global config returns an available empty inventory.
func (s *Scanner) ScanGlobal() *Inventory {
	inv := &Inventory{Available: true}

	// Read the main config file.
	configPath, configData, format, err := s.readConfigFile(s.globalConfigRoot)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		inv.Warnings = append(inv.Warnings, ConfigWarning{
			Source:  configPath,
			Message: fmt.Sprintf("cannot read config: %v", err),
		})
	}
	if configData != nil {
		inv.Sources = append(inv.Sources, ConfigSource{
			Path:   configPath,
			Scope:  "global",
			Format: format,
			Loaded: true,
		})
		s.parseConfig(inv, configData, "global", configPath)
	}

	// Scan subdirectories (even if main config is missing).
	s.scanDir(inv, "agents", s.globalConfigRoot, "global")
	s.scanDir(inv, "skills", s.globalConfigRoot, "global")
	s.scanDir(inv, "plugins", s.globalConfigRoot, "global")

	s.sortAll(inv)
	return inv
}

// ScanProject scans a registered project's .opencode directory.
func (s *Scanner) ScanProject(inv *Inventory, projectRoot string) {
	canonRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		inv.Warnings = append(inv.Warnings, ConfigWarning{
			Source:  projectRoot,
			Message: fmt.Sprintf("cannot resolve project root: %v", err),
		})
		return
	}

	projectConfigDir := filepath.Join(canonRoot, ".opencode")

	configPath, configData, format, err := s.readConfigFileInRoot(projectConfigDir, projectConfigDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		inv.Warnings = append(inv.Warnings, ConfigWarning{
			Source:  configPath,
			Message: fmt.Sprintf("cannot read project config: %v", err),
		})
	}
	if configData != nil {
		inv.Sources = append(inv.Sources, ConfigSource{
			Path:   configPath,
			Scope:  "project",
			Format: format,
			Loaded: true,
		})
		s.parseConfig(inv, configData, "project", configPath)
	}

	s.scanDir(inv, "agents", projectConfigDir, "project")
	s.scanDir(inv, "skills", projectConfigDir, "project")
	s.scanDir(inv, "plugins", projectConfigDir, "project")

	s.sortAll(inv)
}

// ---------------------------------------------------------------------------
// Path containment
// ---------------------------------------------------------------------------

// isContained returns true if path (canonical, resolved symlinks) is within root.
func isContained(path, root string) (bool, error) {
	canonRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}

	// Try to resolve symlinks on path. If it doesn't exist yet, resolve the parent.
	canonPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the file doesn't exist, resolve the parent directory and check containment there.
		parent := filepath.Dir(path)
		canonParent, pErr := filepath.EvalSymlinks(parent)
		if pErr != nil {
			return false, pErr
		}
		canonPath = filepath.Join(canonParent, filepath.Base(path))
	}

	rel, err := filepath.Rel(canonRoot, canonPath)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false, nil
	}
	return true, nil
}

// safeReadDir reads directory entries and rejects symlinks that escape the root.
func (s *Scanner) safeReadDir(dir string, root string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var safe []os.DirEntry
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())
		contained, cErr := isContained(fullPath, root)
		if cErr != nil || !contained {
			continue // Skip symlink escapes and traversal.
		}
		safe = append(safe, e)
	}
	return safe, nil
}

// ---------------------------------------------------------------------------
// Config file reading
// ---------------------------------------------------------------------------

func (s *Scanner) readConfigFile(configDir string) (path string, data []byte, format string, err error) {
	return s.readConfigFileInRoot(configDir, configDir)
}

func (s *Scanner) readConfigFileInRoot(configDir, root string) (path string, data []byte, format string, err error) {
	for _, name := range []string{"opencode.jsonc", "opencode.json"} {
		p := filepath.Join(configDir, name)
		d, fErr := safeReadInRoot(p, root)
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

func (s *Scanner) safeReadFile(path string) ([]byte, error) {
	return safeReadInRoot(path, s.globalConfigRoot)
}

// safeReadInRoot reads a file if it is contained within root.
func safeReadInRoot(path, root string) ([]byte, error) {
	// If root is empty, allow the read (caller is trusted to validate).
	if root == "" {
		return readFileWithLimit(path)
	}
	contained, err := isContained(path, root)
	if err != nil || !contained {
		return nil, fmt.Errorf("path traversal rejected: %s", path)
	}
	return readFileWithLimit(path)
}

func readFileWithLimit(path string) ([]byte, error) {
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

func (s *Scanner) parseConfig(inv *Inventory, data []byte, scope, sourcePath string) {
	if len(data) == 0 {
		return
	}

	cleaned := stripJSONCComments(data)

	var raw map[string]any
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		inv.Warnings = append(inv.Warnings, ConfigWarning{
			Source:  sourcePath,
			Message: fmt.Sprintf("parse error: %v", err),
		})
		return
	}

	s.parseAgents(inv, raw, scope, sourcePath)
	s.parseProviders(inv, raw, scope)
	s.parseMCP(inv, raw, scope, sourcePath)
	s.parsePlugins(inv, raw, scope, sourcePath)
	s.parseShell(inv, raw, scope)

	// Sanitize any remaining credential-bearing fields.
	s.sanitizeAll(inv)
}

func (s *Scanner) sanitizeAll(inv *Inventory) {
	for i := range inv.Agents {
		inv.Agents[i].Model = sanitizeModelField(inv.Agents[i].Model)
	}
	for i := range inv.MCPServers {
		inv.MCPServers[i].URL = sanitizeURL(inv.MCPServers[i].URL)
		inv.MCPServers[i].Command = sanitizeCommand(inv.MCPServers[i].Command)
	}
}

// sanitizeURL removes credentials from URLs (user:password@host).
func sanitizeURL(raw string) string {
	if raw == "" {
		return raw
	}
	// Find @ before the host portion.
	atIdx := strings.LastIndex(raw, "@")
	if atIdx < 0 {
		return raw
	}
	// Check for protocol:// before the @.
	protoEnd := strings.Index(raw, "://")
	if protoEnd >= 0 && protoEnd < atIdx {
		protoEnd += 3
		return raw[:protoEnd] + "<redacted>@" + raw[atIdx+1:]
	}
	return raw
}

// sanitizeCommand redacts credential-bearing command arguments.
func sanitizeCommand(raw string) string {
	if raw == "" {
		return raw
	}
	// Redact suspicious patterns: --api-key, --token, --password, etc.
	sensitiveFlags := []string{"--api-key", "--apikey", "--token", "--password", "--secret", "--key", "-p"}
	result := raw
	for _, flag := range sensitiveFlags {
		idx := strings.Index(strings.ToLower(result), strings.ToLower(flag))
		if idx >= 0 {
			// Find the space after this flag's value.
			afterFlag := result[idx+len(flag):]
			afterFlag = strings.TrimLeft(afterFlag, " =")
			spaceIdx := strings.Index(afterFlag, " ")
			if spaceIdx > 0 {
				result = result[:idx+len(flag)] + "=<redacted>" + afterFlag[spaceIdx:]
			} else {
				result = result[:idx+len(flag)] + "=<redacted>"
			}
		}
	}
	return result
}

// sanitizeModelField is a no-op placeholder for field-level sanitization.
func sanitizeModelField(val string) string { return val }

// sensitiveKeyPattern matches credential-bearing keys.
var sensitiveKeyPatterns = []string{
	"key", "token", "secret", "password", "passwd",
	"auth", "credential", "apikey", "api_key",
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, pat := range sensitiveKeyPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// stripCredentialFields removes known sensitive keys from a map.
func stripCredentialFields(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			result[k] = "<redacted>"
		} else if nested, ok := v.(map[string]any); ok {
			result[k] = stripCredentialFields(nested)
		} else {
			result[k] = v
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Sub-field parsers
// ---------------------------------------------------------------------------

func (s *Scanner) parseAgents(inv *Inventory, raw map[string]any, scope, sourcePath string) {
	agents, ok := raw["agent"].(map[string]any)
	if !ok {
		agents, _ = raw["agents"].(map[string]any)
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
			entry.Model = sanitizeModelField(model)
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
			entry.BaseURL = sanitizeURL(baseURL)
		}
		if baseURL, ok := cfg["base_url"].(string); ok {
			entry.BaseURL = sanitizeURL(baseURL)
		}
		inv.Providers = append(inv.Providers, entry)

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
	mcp, ok := raw["mcp"].(map[string]any)
	if !ok {
		mcp, _ = raw["mcpServers"].(map[string]any)
	}
	for name, val := range mcp {
		cfg, ok := val.(map[string]any)
		if !ok {
			continue
		}

		// Redact sensitive fields.
		cfg = stripCredentialFields(cfg)

		entry := MCPServerEntry{
			Name:       name,
			Scope:      scope,
			SourceFile: sourcePath,
		}
		if t, ok := cfg["type"].(string); ok {
			entry.Type = t
		}
		if url, ok := cfg["url"].(string); ok {
			entry.URL = sanitizeURL(url)
		}
		if cmd, ok := cfg["command"].(string); ok {
			entry.Command = sanitizeCommand(cmd)
		}
		if cmds, ok := cfg["command"].([]any); ok {
			parts := make([]string, 0, len(cmds))
			for _, c := range cmds {
				if s, ok := c.(string); ok {
					parts = append(parts, s)
				}
			}
			entry.Command = sanitizeCommand(strings.Join(parts, " "))
		}
		entry.Enabled = true
		if enabled, ok := cfg["enabled"].(bool); ok {
			entry.Enabled = enabled
		}
		// Strip env/headers fields entirely.
		delete(cfg, "env")
		delete(cfg, "headers")
		delete(cfg, "header")

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

func (s *Scanner) scanDir(inv *Inventory, subdir, configDir, scope string) {
	dirPath := filepath.Join(configDir, subdir)
	_, err := s.safeReadDir(dirPath, configDir)
	if err != nil {
		return
	}

	inv.Sources = append(inv.Sources, ConfigSource{
		Path:   dirPath,
		Scope:  scope,
		Format: "directory",
		Loaded: true,
	})

	switch subdir {
	case "agents":
		s.scanAgentsDir(inv, dirPath, scope)
	case "skills":
		s.scanSkillsDir(inv, dirPath, scope)
	case "plugins":
		s.scanPluginsDir(inv, dirPath, scope)
	}
}

func (s *Scanner) scanAgentsDir(inv *Inventory, agentsDir, scope string) {
	entries, err := s.safeReadDir(agentsDir, agentsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		agentName := strings.TrimSuffix(name, ".md")
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

func (s *Scanner) scanSkillsDir(inv *Inventory, skillsDir, scope string) {
	entries, err := s.safeReadDir(skillsDir, skillsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		skillFile := filepath.Join(skillsDir, skillName, "SKILL.md")
		desc := ""
		if data, err := s.safeReadFile(skillFile); err == nil {
			desc = extractFirstLine(string(data), 120)
		}
		inv.Skills = append(inv.Skills, SkillEntry{
			Name:        skillName,
			Scope:       scope,
			SourceFile:  filepath.Join(skillsDir, skillName),
			Description: desc,
			Enabled:     true,
		})
	}
}

func (s *Scanner) scanPluginsDir(inv *Inventory, pluginsDir, scope string) {
	entries, err := s.safeReadDir(pluginsDir, pluginsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginName := entry.Name()
		if s.hasPlugin(inv, pluginName) {
			continue
		}
		inv.Plugins = append(inv.Plugins, PluginEntry{
			ID:         pluginName,
			Scope:      scope,
			SourceFile: filepath.Join(pluginsDir, pluginName),
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

func extractFirstLine(text string, maxLen int) string {
	lines := strings.SplitN(text, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if len(first) > maxLen {
		first = first[:maxLen] + "..."
	}
	return first
}

func stripJSONCComments(data []byte) []byte {
	var result []byte
	i := 0
	n := len(data)
	for i < n {
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
		if i+1 < n && data[i] == '/' && data[i+1] == '/' {
			for i < n && data[i] != '\n' {
				i++
			}
			if i < n {
				result = append(result, '\n')
				i++
			}
			continue
		}
		if i+1 < n && data[i] == '/' && data[i+1] == '*' {
			i += 2
			for i+1 < n {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			result = append(result, ' ')
			continue
		}
		result = append(result, data[i])
		i++
	}
	return result
}

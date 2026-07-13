// OpenCode config inventory types — mirrors backend config.Inventory response.

export interface ConfigSource {
  path: string
  scope: "global" | "project"
  format: string
  loaded: boolean
  warning?: string
}

export interface AgentEntry {
  name: string
  scope: "global" | "project"
  source: string
  sourceFile?: string
  role?: string
  model?: string
  description?: string
  enabled: boolean
}

export interface ProviderEntry {
  id: string
  scope: "global" | "project"
  name?: string
  baseUrl?: string
  enabled: boolean
}

export interface ModelEntry {
  id: string
  provider: string
  scope: "global" | "project"
  name?: string
  enabled: boolean
}

export interface MCPServerEntry {
  name: string
  type: string
  scope: "global" | "project"
  sourceFile?: string
  url?: string
  command?: string
  enabled: boolean
}

export interface SkillEntry {
  name: string
  scope: "global" | "project"
  sourceFile?: string
  description?: string
  enabled: boolean
}

export interface PluginEntry {
  id: string
  scope: "global" | "project"
  sourceFile?: string
  type?: string
  enabled: boolean
}

export interface ShellProfile {
  name: string
  scope: "global" | "project"
}

export interface ConfigWarning {
  source: string
  message: string
}

export interface ConfigInventory {
  available: boolean
  reason: string
  sources: ConfigSource[]
  agents: AgentEntry[]
  providers: ProviderEntry[]
  models: ModelEntry[]
  mcpServers: MCPServerEntry[]
  skills: SkillEntry[]
  plugins: PluginEntry[]
  shellProfiles: ShellProfile[]
  warnings: ConfigWarning[]
}

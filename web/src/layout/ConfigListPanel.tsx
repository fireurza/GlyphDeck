import { useState, useEffect, useCallback } from "react"
import { requestJson } from "../api/client"
import type { ConfigInventory, MCPServerEntry, SkillEntry, PluginEntry } from "../types/config"

type Category = "mcpServers" | "skills" | "plugins"
type Entry = MCPServerEntry | SkillEntry | PluginEntry

interface ConfigListPanelProps {
  category: Category
  title: string
  selectedProjectId?: string | null
}

const CATEGORY_TEST_IDS: Record<Category, string> = {
  mcpServers: "mcp",
  skills: "skills",
  plugins: "plugins",
}

function getEntryId(entry: Entry, category: Category): string {
  if (category === "mcpServers") return (entry as MCPServerEntry).name
  if (category === "plugins") return (entry as PluginEntry).id
  return (entry as SkillEntry).name
}

function getEntryMeta(entry: Entry, category: Category): Record<string, string> {
  if (category === "mcpServers") {
    const e = entry as MCPServerEntry
    return { type: e.type, url: e.url || "", command: e.command || "" }
  }
  if (category === "plugins") {
    const e = entry as PluginEntry
    return { type: e.type || "" }
  }
  const e = entry as SkillEntry
  return { description: e.description || "" }
}

function ConfigListPanel({ category, title, selectedProjectId }: ConfigListPanelProps) {
  const [entries, setEntries] = useState<Entry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [scopeFilter, setScopeFilter] = useState<"all" | "global" | "project">("all")

  const testId = CATEGORY_TEST_IDS[category]

  const loadConfig = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const params = selectedProjectId ? `?projectId=${encodeURIComponent(selectedProjectId)}` : ""
      const data = await requestJson<ConfigInventory>(`/api/opencode/config/inventory${params}`)
      setEntries((data[category] || []) as Entry[])
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load.")
    } finally {
      setLoading(false)
    }
  }, [category, selectedProjectId])

  useEffect(() => {
    loadConfig()
  }, [loadConfig])

  const filtered = scopeFilter === "all"
    ? entries
    : entries.filter((e) => e.scope === scopeFilter)

  if (loading) {
    return (
      <div className="panel-body panel-placeholder" data-testid={`${testId}-panel`}>
        <p>Loading {title.toLowerCase()}…</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="panel-body panel-placeholder" data-testid={`${testId}-panel`}>
        <p className="panel-error">{error}</p>
        <button className="panel-refresh-btn" onClick={loadConfig}>Retry</button>
      </div>
    )
  }

  return (
    <div className="config-panel" data-testid={`${testId}-panel`}>
      <div className="config-panel__toolbar">
        <span className="config-panel__title">{title}</span>
        <div className="config-panel__filters">
          <button className={`config-filter-btn ${scopeFilter === "all" ? "active" : ""}`} onClick={() => setScopeFilter("all")} data-testid={`${testId}-filter-all`}>All</button>
          <button className={`config-filter-btn ${scopeFilter === "global" ? "active" : ""}`} onClick={() => setScopeFilter("global")} data-testid={`${testId}-filter-global`}>Global</button>
          {selectedProjectId && (
            <button className={`config-filter-btn ${scopeFilter === "project" ? "active" : ""}`} onClick={() => setScopeFilter("project")} data-testid={`${testId}-filter-project`}>Project</button>
          )}
          <button className="config-refresh-btn" onClick={loadConfig} data-testid={`${testId}-refresh`} title="Refresh">↻</button>
        </div>
      </div>
      <div className="panel-body">
        {filtered.length === 0 ? (
          <div className="config-empty" data-testid={`${testId}-empty`}>
            <p>No {title.toLowerCase()} configured.</p>
          </div>
        ) : (
          <ul className="config-list" data-testid={`${testId}-list`}>
            {filtered.map((entry) => {
              const id = getEntryId(entry, category)
              const meta = getEntryMeta(entry, category)
              return (
                <li key={id} className="config-item" data-testid={`${testId}-${id}`}>
                  <div className="config-item__header">
                    <span className="config-item__name">{id}</span>
                    <span className={`config-badge config-badge--${entry.scope}`} data-testid={`${testId}-${id}-scope`}>
                      {entry.scope}
                    </span>
                    {meta.type && <span className="config-badge config-badge--type">{meta.type}</span>}
                    {!entry.enabled && <span className="config-badge config-badge--disabled">disabled</span>}
                  </div>
                  {meta.description && <p className="config-item__desc">{meta.description}</p>}
                  {meta.url && <p className="config-item__meta">{meta.url}</p>}
                  {meta.command && <p className="config-item__meta config-item__path">{meta.command}</p>}
                  {entry.sourceFile && <p className="config-item__meta config-item__path">{entry.sourceFile}</p>}
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </div>
  )
}

export default ConfigListPanel

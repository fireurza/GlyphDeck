import { useState, useEffect, useCallback } from "react"
import { requestJson } from "../api/client"
import type { ConfigInventory, AgentEntry } from "../types/config"

interface AgentsPanelProps {
  selectedProjectId?: string | null
}

function AgentsPanel({ selectedProjectId }: AgentsPanelProps) {
  const [agents, setAgents] = useState<AgentEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [scopeFilter, setScopeFilter] = useState<"all" | "global" | "project">("all")

  const loadConfig = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const params = selectedProjectId ? `?projectId=${encodeURIComponent(selectedProjectId)}` : ""
      const data = await requestJson<ConfigInventory>(`/api/opencode/config/inventory${params}`)
      setAgents(data.agents || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agents.")
    } finally {
      setLoading(false)
    }
  }, [selectedProjectId])

  useEffect(() => {
    loadConfig()
  }, [loadConfig])

  const filtered = scopeFilter === "all" ? agents : agents.filter((a) => a.scope === scopeFilter)

  if (loading) {
    return (
      <div className="panel-body panel-placeholder" data-testid="agents-panel">
        <p>Loading agents…</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="panel-body panel-placeholder" data-testid="agents-panel">
        <p className="panel-error">{error}</p>
        <button className="panel-refresh-btn" onClick={loadConfig}>Retry</button>
      </div>
    )
  }

  return (
    <div className="config-panel" data-testid="agents-panel">
      <div className="config-panel__toolbar">
        <span className="config-panel__title">Agents</span>
        <div className="config-panel__filters">
          <button
            className={`config-filter-btn ${scopeFilter === "all" ? "active" : ""}`}
            onClick={() => setScopeFilter("all")}
            data-testid="agents-filter-all"
          >
            All
          </button>
          <button
            className={`config-filter-btn ${scopeFilter === "global" ? "active" : ""}`}
            onClick={() => setScopeFilter("global")}
            data-testid="agents-filter-global"
          >
            Global
          </button>
          {selectedProjectId && (
            <button
              className={`config-filter-btn ${scopeFilter === "project" ? "active" : ""}`}
              onClick={() => setScopeFilter("project")}
              data-testid="agents-filter-project"
            >
              Project
            </button>
          )}
          <button className="config-refresh-btn" onClick={loadConfig} data-testid="agents-refresh" title="Refresh">
            ↻
          </button>
        </div>
      </div>
      <div className="panel-body">
        {filtered.length === 0 ? (
          <div className="config-empty" data-testid="agents-empty">
            <p>{scopeFilter === "all" ? "No agents configured." : `No ${scopeFilter} agents configured.`}</p>
            <p className="panel-hint">Add agents in opencode.json or the agents/ directory.</p>
          </div>
        ) : (
          <ul className="config-list" data-testid="agents-list">
            {filtered.map((a) => (
              <li key={a.name} className="config-item" data-testid={`agent-${a.name}`}>
                <div className="config-item__header">
                  <span className="config-item__name">{a.name}</span>
                  <span className={`config-badge config-badge--${a.scope}`} data-testid={`agent-${a.name}-scope`}>
                    {a.scope}
                  </span>
                  {a.role && <span className="config-badge config-badge--role">{a.role}</span>}
                </div>
                {a.description && <p className="config-item__desc">{a.description}</p>}
                {a.model && (
                  <p className="config-item__meta" data-testid={`agent-${a.name}-model`}>
                    Model: {a.model}
                  </p>
                )}
                {a.sourceFile && (
                  <p className="config-item__meta config-item__path" data-testid={`agent-${a.name}-source`}>
                    {a.sourceFile}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

export default AgentsPanel

import { useEffect, useState, useCallback } from 'react'

interface ServerConfig {
  id: string
  name: string
  type: string
  url: string
  sshAlias: string
}

interface ActiveServer {
  serverId: string
  baseUrl: string
  attached: boolean
}

function ServersPanel() {
  const [configs, setConfigs] = useState<ServerConfig[]>([])
  const [active, setActive] = useState<ActiveServer | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Add form state
  const [showAdd, setShowAdd] = useState(false)
  const [addId, setAddId] = useState('')
  const [addName, setAddName] = useState('')
  const [addType, setAddType] = useState('local')
  const [addUrl, setAddUrl] = useState('')
  const [addSshAlias, setAddSshAlias] = useState('')

  const loadConfigs = useCallback(async () => {
    try {
      const resp = await fetch('/api/server-configs')
      if (!resp.ok) throw new Error('Failed to load server configs')
      const data = await resp.json()
      setConfigs(data.configs || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load servers')
    }
  }, [])

  const loadActive = useCallback(async () => {
    try {
      const resp = await fetch('/api/active-server')
      if (!resp.ok) return
      const data = await resp.json()
      setActive(data.attached ? data : null)
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    async function init() {
      setLoading(true)
      await Promise.all([loadConfigs(), loadActive()])
      if (!cancelled) setLoading(false)
    }
    init()
    return () => { cancelled = true }
  }, [loadConfigs, loadActive])

  const handleAdd = useCallback(async () => {
    if (!addId || !addName) return
    try {
      const resp = await fetch('/api/server-configs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: addId, name: addName, type: addType, url: addUrl, sshAlias: addSshAlias }),
      })
      if (!resp.ok) throw new Error('Failed to add server config')
      await loadConfigs()
      setShowAdd(false)
      setAddId('')
      setAddName('')
      setAddType('local')
      setAddUrl('')
      setAddSshAlias('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add server')
    }
  }, [addId, addName, addType, addUrl, addSshAlias, loadConfigs])

  const handleDelete = useCallback(async (id: string) => {
    try {
      const resp = await fetch(`/api/server-configs/${encodeURIComponent(id)}`, { method: 'DELETE' })
      if (!resp.ok) throw new Error('Failed to delete server config')
      await loadConfigs()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete server')
    }
  }, [loadConfigs])

  const handleAttach = useCallback(async (cfg: ServerConfig) => {
    try {
      const resp = await fetch('/api/active-server/attach', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ serverId: cfg.id, baseUrl: cfg.url || `http://127.0.0.1:4096` }),
      })
      if (!resp.ok) throw new Error('Failed to attach')
      await loadActive()
      // Clear stale state: sessions, review, usage will be handled by App-level detach
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to attach to server')
    }
  }, [loadActive])

  const handleDetach = useCallback(async () => {
    try {
      const resp = await fetch('/api/active-server/detach', { method: 'POST' })
      if (!resp.ok) throw new Error('Failed to detach')
      setActive(null)
      // App-level handler will clear sessions/review/usage
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to detach')
    }
  }, [])

  if (loading) {
    return (
      <aside className="left-panel">
        <div className="panel-header">Servers</div>
        <div className="panel-body" data-testid="left-panel-body">
          <p className="project-message">Loading servers…</p>
        </div>
      </aside>
    )
  }

  return (
    <aside className="left-panel">
      <div className="panel-header">Servers</div>
      <div className="panel-body" data-testid="left-panel-body">
        {error ? (
          <p className="project-message project-message--error" role="alert">{error}</p>
        ) : null}

        {/* Active server indicator */}
        <div className="server-active-section">
          {active ? (
            <div className="server-active server-active--online">
              <span className="server-active__label">Attached to:</span>
              <span className="server-active__name">{active.serverId}</span>
              <button
                className="server-btn server-btn--detach"
                onClick={handleDetach}
                data-testid="detach-server-button"
              >
                Detach
              </button>
            </div>
          ) : (
            <div className="server-active server-active--none">
              <span className="server-active__label">No server attached</span>
            </div>
          )}
        </div>

        {/* Server config list */}
        <div className="server-list" aria-live="polite">
          {configs.length === 0 && !showAdd ? (
            <p className="project-message project-message--empty">No servers configured.</p>
          ) : null}

          {configs.map((cfg) => {
            const isActive = active?.serverId === cfg.id
            return (
              <div className="server-card" key={cfg.id} data-testid={`server-card-${cfg.id}`}>
                <div className="server-card__main">
                  <h3 className="server-card__name">{cfg.name}</h3>
                  <p className="server-card__type">{cfg.type === 'local' ? 'Local' : cfg.type === 'manual_url' ? 'Manual URL' : 'SSH Alias'}</p>
                  {cfg.url ? <p className="server-card__url">{cfg.url}</p> : null}
                  {cfg.sshAlias ? <p className="server-card__ssh">SSH: {cfg.sshAlias}</p> : null}
                </div>
                <div className="server-card__actions">
                  {isActive ? (
                    <span className="server-badge server-badge--active">Active</span>
                  ) : (
                    <button
                      className="server-btn server-btn--attach"
                      onClick={() => handleAttach(cfg)}
                      data-testid={`attach-${cfg.id}`}
                    >
                      Attach
                    </button>
                  )}
                  <button
                    className="server-btn server-btn--remove"
                    onClick={() => handleDelete(cfg.id)}
                    disabled={isActive}
                    data-testid={`remove-server-${cfg.id}`}
                  >
                    Remove
                  </button>
                </div>
              </div>
            )
          })}
        </div>

        {/* Add form */}
        {showAdd ? (
          <form className="server-form" onSubmit={(e) => { e.preventDefault(); handleAdd() }}>
            <div className="server-form__field">
              <label htmlFor="server-id">ID</label>
              <input id="server-id" value={addId} onChange={(e) => setAddId(e.target.value)} required data-testid="server-add-id" />
            </div>
            <div className="server-form__field">
              <label htmlFor="server-name">Name</label>
              <input id="server-name" value={addName} onChange={(e) => setAddName(e.target.value)} required data-testid="server-add-name" />
            </div>
            <div className="server-form__field">
              <label htmlFor="server-type">Type</label>
              <select id="server-type" value={addType} onChange={(e) => setAddType(e.target.value)} data-testid="server-add-type">
                <option value="local">Local</option>
                <option value="manual_url">Manual URL</option>
                <option value="ssh_alias">SSH Alias</option>
              </select>
            </div>
            {addType === 'manual_url' && (
              <div className="server-form__field">
                <label htmlFor="server-url">URL</label>
                <input id="server-url" value={addUrl} onChange={(e) => setAddUrl(e.target.value)} placeholder="http://127.0.0.1:4096" data-testid="server-add-url" />
              </div>
            )}
            {addType === 'ssh_alias' && (
              <div className="server-form__field">
                <label htmlFor="server-ssh-alias">SSH Alias</label>
                <input id="server-ssh-alias" value={addSshAlias} onChange={(e) => setAddSshAlias(e.target.value)} placeholder="my-server" data-testid="server-add-ssh-alias" />
              </div>
            )}
            <div className="server-form__actions">
              <button type="submit" className="server-btn server-btn--save" data-testid="server-add-submit">Add Server</button>
              <button type="button" className="server-btn server-btn--cancel" onClick={() => setShowAdd(false)}>Cancel</button>
            </div>
          </form>
        ) : (
          <button className="server-btn server-btn--add" onClick={() => setShowAdd(true)} data-testid="server-add-button">
            + Add Server
          </button>
        )}
      </div>
    </aside>
  )
}

export default ServersPanel

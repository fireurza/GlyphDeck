import { useEffect, useState, useCallback } from 'react'

type ServerTypeLabel = 'local' | 'manual_url' | 'ssh_alias'

interface ServerConfig {
  id: string
  name: string
  type: ServerTypeLabel
  url: string
  sshAlias: string
  workingDir?: string
  startCommand?: string
  stopCommand?: string
  statusCommand?: string
  lastPid: number
  lastUrl: string
  lastStatus: string
  lastCheckedAt: string
  startedByGlyphdeck: boolean
}

interface ActiveServer {
  serverId: string
  baseUrl: string
  attached: boolean
}

type TransientOp = 'testing-ssh' | 'detecting' | 'starting' | 'stopping' | null

interface TargetState {
  status: 'online' | 'offline' | 'unknown'
  op: TransientOp
  lastError: string
  lastMsg: string
  lastCheckedAt: string
}

function statusDot(status: string | undefined, op: TransientOp) {
  const effective = op ? 'unknown' : status || 'unknown'
  const color =
    effective === 'online'
      ? 'green'
      : effective === 'offline'
        ? 'red'
        : 'gray'
  return (
    <span
      className={`server-status-dot server-status-dot--${color}`}
      title={effective}
      data-testid={`status-dot-${effective}`}
    />
  )
}

function formatCheckedAt(at: string): string {
  if (!at) return ''
  try {
    const d = new Date(at)
    if (isNaN(d.getTime())) return ''
    return d.toLocaleString()
  } catch {
    return ''
  }
}

function ServersPanel() {
  const [configs, setConfigs] = useState<ServerConfig[]>([])
  const [active, setActive] = useState<ActiveServer | null>(null)
  const [targetStates, setTargetStates] = useState<Record<string, TargetState>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Add form
  const [showAdd, setShowAdd] = useState(false)
  const [addId, setAddId] = useState('')
  const [addName, setAddName] = useState('')
  const [addType, setAddType] = useState<ServerTypeLabel>('local')
  const [addUrl, setAddUrl] = useState('')
  const [addSshAlias, setAddSshAlias] = useState('')
  const [addWorkingDir, setAddWorkingDir] = useState('')
  const [addStartCmd, setAddStartCmd] = useState('')
  const [addStopCmd, setAddStopCmd] = useState('')
  const [addStatusCmd, setAddStatusCmd] = useState('')
  const [addErrors, setAddErrors] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)

  // Edit mode — one at a time
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [editType, setEditType] = useState<ServerTypeLabel>('local')
  const [editUrl, setEditUrl] = useState('')
  const [editSshAlias, setEditSshAlias] = useState('')
  const [editWorkingDir, setEditWorkingDir] = useState('')
  const [editStartCmd, setEditStartCmd] = useState('')
  const [editStopCmd, setEditStopCmd] = useState('')
  const [editStatusCmd, setEditStatusCmd] = useState('')
  const [editErrors, setEditErrors] = useState<Record<string, string>>({})
  const [savingEdit, setSavingEdit] = useState(false)

  // Delete confirmation
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const getState = useCallback(
    (id: string): TargetState =>
      targetStates[id] || { status: 'unknown', op: null, lastError: '', lastMsg: '', lastCheckedAt: '' },
    [targetStates],
  )

  const setState = useCallback((id: string, patch: Partial<TargetState>) => {
    setTargetStates((prev) => {
      const existing = prev[id] || { status: 'unknown', op: null, lastError: '', lastMsg: '', lastCheckedAt: '' }
      return {
        ...prev,
        [id]: { ...existing, ...patch },
      }
    })
  }, [])

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
      /* ignore */
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
    return () => {
      cancelled = true
    }
  }, [loadConfigs, loadActive])

  // ---- Validation ----
  function validateForm(name: string, typ: ServerTypeLabel, sshAlias: string): Record<string, string> {
    const errs: Record<string, string> = {}
    if (!name.trim()) errs.name = 'Name is required'
    if (typ === 'ssh_alias' && !sshAlias.trim()) errs.sshAlias = 'SSH alias is required'
    return errs
  }

  // ---- Add ----
  const handleAdd = useCallback(async () => {
    const errs = validateForm(addName, addType, addSshAlias)
    setAddErrors(errs)
    if (Object.keys(errs).length > 0) return

    setSaving(true)
    try {
      const resp = await fetch('/api/server-configs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: addId || undefined,
          name: addName,
          type: addType,
          url: addUrl,
          sshAlias: addSshAlias,
          workingDir: addWorkingDir,
          startCommand: addStartCmd,
          stopCommand: addStopCmd,
          statusCommand: addStatusCmd,
        }),
      })
      if (!resp.ok) throw new Error('Failed to add server config')
      await loadConfigs()
      setShowAdd(false)
      resetAddForm()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add server')
    } finally {
      setSaving(false)
    }
  }, [addName, addType, addUrl, addSshAlias, addWorkingDir, addStartCmd, addStopCmd, addStatusCmd, addId, loadConfigs])

  function resetAddForm() {
    setAddId('')
    setAddName('')
    setAddType('local')
    setAddUrl('')
    setAddSshAlias('')
    setAddWorkingDir('')
    setAddStartCmd('')
    setAddStopCmd('')
    setAddStatusCmd('')
    setAddErrors({})
  }

  function cancelAdd() {
    setShowAdd(false)
    resetAddForm()
  }

  // ---- Edit ----
  function beginEdit(cfg: ServerConfig) {
    setEditingId(cfg.id)
    setEditName(cfg.name)
    setEditType(cfg.type)
    setEditUrl(cfg.url)
    setEditSshAlias(cfg.sshAlias)
    setEditWorkingDir(cfg.workingDir || '')
    setEditStartCmd(cfg.startCommand || '')
    setEditStopCmd(cfg.stopCommand || '')
    setEditStatusCmd(cfg.statusCommand || '')
    setEditErrors({})
  }

  function cancelEdit() {
    setEditingId(null)
    setEditErrors({})
  }

  const handleEditSave = useCallback(async () => {
    if (!editingId) return
    const errs = validateForm(editName, editType, editSshAlias)
    setEditErrors(errs)
    if (Object.keys(errs).length > 0) return

    setSavingEdit(true)
    try {
      const resp = await fetch(`/api/server-configs/${encodeURIComponent(editingId)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: editName,
          type: editType,
          url: editUrl,
          sshAlias: editSshAlias,
          workingDir: editWorkingDir,
          startCommand: editStartCmd,
          stopCommand: editStopCmd,
          statusCommand: editStatusCmd,
        }),
      })
      if (!resp.ok) throw new Error('Failed to update server config')
      await loadConfigs()
      setEditingId(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update server')
    } finally {
      setSavingEdit(false)
    }
  }, [editingId, editName, editType, editUrl, editSshAlias, editWorkingDir, editStartCmd, editStopCmd, editStatusCmd, loadConfigs])

  // ---- Delete ----
  function confirmDelete(id: string) {
    const cfg = configs.find((c) => c.id === id)
    if (!cfg) return
    setDeleteConfirm(id)
  }

  function cancelDelete() {
    setDeleteConfirm(null)
  }

  const handleDelete = useCallback(
    async (id: string) => {
      setDeleteConfirm(null)
      setDeletingId(id)
      try {
        const resp = await fetch(`/api/server-configs/${encodeURIComponent(id)}`, { method: 'DELETE' })
        if (!resp.ok) throw new Error('Failed to delete server config')
        await loadConfigs()
        setTargetStates((prev) => {
          const next = { ...prev }
          delete next[id]
          return next
        })
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to delete server')
      } finally {
        setDeletingId(null)
      }
    },
    [loadConfigs],
  )

  // ---- Attach / Detach ----
  const handleAttach = useCallback(
    async (cfg: ServerConfig) => {
      try {
        const resp = await fetch('/api/active-server/attach', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ serverId: cfg.id, baseUrl: cfg.url || `http://127.0.0.1:4096` }),
        })
        if (!resp.ok) throw new Error('Failed to attach')
        await loadActive()
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to attach to server')
      }
    },
    [loadActive],
  )

  const handleDetach = useCallback(async () => {
    try {
      const resp = await fetch('/api/active-server/detach', { method: 'POST' })
      if (!resp.ok) throw new Error('Failed to detach')
      setActive(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to detach')
    }
  }, [])

  // ---- Check status ----
  const handleCheck = useCallback(
    async (cfg: ServerConfig) => {
      setState(cfg.id, { op: null })
      try {
        const resp = await fetch(`/api/server-configs/${encodeURIComponent(cfg.id)}/check`, { method: 'POST' })
        if (!resp.ok) throw new Error('Check failed')
        const data = await resp.json()
        setState(cfg.id, {
          status: data.status,
          lastCheckedAt: new Date().toISOString(),
          lastError: '',
        })
      } catch {
        setState(cfg.id, { status: 'offline', lastError: 'Check failed' })
      }
    },
    [setState],
  )

  // ---- Remote lifecycle actions ----
  async function remoteAction(cfg: ServerConfig, action: string) {
    const opMap: Record<string, TransientOp> = {
      'test-ssh': 'testing-ssh',
      detect: 'detecting',
      'start-remote': 'starting',
      'stop-remote': 'stopping',
    }
    setState(cfg.id, { op: opMap[action] || null, lastError: '', lastMsg: '' })
    try {
      const resp = await fetch(`/api/server-configs/${encodeURIComponent(cfg.id)}/${action}`, { method: 'POST' })
      const data = await resp.json()
      setState(cfg.id, {
        op: null,
        lastMsg: data.message || JSON.stringify(data),
        lastError: resp.ok ? '' : data.message || 'Action failed',
        status: action === 'detect' ? data.status || 'unknown' : undefined,
      } as Partial<TargetState> as TargetState)
      if (action === 'start-remote' || action === 'stop-remote') {
        await loadConfigs()
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed'
      setState(cfg.id, { op: null, lastError: msg, lastMsg: '' })
    }
  }

  // ---- Render ----
  if (loading) {
    return (
      <aside className="left-panel" data-testid="servers-panel">
        <div className="panel-header">Servers</div>
        <div className="panel-body" data-testid="left-panel-body">
          <p className="project-message">Loading servers…</p>
        </div>
      </aside>
    )
  }

  const canAttach = (cfg: ServerConfig) => {
    const st = getState(cfg.id)
    return cfg.url && cfg.url.trim() !== '' && st.status === 'online' && !active
  }

  return (
    <aside className="left-panel" data-testid="servers-panel">
      <div className="panel-header">Servers</div>
      <div className="panel-body" data-testid="left-panel-body">
        {error ? (
          <div className="server-error-banner" role="alert" data-testid="server-error-banner">
            <p>{error}</p>
            <button className="server-btn server-btn--dismiss" onClick={() => setError(null)}>
              Dismiss
            </button>
          </div>
        ) : null}

        {/* Active server banner */}
        <div className="server-active-section">
          {active ? (
            <div className="server-active server-active--online" data-testid="active-server-banner">
              <span className="server-active__label">Active:</span>
              <span className="server-active__name">{active.serverId}</span>
              <span className="server-active__url">{active.baseUrl}</span>
              <button className="server-btn server-btn--detach" onClick={handleDetach} data-testid="detach-server-button">
                Detach
              </button>
            </div>
          ) : (
            <div className="server-active server-active--none" data-testid="no-active-server">
              <span className="server-active__label">No server attached</span>
            </div>
          )}
        </div>

        {/* Config cards */}
        <div className="server-list" aria-live="polite">
          {configs.length === 0 && !showAdd ? (
            <p className="project-message project-message--empty">No servers configured.</p>
          ) : null}

          {configs.map((cfg) => {
            const isActive = active?.serverId === cfg.id
            const st = getState(cfg.id)
            const isEditing = editingId === cfg.id
            const isConfirmingDelete = deleteConfirm === cfg.id
            const isAnyOp = st.op !== null || deletingId === cfg.id

            return (
              <div
                className={`server-card ${isActive ? 'server-card--active' : ''}`}
                key={cfg.id}
                data-testid={`server-card-${cfg.id}`}
              >
                {isEditing ? (
                  /* ---- Edit form ---- */
                  <form
                    className="server-form server-form--edit"
                    onSubmit={(e) => {
                      e.preventDefault()
                      handleEditSave()
                    }}
                    data-testid={`edit-form-${cfg.id}`}
                  >
                    <div className="server-form__field">
                      <label htmlFor={`edit-name-${cfg.id}`}>Name *</label>
                      <input
                        id={`edit-name-${cfg.id}`}
                        value={editName}
                        onChange={(e) => setEditName(e.target.value)}
                        data-testid={`edit-name-${cfg.id}`}
                      />
                      {editErrors.name ? <span className="server-form__error">{editErrors.name}</span> : null}
                    </div>
                    <div className="server-form__field">
                      <label htmlFor={`edit-type-${cfg.id}`}>Type</label>
                      <select
                        id={`edit-type-${cfg.id}`}
                        value={editType}
                        onChange={(e) => setEditType(e.target.value as ServerTypeLabel)}
                        data-testid={`edit-type-${cfg.id}`}
                      >
                        <option value="local">Local</option>
                        <option value="manual_url">Manual URL</option>
                        <option value="ssh_alias">SSH Alias</option>
                      </select>
                    </div>
                    {editType === 'manual_url' && (
                      <div className="server-form__field">
                        <label htmlFor={`edit-url-${cfg.id}`}>URL</label>
                        <input
                          id={`edit-url-${cfg.id}`}
                          value={editUrl}
                          onChange={(e) => setEditUrl(e.target.value)}
                          placeholder="http://127.0.0.1:4096"
                          data-testid={`edit-url-${cfg.id}`}
                        />
                      </div>
                    )}
                    {editType === 'ssh_alias' && (
                      <>
                        <div className="server-form__field">
                          <label htmlFor={`edit-ssh-alias-${cfg.id}`}>SSH Alias *</label>
                          <input
                            id={`edit-ssh-alias-${cfg.id}`}
                            value={editSshAlias}
                            onChange={(e) => setEditSshAlias(e.target.value)}
                            placeholder="my-server"
                            data-testid={`edit-ssh-alias-${cfg.id}`}
                          />
                          {editErrors.sshAlias ? <span className="server-form__error">{editErrors.sshAlias}</span> : null}
                        </div>
                        <div className="server-form__field">
                          <label htmlFor={`edit-working-dir-${cfg.id}`}>Working Directory</label>
                          <input
                            id={`edit-working-dir-${cfg.id}`}
                            value={editWorkingDir}
                            onChange={(e) => setEditWorkingDir(e.target.value)}
                            placeholder="~"
                            data-testid={`edit-working-dir-${cfg.id}`}
                          />
                        </div>
                        <div className="server-form__field">
                          <label htmlFor={`edit-start-cmd-${cfg.id}`}>Start Command</label>
                          <input
                            id={`edit-start-cmd-${cfg.id}`}
                            value={editStartCmd}
                            onChange={(e) => setEditStartCmd(e.target.value)}
                            data-testid={`edit-start-cmd-${cfg.id}`}
                          />
                        </div>
                        <div className="server-form__field">
                          <label htmlFor={`edit-stop-cmd-${cfg.id}`}>Stop Command</label>
                          <input
                            id={`edit-stop-cmd-${cfg.id}`}
                            value={editStopCmd}
                            onChange={(e) => setEditStopCmd(e.target.value)}
                            data-testid={`edit-stop-cmd-${cfg.id}`}
                          />
                        </div>
                        <div className="server-form__field">
                          <label htmlFor={`edit-status-cmd-${cfg.id}`}>Status Command</label>
                          <input
                            id={`edit-status-cmd-${cfg.id}`}
                            value={editStatusCmd}
                            onChange={(e) => setEditStatusCmd(e.target.value)}
                            data-testid={`edit-status-cmd-${cfg.id}`}
                          />
                        </div>
                      </>
                    )}
                    <div className="server-form__actions">
                      <button type="submit" className="server-btn server-btn--save" disabled={savingEdit} data-testid={`edit-save-${cfg.id}`}>
                        {savingEdit ? 'Saving…' : 'Save'}
                      </button>
                      <button type="button" className="server-btn server-btn--cancel" onClick={cancelEdit} data-testid={`edit-cancel-${cfg.id}`}>
                        Cancel
                      </button>
                    </div>
                  </form>
                ) : isConfirmingDelete ? (
                  /* ---- Delete confirmation ---- */
                  <div className="server-delete-confirm" data-testid={`delete-confirm-${cfg.id}`}>
                    <p className="server-delete-confirm__text">
                      Delete <strong>{cfg.name}</strong>?
                      {isActive && <span className="server-delete-confirm__warn">Attached targets should be detached first.</span>}
                      {cfg.lastPid > 0 && cfg.startedByGlyphdeck && (
                        <span className="server-delete-confirm__warn">A GlyphDeck-owned process (PID {cfg.lastPid}) is still recorded. Deleting the config will not stop it.</span>
                      )}
                    </p>
                    <div className="server-delete-confirm__actions">
                      <button className="server-btn server-btn--danger" onClick={() => handleDelete(cfg.id)} data-testid={`delete-confirm-yes-${cfg.id}`}>
                        Delete
                      </button>
                      <button className="server-btn server-btn--cancel" onClick={cancelDelete} data-testid={`delete-confirm-no-${cfg.id}`}>
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : (
                  /* ---- Card display ---- */
                  <>
                    <div className="server-card__main">
                      <div className="server-card__header">
                        {statusDot(st.status, st.op)}
                        <h3 className="server-card__name">{cfg.name}</h3>
                        {isActive && (
                          <span className="server-badge server-badge--active" data-testid={`active-badge-${cfg.id}`}>
                            ✓ Active
                          </span>
                        )}
                        {cfg.startedByGlyphdeck && (
                          <span className="server-badge server-badge--owned" data-testid={`owned-badge-${cfg.id}`}>
                            Owned
                          </span>
                        )}
                      </div>
                      <p className="server-card__type">
                        {cfg.type === 'local' ? 'Local' : cfg.type === 'manual_url' ? 'Manual URL' : `SSH: ${cfg.sshAlias || '—'}`}
                      </p>
                      {cfg.lastPid > 0 ? <p className="server-card__pid">PID: {cfg.lastPid}</p> : null}
                      {cfg.url ? <p className="server-card__url">{cfg.url}</p> : null}
                      {st.lastCheckedAt ? <p className="server-card__checked">Checked: {formatCheckedAt(st.lastCheckedAt)}</p> : null}

                      {/* Transient op indicator */}
                      {st.op && (
                        <p className="server-card__op-state" data-testid={`op-state-${cfg.id}`}>
                          {st.op === 'testing-ssh' ? 'Testing SSH…' : st.op === 'detecting' ? 'Detecting…' : st.op === 'starting' ? 'Starting…' : 'Stopping…'}
                        </p>
                      )}

                      {/* Error display */}
                      {st.lastError && !st.op && (
                        <p className="server-card__error" role="alert" data-testid={`error-msg-${cfg.id}`}>
                          <span>{st.lastError}</span>
                          <button
                            className="server-btn server-btn--dismiss-error"
                            onClick={() => setState(cfg.id, { lastError: '' })}
                            data-testid={`dismiss-error-${cfg.id}`}
                          >
                            ✕
                          </button>
                        </p>
                      )}

                      {/* Success message */}
                      {st.lastMsg && !st.op && !st.lastError && (
                        <p className="server-card__msg" data-testid={`action-msg-${cfg.id}`}>
                          {st.lastMsg}
                        </p>
                      )}
                    </div>

                    <div className="server-card__actions">
                      {isActive ? null : (
                        <button
                          className="server-btn server-btn--attach"
                          onClick={() => handleAttach(cfg)}
                          disabled={!canAttach(cfg) || isAnyOp}
                          data-testid={`attach-${cfg.id}`}
                          title={cfg.url ? `Attach to ${cfg.url}` : 'No URL configured'}
                        >
                          Attach
                        </button>
                      )}
                      <button
                        className="server-btn server-btn--check"
                        onClick={() => handleCheck(cfg)}
                        disabled={isAnyOp}
                        data-testid={`check-${cfg.id}`}
                      >
                        {st.op ? '…' : 'Check'}
                      </button>
                      <button
                        className="server-btn server-btn--edit"
                        onClick={() => beginEdit(cfg)}
                        disabled={isAnyOp}
                        data-testid={`edit-${cfg.id}`}
                      >
                        Edit
                      </button>
                      <button
                        className="server-btn server-btn--remove"
                        onClick={() => confirmDelete(cfg.id)}
                        disabled={isAnyOp}
                        data-testid={`remove-server-${cfg.id}`}
                      >
                        Remove
                      </button>

                      {cfg.type === 'ssh_alias' && (
                        <>
                          <button
                            className="server-btn server-btn--ssh"
                            onClick={() => remoteAction(cfg, 'test-ssh')}
                            disabled={isAnyOp}
                            data-testid={`test-ssh-${cfg.id}`}
                          >
                            Test SSH
                          </button>
                          <button
                            className="server-btn server-btn--ssh"
                            onClick={() => remoteAction(cfg, 'detect')}
                            disabled={isAnyOp || !cfg.sshAlias}
                            data-testid={`detect-${cfg.id}`}
                          >
                            Detect
                          </button>
                          <button
                            className="server-btn server-btn--ssh"
                            onClick={() => remoteAction(cfg, 'start-remote')}
                            disabled={isAnyOp || st.status === 'online'}
                            data-testid={`start-remote-${cfg.id}`}
                          >
                            Start
                          </button>
                          <button
                            className="server-btn server-btn--ssh"
                            onClick={() => remoteAction(cfg, 'stop-remote')}
                            disabled={
                              isAnyOp ||
                              cfg.lastPid <= 0 ||
                              !cfg.startedByGlyphdeck
                            }
                            data-testid={`stop-remote-${cfg.id}`}
                            title={
                              cfg.lastPid <= 0
                                ? 'No recorded PID'
                                : !cfg.startedByGlyphdeck
                                  ? 'Not started by GlyphDeck'
                                  : 'Stop remote OpenCode'
                            }
                          >
                            Stop
                          </button>
                        </>
                      )}
                    </div>
                  </>
                )}
              </div>
            )
          })}
        </div>

        {/* Add form */}
        {showAdd ? (
          <form
            className="server-form"
            onSubmit={(e) => {
              e.preventDefault()
              handleAdd()
            }}
            data-testid="server-add-form"
          >
            <div className="server-form__field">
              <label htmlFor="server-id">ID (optional)</label>
              <input id="server-id" value={addId} onChange={(e) => setAddId(e.target.value)} data-testid="server-add-id" />
            </div>
            <div className="server-form__field">
              <label htmlFor="server-name">Name *</label>
              <input id="server-name" value={addName} onChange={(e) => setAddName(e.target.value)} data-testid="server-add-name" />
              {addErrors.name ? <span className="server-form__error">{addErrors.name}</span> : null}
            </div>
            <div className="server-form__field">
              <label htmlFor="server-type">Type</label>
              <select id="server-type" value={addType} onChange={(e) => setAddType(e.target.value as ServerTypeLabel)} data-testid="server-add-type">
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
              <>
                <div className="server-form__field">
                  <label htmlFor="server-ssh-alias">SSH Alias *</label>
                  <input
                    id="server-ssh-alias"
                    value={addSshAlias}
                    onChange={(e) => setAddSshAlias(e.target.value)}
                    placeholder="my-server"
                    data-testid="server-add-ssh-alias"
                  />
                  {addErrors.sshAlias ? <span className="server-form__error">{addErrors.sshAlias}</span> : null}
                </div>
                <div className="server-form__field">
                  <label htmlFor="server-working-dir">Working Directory</label>
                  <input id="server-working-dir" value={addWorkingDir} onChange={(e) => setAddWorkingDir(e.target.value)} placeholder="~" data-testid="server-add-working-dir" />
                </div>
                <div className="server-form__field">
                  <label htmlFor="server-start-cmd">Start Command</label>
                  <input id="server-start-cmd" value={addStartCmd} onChange={(e) => setAddStartCmd(e.target.value)} data-testid="server-add-start-cmd" />
                </div>
                <div className="server-form__field">
                  <label htmlFor="server-stop-cmd">Stop Command</label>
                  <input id="server-stop-cmd" value={addStopCmd} onChange={(e) => setAddStopCmd(e.target.value)} data-testid="server-add-stop-cmd" />
                </div>
                <div className="server-form__field">
                  <label htmlFor="server-status-cmd">Status Command</label>
                  <input id="server-status-cmd" value={addStatusCmd} onChange={(e) => setAddStatusCmd(e.target.value)} data-testid="server-add-status-cmd" />
                </div>
              </>
            )}
            <div className="server-form__actions">
              <button type="submit" className="server-btn server-btn--save" disabled={saving} data-testid="server-add-submit">
                {saving ? 'Saving…' : 'Add Server'}
              </button>
              <button type="button" className="server-btn server-btn--cancel" onClick={cancelAdd} data-testid="server-add-cancel">
                Cancel
              </button>
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

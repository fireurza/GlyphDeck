import { useState, useEffect, useCallback } from 'react'
import { requestJson } from '../api/client'
import type { ConfigInventory } from '../types/config'

function SettingsPanel() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const [config, setConfig] = useState<ConfigInventory | null>(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [configError, setConfigError] = useState('')

  const loadSettings = useCallback(async () => {
    try {
      const data = await requestJson<Record<string, string>>('/api/settings')
      setSettings(data)
    } catch {
      /* Settings unavailable */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadSettings()
  }, [loadSettings])

  const loadConfig = useCallback(async () => {
    setConfigLoading(true)
    setConfigError('')
    try {
      const data = await requestJson<ConfigInventory>('/api/opencode/config/inventory')
      setConfig(data)
    } catch (err) {
      setConfigError(err instanceof Error ? err.message : 'Failed to load.')
    } finally {
      setConfigLoading(false)
    }
  }, [])

  useEffect(() => {
    loadConfig()
  }, [loadConfig])

  function updateField(key: string, value: string) {
    setSettings((prev) => ({ ...prev, [key]: value }))
    setMessage('')
  }

  async function saveAll() {
    setSaving(true)
    setMessage('')
    try {
      await fetch('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings),
      })
      setMessage('Settings saved.')
    } catch {
      setMessage('Save failed.')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="panel-body panel-placeholder" data-testid="settings-panel">
        <p>Loading settings…</p>
      </div>
    )
  }

  return (
    <div className="settings-panel" data-testid="settings-panel">
      <div className="settings-panel__toolbar">
        <span className="settings-panel__title">Settings</span>
        <button
          className="settings-panel__save-btn"
          onClick={saveAll}
          disabled={saving}
          data-testid="settings-save-button"
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </div>
      <div className="panel-body">
        {message && (
          <p className="settings-message" data-testid="settings-message">
            {message}
          </p>
        )}
        <div className="settings-group">
          <label className="settings-label" htmlFor="settings-opencode-path">
            OpenCode Binary Path
          </label>
          <input
            className="settings-input"
            id="settings-opencode-path"
            type="text"
            value={settings['opencode_path'] || ''}
            onChange={(e) => updateField('opencode_path', e.target.value)}
            placeholder="opencode (default: auto-detect)"
            data-testid="settings-opencode-path"
          />
          <p className="settings-hint">Override the path to the OpenCode executable.</p>
        </div>
        <div className="settings-group">
          <label className="settings-label" htmlFor="settings-default-project-dir">
            Default Project Directory
          </label>
          <input
            className="settings-input"
            id="settings-default-project-dir"
            type="text"
            value={settings['default_project_dir'] || ''}
            onChange={(e) => updateField('default_project_dir', e.target.value)}
            placeholder="e.g. C:\Users\Example\Documents\Code"
            data-testid="settings-default-project-dir"
          />
          <p className="settings-hint">Default directory when adding new projects.</p>
        </div>
        <div className="settings-group">
          <label className="settings-label" htmlFor="settings-log-level">
            Log Level
          </label>
          <select
            className="settings-input"
            id="settings-log-level"
            value={settings['log_level'] || 'info'}
            onChange={(e) => updateField('log_level', e.target.value)}
            data-testid="settings-log-level"
          >
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="error">Error</option>
          </select>
          <p className="settings-hint">Backend logging verbosity.</p>
        </div>

        <hr className="settings-divider" />
        <h3 className="settings-section-title">OpenCode Configuration</h3>

        {configLoading ? (
          <p className="panel-hint">Loading config…</p>
        ) : configError ? (
          <p className="panel-error" data-testid="settings-config-error">{configError}</p>
        ) : config && config.available ? (
          <div data-testid="settings-config-section">
            {config.warnings?.length > 0 && (
              <div className="settings-config-warnings" data-testid="settings-config-warnings">
                {config.warnings.map((w, i) => (
                  <p key={i} className="config-warning">⚠ {w.source}: {w.message}</p>
                ))}
              </div>
            )}
            {config.sources?.length > 0 && (
              <div className="settings-group">
                <label className="settings-label">Configuration Sources</label>
                <ul className="config-source-list" data-testid="settings-config-sources">
                  {config.sources.map((s, i) => (
                    <li key={i} className="config-source-item">
                      <span className={`config-badge config-badge--${s.scope}`}>{s.scope}</span>
                      <span className="config-source-path">{s.path}</span>
                      <span className="config-source-format">{s.format}</span>
                      {s.warning && <span className="config-warning">⚠ {s.warning}</span>}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {config.providers.length > 0 && (
              <div className="settings-group">
                <label className="settings-label">Providers</label>
                <ul className="config-item-list" data-testid="settings-config-providers">
                  {config.providers.map((p) => (
                    <li key={p.id} className="config-item">
                      <strong>{p.id}</strong> {p.name && `(${p.name})`}
                      <span className={`config-badge config-badge--${p.scope}`}>{p.scope}</span>
                      {p.baseUrl && <span className="config-item__meta"> — {p.baseUrl}</span>}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {config.models.length > 0 && (
              <div className="settings-group">
                <label className="settings-label">Models</label>
                <ul className="config-item-list" data-testid="settings-config-models">
                  {config.models.map((m) => (
                    <li key={m.id} className="config-item">
                      <strong>{m.id}</strong> {m.name && `(${m.name})`}
                      <span className={`config-badge config-badge--${m.scope}`}>{m.scope}</span>
                      <span className="config-item__meta"> — provider: {m.provider}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {config.shellProfiles.length > 0 && (
              <div className="settings-group">
                <label className="settings-label">Shell Profiles</label>
                <ul className="config-item-list" data-testid="settings-config-shells">
                  {config.shellProfiles.map((s, i) => (
                    <li key={i} className="config-item">
                      <strong>{s.name}</strong>
                      <span className={`config-badge config-badge--${s.scope}`}>{s.scope}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            <button className="config-refresh-btn" onClick={loadConfig} data-testid="settings-config-refresh">
              Refresh Config
            </button>
          </div>
        ) : (
          <p className="panel-hint" data-testid="settings-config-unavailable">OpenCode configuration unavailable.</p>
        )}
      </div>
    </div>
  )
}

export default SettingsPanel

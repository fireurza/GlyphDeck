import { useState, useEffect, useCallback } from 'react'
import { requestJson } from '../api/client'

function SettingsPanel() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

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
          <label className="settings-label">OpenCode Binary Path</label>
          <input
            className="settings-input"
            type="text"
            value={settings['opencode_path'] || ''}
            onChange={(e) => updateField('opencode_path', e.target.value)}
            placeholder="opencode (default: auto-detect)"
            data-testid="settings-opencode-path"
          />
          <p className="settings-hint">Override the path to the OpenCode executable.</p>
        </div>
        <div className="settings-group">
          <label className="settings-label">Default Project Directory</label>
          <input
            className="settings-input"
            type="text"
            value={settings['default_project_dir'] || ''}
            onChange={(e) => updateField('default_project_dir', e.target.value)}
            placeholder="e.g. C:\Users\Fireurza\Documents\Code"
            data-testid="settings-default-project-dir"
          />
          <p className="settings-hint">Default directory when adding new projects.</p>
        </div>
        <div className="settings-group">
          <label className="settings-label">Log Level</label>
          <select
            className="settings-input"
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
      </div>
    </div>
  )
}

export default SettingsPanel

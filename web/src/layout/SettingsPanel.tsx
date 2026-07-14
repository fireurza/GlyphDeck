import { useState, useEffect, useCallback } from "react"
import { requestJson } from "../api/client"
import type { ConfigInventory } from "../types/config"

interface Prefs {
  appearance: string
  interfaceDensity: string
  terminalFontSize: number
  transcriptAutoScroll: boolean
  defaultRightPanelTab: string
  destructiveConfirmations: boolean
}

interface PrefsDocument {
  data: Prefs
  revision: number
  updatedAt?: string
}

interface FieldChange {
  field: string
  oldValue: unknown
  newValue: unknown
}

interface PreviewResult {
  normalized: PrefsDocument
  changes: { fields: FieldChange[] }
  errors?: { field: string; message: string }[]
}

interface BackupEntry {
  id: number
  revision: number
  createdAt: string
  summary: { fields: FieldChange[] }
}

function SettingsPanel() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState("")
  const [config, setConfig] = useState<ConfigInventory | null>(null)
  const [configLoading, setConfigLoading] = useState(false)
  const [configError, setConfigError] = useState("")

  // Preferences state.
  const [prefs, setPrefs] = useState<Prefs>({
    appearance: "system",
    interfaceDensity: "comfortable",
    terminalFontSize: 14,
    transcriptAutoScroll: true,
    defaultRightPanelTab: "review",
    destructiveConfirmations: true,
  })
  const [revision, setRevision] = useState(0)
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)
  const [backups, setBackups] = useState<BackupEntry[]>([])
  const [rawEditorMode, setRawEditorMode] = useState<"json" | "yaml" | null>(null)
  const [rawEditorText, setRawEditorText] = useState("")

  // Combined initial load.
  const loadAll = useCallback(async () => {
    setLoading(true)
    try {
      const [legacyData, prefsDoc, configData, backupsData] = await Promise.allSettled([
        requestJson<Record<string, string>>("/api/settings"),
        requestJson<PrefsDocument>("/api/preferences"),
        requestJson<ConfigInventory>("/api/opencode/config/inventory"),
        requestJson<BackupEntry[]>("/api/preferences/backups"),
      ])
      if (legacyData.status === "fulfilled") {
        setSettings(legacyData.value)
      }
      if (prefsDoc.status === "fulfilled") {
        setPrefs(prefsDoc.value.data)
        setRevision(prefsDoc.value.revision)
      }
      if (configData.status === "fulfilled") {
        setConfig(configData.value)
      } else {
        setConfigError("Failed to load config.")
      }
      if (backupsData.status === "fulfilled") {
        setBackups(backupsData.value)
      }
    } finally {
      setLoading(false)
      setConfigLoading(false)
    }
  }, [])

  useEffect(() => { loadAll() }, [loadAll])

  // Save legacy settings.
  async function saveLegacy() {
    setSaving(true)
    setMessage("")
    try {
      await fetch("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(settings),
      })
      setMessage("Settings saved.")
    } catch {
      setMessage("Save failed.")
    } finally {
      setSaving(false)
    }
  }

  // Preview preferences changes.
  async function handlePreview() {
    try {
      const result = await requestJson<PreviewResult>("/api/preferences/preview", {
        method: "POST",
        body: JSON.stringify(prefs),
      })
      setPreviewResult(result)
      if (result.errors && result.errors.length > 0) {
        setMessage("Validation errors — fix and preview again.")
      } else if (result.changes.fields.length === 0) {
        setMessage("No changes to apply.")
      } else {
        setShowConfirm(true)
      }
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Preview failed.")
    }
  }

  // Apply confirmed changes.
  async function handleApply() {
    setSaving(true)
    setShowConfirm(false)
    try {
      const doc = await requestJson<PrefsDocument>("/api/preferences", {
        method: "PUT",
        body: JSON.stringify({ data: prefs, expectedRevision: revision }),
      })
      setPrefs(doc.data)
      setRevision(doc.revision)
      setPreviewResult(null)
      setMessage("Preferences applied.")
      loadAll()
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Save failed."
      if (msg.includes("409") || msg.includes("conflict")) {
        setMessage("Revision conflict — reloading current settings.")
        loadAll()
      } else {
        setMessage(msg)
      }
    } finally {
      setSaving(false)
    }
  }

  // Restore a backup.
  async function handleRestore(backupId: number) {
    if (!window.confirm("Restore this backup? Current settings will be saved as a new backup.")) return
    try {
      const doc = await requestJson<PrefsDocument>(`/api/preferences/backups/${backupId}/restore`, {
        method: "POST",
      })
      setPrefs(doc.data)
      setRevision(doc.revision)
      setPreviewResult(null)
      setShowConfirm(false)
      setMessage("Backup restored.")
      loadAll()
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Restore failed.")
    }
  }

  function updatePref(field: keyof Prefs, value: string | number | boolean) {
    setPrefs((p) => ({ ...p, [field]: value }))
    setPreviewResult(null) // Invalidate previous preview.
    setShowConfirm(false)
  }

  function openRawEditor(mode: "json" | "yaml") {
    setRawEditorMode(mode)
    setRawEditorText(mode === "json" ? JSON.stringify(prefs, null, 2) : toYAML(prefs))
  }

  function applyRawEditor() {
    try {
      const parsed = rawEditorMode === "json" ? JSON.parse(rawEditorText) : parseSimpleYAML(rawEditorText)
      setPrefs((p) => ({ ...p, ...parsed }))
      setRawEditorMode(null)
      setPreviewResult(null)
    } catch {
      setMessage(`Invalid ${rawEditorMode?.toUpperCase()}.`)
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
        <button className="settings-panel__save-btn" onClick={handlePreview} disabled={saving} data-testid="settings-preview-button">
          Preview Changes
        </button>
      </div>
      <div className="panel-body">
        {message && <p className="settings-message" data-testid="settings-message">{message}</p>}

        <h3 className="settings-section-title">Preferences</h3>

        <div className="settings-group">
          <label className="settings-label" htmlFor="prefs-appearance">Appearance</label>
          <select className="settings-input" id="prefs-appearance" value={prefs.appearance}
            onChange={(e) => updatePref("appearance", e.target.value)} data-testid="prefs-appearance">
            <option value="system">System</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </div>

        <div className="settings-group">
          <label className="settings-label" htmlFor="prefs-density">Interface Density</label>
          <select className="settings-input" id="prefs-density" value={prefs.interfaceDensity}
            onChange={(e) => updatePref("interfaceDensity", e.target.value)} data-testid="prefs-density">
            <option value="comfortable">Comfortable</option>
            <option value="compact">Compact</option>
          </select>
        </div>

        <div className="settings-group">
          <label className="settings-label" htmlFor="prefs-font-size">Terminal Font Size</label>
          <input className="settings-input" id="prefs-font-size" type="number" min={11} max={24}
            value={prefs.terminalFontSize} onChange={(e) => updatePref("terminalFontSize", parseInt(e.target.value) || 14)}
            data-testid="prefs-font-size" />
        </div>

        <div className="settings-group">
          <label className="settings-label">
            <input type="checkbox" checked={prefs.transcriptAutoScroll}
              onChange={(e) => updatePref("transcriptAutoScroll", e.target.checked)}
              data-testid="prefs-auto-scroll" /> Transcript Auto-Scroll
          </label>
        </div>

        <div className="settings-group">
          <label className="settings-label" htmlFor="prefs-default-tab">Default Right-Panel Tab</label>
          <select className="settings-input" id="prefs-default-tab" value={prefs.defaultRightPanelTab}
            onChange={(e) => updatePref("defaultRightPanelTab", e.target.value)} data-testid="prefs-default-tab">
            <option value="review">Review</option>
            <option value="usage">Usage</option>
            <option value="agents">Agents</option>
          </select>
        </div>

        <div className="settings-group">
          <label className="settings-label">
            <input type="checkbox" checked={prefs.destructiveConfirmations}
              onChange={(e) => updatePref("destructiveConfirmations", e.target.checked)}
              data-testid="prefs-confirmations" /> Destructive-Action Confirmations
          </label>
        </div>

        <div className="settings-group" style={{ display: "flex", gap: 8 }}>
          <button className="config-refresh-btn" onClick={() => openRawEditor("json")} data-testid="prefs-raw-json">Edit JSON</button>
          <button className="config-refresh-btn" onClick={() => openRawEditor("yaml")} data-testid="prefs-raw-yaml">Edit YAML</button>
        </div>

        {/* Preview / Confirm dialog */}
        {previewResult && (
          <div className="preview-section" data-testid="preview-section">
            {previewResult.errors && previewResult.errors.length > 0 && (
              <div className="config-warning" data-testid="preview-errors">
                {previewResult.errors.map((e, i) => <p key={i}>{e.field}: {e.message}</p>)}
              </div>
            )}
            {previewResult.changes.fields.length > 0 && (
              <div data-testid="preview-changes">
                <p><strong>Changes:</strong></p>
                <ul>
                  {previewResult.changes.fields.map((c, i) => (
                    <li key={i}>{c.field}: {String(c.oldValue)} → {String(c.newValue)}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {/* Confirmation dialog */}
        {showConfirm && (
          <div className="confirm-dialog" data-testid="confirm-dialog">
            <p>Apply these changes?</p>
            <button className="settings-panel__save-btn" onClick={handleApply} disabled={saving} data-testid="confirm-apply">
              {saving ? "Applying…" : "Apply"}
            </button>
            <button className="config-refresh-btn" onClick={() => setShowConfirm(false)} data-testid="confirm-cancel">Cancel</button>
          </div>
        )}

        {/* Raw editor modal */}
        {rawEditorMode && (
          <div className="raw-editor-overlay" data-testid="raw-editor">
            <div className="raw-editor-content">
              <h3>Edit {rawEditorMode.toUpperCase()}</h3>
              <textarea className="settings-input" rows={12} value={rawEditorText}
                onChange={(e) => setRawEditorText(e.target.value)} data-testid="raw-editor-textarea" />
              <div style={{ marginTop: 8, display: "flex", gap: 8 }}>
                <button className="settings-panel__save-btn" onClick={applyRawEditor}>Apply</button>
                <button className="config-refresh-btn" onClick={() => setRawEditorMode(null)}>Cancel</button>
              </div>
            </div>
          </div>
        )}

        <hr className="settings-divider" />
        <h3 className="settings-section-title">Backups</h3>
        {backups.length === 0 ? (
          <p className="panel-hint">No backups yet.</p>
        ) : (
          <ul className="config-list" data-testid="backups-list">
            {backups.map((b) => (
              <li key={b.id} className="config-item" data-testid={`backup-${b.id}`}>
                <span>Rev {b.revision} — {new Date(b.createdAt).toLocaleString()}</span>
                {b.summary.fields.length > 0 && (
                  <span> ({b.summary.fields.map((f) => `${f.field}: ${String(f.oldValue)}→${String(f.newValue)}`).join(", ")})</span>
                )}
                <button className="config-refresh-btn" style={{ marginLeft: 8 }}
                  onClick={() => handleRestore(b.id)} data-testid={`restore-${b.id}`}>Restore</button>
              </li>
            ))}
          </ul>
        )}

        <hr className="settings-divider" />
        <h3 className="settings-section-title">Legacy Settings</h3>

        <div className="settings-group">
          <label className="settings-label" htmlFor="settings-opencode-path">OpenCode Binary Path</label>
          <input className="settings-input" id="settings-opencode-path" type="text"
            value={settings["opencode_path"] || ""}
            onChange={(e) => setSettings((s) => ({ ...s, opencode_path: e.target.value }))}
            placeholder="opencode (default: auto-detect)" data-testid="settings-opencode-path" />
          <p className="settings-hint">Override the path to the OpenCode executable.</p>
        </div>

        <div className="settings-group">
          <label className="settings-label" htmlFor="settings-default-project-dir">Default Project Directory</label>
          <input className="settings-input" id="settings-default-project-dir" type="text"
            value={settings["default_project_dir"] || ""}
            onChange={(e) => setSettings((s) => ({ ...s, default_project_dir: e.target.value }))}
            placeholder="e.g. C:\Users\Example\Documents\Code" data-testid="settings-default-project-dir" />
        </div>

        <div className="settings-group">
          <label className="settings-label" htmlFor="settings-log-level">Log Level</label>
          <select className="settings-input" id="settings-log-level"
            value={settings["log_level"] || "info"}
            onChange={(e) => setSettings((s) => ({ ...s, log_level: e.target.value }))} data-testid="settings-log-level">
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="error">Error</option>
          </select>
          <p className="settings-hint">Backend logging verbosity.</p>
        </div>

        <button className="settings-panel__save-btn" onClick={saveLegacy} disabled={saving} data-testid="settings-save-button">
          {saving ? "Saving…" : "Save Legacy Settings"}
        </button>

        <hr className="settings-divider" />
        <h3 className="settings-section-title">OpenCode Configuration</h3>

        {configLoading ? (
          <p className="panel-hint">Loading config…</p>
        ) : configError ? (
          <p className="panel-error" data-testid="settings-config-error">{configError}</p>
        ) : config && config.available ? (
          <div data-testid="settings-config-section">
            {config.warnings && config.warnings.length > 0 && (
              <div className="settings-config-warnings" data-testid="settings-config-warnings">
                {config.warnings.map((w, i) => (
                  <p key={i} className="config-warning">{w.source}: {w.message}</p>
                ))}
              </div>
            )}
            {(config.sources?.length ?? 0) > 0 && (
              <div className="settings-group">
                <label className="settings-label">Configuration Sources</label>
                <ul className="config-source-list" data-testid="settings-config-sources">
                  {config.sources.map((s, i) => (
                    <li key={i} className="config-source-item">
                      <span className={`config-badge config-badge--${s.scope}`}>{s.scope}</span>
                      <span className="config-source-path">{s.path}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {(config.providers?.length ?? 0) > 0 && (
              <div className="settings-group">
                <label className="settings-label">Providers</label>
                <ul className="config-item-list" data-testid="settings-config-providers">
                  {config.providers.map((p) => (
                    <li key={p.id} className="config-item">
                      <strong>{p.id}</strong> <span className={`config-badge config-badge--${p.scope}`}>{p.scope}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            <button className="config-refresh-btn" onClick={loadAll} data-testid="settings-config-refresh">Refresh Config</button>
          </div>
        ) : (
          <p className="panel-hint" data-testid="settings-config-unavailable">OpenCode configuration unavailable.</p>
        )}
      </div>
    </div>
  )
}

// Simple YAML-like serializer for preferences (no library dependency).
function toYAML(p: Prefs): string {
  const lines: string[] = []
  for (const [k, v] of Object.entries(p)) {
    if (typeof v === "boolean") lines.push(`${k}: ${v}`)
    else if (typeof v === "number") lines.push(`${k}: ${v}`)
    else lines.push(`${k}: "${v}"`)
  }
  return lines.join("\n")
}

// Simple YAML parser for preferences (key: value, one per line).
function parseSimpleYAML(text: string): Partial<Prefs> {
  const result: Record<string, unknown> = {}
  for (const line of text.split("\n")) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith("#")) continue
    const idx = trimmed.indexOf(":")
    if (idx < 0) continue
    const key = trimmed.slice(0, idx).trim()
    let value = trimmed.slice(idx + 1).trim()
    if (value.startsWith('"') && value.endsWith('"')) value = value.slice(1, -1)
    if (value === "true") result[key] = true
    else if (value === "false") result[key] = false
    else if (/^\d+$/.test(value)) result[key] = parseInt(value, 10)
    else result[key] = value
  }
  return result as Partial<Prefs>
}

export default SettingsPanel

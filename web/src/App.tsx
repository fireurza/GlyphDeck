import { useState, useCallback, useEffect, useRef } from 'react'
import TopBar from './layout/TopBar'
import ActivityRail, { type RailView } from './layout/ActivityRail'
import LeftPanel from './layout/LeftPanel'
import ServersPanel from './layout/ServersPanel'
import CenterPanel from './layout/CenterPanel'
import RightPanel from './layout/RightPanel'
import BottomPanel from './layout/BottomPanel'
import SettingsPanel from './layout/SettingsPanel'
import SetupScreen from './layout/SetupScreen'
import LoginScreen from './layout/LoginScreen'
import { useEventStream } from './api/events'
import { fetchAuthStatus, logout as apiLogout } from './api/auth'
import type { AuthStatus } from './api/auth'
import './styles/layout.css'

const LS_PROJECT_KEY = 'glyphdeck-selected-project'
const LS_SESSION_KEY = 'glyphdeck-selected-session'

function readLS(key: string): string | null {
  try { return localStorage.getItem(key) } catch { return null }
}
function writeLS(key: string, value: string | null) {
  try { if (value) localStorage.setItem(key, value); else localStorage.removeItem(key) } catch {}
}

function App() {
  // Auth state: loading → setup → login → authenticated
  const [authStatus, setAuthStatus] = useState<AuthStatus | null>(null)
  const [authLoading, setAuthLoading] = useState(true)
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(
    () => readLS(LS_PROJECT_KEY),
  )
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(
    () => readLS(LS_SESSION_KEY),
  )
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [activeRailView, setActiveRailView] = useState<RailView>('projects')
  const settingsTriggerRef = useRef<HTMLButtonElement>(null)
  const { status: eventStreamStatus, latestEvent } = useEventStream(selectedProjectId)

  // Check auth status on mount.
  useEffect(() => {
    fetchAuthStatus()
      .then(setAuthStatus)
      .catch(() => setAuthStatus(null))
      .finally(() => setAuthLoading(false))
  }, [])

  const handleAuthComplete = useCallback(async () => {
    try {
      const status = await fetchAuthStatus()
      setAuthStatus(status)
    } catch {
      setAuthStatus(null)
    }
  }, [])

  const handleLogout = useCallback(async () => {
    await apiLogout()
    setAuthStatus({ setupRequired: false, loginRequired: true, adminExists: true })
    setSelectedSessionId(null)
    setSelectedProjectId(null)
    writeLS(LS_SESSION_KEY, null)
    writeLS(LS_PROJECT_KEY, null)
  }, [])

  const handleSelectProject = useCallback((projectId: string) => {
    setSelectedProjectId(projectId)
    setSelectedSessionId(null)
    writeLS(LS_PROJECT_KEY, projectId)
    writeLS(LS_SESSION_KEY, null)
  }, [])

  const handleSelectSession = useCallback((projectId: string, sessionId: string) => {
    setSelectedProjectId(projectId)
    setSelectedSessionId(sessionId)
    writeLS(LS_PROJECT_KEY, projectId)
    writeLS(LS_SESSION_KEY, sessionId)
  }, [])

  const closeSettings = useCallback(() => {
    setSettingsOpen(false)
    window.requestAnimationFrame(() => settingsTriggerRef.current?.focus())
  }, [])

  // Loading state.
  if (authLoading) {
    return <div className="glyphdeck-shell"><p className="project-message">Loading…</p></div>
  }

  // First-run setup.
  if (authStatus?.setupRequired) {
    return <SetupScreen onComplete={handleAuthComplete} />
  }

  // Login required.
  if (authStatus?.loginRequired) {
    return <LoginScreen onComplete={handleAuthComplete} />
  }

  return (
    <div className="glyphdeck-shell" data-testid="app-shell">
      <TopBar eventStreamStatus={eventStreamStatus} />
      <div className="glyphdeck-main">
        <ActivityRail
          activeView={activeRailView}
          onSelectView={setActiveRailView}
          onOpenSettings={() => setSettingsOpen(true)}
          onLogout={handleLogout}
          settingsOpen={settingsOpen}
          settingsButtonRef={settingsTriggerRef}
        />
        {activeRailView === 'projects' ? (
          <LeftPanel
            initialProjectId={selectedProjectId}
            initialSessionId={selectedSessionId}
            onSelectProject={handleSelectProject}
            onSelectSession={handleSelectSession}
          />
        ) : (
          <ServersPanel />
        )}
        <CenterPanel
          selectedProjectId={selectedProjectId}
          selectedSessionId={selectedSessionId}
          eventStreamStatus={eventStreamStatus}
          latestEvent={latestEvent}
        />
        <RightPanel
          selectedProjectId={selectedProjectId}
          selectedSessionId={selectedSessionId}
        />
      </div>
      <BottomPanel
        selectedProjectId={selectedProjectId}
        selectedSessionId={selectedSessionId}
        eventStreamStatus={eventStreamStatus}
        latestEvent={latestEvent}
      />
      {settingsOpen && <SettingsDialog onClose={closeSettings} />}
    </div>
  )
}

function SettingsDialog({ onClose }: { onClose: () => void }) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const closeButtonRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog || dialog.open) return
    dialog.showModal()
    closeButtonRef.current?.focus()
    return () => { if (dialog.open) dialog.close() }
  }, [])
  return (
    <dialog ref={dialogRef} className="settings-dialog" aria-label="Settings"
      onCancel={(e) => { e.preventDefault(); onClose() }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
      data-testid="settings-dialog">
      <div className="settings-dialog__content">
        <button ref={closeButtonRef} type="button" className="settings-dialog__close"
          onClick={onClose} aria-label="Close settings" data-testid="settings-close-button">×</button>
        <SettingsPanel />
      </div>
    </dialog>
  )
}

export default App

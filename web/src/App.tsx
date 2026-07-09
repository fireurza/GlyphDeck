import { useState, useCallback } from 'react'
import TopBar from './layout/TopBar'
import ActivityRail from './layout/ActivityRail'
import LeftPanel from './layout/LeftPanel'
import CenterPanel from './layout/CenterPanel'
import RightPanel from './layout/RightPanel'
import BottomPanel from './layout/BottomPanel'
import { useEventStream } from './api/events'
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
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(
    () => readLS(LS_PROJECT_KEY),
  )
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(
    () => readLS(LS_SESSION_KEY),
  )
  const { status: eventStreamStatus, latestEvent } = useEventStream(selectedProjectId)

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

  return (
    <div className="glyphdeck-shell" data-testid="app-shell">
      <TopBar eventStreamStatus={eventStreamStatus} />
      <div className="glyphdeck-main">
        <ActivityRail />
        <LeftPanel
          initialProjectId={selectedProjectId}
          initialSessionId={selectedSessionId}
          onSelectProject={handleSelectProject}
          onSelectSession={handleSelectSession}
        />
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
    </div>
  )
}

export default App

import { useState } from 'react'
import TopBar from './layout/TopBar'
import ActivityRail from './layout/ActivityRail'
import LeftPanel from './layout/LeftPanel'
import CenterPanel from './layout/CenterPanel'
import RightPanel from './layout/RightPanel'
import BottomPanel from './layout/BottomPanel'
import { useEventStream } from './api/events'
import './styles/layout.css'

function App() {
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null)
  const { status: eventStreamStatus, latestEvent } = useEventStream(selectedProjectId)

  // Connect the event stream as soon as a project is selected (server ready),
  // independent of session selection. This is what drives the "Live" status;
  // the M4 stream must be connectable before a session exists.
  function handleSelectProject(projectId: string) {
    setSelectedProjectId(projectId)
    setSelectedSessionId(null)
  }

  function handleSelectSession(projectId: string, sessionId: string) {
    setSelectedProjectId(projectId)
    setSelectedSessionId(sessionId)
  }

  return (
    <div className="glyphdeck-shell" data-testid="app-shell">
      <TopBar eventStreamStatus={eventStreamStatus} />
      <div className="glyphdeck-main">
        <ActivityRail />
        <LeftPanel
          onSelectProject={handleSelectProject}
          onSelectSession={handleSelectSession}
        />
        <CenterPanel
          selectedProjectId={selectedProjectId}
          selectedSessionId={selectedSessionId}
          eventStreamStatus={eventStreamStatus}
          latestEvent={latestEvent}
        />
        <RightPanel />
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

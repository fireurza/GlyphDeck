import { useState } from 'react'
import TopBar from './layout/TopBar'
import ActivityRail from './layout/ActivityRail'
import LeftPanel from './layout/LeftPanel'
import CenterPanel from './layout/CenterPanel'
import RightPanel from './layout/RightPanel'
import BottomPanel from './layout/BottomPanel'
import './styles/layout.css'

function App() {
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null)

  function handleSelectSession(projectId: string, sessionId: string) {
    setSelectedProjectId(projectId)
    setSelectedSessionId(sessionId)
  }

  return (
    <div className="glyphdeck-shell" data-testid="app-shell">
      <TopBar />
      <div className="glyphdeck-main">
        <ActivityRail />
        <LeftPanel onSelectSession={handleSelectSession} />
        <CenterPanel
          selectedProjectId={selectedProjectId}
          selectedSessionId={selectedSessionId}
        />
        <RightPanel />
      </div>
      <BottomPanel />
    </div>
  )
}

export default App

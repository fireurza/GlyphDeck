import { useState } from 'react'
import AgentTerminal from './AgentTerminal'
import type { EventStreamStatus, StreamEvent } from '../api/events'

const TABS = ['Problems', 'Agent Terminal', 'Terminal'] as const

const TAB_TEST_IDS: Record<string, string> = {
  Problems: 'problems-tab',
  'Agent Terminal': 'bottom-agent-terminal-tab',
  Terminal: 'terminal-tab',
}

interface BottomPanelProps {
  selectedProjectId: string | null
  selectedSessionId: string | null
  eventStreamStatus: EventStreamStatus
  latestEvent: StreamEvent | null
}

function BottomPanel({
  selectedProjectId,
  selectedSessionId,
  eventStreamStatus,
  latestEvent,
}: BottomPanelProps) {
  const [activeTab, setActiveTab] = useState<string>('Problems')

  return (
    <footer className="bottom-panel">
      <div className="tab-bar">
        {TABS.map((tab) => (
          <button
            key={tab}
            className={`tab-bar__tab ${activeTab === tab ? 'active' : ''}`}
            onClick={() => setActiveTab(tab)}
            data-testid={TAB_TEST_IDS[tab]}
          >
            {tab}
          </button>
        ))}
      </div>

      {activeTab === 'Agent Terminal' ? (
        <AgentTerminal
          selectedProjectId={selectedProjectId}
          selectedSessionId={selectedSessionId}
          eventStreamStatus={eventStreamStatus}
          latestEvent={latestEvent}
        />
      ) : (
        <div className="panel-body panel-placeholder">
          <p>{activeTab} — no issues detected.</p>
        </div>
      )}
    </footer>
  )
}

export default BottomPanel

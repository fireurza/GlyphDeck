import { useState } from 'react'
import ReviewPanel from './ReviewPanel'
import UsagePanel from './UsagePanel'
import AgentsPanel from './AgentsPanel'

const TABS = ['Review', 'Usage', 'Tasks', 'Agents'] as const

const TAB_TEST_IDS: Record<string, string> = {
  Review: 'right-review-tab',
  Usage: 'right-usage-tab',
  Tasks: 'tasks-tab',
  Agents: 'agents-tab',
}

interface RightPanelProps {
  selectedProjectId?: string | null
  selectedSessionId?: string | null
}

function RightPanel({ selectedProjectId, selectedSessionId }: RightPanelProps) {
  const [activeTab, setActiveTab] = useState<string>('Review')

  function renderPanel() {
    switch (activeTab) {
      case 'Review':
        return (
          <ReviewPanel
            selectedProjectId={selectedProjectId}
            selectedSessionId={selectedSessionId}
          />
        )
      case 'Usage':
        return (
          <UsagePanel
            selectedProjectId={selectedProjectId}
            selectedSessionId={selectedSessionId}
          />
        )
      case 'Agents':
        return <AgentsPanel selectedProjectId={selectedProjectId} />
      case 'Tasks':
      default:
        return (
          <div className="panel-body panel-placeholder">
            <p>{activeTab} panel</p>
            <p className="panel-hint">(Coming soon)</p>
          </div>
        )
    }
  }

  return (
    <aside className="right-panel">
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
      {selectedProjectId || activeTab === 'Agents' ? renderPanel() : (
        <div className="panel-body panel-placeholder">
          <p>No project selected</p>
          <p className="panel-hint">Select a project to view data.</p>
        </div>
      )}
    </aside>
  )
}

export default RightPanel

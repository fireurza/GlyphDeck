import { useState } from 'react'

const TABS = ['Review', 'Usage', 'Tasks', 'Agents'] as const

const TAB_TEST_IDS: Record<string, string> = {
  Review: 'review-tab',
  Usage: 'usage-tab',
  Tasks: 'tasks-tab',
  Agents: 'agents-tab',
}

function RightPanel() {
  const [activeTab, setActiveTab] = useState<string>('Review')

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
      <div className="panel-body panel-placeholder">
        <p>{activeTab} panel</p>
        <p className="panel-hint">(Coming soon)</p>
      </div>
    </aside>
  )
}

export default RightPanel

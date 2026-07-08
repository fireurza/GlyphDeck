import { useState } from 'react'

const TABS = ['Problems', 'Agent Terminal', 'Terminal'] as const

const TAB_TEST_IDS: Record<string, string> = {
  Problems: 'problems-tab',
  'Agent Terminal': 'agent-terminal-tab',
  Terminal: 'terminal-tab',
}

function BottomPanel() {
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
      <div className="panel-body panel-placeholder">
        <p>{activeTab} — no issues detected.</p>
      </div>
    </footer>
  )
}

export default BottomPanel

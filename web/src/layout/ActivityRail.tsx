import type { RefObject } from 'react'

export type RailView = 'projects' | 'servers'

interface ActivityRailProps {
  activeView: RailView
  onSelectView: (view: RailView) => void
  onOpenSettings: () => void
  onLogout: () => void
  settingsOpen: boolean
  settingsButtonRef: RefObject<HTMLButtonElement | null>
}

function ActivityRail({ activeView, onSelectView, onOpenSettings, onLogout, settingsOpen, settingsButtonRef }: ActivityRailProps) {
  return (
    <nav className="activity-rail">
      <div className="activity-rail__items">
        <button
          className={`activity-rail__item ${activeView === 'projects' && !settingsOpen ? 'active' : ''}`}
          title="Projects"
          onClick={() => onSelectView('projects')}
          data-testid="activity-projects-button"
        >
          📁
        </button>
        <button
          className={`activity-rail__item ${activeView === 'servers' && !settingsOpen ? 'active' : ''}`}
          title="Servers"
          onClick={() => onSelectView('servers')}
          data-testid="activity-servers-button"
        >
          🖥️
        </button>
        <button
          ref={settingsButtonRef}
          type="button"
          className={`activity-rail__item ${settingsOpen ? 'active' : ''}`}
          title="Settings"
          aria-label="Open settings"
          aria-haspopup="dialog"
          aria-expanded={settingsOpen}
          onClick={onOpenSettings}
          data-testid="activity-settings-button"
        >
          ⚙️
        </button>
        <button
          className="activity-rail__item activity-rail__item--logout"
          title="Logout"
          onClick={onLogout}
          data-testid="activity-logout-button"
        >
          🚪
        </button>
      </div>
    </nav>
  )
}

export default ActivityRail

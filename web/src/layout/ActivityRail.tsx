import type { RefObject } from 'react'

interface ActivityRailProps {
  onOpenSettings: () => void
  settingsOpen: boolean
  settingsButtonRef: RefObject<HTMLButtonElement | null>
}

function ActivityRail({ onOpenSettings, settingsOpen, settingsButtonRef }: ActivityRailProps) {
  return (
    <nav className="activity-rail">
      <div className="activity-rail__items">
        <button className={`activity-rail__item ${settingsOpen ? '' : 'active'}`} title="Sessions">
          📂
        </button>
        <button className="activity-rail__item" title="Projects">
          📁
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
      </div>
    </nav>
  )
}

export default ActivityRail

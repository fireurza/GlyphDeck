function ActivityRail() {
  return (
    <nav className="activity-rail">
      <div className="activity-rail__items">
        <button className="activity-rail__item active" title="Sessions">
          📂
        </button>
        <button className="activity-rail__item" title="Projects">
          📁
        </button>
        <button className="activity-rail__item" title="Settings">
          ⚙️
        </button>
      </div>
    </nav>
  )
}

export default ActivityRail

function TopBar() {
  return (
    <header className="top-bar">
      <div className="top-bar__title">GlyphDeck</div>
      <div className="top-bar__status">
        <span className="status-indicator" title="Backend not connected" />
        <span className="status-text">GlyphDeck</span>
      </div>
    </header>
  )
}

export default TopBar

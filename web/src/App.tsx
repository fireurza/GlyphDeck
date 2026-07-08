import TopBar from './layout/TopBar'
import ActivityRail from './layout/ActivityRail'
import LeftPanel from './layout/LeftPanel'
import CenterPanel from './layout/CenterPanel'
import RightPanel from './layout/RightPanel'
import BottomPanel from './layout/BottomPanel'
import './styles/layout.css'

function App() {
  return (
    <div className="glyphdeck-shell">
      <TopBar />
      <div className="glyphdeck-main">
        <ActivityRail />
        <LeftPanel />
        <CenterPanel />
        <RightPanel />
      </div>
      <BottomPanel />
    </div>
  )
}

export default App

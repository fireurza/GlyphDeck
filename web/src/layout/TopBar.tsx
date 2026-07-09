import type { EventStreamStatus } from '../api/events'
import { GLYPHDECK_VERSION } from '../constants'

interface TopBarProps {
  eventStreamStatus: EventStreamStatus
}

const STATUS_LABELS: Record<EventStreamStatus, string> = {
  idle: 'Offline',
  connecting: 'Reconnecting',
  connected: 'Live',
  disconnected: 'Offline',
  error: 'Error',
}

function getStatusClass(status: EventStreamStatus): string {
  switch (status) {
    case 'connected':
      return 'status-indicator--connected'
    case 'connecting':
      return 'status-indicator--reconnecting'
    case 'error':
      return 'status-indicator--error'
    case 'disconnected':
    case 'idle':
    default:
      return 'status-indicator--disconnected'
  }
}

function getStatusTestId(status: EventStreamStatus): string | undefined {
  if (status === 'connected') return 'eventstream-connected-state'
  if (status === 'error') return 'eventstream-error-state'
  return undefined
}

function TopBar({ eventStreamStatus }: TopBarProps) {
  const statusLabel = STATUS_LABELS[eventStreamStatus]
  const statusClass = getStatusClass(eventStreamStatus)
  const stateTestId = getStatusTestId(eventStreamStatus)

  return (
    <header className="top-bar">
      <div className="top-bar__title">GlyphDeck</div>
      <div className="top-bar__status">
        <span
          className={`status-indicator ${statusClass}`}
          data-testid={stateTestId ?? 'event-stream-status'}
          title={`Event stream: ${statusLabel}`}
        />
        <span className="status-text">{statusLabel}</span>
        <span className="status-text" data-testid="top-version-label">
          {GLYPHDECK_VERSION}
        </span>
      </div>
    </header>
  )
}

export default TopBar

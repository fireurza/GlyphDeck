import type { GlyphSession } from '../types/session'

interface SessionListProps {
  sessions: GlyphSession[]
  isCreatingSession: boolean
  sessionsLoading: boolean
  sessionError: string | null
  selectedProjectId: string
  onCreateSession: () => void
  onSelectSession: (projectId: string, sessionId: string) => void
}

function SessionList({
  sessions,
  isCreatingSession,
  sessionsLoading,
  sessionError,
  selectedProjectId,
  onCreateSession,
  onSelectSession,
}: SessionListProps) {
  return (
    <div className="session-section" data-testid="sessions-section">
      <div className="session-section__header">
        <span className="session-section__title">Sessions</span>
        <button
          className="session-section__create-btn"
          type="button"
          onClick={onCreateSession}
          disabled={isCreatingSession}
          data-testid="create-session-button"
        >
          {isCreatingSession ? 'Creating…' : '+ Session'}
        </button>
      </div>

      {sessionError ? (
        <p className="project-message project-message--error" role="alert">
          {sessionError}
        </p>
      ) : null}

      {sessionsLoading ? (
        <p className="project-message">Loading sessions…</p>
      ) : sessions.length === 0 ? (
        <p className="project-message">No sessions yet.</p>
      ) : (
        <ul className="session-list">
          {sessions.map((session) => (
            <li key={session.id} data-testid={`session-item-${session.id}`}>
              <button
                className="session-item"
                type="button"
                onClick={() => onSelectSession(selectedProjectId, session.id)}
                data-testid="session-item"
                data-session-id={session.id}
              >
                {session.title}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

export default SessionList

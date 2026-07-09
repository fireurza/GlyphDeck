import { useCallback, useEffect, useRef, useState } from 'react'
import { fetchReview } from '../api/review'
import type { ReviewResponse } from '../types/review'

interface ReviewPanelProps {
  selectedProjectId?: string | null
  selectedSessionId?: string | null
}

type PanelState = 'empty' | 'loading' | 'error' | 'data'

function ReviewPanel({ selectedProjectId, selectedSessionId }: ReviewPanelProps) {
  const [data, setData] = useState<ReviewResponse | null>(null)
  const [panelState, setPanelState] = useState<PanelState>('empty')
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const loadReview = useCallback(async () => {
    if (!selectedProjectId || !selectedSessionId) {
      setPanelState('empty')
      setData(null)
      setErrorMessage(null)
      return
    }

    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    setPanelState('loading')
    setErrorMessage(null)

    try {
      const result = await fetchReview(selectedProjectId, selectedSessionId, ctrl.signal)
      if (!ctrl.signal.aborted) {
        setData(result)
        setPanelState('data')
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (!ctrl.signal.aborted) {
        setPanelState('error')
        setErrorMessage(err instanceof Error ? err.message : 'Could not load review data.')
      }
    }
  }, [selectedProjectId, selectedSessionId])

  useEffect(() => {
    loadReview()
    return () => abortRef.current?.abort()
  }, [loadReview])

  if (!selectedProjectId || !selectedSessionId) {
    return (
      <div className="panel-body panel-placeholder" data-testid="review-empty-state">
        <p>No session selected.</p>
        <p className="panel-hint">Select a session to view the review summary.</p>
      </div>
    )
  }

  return (
    <div className="review-panel" data-testid="review-panel">
      <div className="review-panel__toolbar">
        <span className="review-panel__title">Review</span>
        <button
          className="review-panel__refresh-btn"
          onClick={loadReview}
          disabled={panelState === 'loading'}
          data-testid="review-refresh-button"
        >
          {panelState === 'loading' ? 'Loading…' : 'Refresh'}
        </button>
      </div>

      <div className="panel-body">
        {panelState === 'loading' && (
          <p className="project-message" data-testid="review-loading-state">
            Loading review data…
          </p>
        )}

        {panelState === 'error' && (
          <p
            className="project-message project-message--error"
            role="alert"
            data-testid="review-error-state"
          >
            {errorMessage}
          </p>
        )}

        {panelState === 'data' && data && (
          <div className="review-sections">
            {/* Project */}
            <section className="review-section">
              <h3 className="review-section__heading">Project</h3>
              <dl className="review-stats">
                <div className="review-stat">
                  <dt>Name</dt>
                  <dd data-testid="review-project-name">{data.project.name}</dd>
                </div>
                <div className="review-stat">
                  <dt>Path</dt>
                  <dd data-testid="review-project-path" title={data.project.path}>
                    {data.project.path}
                  </dd>
                </div>
                <div className="review-stat">
                  <dt>Trusted</dt>
                  <dd>{data.project.trusted ? 'Yes' : 'No'}</dd>
                </div>
              </dl>
            </section>

            {/* Git */}
            <section className="review-section">
              <h3 className="review-section__heading">Git</h3>
              {data.git.available ? (
                <dl className="review-stats">
                  <div className="review-stat">
                    <dt>Branch</dt>
                    <dd data-testid="review-git-branch">
                      {data.git.branch || '\u2014'}
                    </dd>
                  </div>
                  <div className="review-stat">
                    <dt>Status</dt>
                    <dd data-testid="review-git-status">
                      {data.git.dirty ? 'Dirty' : 'Clean'}
                    </dd>
                  </div>
                  {data.git.changedFiles.length > 0 && (
                    <div className="review-stat">
                      <dt>Changed Files</dt>
                      <dd data-testid="review-changed-files">
                        <ul className="review-file-list">
                          {data.git.changedFiles.map((f, i) => (
                            <li key={i}>{f}</li>
                          ))}
                        </ul>
                      </dd>
                    </div>
                  )}
                </dl>
              ) : (
                <p className="project-message">Not a Git repository.</p>
              )}
            </section>

            {/* Session */}
            <section className="review-section">
              <h3 className="review-section__heading">Session</h3>
              <dl className="review-stats">
                <div className="review-stat">
                  <dt>ID</dt>
                  <dd data-testid="review-session-id" className="review-stat__mono">
                    {data.session.id}
                  </dd>
                </div>
                <div className="review-stat">
                  <dt>Messages</dt>
                  <dd data-testid="review-message-count">
                    {data.session.messageCount}
                  </dd>
                </div>
                {data.session.lastAssistantSummary && (
                  <div className="review-stat">
                    <dt>Last Assistant</dt>
                    <dd
                      data-testid="review-last-assistant-summary"
                      className="review-stat__excerpt"
                    >
                      {data.session.lastAssistantSummary}
                    </dd>
                  </div>
                )}
              </dl>
            </section>

            {/* Activity */}
            <section className="review-section">
              <h3 className="review-section__heading">Activity</h3>
              <dl className="review-stats">
                <div className="review-stat">
                  <dt>Messages</dt>
                  <dd data-testid="review-activity-summary">
                    {data.activity.messageCount}
                  </dd>
                </div>
                {data.activity.note && (
                  <p className="project-message review-activity-note">
                    {data.activity.note}
                  </p>
                )}
              </dl>
            </section>

            <p className="review-updated">
              Updated: {new Date(data.updatedAt).toLocaleString()}
            </p>
          </div>
        )}
      </div>
    </div>
  )
}

export default ReviewPanel

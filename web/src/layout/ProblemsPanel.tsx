import { useState, useEffect, useCallback } from 'react'
import type { Problem } from '../types/problems'
import { requestJson } from '../api/client'

function ProblemsPanel() {
  const [problems, setProblems] = useState<Problem[]>([])
  const [loading, setLoading] = useState(true)

  const loadProblems = useCallback(async () => {
    try {
      setLoading(true)
      const data = await requestJson<Problem[]>('/api/problems')
      setProblems(data)
    } catch {
      /* API unavailable — show empty */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadProblems()
    const interval = setInterval(loadProblems, 5000)
    return () => clearInterval(interval)
  }, [loadProblems])

  async function handleClear() {
    try {
      await fetch('/api/problems/clear', { method: 'POST' })
      setProblems([])
    } catch {}
  }

  if (loading && problems.length === 0) {
    return (
      <div className="panel-body panel-placeholder" data-testid="problems-panel">
        <p>Loading problems…</p>
      </div>
    )
  }

  return (
    <div className="problems-panel" data-testid="problems-panel">
      <div className="problems-panel__toolbar">
        <span className="problems-panel__title">Problems</span>
        {problems.length > 0 && (
          <button
            className="problems-panel__clear-btn"
            onClick={handleClear}
            data-testid="problems-clear-button"
          >
            Clear
          </button>
        )}
      </div>
      <div className="panel-body">
        {problems.length === 0 ? (
          <p className="project-message" data-testid="problems-empty-state">
            No problems detected.
          </p>
        ) : (
          <ul className="problems-list">
            {problems.map((p) => (
              <li
                key={p.id}
                className={`problems-item problems-item--${p.level}`}
                data-testid="problems-item"
              >
                <div className="problems-item__header">
                  <span className="problems-item__level">{p.level.toUpperCase()}</span>
                  <span className="problems-item__source">{p.source}</span>
                  <span className="problems-item__time">
                    {new Date(p.createdAt).toLocaleTimeString()}
                  </span>
                </div>
                <p className="problems-item__message">{p.message}</p>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

export default ProblemsPanel

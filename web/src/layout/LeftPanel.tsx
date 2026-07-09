import { useEffect, useState, type FormEvent } from 'react'
import { createProject, deleteProject, fetchProjects } from '../api/projects'
import { fetchOpencodeStatus, fetchServerStatus, startServer, stopServer } from '../api/opencode'
import { createSession, fetchSessions } from '../api/sessions'
import type { Project } from '../types/project'
import type { OpencodeStatus, ServerStatus } from '../types/opencode'
import type { GlyphSession } from '../types/session'

interface LeftPanelProps {
  initialProjectId?: string | null
  initialSessionId?: string | null
  onSelectProject?: (projectId: string) => void
  onSelectSession?: (projectId: string, sessionId: string) => void
}

function LeftPanel({ initialProjectId, onSelectProject, onSelectSession }: LeftPanelProps) {
  const [projects, setProjects] = useState<Project[]>([])
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [trusted, setTrusted] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [deletingIds, setDeletingIds] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)
  const [opencodeStatus, setOpencodeStatus] = useState<OpencodeStatus | null>(null)
  const [serverStatuses, setServerStatuses] = useState<Record<string, ServerStatus>>({})
  const [serverLoading, setServerLoading] = useState<Record<string, boolean>>({})
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(
    initialProjectId ?? null,
  )
  const [sessions, setSessions] = useState<GlyphSession[]>([])
  const [isCreatingSession, setIsCreatingSession] = useState(false)
  const [sessionsLoading, setSessionsLoading] = useState(false)
  const [sessionError, setSessionError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()

    async function loadProjects() {
      try {
        setIsLoading(true)
        setError(null)
        setProjects(await fetchProjects(controller.signal))
      } catch (loadError) {
        if (loadError instanceof DOMException && loadError.name === 'AbortError') {
          return
        }

        setError(loadError instanceof Error ? loadError.message : 'Could not load projects.')
      } finally {
        if (!controller.signal.aborted) {
          setIsLoading(false)
        }
      }
    }

    loadProjects()

    return () => controller.abort()
  }, [])

  useEffect(() => {
    const controller = new AbortController()

    async function loadOpencodeStatus() {
      try {
        const status = await fetchOpencodeStatus(controller.signal)
        if (!controller.signal.aborted) {
          setOpencodeStatus(status)
        }
      } catch {
        // OpenCode status unavailable — leave as null
      }
    }

    loadOpencodeStatus()

    return () => controller.abort()
  }, [])

  useEffect(() => {
    if (projects.length === 0) {
      return
    }

    const controller = new AbortController()

    async function loadServerStatuses() {
      const statuses: Record<string, ServerStatus> = {}

      for (const project of projects) {
        if (controller.signal.aborted) {
          break
        }

        try {
          statuses[project.id] = await fetchServerStatus(project.id, controller.signal)
        } catch {
          // Server status unavailable for this project — skip
        }
      }

      if (!controller.signal.aborted) {
        setServerStatuses(statuses)
      }
    }

    loadServerStatuses()

    return () => controller.abort()
  }, [projects])

  async function handleSelectProject(projectId: string) {
    setSelectedProjectId(projectId)
    onSelectProject?.(projectId)
    setSessions([])
    setSessionError(null)

    try {
      setSessionsLoading(true)
      setSessionError(null)
      const fetched = await fetchSessions(projectId)
      setSessions(fetched)
    } catch (sessionLoadError) {
      setSessionError(
        sessionLoadError instanceof Error
          ? sessionLoadError.message
          : 'Could not load sessions.',
      )
    } finally {
      setSessionsLoading(false)
    }
  }

  async function handleCreateSession() {
    if (!selectedProjectId) return

    try {
      setIsCreatingSession(true)
      setSessionError(null)
      const title = `Session ${new Date().toLocaleTimeString()}`
      const session = await createSession(selectedProjectId, { title })
      setSessions((prev) => [...prev, session])
    } catch (createError) {
      setSessionError(
        createError instanceof Error ? createError.message : 'Could not create session.',
      )
    } finally {
      setIsCreatingSession(false)
    }
  }

  function handleSelectSession(projectId: string, sessionId: string) {
    onSelectSession?.(projectId, sessionId)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const trimmedName = name.trim()
    const trimmedPath = path.trim()

    if (!trimmedName || !trimmedPath) {
      setError('Name and path are required.')
      return
    }

    try {
      setIsSubmitting(true)
      setError(null)
      const project = await createProject({
        name: trimmedName,
        path: trimmedPath,
        trusted,
      })

      setProjects((currentProjects) => [...currentProjects, project])
      setName('')
      setPath('')
      setTrusted(false)
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : 'Could not add project.')
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleDelete(projectId: string) {
    try {
      setDeletingIds((currentIds) => new Set(currentIds).add(projectId))
      setError(null)
      await deleteProject(projectId)
      setProjects((currentProjects) =>
        currentProjects.filter((project) => project.id !== projectId),
      )

      if (selectedProjectId === projectId) {
        setSelectedProjectId(null)
        setSessions([])
      }
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : 'Could not remove project.')
    } finally {
      setDeletingIds((currentIds) => {
        const nextIds = new Set(currentIds)
        nextIds.delete(projectId)
        return nextIds
      })
    }
  }

  async function handleStartServer(projectId: string) {
    try {
      setServerLoading((prev) => ({ ...prev, [projectId]: true }))
      const status = await startServer(projectId)
      setServerStatuses((prev) => ({ ...prev, [projectId]: status }))
    } catch (startError) {
      setError(startError instanceof Error ? startError.message : 'Could not start server')
    } finally {
      setServerLoading((prev) => ({ ...prev, [projectId]: false }))
    }
  }

  async function handleStopServer(projectId: string) {
    try {
      setServerLoading((prev) => ({ ...prev, [projectId]: true }))
      const status = await stopServer(projectId)
      setServerStatuses((prev) => ({ ...prev, [projectId]: status }))
    } catch (stopError) {
      setError(stopError instanceof Error ? stopError.message : 'Could not stop server')
    } finally {
      setServerLoading((prev) => ({ ...prev, [projectId]: false }))
    }
  }

  const selectedProjectReady =
    selectedProjectId != null &&
    serverStatuses[selectedProjectId]?.status === 'ready'

  // Auto-load sessions when selected project becomes ready
  // (covers browser refresh, server start, and manual selection).
  useEffect(() => {
    if (!selectedProjectReady || !selectedProjectId) return

    const controller = new AbortController()
    setSessionsLoading(true)
    setSessionError(null)

    fetchSessions(selectedProjectId, controller.signal)
      .then((fetched) => {
        if (!controller.signal.aborted) setSessions(fetched)
      })
      .catch((err) => {
        if (!controller.signal.aborted)
          setSessionError(err instanceof Error ? err.message : 'Could not load sessions.')
      })
      .finally(() => {
        if (!controller.signal.aborted) setSessionsLoading(false)
      })

    return () => controller.abort()
  }, [selectedProjectId, selectedProjectReady])

  return (
    <aside className="left-panel">
      <div className="panel-header">Projects</div>
      <div className="panel-body projects-panel">
        <form className="project-form" onSubmit={handleSubmit}>
          <div className="project-form__field">
            <label htmlFor="project-name">Name</label>
            <input
              id="project-name"
              name="name"
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              disabled={isSubmitting}
              required
              data-testid="project-name-input"
            />
          </div>
          <div className="project-form__field">
            <label htmlFor="project-path">Path</label>
            <input
              id="project-path"
              name="path"
              type="text"
              value={path}
              onChange={(event) => setPath(event.target.value)}
              disabled={isSubmitting}
              required
              data-testid="project-path-input"
            />
          </div>
          <label className="project-form__checkbox" htmlFor="project-trusted">
            <input
              id="project-trusted"
              name="trusted"
              type="checkbox"
              checked={trusted}
              onChange={(event) => setTrusted(event.target.checked)}
              disabled={isSubmitting}
              data-testid="project-trusted-checkbox"
            />
            Trusted
          </label>
          <button className="project-form__submit" type="submit" disabled={isSubmitting} data-testid="add-project-button">
            {isSubmitting ? 'Adding…' : 'Add Project'}
          </button>
        </form>

        {opencodeStatus ? (
          opencodeStatus.installed ? (
            <div className="opencode-banner opencode-banner--ok" data-testid="opencode-status-banner">
              OpenCode {opencodeStatus.version} ready
            </div>
          ) : (
            <div className="opencode-banner opencode-banner--missing" data-testid="opencode-status-banner">
              OpenCode not found. Install to start servers.
            </div>
          )
        ) : null}

        {error ? (
          <p className="project-message project-message--error" role="alert">
            {error}
          </p>
        ) : null}

        <div className="project-list" aria-live="polite">
          {isLoading ? <p className="project-message">Loading projects…</p> : null}

          {!isLoading && !error && projects.length === 0 ? (
            <p className="project-message project-message--empty">No projects registered yet.</p>
          ) : null}

          {!isLoading && projects.length > 0 ? (
            <ul className="project-list__items" aria-label="Registered projects">
              {projects.map((project) => {
                const isDeleting = deletingIds.has(project.id)
                const isSelected = selectedProjectId === project.id

                return (
                  <li className="project-card" key={project.id} data-testid="project-card">
                    <div className="project-card__main">
                      <div className="project-card__title-row">
                        <h2 className="project-card__name">{project.name}</h2>
                        <span className="project-card__trust">
                          {project.trusted ? 'Trusted' : 'Untrusted'}
                        </span>
                      </div>
                      <p className="project-card__path" title={project.path}>
                        {project.path}
                      </p>
                      <p className="project-card__git">
                        {project.git.isRepo
                          ? `Git repo · ${project.git.branch || 'No branch'}`
                          : 'Not a Git repo'}
                      </p>
                      {opencodeStatus?.installed && (
                        <div className="project-card__server">
                          {serverLoading[project.id] ? (
                            <span className="server-status" data-testid="server-status">Starting server...</span>
                          ) : serverStatuses[project.id]?.status === 'ready' ? (
                            <div className="server-info">
                              <span className="server-status server-status--ready" data-testid="server-status">
                                Ready on port {serverStatuses[project.id].port}
                              </span>
                              {serverStatuses[project.id].version && (
                                <span className="server-version">
                                  v{serverStatuses[project.id].version}
                                </span>
                              )}
                            </div>
                          ) : serverStatuses[project.id]?.status === 'starting' ? (
                            <span className="server-status server-status--starting" data-testid="server-status">
                              Starting...
                            </span>
                          ) : serverStatuses[project.id]?.status === 'stopping' ? (
                            <span className="server-status server-status--stopping" data-testid="server-status">
                              Stopping...
                            </span>
                          ) : serverStatuses[project.id]?.status === 'failed' ? (
                            <span className="server-status server-status--failed" data-testid="server-status">
                              Start failed
                            </span>
                          ) : (
                            <span className="server-status" data-testid="server-status">Server stopped</span>
                          )}
                          <div className="server-actions">
                            <button
                              className="server-btn server-btn--start"
                              onClick={() => handleStartServer(project.id)}
                              disabled={
                                serverLoading[project.id] ||
                                ['starting', 'ready', 'stopping'].includes(
                                  serverStatuses[project.id]?.status ?? '',
                                )
                              }
                              data-testid="project-start-server-button"
                            >
                              Start Server
                            </button>
                            <button
                              className="server-btn server-btn--stop"
                              onClick={() => handleStopServer(project.id)}
                              disabled={
                                serverLoading[project.id] ||
                                !serverStatuses[project.id] ||
                                ['stopped', 'not-installed', undefined].includes(
                                  serverStatuses[project.id]?.status,
                                ) ||
                                serverStatuses[project.id]?.status === 'stopping'
                              }
                              data-testid="project-stop-server-button"
                            >
                              Stop Server
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                    <div className="project-card__actions">
                      <button
                        className="project-card__remove"
                        type="button"
                        onClick={() => handleDelete(project.id)}
                        disabled={isDeleting}
                        aria-label={`Remove ${project.name}`}
                        data-testid="project-remove-button"
                      >
                        {isDeleting ? 'Removing…' : 'Remove'}
                      </button>
                      <button
                        className={
                            'project-card__select' +
                            (isSelected ? ' project-card__select--active' : '')
                          }
                          type="button"
                          onClick={() => handleSelectProject(project.id)}
                          aria-label={`Select ${project.name}`}
                          data-testid="project-select-button"
                      >
                        {isSelected ? 'Selected' : 'Select'}
                      </button>
                    </div>
                  </li>
                )
              })}
            </ul>
          ) : null}
        </div>

        {selectedProjectReady && (
          <div className="session-section" data-testid="sessions-section">
            <div className="session-section__header">
              <span className="session-section__title">Sessions</span>
              <button
                className="session-section__create-btn"
                type="button"
                onClick={() => void handleCreateSession()}
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
                  <li key={session.id}>
                    <button
                      className="session-item"
                      type="button"
                      onClick={() => handleSelectSession(selectedProjectId!, session.id)}
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
        )}
      </div>
    </aside>
  )
}

export default LeftPanel

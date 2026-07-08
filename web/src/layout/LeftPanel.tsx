import { useEffect, useState, type FormEvent } from 'react'
import { createProject, deleteProject, fetchProjects } from '../api/projects'
import { fetchOpencodeStatus, fetchServerStatus, startServer, stopServer } from '../api/opencode'
import type { Project } from '../types/project'
import type { OpencodeStatus, ServerStatus } from '../types/opencode'

function LeftPanel() {
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
            />
            Trusted
          </label>
          <button className="project-form__submit" type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Adding…' : 'Add Project'}
          </button>
        </form>

        {opencodeStatus ? (
          opencodeStatus.installed ? (
            <div className="opencode-banner opencode-banner--ok">
              OpenCode {opencodeStatus.version} ready
            </div>
          ) : (
            <div className="opencode-banner opencode-banner--missing">
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

                return (
                  <li className="project-card" key={project.id}>
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
                            <span className="server-status">Starting server...</span>
                          ) : serverStatuses[project.id]?.status === 'ready' ? (
                            <div className="server-info">
                              <span className="server-status server-status--ready">
                                Ready on port {serverStatuses[project.id].port}
                              </span>
                              {serverStatuses[project.id].version && (
                                <span className="server-version">
                                  v{serverStatuses[project.id].version}
                                </span>
                              )}
                            </div>
                          ) : serverStatuses[project.id]?.status === 'starting' ? (
                            <span className="server-status server-status--starting">
                              Starting...
                            </span>
                          ) : serverStatuses[project.id]?.status === 'stopping' ? (
                            <span className="server-status server-status--stopping">
                              Stopping...
                            </span>
                          ) : serverStatuses[project.id]?.status === 'failed' ? (
                            <span className="server-status server-status--failed">
                              Start failed
                            </span>
                          ) : (
                            <span className="server-status">Server stopped</span>
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
                            >
                              Stop Server
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                    <button
                      className="project-card__remove"
                      type="button"
                      onClick={() => handleDelete(project.id)}
                      disabled={isDeleting}
                      aria-label={`Remove ${project.name}`}
                    >
                      {isDeleting ? 'Removing…' : 'Remove'}
                    </button>
                  </li>
                )
              })}
            </ul>
          ) : null}
        </div>
      </div>
    </aside>
  )
}

export default LeftPanel

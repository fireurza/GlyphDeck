import { type FormEvent } from 'react'
import type { Project } from '../types/project'
import type { OpencodeStatus, ServerStatus } from '../types/opencode'

interface ProjectListProps {
  projects: Project[]
  name: string
  path: string
  trusted: boolean
  isLoading: boolean
  isSubmitting: boolean
  deletingIds: Set<string>
  error: string | null
  opencodeStatus: OpencodeStatus | null
  serverStatuses: Record<string, ServerStatus>
  serverLoading: Record<string, boolean>
  selectedProjectId: string | null
  onNameChange: (value: string) => void
  onPathChange: (value: string) => void
  onTrustedChange: (value: boolean) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onDelete: (projectId: string) => void
  onSelectProject: (projectId: string) => void
  onStartServer: (projectId: string) => void
  onStopServer: (projectId: string) => void
}

function ProjectList({
  projects,
  name,
  path,
  trusted,
  isLoading,
  isSubmitting,
  deletingIds,
  error,
  opencodeStatus,
  serverStatuses,
  serverLoading,
  selectedProjectId,
  onNameChange,
  onPathChange,
  onTrustedChange,
  onSubmit,
  onDelete,
  onSelectProject,
  onStartServer,
  onStopServer,
}: ProjectListProps) {
  return (
    <>
      <div className="panel-header">Projects</div>
      <div className="panel-body projects-panel" data-testid="left-panel-body">
        <form className="project-form" onSubmit={onSubmit}>
          <div className="project-form__field">
            <label htmlFor="project-name">Name</label>
            <input
              id="project-name"
              name="name"
              type="text"
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
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
              onChange={(event) => onPathChange(event.target.value)}
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
              onChange={(event) => onTrustedChange(event.target.checked)}
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
                              onClick={() => onStartServer(project.id)}
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
                              onClick={() => onStopServer(project.id)}
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
                        onClick={() => onDelete(project.id)}
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
                          onClick={() => onSelectProject(project.id)}
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
      </div>
    </>
  )
}

export default ProjectList

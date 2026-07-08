import { useEffect, useState, type FormEvent } from 'react'
import { createProject, deleteProject, fetchProjects } from '../api/projects'
import type { Project } from '../types/project'

function LeftPanel() {
  const [projects, setProjects] = useState<Project[]>([])
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [trusted, setTrusted] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [deletingIds, setDeletingIds] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)

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
      setProjects((currentProjects) => currentProjects.filter((project) => project.id !== projectId))
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

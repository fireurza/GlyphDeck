import { useEffect, useState, useCallback, type FormEvent } from 'react'
import { createProject, deleteProject, fetchProjects } from '../api/projects'
import { fetchOpencodeStatus, fetchServerStatus, startServer, stopServer } from '../api/opencode'
import { createSession, fetchSessions } from '../api/sessions'
import type { Project } from '../types/project'
import type { OpencodeStatus, ServerStatus } from '../types/opencode'
import type { GlyphSession } from '../types/session'
import ProjectList from './ProjectList'
import SessionList from './SessionList'

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
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(initialProjectId ?? null)
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

  const handleSelectProject = useCallback(async (projectId: string) => {
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
  }, [onSelectProject])

  const handleCreateSession = useCallback(async () => {
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
  }, [selectedProjectId])

  const handleSelectSession = useCallback((projectId: string, sessionId: string) => {
    onSelectSession?.(projectId, sessionId)
  }, [onSelectSession])

  const handleSubmit = useCallback(async (event: FormEvent<HTMLFormElement>) => {
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
  }, [name, path, trusted])

  const handleDelete = useCallback(async (projectId: string) => {
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
  }, [selectedProjectId])

  const handleStartServer = useCallback(async (projectId: string) => {
    try {
      setServerLoading((prev) => ({ ...prev, [projectId]: true }))
      const status = await startServer(projectId)
      setServerStatuses((prev) => ({ ...prev, [projectId]: status }))
    } catch (startError) {
      setError(startError instanceof Error ? startError.message : 'Could not start server')
    } finally {
      setServerLoading((prev) => ({ ...prev, [projectId]: false }))
    }
  }, [])

  const handleStopServer = useCallback(async (projectId: string) => {
    try {
      setServerLoading((prev) => ({ ...prev, [projectId]: true }))
      const status = await stopServer(projectId)
      setServerStatuses((prev) => ({ ...prev, [projectId]: status }))
    } catch (stopError) {
      setError(stopError instanceof Error ? stopError.message : 'Could not stop server')
    } finally {
      setServerLoading((prev) => ({ ...prev, [projectId]: false }))
    }
  }, [])

  const selectedProjectReady =
    selectedProjectId != null &&
    serverStatuses[selectedProjectId]?.status === 'ready'

  // Auto-load sessions when selected project becomes ready.
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
      <ProjectList
        projects={projects}
        name={name}
        path={path}
        trusted={trusted}
        isLoading={isLoading}
        isSubmitting={isSubmitting}
        deletingIds={deletingIds}
        error={error}
        opencodeStatus={opencodeStatus}
        serverStatuses={serverStatuses}
        serverLoading={serverLoading}
        selectedProjectId={selectedProjectId}
        onNameChange={setName}
        onPathChange={setPath}
        onTrustedChange={setTrusted}
        onSubmit={handleSubmit}
        onDelete={handleDelete}
        onSelectProject={handleSelectProject}
        onStartServer={handleStartServer}
        onStopServer={handleStopServer}
      />
      {selectedProjectReady && (
        <SessionList
          sessions={sessions}
          isCreatingSession={isCreatingSession}
          sessionsLoading={sessionsLoading}
          sessionError={sessionError}
          selectedProjectId={selectedProjectId}
          onCreateSession={handleCreateSession}
          onSelectSession={handleSelectSession}
        />
      )}
    </aside>
  )
}

export default LeftPanel

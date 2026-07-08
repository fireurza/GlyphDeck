import type { CreateProjectInput, Project } from '../types/project'

type ProjectsResponse = {
  projects: Project[]
}

type ApiErrorPayload = {
  error?: { code?: string; message?: string } | string
  message?: string
}

function getErrorText(value: unknown) {
  return typeof value === 'string' && value.trim().length > 0 ? value : undefined
}

async function getResponseError(response: Response) {
  const fallback = `Request failed (${response.status})`

  try {
    const payload = (await response.json()) as ApiErrorPayload

    const nestedMessage =
      typeof payload.error === 'object' && payload.error !== null
        ? getErrorText(payload.error.message)
        : undefined

    return (
      nestedMessage || getErrorText(payload.error) || getErrorText(payload.message) || fallback
    )
  } catch {
    return fallback
  }
}

async function requestJson<T>(url: string, init?: RequestInit) {
  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
    ...init,
  })

  if (!response.ok) {
    throw new Error(await getResponseError(response))
  }

  return (await response.json()) as T
}

export async function fetchProjects(signal?: AbortSignal) {
  const data = await requestJson<ProjectsResponse>('/api/projects', { signal })
  return data.projects
}

export async function createProject(input: CreateProjectInput) {
  return requestJson<Project>('/api/projects', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function deleteProject(projectId: string) {
  const response = await fetch(`/api/projects/${encodeURIComponent(projectId)}`, {
    method: 'DELETE',
  })

  if (!response.ok) {
    throw new Error(await getResponseError(response))
  }
}

import type { OpencodeStatus, ServerStatus } from '../types/opencode'

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

export async function fetchOpencodeStatus(signal?: AbortSignal) {
  return requestJson<OpencodeStatus>('/api/opencode', { signal })
}

export async function fetchServerStatus(projectId: string, signal?: AbortSignal) {
  return requestJson<ServerStatus>(
    `/api/projects/${encodeURIComponent(projectId)}/server`,
    { signal },
  )
}

export async function startServer(projectId: string) {
  return requestJson<ServerStatus>(
    `/api/projects/${encodeURIComponent(projectId)}/server/start`,
    { method: 'POST' },
  )
}

export async function stopServer(projectId: string) {
  return requestJson<ServerStatus>(
    `/api/projects/${encodeURIComponent(projectId)}/server/stop`,
    { method: 'POST' },
  )
}

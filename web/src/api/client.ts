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
      nestedMessage ||
      getErrorText(payload.error) ||
      getErrorText(payload.message) ||
      fallback
    )
  } catch {
    return fallback
  }
}

export async function requestJson<T>(url: string, init?: RequestInit) {
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

export async function requestDelete(url: string) {
  const response = await fetch(url, { method: 'DELETE' })

  if (!response.ok) {
    throw new Error(await getResponseError(response))
  }
}

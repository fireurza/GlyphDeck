import type { TerminalStatus } from '../types/terminal'
import { requestJson } from './client'

const BACKEND = ''

async function requestNoContent(url: string, init: RequestInit): Promise<void> {
  const response = await fetch(url, init)
  if (response.ok) return

  let message = `Request failed (${response.status}).`
  try {
    const payload = (await response.json()) as { error?: { message?: string } | string; message?: string }
    if (typeof payload.error === 'object' && payload.error?.message) {
      message = payload.error.message
    } else if (typeof payload.error === 'string') {
      message = payload.error
    } else if (payload.message) {
      message = payload.message
    }
  } catch {
    // Preserve the safe status fallback.
  }
  throw new Error(message)
}

export async function startTerminal(
  projectId: string,
  cwd: string,
): Promise<TerminalStatus> {
  return requestJson<TerminalStatus>(
    `/api/projects/${encodeURIComponent(projectId)}/terminals`,
    {
      method: 'POST',
      body: JSON.stringify({ cwd }),
    },
  )
}

export function streamTerminal(
  terminalId: string,
  onData: (text: string) => void,
  onClose: () => void,
  signal: AbortSignal,
): void {
  const url = `${BACKEND}/api/terminals/${encodeURIComponent(terminalId)}/stream`

  fetch(url, { signal })
    .then(async (response) => {
      if (!response.ok || !response.body) {
        onClose()
        return
      }
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            onData(line.slice(6))
          } else if (line.startsWith('event: closed')) {
            onClose()
            return
          }
        }
      }
      onClose()
    })
    .catch(() => onClose())
}

export async function sendTerminalInput(
  terminalId: string,
  input: string,
): Promise<void> {
  await requestNoContent(
    `/api/terminals/${encodeURIComponent(terminalId)}/input`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input }),
    },
  )
}

export async function resizeTerminal(
  terminalId: string,
  rows: number,
  cols: number,
): Promise<void> {
  await requestNoContent(
    `/api/terminals/${encodeURIComponent(terminalId)}/resize`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rows, cols }),
    },
  )
}

export async function closeTerminal(terminalId: string): Promise<void> {
  await requestNoContent(
    `/api/terminals/${encodeURIComponent(terminalId)}/close`,
    { method: 'POST' },
  )
}

export async function terminalStatus(
  terminalId: string,
): Promise<TerminalStatus> {
  return requestJson<TerminalStatus>(
    `/api/terminals/${encodeURIComponent(terminalId)}/status`,
  )
}

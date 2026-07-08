import type {
  CreateSessionRequest,
  GlyphMessage,
  GlyphSession,
  PromptResult,
  SendPromptRequest,
} from '../types/session'
import { requestJson } from './client'

type SessionsResponse = {
  sessions: GlyphSession[]
}

type MessagesResponse = {
  messages: GlyphMessage[]
}

function sessionBase(projectId: string, sessionId?: string) {
  const base = `/api/projects/${encodeURIComponent(projectId)}/sessions`
  return sessionId ? `${base}/${encodeURIComponent(sessionId)}` : base
}

export async function fetchSessions(
  projectId: string,
  signal?: AbortSignal,
): Promise<GlyphSession[]> {
  const data = await requestJson<SessionsResponse>(sessionBase(projectId), { signal })
  return data.sessions
}

export async function createSession(
  projectId: string,
  input: CreateSessionRequest,
): Promise<GlyphSession> {
  return requestJson<GlyphSession>(sessionBase(projectId), {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function fetchSession(
  projectId: string,
  sessionId: string,
  signal?: AbortSignal,
): Promise<GlyphSession> {
  return requestJson<GlyphSession>(sessionBase(projectId, sessionId), { signal })
}

export async function fetchMessages(
  projectId: string,
  sessionId: string,
  signal?: AbortSignal,
): Promise<GlyphMessage[]> {
  const data = await requestJson<MessagesResponse>(
    `${sessionBase(projectId, sessionId)}/messages`,
    { signal },
  )
  return data.messages
}

export async function sendPrompt(
  projectId: string,
  sessionId: string,
  input: SendPromptRequest,
): Promise<PromptResult> {
  return requestJson<PromptResult>(
    `${sessionBase(projectId, sessionId)}/prompt`,
    {
      method: 'POST',
      body: JSON.stringify(input),
    },
  )
}

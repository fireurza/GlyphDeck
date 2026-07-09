import type { UsageResponse } from '../types/usage'
import { requestJson } from './client'

export async function fetchUsage(
  projectId: string,
  sessionId: string,
  signal?: AbortSignal,
): Promise<UsageResponse> {
  return requestJson<UsageResponse>(
    `/api/projects/${encodeURIComponent(projectId)}/sessions/${encodeURIComponent(sessionId)}/usage`,
    { signal },
  )
}

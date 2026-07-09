import type { ReviewResponse } from '../types/review'
import { requestJson } from './client'

export async function fetchReview(
  projectId: string,
  sessionId: string,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  return requestJson<ReviewResponse>(
    `/api/projects/${encodeURIComponent(projectId)}/sessions/${encodeURIComponent(sessionId)}/review`,
    { signal },
  )
}

import type { PermissionRequest, PermissionReply } from '../types/permissions'
import { requestJson } from './client'

export async function fetchPermissions(
  projectId: string,
  signal?: AbortSignal,
): Promise<PermissionRequest[]> {
  return requestJson<PermissionRequest[]>(
    `/api/permissions?projectId=${encodeURIComponent(projectId)}`,
    { signal },
  )
}

export async function replyPermission(
  projectId: string,
  requestId: string,
  reply: PermissionReply,
): Promise<void> {
  await fetch(
    `/api/permissions/${encodeURIComponent(requestId)}/reply?projectId=${encodeURIComponent(projectId)}`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(reply),
    },
  )
}

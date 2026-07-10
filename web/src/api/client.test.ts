import { requestDelete, requestJson } from './client'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

test('requestJson returns parsed JSON for successful responses', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    jsonResponse({ status: 'ok' }),
  )

  await expect(requestJson<{ status: string }>('/api/test')).resolves.toEqual({
    status: 'ok',
  })
})

test('requestJson surfaces nested API error messages', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    jsonResponse(
      { error: { message: 'Project path is required.' } },
      { status: 400 },
    ),
  )

  await expect(requestJson('/api/test')).rejects.toThrow(
    'Project path is required.',
  )
})

test('requestDelete throws a safe status fallback for non-JSON errors', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    new Response('not found', { status: 404 }),
  )

  await expect(requestDelete('/api/test')).rejects.toThrow(
    'Request failed (404)',
  )
})

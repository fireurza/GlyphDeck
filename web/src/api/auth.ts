export interface AuthStatus {
  setupRequired: boolean
  loginRequired: boolean
  adminExists: boolean
  username?: string
}

export async function fetchAuthStatus(): Promise<AuthStatus> {
  const resp = await fetch('/api/auth/status')
  if (!resp.ok) throw new Error('Failed to check auth status')
  return resp.json()
}

export async function setupAdmin(password: string): Promise<void> {
  const resp = await fetch('/api/auth/setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: { message: 'Setup failed' } }))
    throw new Error(err.error?.message || 'Setup failed')
  }
}

export async function login(password: string): Promise<void> {
  const resp = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: { message: 'Login failed' } }))
    throw new Error(err.error?.message || 'Login failed')
  }
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' })
}

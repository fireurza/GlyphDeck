import { useState, type FormEvent } from 'react'
import { setupAdmin } from '../api/auth'

interface SetupScreenProps {
  onComplete: () => void
}

function SetupScreen({ onComplete }: SetupScreenProps) {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)

    if (password.length < 8) {
      setError('Password must be at least 8 characters.')
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match.')
      return
    }

    setLoading(true)
    try {
      await setupAdmin(password)
      onComplete()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-screen" data-testid="setup-screen">
      <div className="auth-screen__card">
        <h1 className="auth-screen__title">GlyphDeck Setup</h1>
        <p className="auth-screen__subtitle">
          Create an admin password to secure your GlyphDeck instance.
        </p>
        <form className="auth-form" onSubmit={handleSubmit}>
          <div className="auth-form__field">
            <label htmlFor="setup-password">Admin Password</label>
            <input
              id="setup-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
              required
              autoFocus
              data-testid="setup-password-input"
            />
          </div>
          <div className="auth-form__field">
            <label htmlFor="setup-confirm">Confirm Password</label>
            <input
              id="setup-confirm"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder="Re-enter password"
              required
              data-testid="setup-confirm-input"
            />
          </div>
          {error && <p className="auth-form__error" role="alert">{error}</p>}
          <button
            type="submit"
            className="auth-form__submit"
            disabled={loading}
            data-testid="setup-submit-button"
          >
            {loading ? 'Creating…' : 'Create Admin'}
          </button>
        </form>
      </div>
    </div>
  )
}

export default SetupScreen

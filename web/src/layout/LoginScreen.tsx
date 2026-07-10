import { useState, type FormEvent } from 'react'
import { login } from '../api/auth'

interface LoginScreenProps {
  onComplete: () => void
}

function LoginScreen({ onComplete }: LoginScreenProps) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await login(password)
      onComplete()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-screen" data-testid="login-screen">
      <div className="auth-screen__card">
        <h1 className="auth-screen__title">GlyphDeck</h1>
        <p className="auth-screen__subtitle">Enter your admin password to continue.</p>
        <form className="auth-form" onSubmit={handleSubmit}>
          <div className="auth-form__field">
            <label htmlFor="login-password">Password</label>
            <input
              id="login-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Admin password"
              required
              autoFocus
              data-testid="login-password-input"
            />
          </div>
          {error && <p className="auth-form__error" role="alert">{error}</p>}
          <button
            type="submit"
            className="auth-form__submit"
            disabled={loading}
            data-testid="login-submit-button"
          >
            {loading ? 'Logging in…' : 'Login'}
          </button>
        </form>
      </div>
    </div>
  )
}

export default LoginScreen

import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate, type Location } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { Notice } from '../components/ui'
import { useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'
import type { MessageKey } from '../i18n/messages'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/** Where to go after logging in: the page that sent the user here, or the catalog. */
function useReturnPath(): string {
  const from = (useLocation().state as { from?: Location } | null)?.from
  return from ? from.pathname + from.search : '/'
}

function useAuthForm(submit: () => Promise<void>, validate: () => MessageKey | null) {
  const { t, error: message } = useI18n()
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const invalid = validate()
    if (invalid) {
      setError(t(invalid))
      return
    }
    setBusy(true)
    setError(null)
    try {
      await submit()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }
  return { error, busy, onSubmit }
}

export function LoginPage() {
  const { t } = useI18n()
  const { user, login, expired } = useAuth()
  const navigate = useNavigate()
  const returnPath = useReturnPath()
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  useTitle(t('auth.loginTitle'))

  const form = useAuthForm(
    async () => {
      await login(email.trim(), password)
      navigate(returnPath, { replace: true })
    },
    () => (!email.trim() || !password ? 'auth.invalidCredentials' : null),
  )

  if (user && !form.busy) return <Navigate to={returnPath} replace />

  return (
    <div className="auth-page">
      <form className="card auth-card" onSubmit={form.onSubmit} noValidate>
        <h1>{t('auth.loginTitle')}</h1>
        {expired && !form.error && <Notice>{t('auth.sessionExpired')}</Notice>}
        {form.error && <Notice kind="error">{form.error}</Notice>}
        <label className="field">
          <span>{t('auth.email')}</span>
          <input type="email" autoComplete="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
        </label>
        <label className="field">
          <span>{t('auth.password')}</span>
          <input
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <button type="submit" className="btn btn-primary btn-block" disabled={form.busy}>
          {t('auth.submitLogin')}
        </button>
        <p className="muted auth-switch">
          {t('auth.noAccount')}{' '}
          <Link to="/register" state={location.state}>
            {t('nav.register')}
          </Link>
        </p>
      </form>
    </div>
  )
}

export function RegisterPage() {
  const { t } = useI18n()
  const { user, register } = useAuth()
  const navigate = useNavigate()
  const returnPath = useReturnPath()
  const location = useLocation()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  useTitle(t('auth.registerTitle'))

  const form = useAuthForm(
    async () => {
      await register(name.trim(), email.trim(), password)
      navigate(returnPath, { replace: true })
    },
    () => {
      if (!name.trim()) return 'auth.nameRequired'
      if (!EMAIL_RE.test(email.trim())) return 'auth.emailInvalid'
      if (password.length < 8) return 'auth.passwordShort'
      if (new TextEncoder().encode(password).length > 72) return 'auth.passwordLong'
      return null
    },
  )

  if (user && !form.busy) return <Navigate to={returnPath} replace />

  return (
    <div className="auth-page">
      <form className="card auth-card" onSubmit={form.onSubmit} noValidate>
        <h1>{t('auth.registerTitle')}</h1>
        {form.error && <Notice kind="error">{form.error}</Notice>}
        <label className="field">
          <span>{t('auth.name')}</span>
          <input autoComplete="name" required maxLength={200} value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="field">
          <span>{t('auth.email')}</span>
          <input type="email" autoComplete="email" required maxLength={254} value={email} onChange={(e) => setEmail(e.target.value)} />
        </label>
        <label className="field">
          <span>{t('auth.password')}</span>
          <input
            type="password"
            autoComplete="new-password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <small className="muted">{t('auth.passwordHint')}</small>
        </label>
        <button type="submit" className="btn btn-primary btn-block" disabled={form.busy}>
          {t('auth.submitRegister')}
        </button>
        <p className="muted auth-switch">
          {t('auth.haveAccount')}{' '}
          <Link to="/login" state={location.state}>
            {t('nav.login')}
          </Link>
        </p>
      </form>
    </div>
  )
}

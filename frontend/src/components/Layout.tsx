import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link, NavLink, useLocation, useNavigate, useSearchParams } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { useCart } from '../cart/CartProvider'
import { useI18n } from '../i18n/I18nProvider'
import { languages } from '../i18n/messages'
import { BagIcon, CartIcon, SearchIcon } from './icons'

function SearchBox() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const current = params.get('search') ?? ''
  const [query, setQuery] = useState(current)

  useEffect(() => {
    setQuery(current)
  }, [current])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const q = query.trim()
    navigate(q ? `/?search=${encodeURIComponent(q)}` : '/')
  }

  return (
    <form className="search" role="search" onSubmit={submit}>
      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={t('search.placeholder')}
        aria-label={t('search.placeholder')}
        maxLength={100}
      />
      <button type="submit" aria-label={t('search.button')}>
        <SearchIcon />
      </button>
    </form>
  )
}

function LanguageSwitch() {
  const { lang, setLang, t } = useI18n()
  return (
    <div className="lang-switch" role="group" aria-label={t('nav.language')}>
      {languages.map((l) => (
        <button key={l} type="button" aria-pressed={l === lang} onClick={() => setLang(l)}>
          {l.toUpperCase()}
        </button>
      ))}
    </div>
  )
}

function Header() {
  const { t } = useI18n()
  const { user, logout } = useAuth()
  const { count } = useCart()
  const navigate = useNavigate()

  return (
    <header className="header">
      <div className="container header-inner">
        <Link to="/" className="logo">
          <span className="logo-mark">
            <BagIcon width={18} height={18} />
          </span>
          {t('app.name')}
        </Link>
        <SearchBox />
        <nav className="nav">
          <LanguageSwitch />
          {user?.role === 'admin' && <NavLink to="/admin">{t('nav.admin')}</NavLink>}
          {user ? (
            <>
              <NavLink to="/orders">{t('nav.orders')}</NavLink>
              <button
                type="button"
                className="link-button"
                onClick={() => {
                  // Leave first, so a protected page doesn't redirect to login as the user disappears.
                  navigate('/')
                  logout()
                }}
              >
                {t('nav.logout')}
              </button>
            </>
          ) : (
            <>
              <NavLink to="/login">{t('nav.login')}</NavLink>
              <NavLink to="/register" className="btn btn-primary btn-sm">
                {t('nav.register')}
              </NavLink>
            </>
          )}
          <NavLink to="/cart" className="cart-link" aria-label={`${t('nav.cart')}: ${count}`}>
            <CartIcon />
            {count > 0 && <span className="cart-count">{count}</span>}
          </NavLink>
        </nav>
      </div>
    </header>
  )
}

export function Layout({ children }: { children: ReactNode }) {
  const { t } = useI18n()
  const { pathname } = useLocation()

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [pathname])

  return (
    <>
      <a className="skip-link" href="#main">
        {t('nav.skip')}
      </a>
      <Header />
      <main id="main" className="container main">
        {children}
      </main>
      <footer className="footer">
        <div className="container footer-inner">
          <span>
            © {new Date().getFullYear()} {t('app.name')}. {t('footer.rights')}
          </span>
          <span className="muted">{t('app.tagline')}</span>
        </div>
      </footer>
    </>
  )
}

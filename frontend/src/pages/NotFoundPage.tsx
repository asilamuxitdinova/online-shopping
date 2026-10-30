import { Link } from 'react-router'
import { useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'

export function NotFoundPage() {
  const { t } = useI18n()
  useTitle(t('notFound.title'))
  return (
    <div className="empty card">
      <h1>{t('notFound.title')}</h1>
      <p className="muted">{t('notFound.text')}</p>
      <Link to="/" className="btn btn-primary">
        {t('notFound.home')}
      </Link>
    </div>
  )
}

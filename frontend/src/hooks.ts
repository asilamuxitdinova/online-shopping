import { useEffect, useState, type DependencyList } from 'react'
import { useI18n } from './i18n/I18nProvider'

interface AsyncState<T> {
  data: T | undefined
  error: unknown
  loading: boolean
  reload: () => void
  setData: (data: T) => void
}

/**
 * Runs load() whenever deps change and tracks its result. Previous data stays
 * visible while reloading, so lists don't flash empty between pages.
 */
export function useAsync<T>(load: () => Promise<T>, deps: DependencyList): AsyncState<T> {
  const [data, setData] = useState<T>()
  const [error, setError] = useState<unknown>()
  const [loading, setLoading] = useState(true)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError(undefined)
    load().then(
      (result) => {
        if (!active) return
        setData(result)
        setLoading(false)
      },
      (err: unknown) => {
        if (!active) return
        setError(err)
        setLoading(false)
      },
    )
    return () => {
      active = false
    }
    // load is a fresh closure on every render; callers list its inputs in deps instead.
  }, [...deps, attempt])

  return { data, error, loading, reload: () => setAttempt((n) => n + 1), setData }
}

/** Sets the browser tab title, e.g. "Cart — Bozor". */
export function useTitle(title?: string) {
  const { t } = useI18n()
  const appName = t('app.name')
  useEffect(() => {
    document.title = title ? `${title} — ${appName}` : `${appName} — ${t('app.tagline')}`
  }, [title, appName, t])
}

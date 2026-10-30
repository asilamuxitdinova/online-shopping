import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { categoryName, errorMessage, formatDate, formatPrice, translate, type Params } from './format'
import { languages, type Lang, type MessageKey } from './messages'

interface I18n {
  lang: Lang
  setLang: (lang: Lang) => void
  t: (key: MessageKey, params?: Params) => string
  price: (amount: number) => string
  date: (iso: string) => string
  category: (category: string) => string
  error: (err: unknown) => string
}

const I18nContext = createContext<I18n | null>(null)
const STORAGE_KEY = 'shop.lang'

function isLang(value: unknown): value is Lang {
  return languages.includes(value as Lang)
}

/** Saved choice first, then the browser's languages, then Uzbek. */
function initialLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (isLang(saved)) return saved
  } catch {
    // storage unavailable; fall through
  }
  for (const tag of navigator.languages ?? []) {
    const base = tag.slice(0, 2).toLowerCase()
    if (isLang(base)) return base
  }
  return 'uz'
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(initialLang)

  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])

  const setLang = useCallback((next: Lang) => {
    setLangState(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // the choice just won't survive a reload
    }
  }, [])

  const value = useMemo<I18n>(
    () => ({
      lang,
      setLang,
      t: (key, params) => translate(lang, key, params),
      price: (amount) => formatPrice(amount, lang),
      date: (iso) => formatDate(iso, lang),
      category: (c) => categoryName(c, lang),
      error: (err) => errorMessage(err, lang),
    }),
    [lang, setLang],
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18n {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used inside I18nProvider')
  return ctx
}

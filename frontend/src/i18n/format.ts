import { ApiError } from '../api/client'
import { categoryNames, messages, type Lang, type MessageKey } from './messages'

export type Params = Record<string, string | number>

export function translate(lang: Lang, key: MessageKey, params?: Params): string {
  const text = messages[lang][key] ?? messages.en[key] ?? key
  if (!params) return text
  return text.replace(/\{(\w+)\}/g, (match, name: string) => (name in params ? String(params[name]) : match))
}

const locales: Record<Exclude<Lang, 'uz'>, string> = { ru: 'ru-RU', en: 'en-GB' }

const groupDigits = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 })

/** Formats whole so'm with space-grouped thousands, e.g. "1 150 000 soʻm". */
export function formatPrice(amount: number, lang: Lang): string {
  const digits = groupDigits.format(Math.round(amount)).replace(/\s/g, ' ')
  return `${digits} ${messages[lang].currency}`
}

const uzMonths = [
  'yanvar', 'fevral', 'mart', 'aprel', 'may', 'iyun',
  'iyul', 'avgust', 'sentabr', 'oktabr', 'noyabr', 'dekabr',
]

export function formatDate(iso: string, lang: Lang): string {
  const d = new Date(iso)
  if (lang === 'uz') {
    // Browsers' Uzbek date data is incomplete (e.g. "2026 M10 30"), so build it by hand.
    const time = new Intl.DateTimeFormat('ru-RU', { hour: '2-digit', minute: '2-digit' }).format(d)
    return `${d.getDate()}-${uzMonths[d.getMonth()]}, ${d.getFullYear()}, ${time}`
  }
  return new Intl.DateTimeFormat(locales[lang], { dateStyle: 'medium', timeStyle: 'short' }).format(d)
}

export function categoryName(category: string, lang: Lang): string {
  return categoryNames[lang][category] ?? category
}

// The API answers with English messages. These patterns turn the ones a
// shopper can trigger into translated text.
const knownErrors: [RegExp, (m: RegExpMatchArray) => [MessageKey, Params?]][] = [
  [/^Invalid email or password$/, () => ['auth.invalidCredentials']],
  [/^Email is already registered$/, () => ['auth.emailTaken']],
  [/^Only (\d+) item\(s\) in stock$/, (m) => ['error.onlyInStock', { count: m[1] }]],
  [/^At most (\d+) of one product/, (m) => ['error.maxQuantity', { count: m[1] }]],
  [/^Not enough stock for "(.*)"$/, (m) => ['error.notEnoughStock', { name: m[1] }]],
  [/^Cart is empty$/, () => ['error.cartEmpty']],
  [/^Cannot cancel an order/, () => ['error.cannotCancel']],
  [/^Too many attempts/, () => ['error.tooMany']],
  // Admin product form
  [/^Name is required$/, () => ['error.nameRequired']],
  [/^Image URL must be/, () => ['error.imageUrl']],
  [/^Price must be a number from 0 to (\d+)/, (m) => ['error.price', { max: m[1] }]],
  [/^Stock must be a whole number from 0 to (\d+)/, (m) => ['error.stock', { max: m[1] }]],
  [/must be at most (\d+) characters$/, (m) => ['error.tooLong', { max: m[1] }]],
  [/not found$/i, () => ['error.notFound']],
]

/** Turns an error from the API client into a message for the current language. */
export function errorMessage(err: unknown, lang: Lang): string {
  if (err instanceof TypeError) return translate(lang, 'error.network') // fetch could not connect
  if (!(err instanceof ApiError)) return translate(lang, 'error.generic')
  if (err.status === 429) return translate(lang, 'error.tooMany')
  if (err.status >= 500) return translate(lang, 'error.server')
  for (const [pattern, toMessage] of knownErrors) {
    const match = err.message.match(pattern)
    if (match) return translate(lang, ...toMessage(match))
  }
  return lang === 'en' ? err.message : translate(lang, 'error.generic')
}

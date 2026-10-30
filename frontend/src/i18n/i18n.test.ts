import { describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import { categoryName, errorMessage, formatDate, formatPrice, translate } from './format'
import { languages, messages, type MessageKey } from './messages'

const placeholders = (text: string) => [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()

describe('messages', () => {
  it('use the same placeholders in every language', () => {
    for (const key of Object.keys(messages.en) as MessageKey[]) {
      for (const lang of languages) {
        expect(placeholders(messages[lang][key]), `${lang} ${key}`).toEqual(placeholders(messages.en[key]))
      }
    }
  })

  it('have no empty translations', () => {
    for (const lang of languages) {
      for (const [key, text] of Object.entries(messages[lang])) {
        expect(text.trim(), `${lang} ${key}`).not.toBe('')
      }
    }
  })
})

describe('translate', () => {
  it('fills in placeholders', () => {
    expect(translate('en', 'product.fewLeft', { count: 3 })).toBe('Only 3 left')
    expect(translate('ru', 'order.title', { id: 7 })).toBe('Заказ №7')
  })
})

describe('formatPrice', () => {
  it('groups thousands with spaces and adds the currency', () => {
    const nbsp = ' '
    expect(formatPrice(1150000, 'uz')).toBe(`1${nbsp}150${nbsp}000${nbsp}soʻm`)
    expect(formatPrice(95000, 'ru')).toBe(`95${nbsp}000${nbsp}сум`)
    expect(formatPrice(999.6, 'en')).toBe(`1${nbsp}000${nbsp}UZS`)
  })
})

describe('formatDate', () => {
  it('writes Uzbek dates with month names', () => {
    const local = new Date(2026, 9, 30, 9, 5).toISOString()
    expect(formatDate(local, 'uz')).toBe('30-oktabr, 2026, 09:05')
    expect(formatDate(local, 'ru')).toContain('2026')
  })
})

describe('categoryName', () => {
  it('translates known categories and keeps others', () => {
    expect(categoryName('home', 'uz')).toBe('Uy-roʻzgʻor')
    expect(categoryName('bags', 'ru')).toBe('bags')
  })
})

describe('errorMessage', () => {
  it('translates known API errors', () => {
    expect(errorMessage(new ApiError(400, 'Only 2 item(s) in stock'), 'ru')).toBe('В наличии только 2 шт.')
    expect(errorMessage(new ApiError(409, 'Not enough stock for "Mug"'), 'en')).toBe('Not enough “Mug” in stock.')
    expect(errorMessage(new ApiError(401, 'Invalid email or password'), 'uz')).toBe(messages.uz['auth.invalidCredentials'])
    expect(errorMessage(new ApiError(400, 'Price must be a number from 0 to 1000000000'), 'ru')).toBe('Цена должна быть от 0 до 1000000000.')
    expect(errorMessage(new ApiError(400, 'Image URL must be an http or https link'), 'uz')).toBe(messages.uz['error.imageUrl'])
  })

  it('falls back by status and connection state', () => {
    expect(errorMessage(new ApiError(429, 'Too many attempts, try again later'), 'en')).toBe(messages.en['error.tooMany'])
    expect(errorMessage(new ApiError(503, 'Internal server error'), 'uz')).toBe(messages.uz['error.server'])
    expect(errorMessage(new TypeError('Failed to fetch'), 'ru')).toBe(messages.ru['error.network'])
    expect(errorMessage(new ApiError(400, 'Something new'), 'en')).toBe('Something new')
    expect(errorMessage(new ApiError(400, 'Something new'), 'uz')).toBe(messages.uz['error.generic'])
  })
})

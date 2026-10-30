import type { AuthResponse } from './types'

/** Base URL of the API; empty means same origin (nginx and the dev server proxy /api). */
const baseURL: string = import.meta.env?.VITE_API_URL ?? ''

export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// Tokens live in localStorage so every tab shares one session. Storage can
// throw (private mode, blocked site data), in which case the session only
// lasts as long as the page.
const ACCESS_KEY = 'shop.accessToken'
const REFRESH_KEY = 'shop.refreshToken'
const memory = new Map<string, string>()

function read(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return memory.get(key) ?? null
  }
}

function write(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key)
    else localStorage.setItem(key, value)
  } catch {
    if (value === null) memory.delete(key)
    else memory.set(key, value)
  }
}

export const tokens = {
  get access() {
    return read(ACCESS_KEY)
  },
  get refresh() {
    return read(REFRESH_KEY)
  },
  set(auth: Pick<AuthResponse, 'access_token' | 'refresh_token'>) {
    write(ACCESS_KEY, auth.access_token)
    write(REFRESH_KEY, auth.refresh_token)
  },
  clear() {
    write(ACCESS_KEY, null)
    write(REFRESH_KEY, null)
  },
  /** Storage key whose changes in other tabs signal login or logout there. */
  storageKey: REFRESH_KEY,
}

let onSessionExpired: () => void = () => {}

/** Registers what to do when the session can no longer be refreshed. */
export function setSessionExpiredHandler(handler: () => void) {
  onSessionExpired = handler
}

let refreshing: Promise<boolean> | null = null

/**
 * Gets a new token pair after `failedAccess` was rejected. The backend treats a
 * reused refresh token as theft and ends every session, so refreshes must never
 * overlap: within a tab they share one promise, across tabs they take turns
 * through a Web Lock, and a tab that finds new tokens already stored by another
 * tab uses those instead of refreshing again.
 */
export function refreshSession(failedAccess: string | null): Promise<boolean> {
  refreshing ??= withLock(async () => {
    const current = tokens.access
    if (current && current !== failedAccess) return true

    const refreshToken = tokens.refresh
    if (!refreshToken) return false

    const res = await fetch(baseURL + '/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken }),
    })
    if (!res.ok) {
      // 401 means the refresh token is dead. Keep it on 429/5xx so a later try can work.
      if (res.status === 400 || res.status === 401) tokens.clear()
      return false
    }
    tokens.set((await res.json()) as AuthResponse)
    return true
  }).finally(() => {
    refreshing = null
  })
  return refreshing
}

function withLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== 'undefined' ? navigator.locks : undefined
  return locks ? locks.request('shop-auth-refresh', fn) : fn()
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  /** Send the access token and refresh it on 401. Off for the auth endpoints themselves. */
  auth?: boolean
}

/** Calls the API and returns its JSON, throwing ApiError for error responses. */
export async function request<T>(path: string, { method = 'GET', body, auth = true }: RequestOptions = {}): Promise<T> {
  const send = (token: string | null) => {
    const headers: Record<string, string> = {}
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    if (token) headers.Authorization = `Bearer ${token}`
    return fetch(baseURL + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  }

  const token = auth ? tokens.access : null
  let res = await send(token)
  if (res.status === 401 && token) {
    if (await refreshSession(token)) {
      res = await send(tokens.access)
    } else if (!tokens.refresh) {
      onSessionExpired()
    }
  }

  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, (data as { error?: string } | null)?.error ?? res.statusText)
  }
  return data as T
}

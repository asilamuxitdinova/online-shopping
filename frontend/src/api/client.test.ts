import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, request, setSessionExpiredHandler, tokens } from './client'

const json = (status: number, body?: unknown) =>
  new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })

/** A fake API: protected routes accept only `validAccess`; refresh rotates the token pair. */
function fakeServer() {
  const state = { validAccess: 'access-2', refreshCalls: 0, refreshToken: 'refresh-1' }
  const fetchMock = vi.fn(async (url: string, init: RequestInit = {}) => {
    if (url === '/api/auth/refresh') {
      state.refreshCalls++
      const { refreshToken } = JSON.parse(String(init.body))
      if (refreshToken !== state.refreshToken) return json(401, { error: 'Invalid refresh token' })
      state.refreshToken = 'refresh-2'
      return json(200, { access_token: state.validAccess, refresh_token: 'refresh-2', token_type: 'Bearer', expires_in: 900 })
    }
    const auth = new Headers(init.headers).get('Authorization')
    if (auth !== `Bearer ${state.validAccess}`) return json(401, { error: 'Invalid or expired token' })
    return json(200, { ok: true })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { state, fetchMock }
}

beforeEach(() => {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => store.set(k, v),
    removeItem: (k: string) => store.delete(k),
  })
  tokens.set({ access_token: 'access-1', refresh_token: 'refresh-1' })
})

afterEach(() => {
  vi.unstubAllGlobals()
  setSessionExpiredHandler(() => {})
})

describe('request', () => {
  it('refreshes an expired access token once and retries', async () => {
    const { state } = fakeServer()
    await expect(request('/api/cart')).resolves.toEqual({ ok: true })
    expect(state.refreshCalls).toBe(1)
    expect(tokens.access).toBe('access-2')
    expect(tokens.refresh).toBe('refresh-2')
  })

  it('shares one refresh between parallel requests', async () => {
    // Two refreshes with the same token would look like token theft to the backend.
    const { state } = fakeServer()
    const results = await Promise.all([request('/api/cart'), request('/api/orders'), request('/api/auth/me')])
    expect(results).toEqual([{ ok: true }, { ok: true }, { ok: true }])
    expect(state.refreshCalls).toBe(1)
  })

  it('ends the session when the refresh token is rejected', async () => {
    const { state } = fakeServer()
    state.refreshToken = 'something-else'
    const expired = vi.fn()
    setSessionExpiredHandler(expired)

    await expect(request('/api/cart')).rejects.toBeInstanceOf(ApiError)
    expect(expired).toHaveBeenCalledOnce()
    expect(tokens.access).toBeNull()
    expect(tokens.refresh).toBeNull()
  })

  it('does not try to refresh for auth endpoints', async () => {
    const { state, fetchMock } = fakeServer()
    fetchMock.mockResolvedValueOnce(json(401, { error: 'Invalid email or password' }))
    const err = await request('/api/auth/login', { method: 'POST', body: {}, auth: false }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 401, message: 'Invalid email or password' })
    expect(state.refreshCalls).toBe(0)
  })

  it('returns undefined for 204 responses', async () => {
    const { fetchMock } = fakeServer()
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))
    await expect(request('/api/auth/logout', { method: 'POST', auth: false })).resolves.toBeUndefined()
  })
})

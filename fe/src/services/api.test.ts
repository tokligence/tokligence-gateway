import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  fetchProfile,
  fetchProviders,
  requestAuthLogin,
  requestAuthVerify,
  isUnauthorized,
} from './api'
import { sampleProviders } from './sampleData'

describe('gateway api client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('falls back to sample providers when network fails', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
    const mockFetch = vi.fn().mockRejectedValue(new Error('offline'))
    vi.stubGlobal('fetch', mockFetch)

    const result = await fetchProviders()

    expect(result).toEqual(sampleProviders)
    expect(warn).toHaveBeenCalled()
    expect(mockFetch).toHaveBeenCalledTimes(1)

    warn.mockRestore()
  })

  it('parses profile response', async () => {
    const mockPayload = {
      user: { id: 9, email: 'user@example.com', roles: ['consumer'] },
      provider: null,
    }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => mockPayload,
    })
    vi.stubGlobal('fetch', mockFetch)

    const result = await fetchProfile()

    expect(result).toEqual(mockPayload)
    expect(mockFetch).toHaveBeenCalled()
  })

  it('throws unauthorized errors for profile requests', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ error: 'unauthorized' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    await expect(fetchProfile()).rejects.toMatchObject({ status: 401 })
  })

  it('performs auth login and verify requests', async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ challenge_id: 'abc', code: '123456', expires_at: '2024-01-01T00:00:00Z' }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ token: 'token', user: { id: 1, email: 'user@example.com', roles: ['consumer'] } }),
      })
    vi.stubGlobal('fetch', mockFetch)

    const loginResp = await requestAuthLogin('user@example.com')
    expect(loginResp.challenge_id).toBe('abc')

    await requestAuthVerify({ challenge_id: 'abc', code: '123456' })
    expect(mockFetch).toHaveBeenCalledTimes(2)
  })

  it('identifies unauthorized errors', () => {
    expect(isUnauthorized({ status: 401 } as any)).toBe(true)
    expect(isUnauthorized({ status: 500 } as any)).toBe(false)
  })
})

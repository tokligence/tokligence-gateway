import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { fetchProfile } from './api'
import { sampleProfile } from './sampleData'

describe('gateway api client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('returns fallback profile when network fails', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
    const mockFetch = vi.fn().mockRejectedValue(new Error('offline'))
    vi.stubGlobal('fetch', mockFetch)

    const result = await fetchProfile()

    expect(result).toEqual(sampleProfile)
    expect(warn).toHaveBeenCalled()
    expect(mockFetch).toHaveBeenCalledTimes(1)

    warn.mockRestore()
  })

  it('parses gateway response when available', async () => {
    const mockPayload = {
      user: { id: 9, email: 'user@example.com', roles: ['consumer'] },
      provider: null,
    }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockPayload,
    })
    vi.stubGlobal('fetch', mockFetch)

    const result = await fetchProfile()

    expect(result).toEqual(mockPayload)
    expect(mockFetch).toHaveBeenCalled()
    const [, init] = mockFetch.mock.calls[0] ?? []
    expect(init?.credentials).toBe('include')
    expect(init?.headers).toMatchObject({ 'Content-Type': 'application/json' })
  })
})

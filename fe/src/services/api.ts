import type {
  ProfileResponse,
  ProviderCatalogResponse,
  ServicesResponse,
  UsageSummaryResponse,
  ApiListParams,
  ApiError,
  UsageLogsResponse,
  AdminUsersResponse,
  AdminUser,
  AdminUserResponse,
  AdminAPIKeysResponse,
  AdminAPIKey,
  AuthLoginResult,
  AuthVerifyRequest,
  AuthVerifyResponse,
} from '../types/api'
import {
  sampleProviders,
  sampleServices,
  sampleUsageSummary,
} from './sampleData'

const API_BASE = import.meta.env.VITE_GATEWAY_API_URL?.replace(/\/$/, '') ?? '/api'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })

  if (!response.ok) {
    const message = await safeErrorMessage(response)
    const err: ApiError = { message, status: response.status }
    throw err
  }

  if (response.status === 204) {
    return undefined as T
  }

  const contentType = response.headers?.get?.('content-type') ?? ''
  if (!contentType.includes('json')) {
    let snippet = ''
    try {
      if (typeof response.text === 'function') {
        snippet = await response.text()
      }
    } catch {
      snippet = ''
    }
    const err: ApiError = {
      status: response.status || 500,
      message: `Gateway response is not JSON (content-type: ${contentType || 'unknown'}). First bytes: ${snippet.slice(0, 120)}`,
    }
    throw err
  }

  try {
    return (await response.json()) as T
  } catch (error) {
    const err: ApiError = {
      status: response.status || 500,
      message: error instanceof Error ? error.message : 'Failed to decode JSON response',
    }
    throw err
  }
}

function isUnauthorized(error: unknown): error is ApiError {
  return Boolean((error as ApiError)?.status === 401)
}

async function safeErrorMessage(response: Response): Promise<string> {
  try {
    const payload = await response.json()
    if (typeof payload?.error === 'string') {
      return payload.error
    }
    if (payload?.message) {
      return payload.message
    }
  } catch {
    // ignore JSON parsing failures
  }
  return `Request failed with status ${response.status}`
}

function withFallback<T>(
  action: () => Promise<T>,
  fallback: T,
  label: string,
): Promise<T> {
  return action().catch((error: unknown) => {
    if (isUnauthorized(error)) {
      throw error
    }
    console.warn(`API call failed for ${label}, using fallback data`, error)
    return fallback
  })
}

export function fetchProfile(): Promise<ProfileResponse> {
  return request<ProfileResponse>('/v1/profile')
}

export function fetchProviders(): Promise<ProviderCatalogResponse> {
  return withFallback(() => request<ProviderCatalogResponse>('/v1/providers'), sampleProviders, 'providers')
}

export function fetchServices(params?: ApiListParams): Promise<ServicesResponse> {
  const query = params?.scope ? `?scope=${encodeURIComponent(params.scope)}` : ''
  return withFallback(
    () => request<ServicesResponse>(`/v1/services${query}`),
    sampleServices,
    'services',
  )
}

export function fetchUsageSummary(): Promise<UsageSummaryResponse> {
  return withFallback(
    () => request<UsageSummaryResponse>('/v1/usage/summary'),
    sampleUsageSummary,
    'usage-summary',
  )
}

export function fetchUsageLogs(limit = 20): Promise<UsageLogsResponse> {
  return request<UsageLogsResponse>(`/v1/usage/logs?limit=${limit}`)
}

export function requestAuthLogin(email: string): Promise<AuthLoginResult> {
  return request<AuthLoginResult>('/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email }),
  })
}

export function requestAuthVerify(payload: AuthVerifyRequest): Promise<AuthVerifyResponse> {
  return request<AuthVerifyResponse>('/v1/auth/verify', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export { isUnauthorized }

export async function fetchAdminUsers(): Promise<AdminUser[]> {
  const data = await request<AdminUsersResponse>('/v1/admin/users')
  return data.users.map(mapAdminUser)
}

export async function createAdminUser(payload: { email: string; role: string; displayName?: string }): Promise<AdminUser> {
  const data = await request<AdminUserResponse>('/v1/admin/users', {
    method: 'POST',
    body: JSON.stringify({
      email: payload.email,
      role: payload.role,
      display_name: payload.displayName,
    }),
  })
  return mapAdminUser(data.user)
}

export async function updateAdminUser(id: number, payload: { role?: string; displayName?: string }): Promise<AdminUser> {
  const data = await request<AdminUserResponse>(`/v1/admin/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({
      role: payload.role,
      display_name: payload.displayName,
    }),
  })
  return mapAdminUser(data.user)
}

export async function setAdminUserStatus(id: number, status: 'active' | 'inactive'): Promise<void> {
  await request(`/v1/admin/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export async function deleteAdminUser(id: number): Promise<void> {
  await request(`/v1/admin/users/${id}`, { method: 'DELETE' })
}

export async function fetchAdminAPIKeys(userId: number): Promise<AdminAPIKey[]> {
  const data = await request<AdminAPIKeysResponse>(`/v1/admin/users/${userId}/api-keys`)
  return data.api_keys.map(mapAdminAPIKey)
}

export async function createAdminAPIKey(
  userId: number,
  payload: { scopes?: string[]; ttl?: string },
): Promise<{ token: string; apiKey: AdminAPIKey }> {
  const body: Record<string, unknown> = {}
  if (payload.scopes && payload.scopes.length > 0) {
    body.scopes = payload.scopes
  }
  if (payload.ttl) {
    const hours = Number(payload.ttl)
    if (!Number.isNaN(hours) && hours > 0) {
      body.expires_at = new Date(Date.now() + hours * 60 * 60 * 1000).toISOString()
    }
  }
  const data = await request<{ token: string; api_key: AdminAPIKey }>(`/v1/admin/users/${userId}/api-keys`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
  return { token: data.token, apiKey: mapAdminAPIKey(data.api_key) }
}

export async function deleteAdminAPIKey(id: number): Promise<void> {
  await request(`/v1/admin/api-keys/${id}`, { method: 'DELETE' })
}

function mapAdminUser(raw: any): AdminUser {
  return {
    id: raw?.id ?? 0,
    email: raw?.email ?? '',
    role: raw?.role ?? 'gateway_user',
    displayName: raw?.display_name ?? null,
    status: raw?.status ?? 'active',
    createdAt: raw?.created_at ?? '',
    updatedAt: raw?.updated_at ?? '',
  }
}

function mapAdminAPIKey(raw: any): AdminAPIKey {
  return {
    id: raw?.id ?? 0,
    userId: raw?.user_id ?? 0,
    prefix: raw?.prefix ?? '',
    scopes: Array.isArray(raw?.scopes) ? raw.scopes : [],
    expiresAt: raw?.expires_at ?? null,
    createdAt: raw?.created_at ?? '',
    updatedAt: raw?.updated_at ?? '',
  }
}

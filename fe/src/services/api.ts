import type {
  ProfileResponse,
  ProviderCatalogResponse,
  ServicesResponse,
  UsageSummaryResponse,
  ApiListParams,
  ApiError,
} from '../types/api'
import {
  sampleProfile,
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

  return (await response.json()) as T
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
    console.warn(`API call failed for ${label}, using fallback data`, error)
    return fallback
  })
}

export function fetchProfile(): Promise<ProfileResponse> {
  return withFallback(() => request<ProfileResponse>('/v1/profile'), sampleProfile, 'profile')
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

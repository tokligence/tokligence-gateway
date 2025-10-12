import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import {
  fetchProfile,
  fetchProviders,
  fetchServices,
  fetchUsageSummary,
  fetchUsageLogs,
} from '../services/api'
import type { ApiListParams, UsageLogsResponse } from '../types/api'

export function useProfileQuery() {
  return useQuery({ queryKey: ['profile'], queryFn: fetchProfile, retry: false })
}

export function useProvidersQuery(enabled = true) {
  return useQuery({ queryKey: ['providers'], queryFn: fetchProviders, enabled })
}

export function useServicesQuery(params?: ApiListParams, enabled = true) {
  return useQuery({
    queryKey: ['services', params?.scope ?? 'all'],
    queryFn: () => fetchServices(params),
    enabled,
  })
}

export function useUsageSummaryQuery(enabled = true) {
  return useQuery({ queryKey: ['usage-summary'], queryFn: fetchUsageSummary, enabled })
}

export function useUsageLogsQuery(limit = 20, options?: UseQueryOptions<UsageLogsResponse>) {
  return useQuery({
    queryKey: ['usage-logs', limit],
    queryFn: () => fetchUsageLogs(limit),
    ...options,
  })
}

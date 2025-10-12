import { useQuery } from '@tanstack/react-query'
import { fetchProfile, fetchProviders, fetchServices, fetchUsageSummary } from '../services/api'
import type { ApiListParams } from '../types/api'

export function useProfileQuery() {
  return useQuery({ queryKey: ['profile'], queryFn: fetchProfile })
}

export function useProvidersQuery() {
  return useQuery({ queryKey: ['providers'], queryFn: fetchProviders })
}

export function useServicesQuery(params?: ApiListParams) {
  return useQuery({
    queryKey: ['services', params?.scope ?? 'all'],
    queryFn: () => fetchServices(params),
  })
}

export function useUsageSummaryQuery() {
  return useQuery({ queryKey: ['usage-summary'], queryFn: fetchUsageSummary })
}

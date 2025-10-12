export interface User {
  id: number
  email: string
  roles: string[]
  displayName?: string | null
}

export interface ProviderProfile {
  id: number
  userId: number
  displayName: string
  description?: string | null
}

export interface ProfileResponse {
  user: User
  provider?: ProviderProfile | null
}

export interface ServiceOffering {
  id: number
  providerId: number
  name: string
  modelFamily: string
  pricePer1KTokens: number
  trialTokens?: number
}

export interface ServicesResponse {
  services: ServiceOffering[]
}

export interface ProviderCatalogResponse {
  providers: ProviderProfile[]
}

export interface UsageSummary {
  userId: number
  consumedTokens: number
  suppliedTokens: number
  netTokens: number
}

export interface UsageSummaryResponse {
  summary: UsageSummary
}

export interface ApiError {
  message: string
  status?: number
}

export interface ApiListParams {
  scope?: 'all' | 'mine'
}

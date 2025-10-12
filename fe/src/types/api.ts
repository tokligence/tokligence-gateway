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

export interface UsageEntry {
  id: number
  user_id: number
  service_id: number
  prompt_tokens: number
  completion_tokens: number
  direction: 'consume' | 'supply'
  memo: string
  created_at: string
}

export interface UsageLogsResponse {
  entries: UsageEntry[]
}

export interface AuthLoginResponse {
  challenge_id: string
  code: string
  expires_at: string
}

export interface AuthVerifyRequest {
  challenge_id: string
  code: string
  display_name?: string
  enable_provider?: boolean
}

export interface AuthVerifyResponse {
  token: string
  user: User
  provider?: ProviderProfile | null
}

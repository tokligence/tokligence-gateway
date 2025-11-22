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
  marketplace?: {
    connected: boolean
  }
}

export interface AdminUser {
  id: number
  email: string
  role: string
  displayName?: string | null
  status: string
  createdAt: string
  updatedAt: string
}

export interface AdminUserResponse {
  user: AdminUser
}

export interface AdminUsersResponse {
  users: AdminUser[]
}

export interface AdminAPIKey {
  id: number
  userId: number
  prefix: string
  scopes: string[]
  expiresAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface AdminAPIKeysResponse {
  api_keys: AdminAPIKey[]
}

export interface ServiceOffering {
  id: number
  providerId: number
  name: string
  description?: string
  modelFamily: string
  baseModel?: string

  // Pricing - OpenRouter style with separate input/output pricing
  pricePer1KTokens: number  // Legacy field for backward compatibility
  inputPricePer1MTokens?: number  // Input price per 1M tokens (OpenRouter format)
  outputPricePer1MTokens?: number  // Output price per 1M tokens (OpenRouter format)
  trialTokens?: number

  providerName?: string
  providerVerified?: boolean

  // Deployment type - to support self-hosted models
  deploymentType?: 'cloud' | 'self-hosted' | 'hybrid'
  selfHostedConfig?: {
    requiresOwnInfrastructure?: boolean
    minimumSpecs?: string
    dockerImage?: string
    setupComplexity?: 'easy' | 'medium' | 'advanced'
  }

  // Technical specs
  contextWindow?: number
  maxOutputTokens?: number
  features?: {
    functionCalling?: boolean
    vision?: boolean
    streaming?: boolean
    jsonMode?: boolean
  }
  apiCompatibility?: string[]

  // Modalities - OpenRouter style
  inputModalities?: ('text' | 'image' | 'audio' | 'video' | 'file')[]
  outputModalities?: ('text' | 'image' | 'audio' | 'embeddings')[]

  // Use cases - for categorization
  useCases?: ('programming' | 'roleplay' | 'marketing' | 'research' | 'translation' | 'analysis' | 'creative-writing' | 'data-extraction')[]

  // Geographic info
  geographic?: {
    country?: string
    region?: string
    city?: string
    dataCenters?: string[]
  }
  compliance?: string[]

  // Performance metrics
  metrics?: {
    uptime7d?: number
    uptime30d?: number
    latencyP50?: number
    latencyP95?: number
    latencyP99?: number
    throughputRps?: number
    throughputTps?: number
  }

  // Availability
  availability?: {
    schedule?: '24/7' | 'business_hours' | 'custom'
    timezone?: string
    customHours?: { start: string; end: string }[]
    maintenanceWindows?: { start: string; end: string }[]
  }

  // Social proof
  rating?: number
  reviewCount?: number
  usageStats?: {
    totalTokensServed?: number
    activeUsers?: number
    monthlyRequests?: number
  }

  // Metadata
  status?: 'active' | 'maintenance' | 'deprecated'
  createdAt?: string
  updatedAt?: string
}

// Alias for clarity
export type Service = ServiceOffering

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
  api_key_id?: number | null
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

export type AuthLoginResult = AuthLoginResponse | AuthVerifyResponse

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

import type {
  ProfileResponse,
  ProviderCatalogResponse,
  ServicesResponse,
  UsageSummaryResponse,
} from '../types/api'
import { mockServices } from '../data/mockServices'

export const sampleProfile: ProfileResponse = {
  user: {
    id: 1,
    email: 'dev@example.com',
    roles: ['consumer', 'provider'],
    displayName: 'Tokligence Dev',
  },
  provider: {
    id: 1,
    userId: 1,
    displayName: 'AI Solutions Inc.',
    description: 'Enterprise-grade AI infrastructure provider',
  },
  marketplace: {
    connected: true,
  },
}

export const sampleProviders: ProviderCatalogResponse = {
  providers: [
    { id: 1, userId: 2, displayName: 'AI Solutions Inc.', description: 'Enterprise-grade AI infrastructure provider' },
    { id: 2, userId: 3, displayName: 'EuroAI GmbH', description: 'GDPR-compliant AI services in Europe' },
    { id: 3, userId: 4, displayName: 'OpenMind AI (Singapore)', description: 'Cost-effective open-source models in APAC' },
    { id: 4, userId: 5, displayName: '智能云 (Smart Cloud)', description: 'China-optimized AI services' },
    { id: 5, userId: 6, displayName: 'MistralTech SAS', description: 'French AI excellence with EU hosting' },
    { id: 6, userId: 7, displayName: 'TimeFlex AI', description: 'Business hours AI services at lower costs' },
  ],
}

export const sampleServices: ServicesResponse = {
  services: mockServices,
}

export const sampleUsageSummary: UsageSummaryResponse = {
  summary: {
    userId: 1,
    consumedTokens: 128000,
    suppliedTokens: 32000,
    netTokens: 96000,
  },
}

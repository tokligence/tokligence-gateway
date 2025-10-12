import type {
  ProfileResponse,
  ProviderCatalogResponse,
  ServicesResponse,
  UsageSummaryResponse,
} from '../types/api'

export const sampleProfile: ProfileResponse = {
  user: {
    id: 1,
    email: 'dev@example.com',
    roles: ['consumer', 'provider'],
    displayName: 'Tokligence Dev',
  },
  provider: {
    id: 7,
    userId: 1,
    displayName: 'Tokligence Studio',
    description: 'Local adapters for demo usage',
  },
}

export const sampleProviders: ProviderCatalogResponse = {
  providers: [
    {
      id: 7,
      userId: 1,
      displayName: 'Tokligence Studio',
      description: 'Local adapters for demo usage',
    },
    {
      id: 11,
      userId: 5,
      displayName: 'Anthropic',
      description: 'Claude family models available via adapter.',
    },
    {
      id: 12,
      userId: 6,
      displayName: 'OpenAI',
      description: 'GPT-4o / GPT-4.1 routes.',
    },
  ],
}

export const sampleServices: ServicesResponse = {
  services: [
    {
      id: 101,
      providerId: 7,
      name: 'local-claude-sonnet',
      modelFamily: 'claude-3.5-sonnet',
      pricePer1KTokens: 0.45,
      trialTokens: 5000,
    },
    {
      id: 205,
      providerId: 12,
      name: 'gpt-4o-mini',
      modelFamily: 'gpt-4o-mini',
      pricePer1KTokens: 0.6,
      trialTokens: 2000,
    },
  ],
}

export const sampleUsageSummary: UsageSummaryResponse = {
  summary: {
    userId: 1,
    consumedTokens: 128000,
    suppliedTokens: 32000,
    netTokens: 96000,
  },
}

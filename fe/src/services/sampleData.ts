import type {
  ProfileResponse,
  ProviderCatalogResponse,
  ServicesResponse,
  UsageSummaryResponse,
} from '../types/api'
import { mockServices } from '../data/mockServices'
import { COMPREHENSIVE_MODELS } from '../data/comprehensiveModels'

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
    displayName: 'OpenAI',
    description: 'Leading AI research and deployment company',
  },
  marketplace: {
    connected: true,
  },
}

export const sampleProviders: ProviderCatalogResponse = {
  providers: [
    { id: 1, userId: 2, displayName: 'OpenAI', description: 'GPT series, o1, o3 models' },
    { id: 2, userId: 3, displayName: 'Anthropic', description: 'Claude series models' },
    { id: 3, userId: 4, displayName: 'Google', description: 'Gemini, PaLM series' },
    { id: 4, userId: 5, displayName: 'Meta', description: 'Llama open-source models' },
    { id: 5, userId: 6, displayName: 'DeepSeek', description: 'Advanced Chinese reasoning models' },
    { id: 6, userId: 7, displayName: 'Alibaba Qwen', description: 'Multilingual models with Chinese language support' },
    { id: 7, userId: 8, displayName: 'Mistral AI', description: 'European AI excellence' },
    { id: 8, userId: 9, displayName: 'xAI', description: 'Grok conversational models' },
    { id: 9, userId: 10, displayName: 'Amazon AWS', description: 'Nova series and Bedrock platform' },
    { id: 10, userId: 11, displayName: 'Perplexity', description: 'Search-augmented models' },
    { id: 11, userId: 12, displayName: 'Allen Institute for AI', description: 'Open-source OLMo models' },
    { id: 12, userId: 13, displayName: 'NVIDIA', description: 'Nemotron series models' },
  ],
}

// Use comprehensive models as the main dataset
export const sampleServices: ServicesResponse = {
  services: COMPREHENSIVE_MODELS,
}

export const sampleUsageSummary: UsageSummaryResponse = {
  summary: {
    userId: 1,
    consumedTokens: 128000,
    suppliedTokens: 32000,
    netTokens: 96000,
  },
}

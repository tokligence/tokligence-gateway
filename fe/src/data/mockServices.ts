import type { Service } from '../types/api'

export const mockServices: Service[] = [
  {
    id: 1,
    providerId: 1,
    name: 'Fast GPT-4 Turbo',
    description:
      'High-performance GPT-4 Turbo API with ultra-low latency. Hosted in US-West with 99.99% uptime guarantee.',
    modelFamily: 'gpt',
    baseModel: 'gpt-4-turbo-preview',
    pricePer1KTokens: 0.003,
    trialTokens: 50000,
    providerName: 'AI Solutions Inc.',
    providerVerified: true,

    contextWindow: 128000,
    maxOutputTokens: 4096,
    features: {
      functionCalling: true,
      vision: true,
      streaming: true,
      jsonMode: true,
    },
    apiCompatibility: ['openai'],

    geographic: {
      country: 'USA',
      region: 'US-West',
      city: 'San Francisco',
      dataCenters: ['us-west-1', 'us-west-2'],
    },
    compliance: ['SOC2', 'GDPR', 'HIPAA'],

    metrics: {
      uptime7d: 99.98,
      uptime30d: 99.97,
      latencyP50: 45,
      latencyP95: 120,
      latencyP99: 250,
      throughputRps: 500,
      throughputTps: 2000,
    },

    availability: {
      schedule: '24/7',
      timezone: 'America/Los_Angeles',
    },

    rating: 4.8,
    reviewCount: 1247,
    usageStats: {
      totalTokensServed: 5200000000,
      activeUsers: 450,
      monthlyRequests: 2500000,
    },

    status: 'active',
    createdAt: '2024-01-15T00:00:00Z',
    updatedAt: '2025-01-20T00:00:00Z',
  },
  {
    id: 2,
    providerId: 2,
    name: 'Claude 3.5 Sonnet - EU',
    description:
      'Claude 3.5 Sonnet hosted in EU data centers for GDPR compliance. Fast, reliable, and cost-effective.',
    modelFamily: 'claude',
    baseModel: 'claude-3-5-sonnet-20241022',
    pricePer1KTokens: 0.0025,
    trialTokens: 100000,
    providerName: 'EuroAI GmbH',
    providerVerified: true,

    contextWindow: 200000,
    maxOutputTokens: 8192,
    features: {
      functionCalling: true,
      vision: true,
      streaming: true,
      jsonMode: false,
    },
    apiCompatibility: ['anthropic', 'openai'],

    geographic: {
      country: 'Germany',
      region: 'EU-Central',
      city: 'Frankfurt',
      dataCenters: ['eu-central-1', 'eu-central-2'],
    },
    compliance: ['GDPR', 'ISO27001'],

    metrics: {
      uptime7d: 99.95,
      uptime30d: 99.92,
      latencyP50: 55,
      latencyP95: 150,
      latencyP99: 300,
      throughputRps: 300,
      throughputTps: 1500,
    },

    availability: {
      schedule: '24/7',
      timezone: 'Europe/Berlin',
    },

    rating: 4.9,
    reviewCount: 856,
    usageStats: {
      totalTokensServed: 3100000000,
      activeUsers: 320,
      monthlyRequests: 1800000,
    },

    status: 'active',
    createdAt: '2024-02-01T00:00:00Z',
    updatedAt: '2025-01-18T00:00:00Z',
  },
  {
    id: 3,
    providerId: 3,
    name: 'Llama 3 70B - Budget Friendly',
    description:
      'Open-source Llama 3 70B model at competitive prices. Perfect for cost-conscious developers. Hosted in Asia-Pacific.',
    modelFamily: 'llama',
    baseModel: 'llama-3-70b-instruct',
    pricePer1KTokens: 0.0006,
    trialTokens: 200000,
    providerName: 'OpenMind AI (Singapore)',
    providerVerified: true,

    contextWindow: 8000,
    maxOutputTokens: 2048,
    features: {
      functionCalling: false,
      vision: false,
      streaming: true,
      jsonMode: true,
    },
    apiCompatibility: ['openai'],

    geographic: {
      country: 'Singapore',
      region: 'AP-Southeast',
      city: 'Singapore',
      dataCenters: ['ap-southeast-1'],
    },
    compliance: ['ISO27001'],

    metrics: {
      uptime7d: 99.5,
      uptime30d: 99.3,
      latencyP50: 80,
      latencyP95: 200,
      latencyP99: 400,
      throughputRps: 200,
      throughputTps: 800,
    },

    availability: {
      schedule: '24/7',
      timezone: 'Asia/Singapore',
    },

    rating: 4.6,
    reviewCount: 432,
    usageStats: {
      totalTokensServed: 1500000000,
      activeUsers: 280,
      monthlyRequests: 950000,
    },

    status: 'active',
    createdAt: '2024-03-10T00:00:00Z',
    updatedAt: '2025-01-15T00:00:00Z',
  },
  {
    id: 4,
    providerId: 4,
    name: 'Qwen-2.5-72B - China Optimized',
    description:
      'Alibaba Qwen 2.5 72B model optimized for Chinese language tasks. Low latency within China. Compliant with local regulations.',
    modelFamily: 'other',
    baseModel: 'qwen-2.5-72b-instruct',
    pricePer1KTokens: 0.0008,
    trialTokens: 150000,
    providerName: '智能云 (Smart Cloud)',
    providerVerified: true,

    contextWindow: 32000,
    maxOutputTokens: 4096,
    features: {
      functionCalling: true,
      vision: false,
      streaming: true,
      jsonMode: true,
    },
    apiCompatibility: ['openai'],

    geographic: {
      country: 'China',
      region: 'CN-East',
      city: 'Shanghai',
      dataCenters: ['cn-shanghai-1', 'cn-beijing-1'],
    },
    compliance: [],

    metrics: {
      uptime7d: 99.8,
      uptime30d: 99.7,
      latencyP50: 35,
      latencyP95: 90,
      latencyP99: 180,
      throughputRps: 400,
      throughputTps: 1600,
    },

    availability: {
      schedule: '24/7',
      timezone: 'Asia/Shanghai',
    },

    rating: 4.7,
    reviewCount: 678,
    usageStats: {
      totalTokensServed: 2800000000,
      activeUsers: 520,
      monthlyRequests: 1600000,
    },

    status: 'active',
    createdAt: '2024-04-05T00:00:00Z',
    updatedAt: '2025-01-22T00:00:00Z',
  },
  {
    id: 5,
    providerId: 5,
    name: 'Mistral Large - Premium',
    description:
      'Mistral Large model with enterprise-grade SLA. French and EU hosting for regulatory compliance.',
    modelFamily: 'mistral',
    baseModel: 'mistral-large-latest',
    pricePer1KTokens: 0.004,
    trialTokens: 25000,
    providerName: 'MistralTech SAS',
    providerVerified: true,

    contextWindow: 32000,
    maxOutputTokens: 8192,
    features: {
      functionCalling: true,
      vision: false,
      streaming: true,
      jsonMode: true,
    },
    apiCompatibility: ['openai', 'mistral'],

    geographic: {
      country: 'France',
      region: 'EU-West',
      city: 'Paris',
      dataCenters: ['eu-west-3'],
    },
    compliance: ['GDPR', 'SOC2', 'ISO27001'],

    metrics: {
      uptime7d: 99.99,
      uptime30d: 99.98,
      latencyP50: 42,
      latencyP95: 110,
      latencyP99: 220,
      throughputRps: 350,
      throughputTps: 1800,
    },

    availability: {
      schedule: '24/7',
      timezone: 'Europe/Paris',
    },

    rating: 4.9,
    reviewCount: 523,
    usageStats: {
      totalTokensServed: 1900000000,
      activeUsers: 210,
      monthlyRequests: 1100000,
    },

    status: 'active',
    createdAt: '2024-05-20T00:00:00Z',
    updatedAt: '2025-01-19T00:00:00Z',
  },
  {
    id: 6,
    providerId: 1,
    name: 'GPT-3.5 Turbo Ultra Fast',
    description: 'Lightning-fast GPT-3.5 Turbo for high-volume applications. Best price-performance ratio.',
    modelFamily: 'gpt',
    baseModel: 'gpt-3.5-turbo',
    pricePer1KTokens: 0.0002,
    trialTokens: 500000,
    providerName: 'AI Solutions Inc.',
    providerVerified: true,

    contextWindow: 16000,
    maxOutputTokens: 4096,
    features: {
      functionCalling: true,
      vision: false,
      streaming: true,
      jsonMode: true,
    },
    apiCompatibility: ['openai'],

    geographic: {
      country: 'USA',
      region: 'US-East',
      city: 'Virginia',
      dataCenters: ['us-east-1', 'us-east-2'],
    },
    compliance: ['SOC2', 'GDPR'],

    metrics: {
      uptime7d: 99.95,
      uptime30d: 99.91,
      latencyP50: 25,
      latencyP95: 65,
      latencyP99: 130,
      throughputRps: 800,
      throughputTps: 3500,
    },

    availability: {
      schedule: '24/7',
      timezone: 'America/New_York',
    },

    rating: 4.7,
    reviewCount: 2134,
    usageStats: {
      totalTokensServed: 8500000000,
      activeUsers: 1250,
      monthlyRequests: 5200000,
    },

    status: 'active',
    createdAt: '2023-11-01T00:00:00Z',
    updatedAt: '2025-01-21T00:00:00Z',
  },
  {
    id: 7,
    providerId: 6,
    name: 'Gemini Pro - Business Hours',
    description:
      'Google Gemini Pro available during business hours (9AM-6PM EST). Lower pricing for time-flexible workloads.',
    modelFamily: 'gemini',
    baseModel: 'gemini-1.5-pro',
    pricePer1KTokens: 0.0015,
    trialTokens: 75000,
    providerName: 'TimeFlex AI',
    providerVerified: false,

    contextWindow: 1000000,
    maxOutputTokens: 8192,
    features: {
      functionCalling: true,
      vision: true,
      streaming: true,
      jsonMode: false,
    },
    apiCompatibility: ['openai', 'google'],

    geographic: {
      country: 'USA',
      region: 'US-Central',
      city: 'Iowa',
      dataCenters: ['us-central-1'],
    },
    compliance: ['SOC2'],

    metrics: {
      uptime7d: 98.5,
      uptime30d: 98.2,
      latencyP50: 70,
      latencyP95: 180,
      latencyP99: 350,
      throughputRps: 150,
      throughputTps: 1000,
    },

    availability: {
      schedule: 'business_hours',
      timezone: 'America/Chicago',
      customHours: [{ start: '09:00', end: '18:00' }],
    },

    rating: 4.3,
    reviewCount: 187,
    usageStats: {
      totalTokensServed: 650000000,
      activeUsers: 95,
      monthlyRequests: 420000,
    },

    status: 'active',
    createdAt: '2024-06-15T00:00:00Z',
    updatedAt: '2025-01-10T00:00:00Z',
  },
  {
    id: 8,
    providerId: 3,
    name: 'Llama 3.3 70B - Latest',
    description: 'Latest Llama 3.3 70B with improved reasoning and multilingual capabilities. Beta pricing.',
    modelFamily: 'llama',
    baseModel: 'llama-3.3-70b-versatile',
    pricePer1KTokens: 0.00059,
    trialTokens: 300000,
    providerName: 'OpenMind AI (Singapore)',
    providerVerified: true,

    contextWindow: 32000,
    maxOutputTokens: 4096,
    features: {
      functionCalling: true,
      vision: false,
      streaming: true,
      jsonMode: true,
    },
    apiCompatibility: ['openai'],

    geographic: {
      country: 'Singapore',
      region: 'AP-Southeast',
      city: 'Singapore',
      dataCenters: ['ap-southeast-1', 'ap-southeast-2'],
    },
    compliance: ['ISO27001'],

    metrics: {
      uptime7d: 99.7,
      uptime30d: 99.5,
      latencyP50: 65,
      latencyP95: 170,
      latencyP99: 330,
      throughputRps: 250,
      throughputTps: 1100,
    },

    availability: {
      schedule: '24/7',
      timezone: 'Asia/Singapore',
    },

    rating: 4.8,
    reviewCount: 89,
    usageStats: {
      totalTokensServed: 320000000,
      activeUsers: 142,
      monthlyRequests: 280000,
    },

    status: 'active',
    createdAt: '2024-12-20T00:00:00Z',
    updatedAt: '2025-01-22T00:00:00Z',
  },
]

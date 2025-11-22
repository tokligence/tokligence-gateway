/**
 * Known AI model providers - based on OpenRouter's provider list
 * This list includes both cloud providers and self-hosted options
 */

export interface ProviderInfo {
  id: string
  name: string
  type: 'cloud' | 'self-hosted' | 'both'
  description: string
  homepage?: string
  region?: string[]
}

export const KNOWN_PROVIDERS: ProviderInfo[] = [
  // Major Cloud Providers
  { id: 'openai', name: 'OpenAI', type: 'cloud', description: 'GPT series, o1, o3 models', region: ['USA'], homepage: 'https://openai.com' },
  { id: 'anthropic', name: 'Anthropic', type: 'cloud', description: 'Claude series models', region: ['USA'], homepage: 'https://anthropic.com' },
  { id: 'google', name: 'Google', type: 'cloud', description: 'Gemini, PaLM series', region: ['USA'], homepage: 'https://ai.google.dev' },
  { id: 'xai', name: 'xAI', type: 'cloud', description: 'Grok series models', region: ['USA'], homepage: 'https://x.ai' },
  { id: 'mistral', name: 'Mistral AI', type: 'both', description: 'Mistral, Mixtral series', region: ['France', 'EU'], homepage: 'https://mistral.ai' },
  { id: 'cohere', name: 'Cohere', type: 'cloud', description: 'Command, Embed models', region: ['Canada', 'USA'], homepage: 'https://cohere.com' },

  // Chinese Providers
  { id: 'qwen', name: 'Alibaba Qwen', type: 'both', description: 'Qwen series models', region: ['China'], homepage: 'https://qwenlm.github.io' },
  { id: 'deepseek', name: 'DeepSeek', type: 'both', description: 'DeepSeek-V3, R1 series', region: ['China'], homepage: 'https://deepseek.com' },
  { id: 'baidu', name: 'Baidu', type: 'cloud', description: 'ERNIE series models', region: ['China'], homepage: 'https://cloud.baidu.com' },
  { id: 'zhipu', name: 'Zhipu AI (智谱AI)', type: 'cloud', description: 'GLM series models', region: ['China'], homepage: 'https://zhipuai.cn' },
  { id: 'moonshot', name: 'MoonshotAI (月之暗面)', type: 'cloud', description: 'Kimi series models', region: ['China'], homepage: 'https://kimi.moonshot.cn' },
  { id: 'minimax', name: 'MiniMax', type: 'cloud', description: 'MiniMax series models', region: ['China'], homepage: 'https://minimax.chat' },

  // Open Source & Self-Hosted
  { id: 'meta', name: 'Meta', type: 'both', description: 'Llama series (open source)', region: ['USA'], homepage: 'https://llama.meta.com' },
  { id: 'nvidia', name: 'NVIDIA', type: 'both', description: 'Nemotron series', region: ['USA'], homepage: 'https://nvidia.com' },
  { id: 'allenai', name: 'Allen Institute for AI', type: 'self-hosted', description: 'OLMo series (open source)', region: ['USA'], homepage: 'https://allenai.org' },
  { id: 'huggingface', name: 'Hugging Face', type: 'both', description: 'Various open source models', region: ['USA', 'EU'], homepage: 'https://huggingface.co' },

  // Enterprise & Specialized
  { id: 'amazon', name: 'Amazon AWS', type: 'cloud', description: 'Nova series, Bedrock', region: ['USA', 'Global'], homepage: 'https://aws.amazon.com' },
  { id: 'perplexity', name: 'Perplexity', type: 'cloud', description: 'Sonar series with search', region: ['USA'], homepage: 'https://perplexity.ai' },
  { id: 'ibm', name: 'IBM', type: 'cloud', description: 'Granite series models', region: ['USA'], homepage: 'https://ibm.com/watsonx' },
  { id: 'liquid', name: 'Liquid AI', type: 'cloud', description: 'LFM series models', region: ['USA'], homepage: 'https://liquid.ai' },

  // Smaller/Emerging Providers
  { id: 'arcee', name: 'Arcee AI', type: 'cloud', description: 'AFM series models', region: ['USA'] },
  { id: 'deepcogito', name: 'Deep Cogito', type: 'cloud', description: 'Reasoning models', region: ['USA'] },
  { id: 'together', name: 'Together AI', type: 'cloud', description: 'Various open source models', region: ['USA'], homepage: 'https://together.ai' },
  { id: 'fireworks', name: 'Fireworks AI', type: 'cloud', description: 'Various models with fast inference', region: ['USA'], homepage: 'https://fireworks.ai' },
  { id: 'replicate', name: 'Replicate', type: 'cloud', description: 'Model hosting platform', region: ['USA'], homepage: 'https://replicate.com' },

  // Regional Providers
  { id: 'opengvlab', name: 'OpenGVLab (Shanghai AI Lab)', type: 'both', description: 'InternVL series', region: ['China'] },
  { id: 'meituan', name: 'Meituan', type: 'cloud', description: 'LongCat series', region: ['China'] },
  { id: 'tongyi', name: 'Tongyi Lab', type: 'cloud', description: 'DeepResearch models', region: ['China'] },
]

export const PROVIDER_BY_ID = KNOWN_PROVIDERS.reduce((acc, provider) => {
  acc[provider.id] = provider
  return acc
}, {} as Record<string, ProviderInfo>)

/**
 * Get provider info by ID or name
 */
export function getProviderInfo(idOrName: string): ProviderInfo | undefined {
  const normalized = idOrName.toLowerCase()
  return KNOWN_PROVIDERS.find(
    p => p.id === normalized || p.name.toLowerCase() === normalized
  )
}

/**
 * Get all providers by type
 */
export function getProvidersByType(type: 'cloud' | 'self-hosted' | 'both'): ProviderInfo[] {
  return KNOWN_PROVIDERS.filter(p => p.type === type || p.type === 'both')
}

/**
 * Get all providers by region
 */
export function getProvidersByRegion(region: string): ProviderInfo[] {
  return KNOWN_PROVIDERS.filter(p =>
    p.region?.some(r => r.toLowerCase().includes(region.toLowerCase()))
  )
}

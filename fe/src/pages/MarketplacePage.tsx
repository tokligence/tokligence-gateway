import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useServicesQuery, useProvidersQuery } from '../hooks/useGatewayQueries'
import { ServiceDetailModal } from '../components/ServiceDetailModal'
import { StartUsingModal } from '../components/StartUsingModal'
import type { Service } from '../types/api'

type ViewMode = 'providers' | 'services'
type SortOption = 'price-asc' | 'price-desc' | 'rating' | 'popularity'

export function MarketplacePage() {
  const { t } = useTranslation()
  const [viewMode, setViewMode] = useState<ViewMode>('services')
  const [selectedService, setSelectedService] = useState<Service | null>(null)
  const [subscribeService, setSubscribeService] = useState<Service | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [modelFilter, setModelFilter] = useState<string>('all')
  const [sortBy, setSortBy] = useState<SortOption>('popularity')
  const [priceFilter, setPriceFilter] = useState<string>('all')
  const [contextFilter, setContextFilter] = useState<string>('all')
  const [locationFilter, setLocationFilter] = useState<string>('all')
  const [uptimeFilter, setUptimeFilter] = useState<string>('all')

  // OpenRouter-style filters
  const [inputModalityFilter, setInputModalityFilter] = useState<string>('all')
  const [outputModalityFilter, setOutputModalityFilter] = useState<string>('all')
  const [useCaseFilter, setUseCaseFilter] = useState<string>('all')
  const [deploymentTypeFilter, setDeploymentTypeFilter] = useState<string>('all')
  const [providerFilter, setProviderFilter] = useState<string>('all')

  const { data: servicesData, isPending: servicesPending } = useServicesQuery({ scope: 'all' })
  const { data: providersData, isPending: providersPending } = useProvidersQuery()

  const allServices = servicesData?.services ?? []
  const providers = providersData?.providers ?? []

  // Filtered and sorted services
  const services = useMemo(() => {
    let filtered = [...allServices]

    // Search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase()
      filtered = filtered.filter(
        (s) =>
          s.name.toLowerCase().includes(query) ||
          s.description?.toLowerCase().includes(query) ||
          s.modelFamily?.toLowerCase().includes(query) ||
          s.providerName?.toLowerCase().includes(query)
      )
    }

    // Model family filter
    if (modelFilter !== 'all') {
      filtered = filtered.filter((s) => s.modelFamily?.toLowerCase() === modelFilter.toLowerCase())
    }

    // Provider filter
    if (providerFilter !== 'all') {
      filtered = filtered.filter((s) => s.providerName?.toLowerCase() === providerFilter.toLowerCase())
    }

    // Input modality filter
    if (inputModalityFilter !== 'all') {
      filtered = filtered.filter((s) =>
        s.inputModalities?.includes(inputModalityFilter as any)
      )
    }

    // Output modality filter
    if (outputModalityFilter !== 'all') {
      filtered = filtered.filter((s) =>
        s.outputModalities?.includes(outputModalityFilter as any)
      )
    }

    // Use case filter
    if (useCaseFilter !== 'all') {
      filtered = filtered.filter((s) =>
        s.useCases?.includes(useCaseFilter as any)
      )
    }

    // Deployment type filter
    if (deploymentTypeFilter !== 'all') {
      if (deploymentTypeFilter === 'self-hosted') {
        filtered = filtered.filter((s) => s.deploymentType === 'self-hosted' || s.deploymentType === 'both')
      } else if (deploymentTypeFilter === 'cloud') {
        filtered = filtered.filter((s) => s.deploymentType === 'cloud' || s.deploymentType === 'both')
      }
    }

    // Price filter
    if (priceFilter !== 'all') {
      filtered = filtered.filter((s) => {
        const price = s.pricePer1KTokens
        if (priceFilter === 'free') return price === 0 || (s.trialTokens && s.trialTokens > 0)
        if (priceFilter === 'low') return price < 0.01 && price > 0
        if (priceFilter === 'medium') return price >= 0.01 && price <= 0.1
        if (priceFilter === 'high') return price > 0.1
        return true
      })
    }

    // Context window filter
    if (contextFilter !== 'all') {
      filtered = filtered.filter((s) => {
        const ctx = s.contextWindow || 0
        if (contextFilter === 'small') return ctx > 0 && ctx < 16000
        if (contextFilter === 'medium') return ctx >= 16000 && ctx < 64000
        if (contextFilter === 'large') return ctx >= 64000 && ctx < 200000
        if (contextFilter === 'xlarge') return ctx >= 200000
        return true
      })
    }

    // Location filter
    if (locationFilter !== 'all') {
      filtered = filtered.filter((s) => s.geographic?.country?.toLowerCase() === locationFilter.toLowerCase())
    }

    // Uptime filter
    if (uptimeFilter !== 'all') {
      filtered = filtered.filter((s) => {
        const uptime = s.metrics?.uptime30d || 0
        if (uptimeFilter === 'high') return uptime >= 99.9
        if (uptimeFilter === 'medium') return uptime >= 99
        return true
      })
    }

    // Sorting
    filtered.sort((a, b) => {
      if (sortBy === 'price-asc') return a.pricePer1KTokens - b.pricePer1KTokens
      if (sortBy === 'price-desc') return b.pricePer1KTokens - a.pricePer1KTokens
      if (sortBy === 'rating') return (b.rating || 0) - (a.rating || 0)
      if (sortBy === 'popularity') return (b.usageStats?.activeUsers || 0) - (a.usageStats?.activeUsers || 0)
      return 0
    })

    return filtered
  }, [
    allServices, searchQuery, modelFilter, sortBy, priceFilter, contextFilter,
    locationFilter, uptimeFilter, inputModalityFilter, outputModalityFilter,
    useCaseFilter, deploymentTypeFilter, providerFilter
  ])

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">🛒 {t('marketplace.title')}</h2>
          <p className="text-sm text-slate-500">
            {t('marketplace.subtitle')}
          </p>
        </div>

        {/* View mode toggle */}
        <div className="flex items-center gap-2 rounded-full border border-slate-200 bg-white p-1 shadow-sm">
          <button
            type="button"
            onClick={() => setViewMode('services')}
            className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
              viewMode === 'services' ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100'
            }`}
          >
            {t('marketplace.services')}
          </button>
          <button
            type="button"
            onClick={() => setViewMode('providers')}
            className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
              viewMode === 'providers' ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100'
            }`}
          >
            {t('marketplace.providers')}
          </button>
        </div>
      </header>

      {/* Services view */}
      {viewMode === 'services' && (
        <section className="space-y-4">
          {/* Search Bar */}
          <div className="relative">
            <input
              type="text"
              placeholder="Search services by name, description, or model..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-lg border border-slate-200 py-2 pl-10 pr-4 text-sm focus:border-slate-400 focus:outline-none"
            />
            <svg
              className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
          </div>

          {/* Primary Filters - OpenRouter Style */}
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
            {/* Provider Filter */}
            <select
              value={providerFilter}
              onChange={(e) => setProviderFilter(e.target.value)}
              className="rounded-lg border border-slate-200 px-3 py-2 text-sm"
            >
              <option value="all">All Providers</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="google">Google</option>
              <option value="meta">Meta (Llama)</option>
              <option value="deepseek">DeepSeek</option>
              <option value="alibaba qwen">Alibaba Qwen</option>
              <option value="mistral ai">Mistral AI</option>
              <option value="xai">xAI (Grok)</option>
            </select>

            {/* Model Family Filter */}
            <select
              value={modelFilter}
              onChange={(e) => setModelFilter(e.target.value)}
              className="rounded-lg border border-slate-200 px-3 py-2 text-sm"
            >
              <option value="all">All Models</option>
              <option value="gpt">GPT</option>
              <option value="claude">Claude</option>
              <option value="llama">Llama</option>
              <option value="gemini">Gemini</option>
              <option value="mistral">Mistral</option>
              <option value="deepseek">DeepSeek</option>
              <option value="qwen">Qwen</option>
            </select>

            {/* Context Window Filter */}
            <select
              value={contextFilter}
              onChange={(e) => setContextFilter(e.target.value)}
              className="rounded-lg border border-slate-200 px-3 py-2 text-sm"
            >
              <option value="all">Context: All</option>
              <option value="small">&lt; 16K</option>
              <option value="medium">16K - 64K</option>
              <option value="large">64K - 200K</option>
              <option value="xlarge">&gt; 200K</option>
            </select>

            {/* Sort */}
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as SortOption)}
              className="rounded-lg border border-slate-200 px-3 py-2 text-sm"
            >
              <option value="popularity">Sort: Most Popular</option>
              <option value="price-asc">Sort: Price (Low to High)</option>
              <option value="price-desc">Sort: Price (High to Low)</option>
              <option value="rating">Sort: Highest Rated</option>
            </select>
          </div>

          {/* Modality Filters - OpenRouter Style */}
          <div className="flex flex-wrap gap-2">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-slate-600">Input:</span>
              <select
                value={inputModalityFilter}
                onChange={(e) => setInputModalityFilter(e.target.value)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs"
              >
                <option value="all">All</option>
                <option value="text">Text</option>
                <option value="image">Image</option>
                <option value="audio">Audio</option>
                <option value="video">Video</option>
                <option value="file">File</option>
              </select>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-slate-600">Output:</span>
              <select
                value={outputModalityFilter}
                onChange={(e) => setOutputModalityFilter(e.target.value)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs"
              >
                <option value="all">All</option>
                <option value="text">Text</option>
                <option value="image">Image</option>
                <option value="audio">Audio</option>
                <option value="embeddings">Embeddings</option>
              </select>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-slate-600">Use Case:</span>
              <select
                value={useCaseFilter}
                onChange={(e) => setUseCaseFilter(e.target.value)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs"
              >
                <option value="all">All</option>
                <option value="programming">Programming</option>
                <option value="research">Research</option>
                <option value="analysis">Analysis</option>
                <option value="creative-writing">Creative Writing</option>
                <option value="translation">Translation</option>
                <option value="marketing">Marketing</option>
              </select>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-slate-600">Deployment:</span>
              <select
                value={deploymentTypeFilter}
                onChange={(e) => setDeploymentTypeFilter(e.target.value)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs"
              >
                <option value="all">All</option>
                <option value="cloud">Cloud Only</option>
                <option value="self-hosted">Self-Hosted</option>
              </select>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-slate-600">Price:</span>
              <select
                value={priceFilter}
                onChange={(e) => setPriceFilter(e.target.value)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs"
              >
                <option value="all">All</option>
                <option value="free">Free</option>
                <option value="low">&lt; $0.01/1K</option>
                <option value="medium">$0.01 - $0.10/1K</option>
                <option value="high">&gt; $0.10/1K</option>
              </select>
            </div>

            {/* Clear Filters */}
            {(searchQuery ||
              modelFilter !== 'all' ||
              providerFilter !== 'all' ||
              priceFilter !== 'all' ||
              contextFilter !== 'all' ||
              inputModalityFilter !== 'all' ||
              outputModalityFilter !== 'all' ||
              useCaseFilter !== 'all' ||
              deploymentTypeFilter !== 'all' ||
              locationFilter !== 'all' ||
              uptimeFilter !== 'all') && (
              <button
                onClick={() => {
                  setSearchQuery('')
                  setModelFilter('all')
                  setProviderFilter('all')
                  setPriceFilter('all')
                  setInputModalityFilter('all')
                  setOutputModalityFilter('all')
                  setUseCaseFilter('all')
                  setDeploymentTypeFilter('all')
                  setContextFilter('all')
                  setLocationFilter('all')
                  setUptimeFilter('all')
                }}
                className="rounded-lg bg-slate-100 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-200"
              >
                Clear Filters
              </button>
            )}
          </div>

          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-slate-900">
              {services.length} {services.length === 1 ? 'Service' : 'Services'}
            </h3>
          </div>

          {servicesPending && <ServiceSkeleton />}

          <div className="grid gap-4">
            {services.map((service) => (
              <article key={service.id} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm hover:border-slate-300 transition-colors">
                <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                  <div className="flex-1">
                    <div className="flex items-start gap-3">
                      <div className="flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3 className="text-base font-semibold text-slate-900">{service.name}</h3>
                          {service.deploymentType === 'self-hosted' && (
                            <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700">
                              Self-Hosted
                            </span>
                          )}
                          {service.deploymentType === 'both' && (
                            <span className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">
                              Cloud + Self-Hosted
                            </span>
                          )}
                          {service.pricePer1KTokens === 0 && (
                            <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
                              FREE
                            </span>
                          )}
                        </div>
                        <p className="mt-1 text-xs text-slate-500">
                          {service.providerName} • {service.modelFamily?.toUpperCase()}
                        </p>
                        {service.description && (
                          <p className="mt-2 text-sm text-slate-600 line-clamp-2">{service.description}</p>
                        )}
                        <div className="mt-3 flex items-center gap-3 flex-wrap">
                          {/* Modalities */}
                          {service.inputModalities && service.inputModalities.length > 0 && (
                            <div className="flex items-center gap-1">
                              <span className="text-xs text-slate-400">In:</span>
                              {service.inputModalities.map((m) => (
                                <span key={m} className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">
                                  {m}
                                </span>
                              ))}
                            </div>
                          )}
                          {service.outputModalities && service.outputModalities.length > 0 && (
                            <div className="flex items-center gap-1">
                              <span className="text-xs text-slate-400">Out:</span>
                              {service.outputModalities.map((m) => (
                                <span key={m} className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">
                                  {m}
                                </span>
                              ))}
                            </div>
                          )}
                          {service.contextWindow && (
                            <span className="text-xs text-slate-500">
                              {service.contextWindow >= 1000000
                                ? `${(service.contextWindow / 1000000).toFixed(1)}M`
                                : `${(service.contextWindow / 1000).toFixed(0)}K`}{' '}
                              context
                            </span>
                          )}
                        </div>
                        <div className="mt-2 flex items-center gap-2">
                          {service.rating && (
                            <>
                              <span className="text-xs text-slate-500">
                                ⭐ {service.rating.toFixed(1)} ({service.reviewCount?.toLocaleString()} reviews)
                              </span>
                              <span className="text-xs text-slate-400">•</span>
                            </>
                          )}
                          {service.usageStats?.activeUsers && (
                            <span className="text-xs text-slate-500">{service.usageStats.activeUsers} users</span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="flex flex-col items-end gap-2">
                    <div className="text-right">
                      {service.inputPricePer1MTokens !== undefined && service.outputPricePer1MTokens !== undefined ? (
                        <>
                          <div className="text-sm text-slate-600">
                            ${service.inputPricePer1MTokens.toFixed(2)} in / ${service.outputPricePer1MTokens.toFixed(2)} out
                          </div>
                          <div className="text-xs text-slate-500">per 1M tokens</div>
                        </>
                      ) : service.pricePer1KTokens === 0 ? (
                        <>
                          <div className="text-2xl font-bold text-green-600">FREE</div>
                          <div className="text-xs text-slate-500">Self-hosted</div>
                        </>
                      ) : (
                        <>
                          <div className="text-2xl font-bold text-slate-900">${service.pricePer1KTokens.toFixed(4)}</div>
                          <div className="text-xs text-slate-500">per 1K tokens</div>
                        </>
                      )}
                    </div>
                    {service.trialTokens && service.trialTokens > 0 && (
                      <div className="text-xs text-blue-600">🎁 {service.trialTokens.toLocaleString()} free tokens</div>
                    )}
                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => setSelectedService(service)}
                        className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                      >
                        {t('marketplace.details')}
                      </button>
                      <button
                        type="button"
                        onClick={() => setSubscribeService(service)}
                        className="rounded-lg bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800"
                      >
                        {t('marketplace.startUsing')}
                      </button>
                    </div>
                  </div>
                </div>
              </article>
            ))}
          </div>

          {services.length === 0 && !servicesPending && (
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
              <p className="text-sm text-slate-600">{t('marketplace.noServices')}</p>
            </div>
          )}
        </section>
      )}

      {/* Providers view */}
      {viewMode === 'providers' && (
        <section className="space-y-4">
          <h3 className="text-base font-semibold text-slate-900">{t('marketplace.providers')} ({providers.length})</h3>

          {providersPending && <ProviderSkeleton />}

          <div className="grid gap-4 md:grid-cols-2">
            {providers.map((provider) => (
              <article key={provider.id} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="text-base font-semibold text-slate-900">{provider.displayName}</h3>
                    <p className="text-xs uppercase tracking-wide text-slate-400">Provider #{provider.id}</p>
                  </div>
                  <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700">
                    ✓ {t('marketplace.verified')}
                  </span>
                </div>
                <p className="mt-3 text-sm text-slate-600">{provider.description ?? t('marketplace.noDescription')}</p>
                <div className="mt-4 flex items-center gap-2">
                  <span className="text-xs text-slate-500">⭐ 4.7 (234 {t('marketplace.reviews')})</span>
                  <span className="text-xs text-slate-400">•</span>
                  <span className="text-xs text-slate-500">5 {t('marketplace.services')}</span>
                </div>
              </article>
            ))}
          </div>

          {providers.length === 0 && !providersPending && (
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
              <p className="text-sm text-slate-600">{t('marketplace.noProviders')}</p>
            </div>
          )}
        </section>
      )}

      {/* Modals */}
      {selectedService && (
        <ServiceDetailModal
          service={selectedService}
          onClose={() => setSelectedService(null)}
          onStartUsing={(service) => {
            setSelectedService(null)
            setSubscribeService(service)
          }}
        />
      )}

      {subscribeService && (
        <StartUsingModal
          service={subscribeService}
          onClose={() => setSubscribeService(null)}
          onSubscribe={async (serviceId) => {
            // This will be connected to actual API later
            return {
              apiKey: 'tok_demo_' + Math.random().toString(36).substring(2, 15),
              endpoint: `https://gateway.tokligence.ai/v1/${subscribeService.modelFamily}`,
            }
          }}
        />
      )}
    </div>
  )
}

function ServiceSkeleton() {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="h-4 w-1/3 animate-pulse rounded bg-slate-200" />
      <div className="mt-3 h-3 w-2/3 animate-pulse rounded bg-slate-200" />
      <div className="mt-4 grid grid-cols-2 gap-4">
        <div className="h-3 w-full animate-pulse rounded bg-slate-200" />
        <div className="h-3 w-1/2 animate-pulse rounded bg-slate-200" />
      </div>
    </div>
  )
}

function ProviderSkeleton() {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="h-4 w-1/3 animate-pulse rounded bg-slate-200" />
      <div className="mt-2 h-3 w-1/4 animate-pulse rounded bg-slate-200" />
      <div className="mt-3 h-3 w-2/3 animate-pulse rounded bg-slate-200" />
    </div>
  )
}

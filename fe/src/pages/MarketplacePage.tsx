import { useState } from 'react'
import { useServicesQuery, useProvidersQuery } from '../hooks/useGatewayQueries'

type ViewMode = 'providers' | 'services'

export function MarketplacePage() {
  const [viewMode, setViewMode] = useState<ViewMode>('services')

  const { data: servicesData, isPending: servicesPending } = useServicesQuery({ scope: 'all' })
  const { data: providersData, isPending: providersPending } = useProvidersQuery()

  const services = servicesData?.services ?? []
  const providers = providersData?.providers ?? []

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">🛒 Token Marketplace</h2>
          <p className="text-sm text-slate-500">
            Browse providers, subscribe to services, and discover the best token prices
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
            Services
          </button>
          <button
            type="button"
            onClick={() => setViewMode('providers')}
            className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
              viewMode === 'providers' ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100'
            }`}
          >
            Providers
          </button>
        </div>
      </header>

      {/* Services view */}
      {viewMode === 'services' && (
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-slate-900">Available Services ({services.length})</h3>
            <div className="flex gap-2">
              <select className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm">
                <option>All Models</option>
                <option>GPT-4</option>
                <option>Claude</option>
                <option>Llama</option>
              </select>
              <select className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm">
                <option>Sort by: Price</option>
                <option>Sort by: Rating</option>
                <option>Sort by: Popularity</option>
              </select>
            </div>
          </div>

          {servicesPending && <ServiceSkeleton />}

          <div className="grid gap-4">
            {services.map((service) => (
              <article key={service.id} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
                <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                  <div className="flex-1">
                    <div className="flex items-start gap-3">
                      <div>
                        <h3 className="text-base font-semibold text-slate-900">{service.name}</h3>
                        <p className="text-xs uppercase tracking-wide text-slate-400">
                          Model: {service.modelFamily}
                        </p>
                        <div className="mt-2 flex items-center gap-2">
                          <span className="text-xs text-slate-500">⭐ 4.8 (1.2K reviews)</span>
                          <span className="text-xs text-slate-400">•</span>
                          <span className="text-xs text-slate-500">99.5% uptime</span>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="flex flex-col items-end gap-2">
                    <div className="text-right">
                      <div className="text-2xl font-bold text-slate-900">${service.pricePer1KTokens.toFixed(4)}</div>
                      <div className="text-xs text-slate-500">per 1K tokens</div>
                      {service.pricePer1KTokens < 0.025 && (
                        <div className="mt-1 rounded bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700">
                          💰 20% cheaper
                        </div>
                      )}
                    </div>
                    {service.trialTokens && service.trialTokens > 0 && (
                      <div className="text-xs text-blue-600">🎁 {service.trialTokens.toLocaleString()} free tokens</div>
                    )}
                    <div className="flex gap-2">
                      <button
                        type="button"
                        className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                      >
                        Details
                      </button>
                      <button
                        type="button"
                        className="rounded-lg bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800"
                      >
                        Subscribe
                      </button>
                    </div>
                  </div>
                </div>
              </article>
            ))}
          </div>

          {services.length === 0 && !servicesPending && (
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
              <p className="text-sm text-slate-600">No services available in the marketplace yet.</p>
            </div>
          )}
        </section>
      )}

      {/* Providers view */}
      {viewMode === 'providers' && (
        <section className="space-y-4">
          <h3 className="text-base font-semibold text-slate-900">Providers ({providers.length})</h3>

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
                    ✓ Verified
                  </span>
                </div>
                <p className="mt-3 text-sm text-slate-600">{provider.description ?? 'No description provided.'}</p>
                <div className="mt-4 flex items-center gap-2">
                  <span className="text-xs text-slate-500">⭐ 4.7 (234 reviews)</span>
                  <span className="text-xs text-slate-400">•</span>
                  <span className="text-xs text-slate-500">5 services</span>
                </div>
              </article>
            ))}
          </div>

          {providers.length === 0 && !providersPending && (
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
              <p className="text-sm text-slate-600">No providers available yet.</p>
            </div>
          )}
        </section>
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

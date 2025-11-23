import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useUsageSummaryQuery, useServicesQuery } from '../hooks/useGatewayQueries'
import { useProfileContext } from '../context/ProfileContext'
import { formatNumber } from '../utils/format'
import { PublishServiceModal } from '../components/PublishServiceModal'
import type { Service } from '../types/api'

export function ProviderDashboardPage() {
  const { t } = useTranslation()
  const profile = useProfileContext()
  const isAuthenticated = Boolean(profile)
  const { data: usage } = useUsageSummaryQuery(isAuthenticated)
  const { data: servicesData } = useServicesQuery({ scope: 'all' })

  const [showPublishModal, setShowPublishModal] = useState(false)
  const [selectedService, setSelectedService] = useState<Service | null>(null)

  const suppliedTokens = usage?.summary.suppliedTokens ?? 0
  const myServices = servicesData?.services.filter(s => s.providerId === profile?.provider?.id) ?? []

  // Provider dashboard (available to ALL editions)
  return (
    <div className="space-y-6">
      <header>
        <h2 className="text-lg font-semibold text-slate-900">💰 {t('provider.title')}</h2>
        <p className="text-sm text-slate-500">{t('provider.subtitle')}</p>
      </header>

      {/* Revenue overview */}
      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title={t('provider.tokensSupplied')}
          value={formatNumber(suppliedTokens)}
          description={t('provider.tokensSuppliedDesc')}
        />
        <StatCard title={t('provider.grossRevenue')} value="$1,420.00" description={t('provider.grossRevenueDesc')} />
        <StatCard title={t('provider.platformFee')} value="-$142.00" description={t('provider.platformFeeDesc')} />
        <StatCard title={t('provider.netEarnings')} value="$1,278.00" description={t('provider.netEarningsDesc')} />
      </section>

      {/* Payout info */}
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-base font-semibold text-slate-900">{t('provider.nextPayout')}</h3>
            <p className="text-sm text-slate-500">December 1, 2025</p>
          </div>
          <div className="text-right">
            <div className="text-2xl font-bold text-emerald-600">$1,278.00</div>
            <p className="text-xs text-slate-500">{t('provider.estimatedAmount')}</p>
          </div>
        </div>
        <div className="mt-4 flex gap-2">
          <button
            type="button"
            className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            {t('provider.viewPayoutHistory')}
          </button>
          <button
            type="button"
            className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            {t('provider.updateBankAccount')}
          </button>
        </div>
      </section>

      {/* Published services */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-slate-900">{t('provider.publishedServices')} ({myServices.length})</h3>
          <button
            type="button"
            onClick={() => setShowPublishModal(true)}
            className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
          >
            + {t('provider.publishNewService')}
          </button>
        </div>

        {/* Services List */}
        {myServices.length > 0 ? (
          <div className="space-y-3">
            {myServices.map((service) => (
              <div key={service.id} className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <h4 className="text-base font-semibold text-slate-900">{service.name}</h4>
                      {service.status === 'active' && (
                        <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700">
                          Active
                        </span>
                      )}
                      {service.status === 'maintenance' && (
                        <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">
                          Maintenance
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-sm text-slate-600">{service.description}</p>
                    <div className="mt-2 flex items-center gap-4 text-xs text-slate-500">
                      <span>Model: {service.modelFamily?.toUpperCase()}</span>
                      <span>•</span>
                      <span>Price: ${service.pricePer1KTokens.toFixed(4)}/1K</span>
                      {service.rating && (
                        <>
                          <span>•</span>
                          <span>⭐ {service.rating.toFixed(1)} ({service.reviewCount} reviews)</span>
                        </>
                      )}
                      {service.usageStats?.activeUsers && (
                        <>
                          <span>•</span>
                          <span>{service.usageStats.activeUsers} active users</span>
                        </>
                      )}
                    </div>
                  </div>
                  <div className="ml-4 flex gap-2">
                    <button
                      onClick={() => setSelectedService(service)}
                      className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                    >
                      View Stats
                    </button>
                    <button className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50">
                      Edit
                    </button>
                    <button className="rounded-lg border border-rose-200 px-3 py-1.5 text-sm font-medium text-rose-700 hover:bg-rose-50">
                      Unpublish
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
            <p className="text-sm text-slate-600">{t('provider.noServices')}</p>
            <p className="mt-1 text-sm text-slate-500">
              {t('provider.noServicesDesc')}
            </p>
            <button
              type="button"
              onClick={() => setShowPublishModal(true)}
              className="mt-4 rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
            >
              {t('provider.getStarted')}
            </button>
          </div>
        )}
      </section>

      {/* Revenue trends */}
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-base font-semibold text-slate-900">{t('provider.revenueTrends')}</h3>
        <div className="mt-4 flex h-40 items-center justify-center border border-slate-200 bg-slate-50">
          <p className="text-sm text-slate-500">{t('provider.chartPlaceholder')}</p>
        </div>
      </section>

      {/* Top customers */}
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-base font-semibold text-slate-900">{t('provider.topCustomers')}</h3>
        <div className="mt-4 space-y-2">
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div>
              <span className="text-sm font-medium text-slate-900">customer_abc</span>
              <span className="ml-2 text-xs text-slate-500">45K {t('provider.tokens')}</span>
            </div>
            <span className="text-sm font-semibold text-slate-900">$142.00</span>
          </div>
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div>
              <span className="text-sm font-medium text-slate-900">customer_xyz</span>
              <span className="ml-2 text-xs text-slate-500">32K {t('provider.tokens')}</span>
            </div>
            <span className="text-sm font-semibold text-slate-900">$98.00</span>
          </div>
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div>
              <span className="text-sm font-medium text-slate-900">customer_def</span>
              <span className="ml-2 text-xs text-slate-500">28K {t('provider.tokens')}</span>
            </div>
            <span className="text-sm font-semibold text-slate-900">$76.00</span>
          </div>
        </div>
      </section>

      {/* Service Stats Modal (placeholder for selected service analytics) */}
      {selectedService && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-4xl rounded-xl bg-white p-6 shadow-xl">
            <div className="flex items-start justify-between">
              <div>
                <h3 className="text-xl font-bold text-slate-900">{selectedService.name} - Analytics</h3>
                <p className="mt-1 text-sm text-slate-500">Performance metrics and usage statistics</p>
              </div>
              <button
                onClick={() => setSelectedService(null)}
                className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
              >
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="mt-6 grid gap-4 sm:grid-cols-3">
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <div className="text-xs text-slate-500">Total Requests (30d)</div>
                <div className="mt-1 text-2xl font-bold text-slate-900">
                  {selectedService.usageStats?.monthlyRequests?.toLocaleString() || '0'}
                </div>
              </div>
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <div className="text-xs text-slate-500">Tokens Served</div>
                <div className="mt-1 text-2xl font-bold text-slate-900">
                  {selectedService.usageStats?.totalTokensServed
                    ? (selectedService.usageStats.totalTokensServed / 1_000_000).toFixed(1) + 'M'
                    : '0'}
                </div>
              </div>
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <div className="text-xs text-slate-500">Active Users</div>
                <div className="mt-1 text-2xl font-bold text-slate-900">
                  {selectedService.usageStats?.activeUsers?.toLocaleString() || '0'}
                </div>
              </div>
            </div>

            <div className="mt-4 rounded-lg border border-slate-200 p-4">
              <h4 className="text-sm font-semibold text-slate-900">Performance Metrics</h4>
              <div className="mt-3 grid gap-3 sm:grid-cols-3">
                <div>
                  <div className="text-xs text-slate-500">Avg Latency (p50)</div>
                  <div className="mt-1 text-lg font-semibold text-slate-900">
                    {selectedService.metrics?.latencyP50 || 0}ms
                  </div>
                </div>
                <div>
                  <div className="text-xs text-slate-500">Uptime (30d)</div>
                  <div className="mt-1 text-lg font-semibold text-emerald-600">
                    {selectedService.metrics?.uptime30d?.toFixed(2) || 0}%
                  </div>
                </div>
                <div>
                  <div className="text-xs text-slate-500">Rating</div>
                  <div className="mt-1 text-lg font-semibold text-slate-900">
                    ⭐ {selectedService.rating?.toFixed(1) || 'N/A'}
                  </div>
                </div>
              </div>
            </div>

            <div className="mt-6 flex justify-end">
              <button
                onClick={() => setSelectedService(null)}
                className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Publish Service Modal */}
      {showPublishModal && (
        <PublishServiceModal
          onClose={() => setShowPublishModal(false)}
          onPublish={async (service) => {
            console.log('Publishing service:', service)
            // This will be connected to actual API later
            setShowPublishModal(false)
          }}
        />
      )}
    </div>
  )
}

function StatCard({
  title,
  value,
  description,
  loading,
}: {
  title: string
  value: string
  description: string
  loading?: boolean
}) {
  return (
    <article className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h3 className="text-xs font-medium uppercase tracking-wide text-slate-500">{title}</h3>
      {loading ? (
        <div className="mt-2 h-8 w-24 animate-pulse rounded bg-slate-200" />
      ) : (
        <p className="mt-2 text-2xl font-bold text-slate-900">{value}</p>
      )}
      <p className="mt-1 text-xs text-slate-500">{description}</p>
    </article>
  )
}

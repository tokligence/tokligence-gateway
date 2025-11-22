import { useTranslation } from 'react-i18next'
import { useUsageSummaryQuery } from '../hooks/useGatewayQueries'
import { useProfileContext } from '../context/ProfileContext'
import { formatNumber } from '../utils/format'

export function ProviderDashboardPage() {
  const { t } = useTranslation()
  const profile = useProfileContext()
  const isAuthenticated = Boolean(profile)
  const { data: usage } = useUsageSummaryQuery(isAuthenticated)

  const suppliedTokens = usage?.summary.suppliedTokens ?? 0

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
          <h3 className="text-base font-semibold text-slate-900">{t('provider.publishedServices')}</h3>
          <button
            type="button"
            className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
          >
            + {t('provider.publishNewService')}
          </button>
        </div>

        {/* Placeholder for services */}
        <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
          <p className="text-sm text-slate-600">{t('provider.noServices')}</p>
          <p className="mt-1 text-sm text-slate-500">
            {t('provider.noServicesDesc')}
          </p>
          <button
            type="button"
            className="mt-4 rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
          >
            {t('provider.getStarted')}
          </button>
        </div>
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

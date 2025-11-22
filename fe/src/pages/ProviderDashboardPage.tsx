import { useFeature, useEdition } from '../context/EditionContext'
import { useUsageSummaryQuery } from '../hooks/useGatewayQueries'
import { useProfileContext } from '../context/ProfileContext'
import { formatNumber } from '../utils/format'

export function ProviderDashboardPage() {
  const { edition } = useEdition()
  const canSellTokens = useFeature('marketplaceProvider')
  const profile = useProfileContext()
  const isAuthenticated = Boolean(profile)
  const { data: usage } = useUsageSummaryQuery(isAuthenticated)

  const suppliedTokens = usage?.summary.suppliedTokens ?? 0

  // If user doesn't have provider features, show upgrade prompt
  if (!canSellTokens) {
    return (
      <div className="space-y-6">
        <section className="rounded-xl border-2 border-amber-300 bg-gradient-to-br from-amber-50 to-yellow-50 p-8">
          <div className="flex items-start gap-4">
            <span className="text-5xl">🔒</span>
            <div className="flex-1">
              <h2 className="text-2xl font-bold text-amber-900">Provider Features Locked</h2>
              <p className="mt-2 text-base text-amber-800">
                You're currently using <span className="font-semibold">{edition}</span> edition, which only allows you
                to <strong>buy tokens</strong> from the marketplace.
              </p>

              <div className="mt-6 rounded-lg bg-white p-6 shadow-sm">
                <h3 className="text-lg font-semibold text-slate-900">Unlock Provider Features</h3>
                <p className="mt-2 text-sm text-slate-600">Upgrade to Team edition to:</p>
                <ul className="mt-3 space-y-2">
                  <li className="flex items-start gap-2 text-sm text-slate-700">
                    <span className="text-emerald-600">✓</span>
                    <span>Publish your own token services to the marketplace</span>
                  </li>
                  <li className="flex items-start gap-2 text-sm text-slate-700">
                    <span className="text-emerald-600">✓</span>
                    <span>Set your own pricing and earn revenue</span>
                  </li>
                  <li className="flex items-start gap-2 text-sm text-slate-700">
                    <span className="text-emerald-600">✓</span>
                    <span>Track revenue and customer analytics</span>
                  </li>
                  <li className="flex items-start gap-2 text-sm text-slate-700">
                    <span className="text-emerald-600">✓</span>
                    <span>Monthly payouts directly to your bank account</span>
                  </li>
                </ul>

                <div className="mt-6 rounded-lg bg-emerald-50 p-4">
                  <p className="text-sm font-semibold text-emerald-900">💰 Earnings Potential</p>
                  <p className="mt-1 text-sm text-emerald-800">
                    Based on marketplace data, providers with your token consumption pattern could earn approximately{' '}
                    <strong className="text-lg">$850/month</strong> by selling tokens.
                  </p>
                </div>

                <div className="mt-6 flex gap-3">
                  <button
                    type="button"
                    className="rounded-lg bg-amber-900 px-6 py-3 font-semibold text-white shadow-md hover:bg-amber-800"
                  >
                    Upgrade to Team Edition ($29/mo)
                  </button>
                  <button
                    type="button"
                    className="rounded-lg border-2 border-amber-900 px-6 py-3 font-semibold text-amber-900 hover:bg-amber-100"
                  >
                    Learn More
                  </button>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    )
  }

  // Provider dashboard for users with provider features
  return (
    <div className="space-y-6">
      <header>
        <h2 className="text-lg font-semibold text-slate-900">💰 Provider Dashboard</h2>
        <p className="text-sm text-slate-500">Manage your token services, track revenue, and view customer analytics</p>
      </header>

      {/* Revenue overview */}
      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Tokens Supplied"
          value={formatNumber(suppliedTokens)}
          description="Total tokens sold to consumers"
        />
        <StatCard title="Gross Revenue" value="$1,420.00" description="Before platform fee" />
        <StatCard title="Platform Fee (10%)" value="-$142.00" description="Marketplace commission" />
        <StatCard title="Net Earnings" value="$1,278.00" description="Your revenue this month" />
      </section>

      {/* Payout info */}
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-base font-semibold text-slate-900">Next Payout</h3>
            <p className="text-sm text-slate-500">December 1, 2025</p>
          </div>
          <div className="text-right">
            <div className="text-2xl font-bold text-emerald-600">$1,278.00</div>
            <p className="text-xs text-slate-500">Estimated amount</p>
          </div>
        </div>
        <div className="mt-4 flex gap-2">
          <button
            type="button"
            className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            View Payout History
          </button>
          <button
            type="button"
            className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            Update Bank Account
          </button>
        </div>
      </section>

      {/* Published services */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-slate-900">Your Published Services</h3>
          <button
            type="button"
            className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
          >
            + Publish New Service
          </button>
        </div>

        {/* Placeholder for services */}
        <div className="rounded-xl border border-slate-200 bg-slate-50 p-8 text-center">
          <p className="text-sm text-slate-600">You haven't published any services yet.</p>
          <p className="mt-1 text-sm text-slate-500">
            Publish your first service to start earning revenue from the marketplace.
          </p>
          <button
            type="button"
            className="mt-4 rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
          >
            Get Started
          </button>
        </div>
      </section>

      {/* Revenue trends */}
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-base font-semibold text-slate-900">Revenue Trends (Last 30 Days)</h3>
        <div className="mt-4 flex h-40 items-center justify-center border border-slate-200 bg-slate-50">
          <p className="text-sm text-slate-500">[Chart placeholder - Daily revenue and token supply]</p>
        </div>
      </section>

      {/* Top customers */}
      <section className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-base font-semibold text-slate-900">Top Customers by Volume</h3>
        <div className="mt-4 space-y-2">
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div>
              <span className="text-sm font-medium text-slate-900">customer_abc</span>
              <span className="ml-2 text-xs text-slate-500">45K tokens</span>
            </div>
            <span className="text-sm font-semibold text-slate-900">$142.00</span>
          </div>
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div>
              <span className="text-sm font-medium text-slate-900">customer_xyz</span>
              <span className="ml-2 text-xs text-slate-500">32K tokens</span>
            </div>
            <span className="text-sm font-semibold text-slate-900">$98.00</span>
          </div>
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 p-3">
            <div>
              <span className="text-sm font-medium text-slate-900">customer_def</span>
              <span className="ml-2 text-xs text-slate-500">28K tokens</span>
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

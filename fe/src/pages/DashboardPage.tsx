import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useUsageSummaryQuery, useUsageLogsQuery, useServicesQuery } from '../hooks/useGatewayQueries'
import { useProfileContext } from '../context/ProfileContext'
import { formatNumber } from '../utils/format'

export function DashboardPage() {
  const profile = useProfileContext()
  const isAuthenticated = Boolean(profile)
  const { data: usage, isPending: usageLoading } = useUsageSummaryQuery(isAuthenticated)
  const { data: usageLogs } = useUsageLogsQuery(10, { enabled: isAuthenticated })
  const { data: servicesData } = useServicesQuery({ scope: 'all' })

  const consumed = usage?.summary.consumedTokens ?? 0
  const supplied = usage?.summary.suppliedTokens ?? 0
  const net = usage?.summary.netTokens ?? 0

  const roleLabel = profile?.user.roles.includes('provider') ? 'Consumer & Provider' : 'Consumer'

  // Get top 2 featured services for dashboard
  const featuredServices = servicesData?.services.slice(0, 2) ?? []

  return (
    <div className="space-y-6">
      {!profile?.marketplace?.connected && (
        <section className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
          <p className="font-medium">Marketplace unavailable</p>
          <p className="mt-1">
            Tokligence Exchange is currently offline or disabled. Local adapters continue to work, but browsing or publishing
            marketplace services is paused.
          </p>
        </section>
      )}
      <section>
        <h2 className="text-lg font-semibold text-slate-900">Welcome back</h2>
        <p className="text-sm text-slate-500">{roleLabel} account overview with live usage totals.</p>
      </section>
      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard
          title="Consumed tokens"
          value={formatNumber(consumed)}
          description="Total downstream usage reported to Tokligence Exchange"
          loading={usageLoading}
        />
        <StatCard
          title="Supplied tokens"
          value={formatNumber(supplied)}
          description="Tokens served to consumers via published services"
          loading={usageLoading}
        />
        <StatCard
          title="Net position"
          value={formatNumber(net)}
          description="Consumed minus supplied. Positive means more demand."
          loading={usageLoading}
        />
      </section>

      {/* Marketplace Featured Providers */}
      {featuredServices.length > 0 && (
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-slate-900">🛒 Featured Marketplace Providers</h2>
            <Link
              to="/marketplace"
              className="text-sm font-medium text-blue-600 hover:text-blue-700 hover:underline"
            >
              Browse all →
            </Link>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            {featuredServices.map((service) => (
              <article key={service.id} className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="font-semibold text-slate-900">{service.name}</h3>
                    <p className="text-xs text-slate-500">{service.modelFamily}</p>
                  </div>
                  <div className="text-right">
                    <div className="text-lg font-bold text-slate-900">${service.pricePer1KTokens.toFixed(4)}</div>
                    <div className="text-xs text-slate-500">per 1K tokens</div>
                  </div>
                </div>
                <div className="mt-3 flex items-center gap-2 text-xs text-slate-500">
                  <span>⭐ 4.8 (1.2K)</span>
                  <span>•</span>
                  <span>99.5% uptime</span>
                </div>
                {service.trialTokens && service.trialTokens > 0 && (
                  <div className="mt-2 text-xs text-blue-600">
                    🎁 {service.trialTokens.toLocaleString()} free tokens
                  </div>
                )}
                <div className="mt-3 flex gap-2">
                  <button
                    type="button"
                    className="flex-1 rounded-lg border border-slate-200 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                  >
                    Details
                  </button>
                  <button
                    type="button"
                    className="flex-1 rounded-lg bg-slate-900 py-1.5 text-sm font-medium text-white hover:bg-slate-800"
                  >
                    Start Using
                  </button>
                </div>
              </article>
            ))}
          </div>
        </section>
      )}

      <section className="grid gap-4 lg:grid-cols-2">
        <Card title="Account roles">
          <ul className="space-y-2 text-sm text-slate-600">
            {profile?.user.roles.map((role) => (
              <li key={role} className="flex items-center gap-2">
                <span className="inline-flex h-2 w-2 rounded-full bg-slate-400" aria-hidden />
                <span className="capitalize">{role}</span>
              </li>
            )) ?? (
              <li className="text-slate-400">No roles detected.</li>
            )}
          </ul>
        </Card>
        <Card title="Next steps">
          <ol className="list-decimal space-y-2 pl-5 text-sm text-slate-600">
            <li>Publish a local adapter to the Exchange marketplace.</li>
            <li>Connect OpenAI-compatible clients to the Tokligence Gateway endpoint.</li>
            <li>Review usage ledger and reconcile with Exchange statements.</li>
          </ol>
        </Card>
      </section>

      <section>
        <Card title="Recent usage entries">
          {usageLogs?.entries?.length ? (
            <table className="min-w-full text-left text-sm text-slate-600">
              <thead className="text-xs uppercase tracking-wide text-slate-400">
                <tr>
                  <th className="py-2">Timestamp</th>
                  <th className="py-2">Direction</th>
                  <th className="py-2">Prompt</th>
                  <th className="py-2">Completion</th>
                  <th className="py-2">Memo</th>
                </tr>
              </thead>
              <tbody>
                {usageLogs.entries.map((entry) => (
                  <tr key={entry.id} className="border-t border-slate-100">
                    <td className="py-2 text-slate-500">{new Date(entry.created_at).toLocaleString()}</td>
                    <td className="py-2 capitalize">{entry.direction}</td>
                    <td className="py-2">{entry.prompt_tokens}</td>
                    <td className="py-2">{entry.completion_tokens}</td>
                    <td className="py-2 text-slate-500">{entry.memo}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="text-sm text-slate-500">No usage entries recorded yet.</p>
          )}
        </Card>
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
    <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
      <h3 className="text-sm font-medium text-slate-500">{title}</h3>
      <p className="mt-2 text-3xl font-semibold text-slate-900">
        {loading ? <span className="text-base text-slate-400">Loading...</span> : value}
      </p>
      <p className="mt-2 text-xs text-slate-500">{description}</p>
    </div>
  )
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h3 className="text-sm font-semibold text-slate-700">{title}</h3>
      <div className="mt-3 text-sm text-slate-600">{children}</div>
    </div>
  )
}

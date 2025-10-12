import type { ReactNode } from 'react'
import { useUsageSummaryQuery, useProfileQuery } from '../hooks/useGatewayQueries'
import { formatNumber } from '../utils/format'

export function DashboardPage() {
  const { data: usage, isPending: usageLoading } = useUsageSummaryQuery()
  const { data: profile } = useProfileQuery()

  const consumed = usage?.summary.consumedTokens ?? 0
  const supplied = usage?.summary.suppliedTokens ?? 0
  const net = usage?.summary.netTokens ?? 0

  const roleLabel = profile?.user.roles.includes('provider') ? 'Consumer & Provider' : 'Consumer'

  return (
    <div className="space-y-6">
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

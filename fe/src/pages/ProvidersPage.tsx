import { useProvidersQuery } from '../hooks/useGatewayQueries'

export function ProvidersPage() {
  const { data, isPending } = useProvidersQuery()

  return (
    <div className="space-y-6">
      <header>
        <h2 className="text-lg font-semibold text-slate-900">Provider catalog</h2>
        <p className="text-sm text-slate-500">Browse available adapters sourced from Tokligence Exchange.</p>
      </header>
      <div className="grid gap-4 md:grid-cols-2">
        {isPending && <Skeleton />}
        {data?.providers.map((provider) => (
          <article
            key={provider.id}
            className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm"
          >
            <div className="flex items-start justify-between">
              <div>
                <h3 className="text-base font-semibold text-slate-900">{provider.displayName}</h3>
                <p className="text-xs uppercase tracking-wide text-slate-400">Provider #{provider.id}</p>
              </div>
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700">
                User {provider.userId}
              </span>
            </div>
            <p className="mt-3 text-sm text-slate-600">
              {provider.description ?? 'No description provided.'}
            </p>
          </article>
        ))}
      </div>
    </div>
  )
}

function Skeleton() {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="h-4 w-1/3 animate-pulse rounded bg-slate-200" />
      <div className="mt-3 h-3 w-2/3 animate-pulse rounded bg-slate-200" />
      <div className="mt-2 h-3 w-1/2 animate-pulse rounded bg-slate-200" />
    </div>
  )
}

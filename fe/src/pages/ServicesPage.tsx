import { useState } from 'react'
import { useServicesQuery } from '../hooks/useGatewayQueries'
import type { ApiListParams } from '../types/api'

const scopes: { label: string; value: ApiListParams['scope'] }[] = [
  { label: 'All services', value: 'all' },
  { label: 'My services', value: 'mine' },
]

export function ServicesPage() {
  const [scope, setScope] = useState<ApiListParams['scope']>('all')
  const { data, isPending } = useServicesQuery({ scope })

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">Services</h2>
          <p className="text-sm text-slate-500">Manage local adapters and discover marketplace offerings.</p>
        </div>
        <div className="flex items-center gap-2 rounded-full border border-slate-200 bg-white p-1 shadow-inner">
          {scopes.map((item) => (
            <button
              key={item.value}
              type="button"
              onClick={() => setScope(item.value)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                scope === item.value ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100'
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </header>

      <div className="grid gap-4">
        {isPending && <ServiceSkeleton />}
        {data?.services.map((service) => (
          <article key={service.id} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
              <div>
                <h3 className="text-base font-semibold text-slate-900">{service.name}</h3>
                <p className="text-xs uppercase tracking-wide text-slate-400">Service #{service.id}</p>
                <p className="mt-2 text-sm text-slate-600">Model family: {service.modelFamily}</p>
              </div>
              <dl className="grid grid-cols-2 gap-4 text-sm text-slate-600">
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-400">Price / 1K tokens</dt>
                  <dd className="font-medium text-slate-900">${service.pricePer1KTokens.toFixed(2)}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-400">Trial tokens</dt>
                  <dd className="font-medium text-slate-900">{service.trialTokens ?? 0}</dd>
                </div>
                <div className="col-span-2">
                  <dt className="text-xs uppercase tracking-wide text-slate-400">Provider</dt>
                  <dd className="font-medium text-slate-900">#{service.providerId}</dd>
                </div>
              </dl>
            </div>
          </article>
        ))}
      </div>
    </div>
  )
}

function ServiceSkeleton() {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="h-4 w-1/4 animate-pulse rounded bg-slate-200" />
      <div className="mt-2 h-3 w-1/3 animate-pulse rounded bg-slate-200" />
      <div className="mt-4 grid grid-cols-2 gap-4">
        <div className="h-3 w-2/3 animate-pulse rounded bg-slate-200" />
        <div className="h-3 w-1/2 animate-pulse rounded bg-slate-200" />
      </div>
    </div>
  )
}

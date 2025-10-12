import { useProfileContext } from '../context/ProfileContext'

export function SettingsPage() {
  const profile = useProfileContext()

  if (!profile) {
    return <p className="text-sm text-slate-500">Loading account details…</p>
  }

  const { user, provider } = profile

  return (
    <div className="space-y-6">
      <section>
        <h2 className="text-lg font-semibold text-slate-900">Account settings</h2>
        <p className="text-sm text-slate-500">Manage profile metadata and provider enrollment.</p>
      </section>
      <section className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
          <h3 className="text-sm font-semibold text-slate-700">Profile</h3>
          <dl className="mt-3 space-y-2 text-sm text-slate-600">
            <div>
              <dt className="text-xs uppercase tracking-wide text-slate-400">Email</dt>
              <dd>{user.email}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-slate-400">Display name</dt>
              <dd>{user.displayName ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-slate-400">Roles</dt>
              <dd className="flex flex-wrap gap-2">
                {user.roles.map((role) => (
                  <span
                    key={role}
                    className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700"
                  >
                    {role}
                  </span>
                ))}
              </dd>
            </div>
          </dl>
        </div>

        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
          <h3 className="text-sm font-semibold text-slate-700">Provider profile</h3>
          {provider ? (
            <dl className="mt-3 space-y-2 text-sm text-slate-600">
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">Display name</dt>
                <dd>{provider.displayName}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">Description</dt>
                <dd>{provider.description ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">Provider ID</dt>
                <dd>{provider.id}</dd>
              </div>
            </dl>
          ) : (
            <p className="mt-3 text-sm text-slate-500">
              Provider role not enabled. Once registered, published services will appear under your catalog.
            </p>
          )}
        </div>
      </section>
    </div>
  )
}

import { useTranslation } from 'react-i18next'
import { useProfileContext } from '../context/ProfileContext'

export function SettingsPage() {
  const { t } = useTranslation()
  const profile = useProfileContext()

  if (!profile) {
    return <p className="text-sm text-slate-500">{t('settings.loadingAccount')}</p>
  }

  const { user, provider } = profile

  return (
    <div className="space-y-6">
      <section>
        <h2 className="text-lg font-semibold text-slate-900">{t('settings.title')}</h2>
        <p className="text-sm text-slate-500">{t('settings.subtitle')}</p>
      </section>
      <section className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
          <h3 className="text-sm font-semibold text-slate-700">{t('settings.profile')}</h3>
          <dl className="mt-3 space-y-2 text-sm text-slate-600">
            <div>
              <dt className="text-xs uppercase tracking-wide text-slate-400">{t('settings.email')}</dt>
              <dd>{user.email}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-slate-400">{t('settings.displayName')}</dt>
              <dd>{user.displayName ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-slate-400">{t('settings.roles')}</dt>
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
          <h3 className="text-sm font-semibold text-slate-700">{t('settings.providerProfile')}</h3>
          {provider ? (
            <dl className="mt-3 space-y-2 text-sm text-slate-600">
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">{t('settings.displayName')}</dt>
                <dd>{provider.displayName}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">{t('settings.description')}</dt>
                <dd>{provider.description ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">{t('settings.providerId')}</dt>
                <dd>{provider.id}</dd>
              </div>
            </dl>
          ) : (
            <p className="mt-3 text-sm text-slate-500">
              {t('settings.providerNotEnabled')}
            </p>
          )}
        </div>
      </section>
    </div>
  )
}

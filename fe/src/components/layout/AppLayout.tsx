import { NavLink } from 'react-router-dom'
import type { PropsWithChildren } from 'react'
import { useProfileContext } from '../../context/ProfileContext'
import { useFeature } from '../../context/EditionContext'
import { LanguageSwitcher } from '../LanguageSwitcher'

const consumerNavigation = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/marketplace', label: '🛒 Buy Tokens' }, // Consumer action
  { to: '/settings', label: 'Settings' },
]

const providerNavigation = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/marketplace', label: '🛒 Buy Tokens' }, // Consumer action
  { to: '/provider', label: '💰 Sell Tokens' }, // Provider action
  { to: '/settings', label: 'Settings' },
]

export function AppLayout({ children }: PropsWithChildren) {
  const profile = useProfileContext()
  const canSellTokens = useFeature('marketplaceProvider')
  const displayName = profile?.user.displayName ?? profile?.user.email ?? 'Unknown user'
  const roles = profile?.user.roles ?? []
  const isProvider = Boolean(profile?.provider)
  const isRootAdmin = roles.includes('root_admin')

  // Choose navigation based on provider capability
  let navigation = canSellTokens ? providerNavigation : consumerNavigation

  // Add admin link if root admin
  if (isRootAdmin) {
    navigation = [...navigation, { to: '/admin/users', label: 'Admin' }]
  }

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <header className="border-b border-slate-200 bg-white/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-slate-900">Tokligence Gateway</h1>
            <p className="text-sm text-slate-500">Unified control plane for adapters & usage</p>
          </div>
          <div className="flex items-center gap-6">
            <nav className="flex flex-wrap gap-2 md:gap-3">
              {navigation.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    `rounded-full px-3 py-1 text-sm font-medium transition-colors ${
                      isActive
                        ? 'bg-slate-900 text-white'
                        : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
                    }`
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
            <LanguageSwitcher />
            <div className="hidden shrink-0 items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1 text-sm text-slate-600 shadow-sm md:flex">
              <span className="font-medium text-slate-900">{displayName}</span>
              <span
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                  isRootAdmin
                    ? 'bg-slate-900 text-white'
                    : isProvider
                    ? 'bg-emerald-100 text-emerald-700'
                    : 'bg-blue-100 text-blue-700'
                }`}
              >
                {isRootAdmin ? 'Root Admin' : isProvider ? 'Provider' : 'Consumer'}
              </span>
            </div>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">{children}</main>
    </div>
  )
}

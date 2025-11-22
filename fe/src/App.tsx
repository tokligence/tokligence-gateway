import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider, useQueryClient } from '@tanstack/react-query'
import { AppLayout } from './components/layout/AppLayout'
import { DashboardPage } from './pages/DashboardPage'
import { MarketplacePage } from './pages/MarketplacePage'
import { ProviderDashboardPage } from './pages/ProviderDashboardPage'
import { ServicesPage } from './pages/ServicesPage'
import { SettingsPage } from './pages/SettingsPage'
import { LoginPage } from './pages/LoginPage'
import { AdminUsersPage } from './pages/AdminUsersPage'
import { useProfileQuery } from './hooks/useGatewayQueries'
import { ProfileProvider } from './context/ProfileContext'
import { EditionProvider } from './context/EditionContext'
import type { ApiError } from './types/api'

const queryClient = new QueryClient()

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppShell />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

function AppShell() {
  const profileQuery = useProfileQuery()
  const queryClient = useQueryClient()

  if (profileQuery.isLoading) {
    return <FullScreenMessage message="Loading profile…" />
  }

  if (profileQuery.isError) {
    const err = profileQuery.error as ApiError
    if (err.status === 401) {
      return (
        <LoginPage
          onSuccess={async () => {
            await queryClient.invalidateQueries({ queryKey: ['profile'] })
          }}
        />
      )
    }
    return <FullScreenMessage message={`Failed to load profile: ${err.message}`} variant="error" />
  }

  const profile = profileQuery.data ?? null

  return (
    <ProfileProvider value={profile}>
      <EditionProvider>
        <AppLayout>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/marketplace" element={<MarketplacePage />} />
            <Route path="/provider" element={<ProviderDashboardPage />} />
            <Route path="/providers" element={<Navigate to="/marketplace" replace />} />
            <Route path="/services" element={<ServicesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/admin/users" element={<AdminUsersPage />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </AppLayout>
      </EditionProvider>
    </ProfileProvider>
  )
}

function NotFound() {
  return (
    <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-sm text-rose-700">
      The requested page could not be found.
    </div>
  )
}

function FullScreenMessage({ message, variant = 'info' }: { message: string; variant?: 'info' | 'error' }) {
  const colors =
    variant === 'error'
      ? 'border-rose-200 bg-rose-50 text-rose-700'
      : 'border-slate-200 bg-white text-slate-700'
  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <div className={`w-full max-w-md rounded-2xl border p-6 text-center shadow ${colors}`}>{message}</div>
    </div>
  )
}

import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AppLayout } from './components/layout/AppLayout'
import { DashboardPage } from './pages/DashboardPage'
import { ProvidersPage } from './pages/ProvidersPage'
import { ServicesPage } from './pages/ServicesPage'
import { SettingsPage } from './pages/SettingsPage'

const queryClient = new QueryClient()

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppLayout>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/providers" element={<ProvidersPage />} />
            <Route path="/services" element={<ServicesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </AppLayout>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

function NotFound() {
  return (
    <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-sm text-rose-700">
      The requested page could not be found.
    </div>
  )
}

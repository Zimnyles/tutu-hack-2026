import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, WifiOff } from 'lucide-react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { api } from './api'
import { AdminView } from './features/admin/AdminView'
import { AuthScreen } from './features/auth/AuthScreen'
import { Onboarding } from './features/onboarding/Onboarding'
import { Splash } from './features/shared/Splash'
import { WorldApp } from './features/world/WorldApp'
import { useApp } from './state'
import type { PublicSettings, User } from './types'

export default function App() {
  const queryClient = useQueryClient()
  const toast = useApp((state) => state.toast)
  const setToast = useApp((state) => state.setToast)
  const [offline, setOffline] = useState(!navigator.onLine)

  const settings = useQuery({
    queryKey: ['public-settings'],
    queryFn: () => api<PublicSettings>('/config'),
    staleTime: 5 * 60_000,
  })
  const session = useQuery({
    queryKey: ['current-user'],
    queryFn: () => api<{ user: User }>('/auth/me'),
    retry: false,
  })

  useEffect(() => {
    const handleOnline = () => setOffline(false)
    const handleOffline = () => setOffline(true)
    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)

    return () => {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
    }
  }, [])

  useEffect(() => {
    if (!toast) {
      return
    }
    const timeout = window.setTimeout(() => setToast(''), 3200)

    return () => window.clearTimeout(timeout)
  }, [setToast, toast])

  if (settings.isLoading || session.isLoading) {
    return <Splash />
  }
  if (settings.isError || !settings.data) {
    return <FatalState message="Не удалось загрузить настройки приложения" />
  }
  if (session.isError || !session.data) {
    return (
      <AuthScreen
        onAuthenticated={() => queryClient.invalidateQueries({ queryKey: ['current-user'] })}
      />
    )
  }

  const user = session.data.user
  if (!user.onboarding_completed) {
    return (
      <Onboarding
        user={user}
        settings={settings.data}
        onCompleted={() => queryClient.invalidateQueries({ queryKey: ['current-user'] })}
      />
    )
  }

  return (
    <>
      {offline && (
        <div className="offline-banner" role="status" aria-live="polite">
          <WifiOff size={16} aria-hidden="true" /> Нет сети: подбор и билеты временно недоступны
        </div>
      )}
      <Routes>
        <Route path="/" element={<WorldApp user={user} settings={settings.data} />} />
        <Route
          path="/admin"
          element={user.role === 'demo_admin' ? <AdminView user={user} /> : <Navigate to="/" replace />}
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <div className={`toast ${toast ? 'toast-visible' : ''}`} role="status" aria-live="polite">
        <Check size={17} aria-hidden="true" />
        {toast}
      </div>
    </>
  )
}

function FatalState({ message }: { message: string }) {
  return (
    <main className="centered-state">
      <h1>Не удалось открыть приложение</h1>
      <p>{message}</p>
      <button className="primary-button" onClick={() => window.location.reload()}>
        Повторить
      </button>
    </main>
  )
}

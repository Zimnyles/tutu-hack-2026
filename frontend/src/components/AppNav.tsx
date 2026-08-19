import { Map, Route, Shield, Trophy, UserRound } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { Logo } from './Logo'
import { useApp, type AppView } from '../state'
import type { User } from '../types'

const items: Array<{ id: AppView; label: string; icon: typeof Map }> = [
  { id: 'world', label: 'Мир', icon: Map },
  { id: 'trips', label: 'Поездки', icon: Route },
  { id: 'community', label: 'Сезон', icon: Trophy },
  { id: 'profile', label: 'Профиль', icon: UserRound },
]

export function AppNav({ user }: { user: User }) {
  const view = useApp((state) => state.view)
  const setView = useApp((state) => state.setView)
  const navigate = useNavigate()

  return (
    <>
      <aside className="app-sidebar">
        <Logo inverse />
        <nav aria-label="Основная навигация">
          {items.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className={view === id ? 'active' : ''}
              aria-current={view === id ? 'page' : undefined}
              onClick={() => setView(id)}
            >
              <Icon aria-hidden="true" /> <span>{label}</span>
            </button>
          ))}
        </nav>
        {user.role === 'demo_admin' && (
          <button className="sidebar-admin" onClick={() => navigate('/admin')}>
            <Shield aria-hidden="true" /> <span>Управление</span>
          </button>
        )}
        <div className="sidebar-user">
          <span aria-hidden="true">{user.display_name.slice(0, 1).toUpperCase()}</span>
          <div><strong>{user.display_name}</strong><small>{user.email}</small></div>
        </div>
      </aside>

      <nav className="mobile-nav" aria-label="Основная навигация">
        {items.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            className={view === id ? 'active' : ''}
            aria-current={view === id ? 'page' : undefined}
            onClick={() => setView(id)}
          >
            <Icon aria-hidden="true" /><span>{label}</span>
          </button>
        ))}
      </nav>
    </>
  )
}

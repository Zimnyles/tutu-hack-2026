import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Award,
  ClipboardCheck,
  Coins,
  Copy,
  Home,
  LogOut,
  Map,
  Settings2,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  TicketPercent,
} from 'lucide-react'
import { api } from '../../api'
import { EmptyState, ErrorState, LoadingState } from '../../components/States'
import { errorMessage, formatDate, formatDuration, percentage } from '../../shared/format'
import { Glyph } from '../../shared/icons'
import { useApp } from '../../state'
import type { Achievement, Preferences, ProfileResponse, PromoCode, PublicSettings, User } from '../../types'

const spontaneityLabels = ['Всё по плану', 'Скорее планирую', 'Как получится', 'Скорее спонтанно', 'Решаю на месте']
const durationOptions = [
  { value: 2, label: 'Выходные' },
  { value: 3, label: '3 дня' },
  { value: 5, label: '5 дней' },
  { value: 7, label: 'Неделя' },
  { value: 10, label: '10 дней' },
]

const visibilityOptions = [
  { value: 'private', label: 'Только я', hint: 'Никто не видит ваши планы' },
  { value: 'aggregate', label: 'Обезличенно для гильдии', hint: 'Только количество людей, без имён' },
]

function PreferencesSection({ user, settings }: { user: User; settings: PublicSettings }) {
  const queryClient = useQueryClient()
  const setToast = useApp((state) => state.setToast)
  const [draft, setDraft] = useState<Preferences>(() => ({
    themes: user.preferences.themes ?? [],
    transport_modes: user.preferences.transport_modes ?? [],
    max_travel_minutes: user.preferences.max_travel_minutes || 720,
    typical_budget: user.preferences.typical_budget || settings.onboarding.budget_min,
    trip_duration_days: user.preferences.trip_duration_days || 3,
    spontaneity: user.preferences.spontaneity || 3,
    avoid: user.preferences.avoid ?? [],
  }))

  const save = useMutation({
    mutationFn: () => api('/profile/preferences', { method: 'PUT', body: JSON.stringify(draft) }),
    onSuccess: () => {
      setToast('Предпочтения сохранены, обновляем подбор')
      queryClient.invalidateQueries({ queryKey: ['current-user'] })
      queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
    },
  })

  const changed = useMemo(
    () => JSON.stringify(draft) !== JSON.stringify({
      themes: user.preferences.themes ?? [],
      transport_modes: user.preferences.transport_modes ?? [],
      max_travel_minutes: user.preferences.max_travel_minutes || 720,
      typical_budget: user.preferences.typical_budget || settings.onboarding.budget_min,
      trip_duration_days: user.preferences.trip_duration_days || 3,
      spontaneity: user.preferences.spontaneity || 3,
      avoid: user.preferences.avoid ?? [],
    }),
    [draft, settings.onboarding.budget_min, user.preferences],
  )

  const travelTimeOptions = settings.onboarding.travel_time_options.length > 0
    ? settings.onboarding.travel_time_options
    : [180, 360, 720, 1440, 2880]

  const toggle = (key: 'themes' | 'transport_modes', code: string) => setDraft((current) => ({
    ...current,
    [key]: current[key].includes(code)
      ? current[key].filter((item) => item !== code)
      : [...current[key], code],
  }))

  const incomplete = draft.themes.length === 0 || draft.transport_modes.length === 0

  return (
    <section className="surface preferences-section">
      <div className="section-title">
        <div>
          <span className="section-icon orange"><SlidersHorizontal aria-hidden="true" /></span>
          <div><h2>Предпочтения</h2><p>По ним подбираем города и билеты</p></div>
        </div>
      </div>

      <fieldset className="chip-field">
        <legend>Интересы</legend>
        <div className="chip-row">
          {settings.onboarding.themes.map((option) => {
            const active = draft.themes.includes(option.code)

            return (
              <button
                key={option.code}
                type="button"
                className={active ? 'chip active' : 'chip'}
                aria-pressed={active}
                onClick={() => toggle('themes', option.code)}
              >
                <Glyph code={option.icon} />
                <span>{option.label}</span>
              </button>
            )
          })}
        </div>
      </fieldset>

      <fieldset className="chip-field">
        <legend>Транспорт</legend>
        <div className="chip-row">
          {settings.onboarding.transport_modes.map((option) => {
            const active = draft.transport_modes.includes(option.code)

            return (
              <button
                key={option.code}
                type="button"
                className={active ? 'chip active' : 'chip'}
                aria-pressed={active}
                onClick={() => toggle('transport_modes', option.code)}
              >
                <Glyph code={option.icon} />
                <span>{option.label}</span>
              </button>
            )
          })}
        </div>
      </fieldset>

      <label className="range-field">
        <span>Обычный бюджет <strong>{draft.typical_budget.toLocaleString('ru-RU')} ₽</strong></span>
        <input
          type="range"
          min={settings.onboarding.budget_min}
          max={settings.onboarding.budget_max}
          step={settings.onboarding.budget_step}
          value={draft.typical_budget}
          onChange={(event) => setDraft((current) => ({ ...current, typical_budget: Number(event.target.value) }))}
        />
      </label>

      <div className="preference-grid">
        <label>
          <span>Длительность</span>
          <select
            value={draft.trip_duration_days}
            onChange={(event) => setDraft((current) => ({ ...current, trip_duration_days: Number(event.target.value) }))}
          >
            {durationOptions.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>
        <label>
          <span>Максимум в пути</span>
          <select
            value={draft.max_travel_minutes}
            onChange={(event) => setDraft((current) => ({ ...current, max_travel_minutes: Number(event.target.value) }))}
          >
            {travelTimeOptions.map((minutes) => (
              <option key={minutes} value={minutes}>{formatDuration(minutes)}</option>
            ))}
          </select>
        </label>
      </div>

      <label className="range-field">
        <span>Планирование <strong>{spontaneityLabels[draft.spontaneity - 1]}</strong></span>
        <input
          type="range"
          min={1}
          max={5}
          step={1}
          value={draft.spontaneity}
          onChange={(event) => setDraft((current) => ({ ...current, spontaneity: Number(event.target.value) }))}
        />
      </label>

      {incomplete && <p className="field-hint">Выберите хотя бы один интерес и один вид транспорта</p>}
      {save.error && <div className="form-error" role="alert">{errorMessage(save.error)}</div>}
      <button
        className="primary-button"
        disabled={!changed || incomplete || save.isPending}
        onClick={() => save.mutate()}
      >
        {save.isPending ? 'Сохраняем…' : 'Сохранить предпочтения'}
      </button>
    </section>
  )
}

function PromoCodesSection() {
  const setToast = useApp((state) => state.setToast)
  const promoCodes = useQuery({
    queryKey: ['promo-codes'],
    queryFn: () => api<{ items: PromoCode[] }>('/rewards/promo-codes'),
  })
  const [copied, setCopied] = useState('')

  const copy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(code)
      setToast(`Промокод ${code} скопирован`)
    } catch {
      setToast('Не удалось скопировать — выделите код вручную')
    }
  }

  const active = promoCodes.data?.items.filter((item) => item.status === 'active').length ?? 0

  return (
    <section className="surface promo-section">
      <div className="section-title">
        <div>
          <span className="section-icon green"><TicketPercent aria-hidden="true" /></span>
          <div><h2>Промокоды Туту</h2><p>{active} действует</p></div>
        </div>
      </div>
      {promoCodes.isLoading && <LoadingState />}
      {promoCodes.isError && <ErrorState message={promoCodes.error.message} retry={() => promoCodes.refetch()} />}
      {promoCodes.data?.items.length === 0 && (
        <EmptyState
          title="Промокодов пока нет"
          text="Часть городов кроме баллов даёт скидку 5–10% на билеты Туту. Откройте такой город — код появится здесь"
        />
      )}
      {promoCodes.data && promoCodes.data.items.length > 0 && (
        <ul className="promo-list">
          {promoCodes.data.items.map((item) => (
            <li key={item.id} className={item.status}>
              <span className="promo-value">−{item.discount_percent}%</span>
              <div>
                <strong>{item.code}</strong>
                <small>
                  {item.city_name}
                  {item.status === 'active'
                    ? ` · действует до ${formatDate(item.expires_at)}`
                    : item.status === 'used' ? ' · использован' : ' · истёк'}
                </small>
              </div>
              <button
                onClick={() => copy(item.code)}
                disabled={item.status !== 'active'}
                aria-label={`Скопировать промокод ${item.code}`}
              >
                {copied === item.code ? <ClipboardCheck aria-hidden="true" /> : <Copy aria-hidden="true" />}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export function ProfileView({ user }: { user: User }) {
  const queryClient = useQueryClient()
  const setToast = useApp((state) => state.setToast)
  const profile = useQuery({ queryKey: ['profile'], queryFn: () => api<ProfileResponse>('/profile') })
  const achievements = useQuery({ queryKey: ['achievements'], queryFn: () => api<{ items: Achievement[] }>('/achievements') })
  const settings = useQuery({
    queryKey: ['public-settings'],
    queryFn: () => api<PublicSettings>('/config'),
    staleTime: 5 * 60_000,
  })
  const homeCity = useMutation({
    mutationFn: (value: string) => api('/profile/home-city', { method: 'PUT', body: JSON.stringify({ home_city_id: value }) }),
    onSuccess: () => {
      setToast('Домашний город обновлён')
      queryClient.invalidateQueries({ queryKey: ['current-user'] })
      queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
    },
  })
  const visibility = useMutation({
    mutationFn: (value: string) => api('/profile/travel-visibility', { method: 'PUT', body: JSON.stringify({ visibility: value }) }),
    onSuccess: () => {
      setToast('Настройки приватности сохранены')
      queryClient.invalidateQueries({ queryKey: ['current-user'] })
      queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] })
    },
  })
  const logout = useMutation({
    mutationFn: () => api('/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      queryClient.clear()
      window.location.href = '/'
    },
  })
  const currentVisibility = user.travel_visibility || 'aggregate'
  const unlocked = achievements.data?.items.filter((item) => item.unlocked).length ?? 0

  return (
    <div className="content-page profile-page">
      <header className="profile-header">
        <div className="profile-avatar" aria-hidden="true">{user.display_name.slice(0, 1).toUpperCase()}</div>
        <div>
          <span>Исследователь</span>
          <h1>{user.display_name}</h1>
          <p>{user.email}</p>
        </div>
      </header>

      {profile.isLoading && <LoadingState />}
      {profile.isError && <ErrorState message={profile.error.message} retry={() => profile.refetch()} />}
      {profile.data && (
        <div className="profile-stats">
          <div><Map aria-hidden="true" /><span><strong>{profile.data.progress.opened}</strong> городов</span></div>
          <div><Sparkles aria-hidden="true" /><span><strong>{Math.round(profile.data.progress.world_percent)}%</strong> мира</span></div>
          <div><Coins aria-hidden="true" /><span><strong>{profile.data.balance}</strong> баллов</span></div>
        </div>
      )}

      <div className="profile-grid">
        <section className="surface achievements-section">
          <div className="section-title">
            <div>
              <span className="section-icon purple"><Award aria-hidden="true" /></span>
              <div><h2>Достижения</h2><p>{unlocked} получено</p></div>
            </div>
          </div>
          {achievements.isLoading && <LoadingState />}
          {achievements.isError && <ErrorState message={achievements.error.message} retry={() => achievements.refetch()} />}
          {achievements.data?.items.length === 0 && <EmptyState title="Всё впереди" text="Откройте первый город, чтобы получить достижение" />}
          {achievements.data && achievements.data.items.length > 0 && (
            <div className="achievement-grid">
              {achievements.data.items.map((item) => (
                <article key={item.id} className={item.unlocked ? 'unlocked' : ''}>
                  <span><Glyph code={item.icon} /></span>
                  <div>
                    <strong>{item.title}</strong>
                    <p>{item.description}</p>
                    <div className="progress-track"><span style={{ width: `${percentage(item.progress, item.target)}%` }} /></div>
                    <small>{Math.min(item.progress, item.target)} / {item.target}</small>
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>

        <PromoCodesSection />

        {settings.data && (
          <PreferencesSection user={user} settings={settings.data} />
        )}

        <section className="surface settings-section">
          <div className="section-title">
            <div>
              <span className="section-icon blue"><Settings2 aria-hidden="true" /></span>
              <div><h2>Настройки</h2><p>Приватность и аккаунт</p></div>
            </div>
          </div>
          <label className="settings-row">
            <span>
              <Home aria-hidden="true" />
              <span>
                <strong>Домашний город</strong>
                <small>Отсюда по умолчанию ищем билеты и считаем поездки</small>
              </span>
            </span>
            <select
              value={user.home_city_id}
              disabled={homeCity.isPending || !settings.data}
              onChange={(event) => homeCity.mutate(event.target.value)}
            >
              {settings.data?.home_cities.map((city) => (
                <option key={city.id} value={city.id}>{city.name} · {city.region}</option>
              ))}
            </select>
          </label>
          {homeCity.error && <div className="form-error" role="alert">{errorMessage(homeCity.error)}</div>}
          <label className="settings-row">
            <span>
              <ShieldCheck aria-hidden="true" />
              <span>
                <strong>Планы поездок</strong>
                <small>{visibilityOptions.find((option) => option.value === currentVisibility)?.hint}</small>
              </span>
            </span>
            <select
              value={currentVisibility}
              disabled={visibility.isPending}
              onChange={(event) => visibility.mutate(event.target.value)}
            >
              {visibilityOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
          {visibility.error && <div className="form-error" role="alert">{visibility.error.message}</div>}
          <button className="logout-button" onClick={() => logout.mutate()} disabled={logout.isPending}>
            <LogOut aria-hidden="true" /> {logout.isPending ? 'Выходим…' : 'Выйти'}
          </button>
        </section>
      </div>
    </div>
  )
}

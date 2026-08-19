import { useMemo, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, Check, MapPin } from 'lucide-react'
import { api } from '../../api'
import { Logo } from '../../components/Logo'
import { Glyph } from '../../shared/icons'
import type { Preferences, PublicSettings, User } from '../../types'

const spontaneityLabels = ['Всё по плану', 'Скорее планирую', 'Как получится', 'Скорее спонтанно', 'Решаю на месте']
const durationOptions = [
  { value: 2, label: 'Выходные' },
  { value: 3, label: '3 дня' },
  { value: 5, label: '5 дней' },
  { value: 7, label: 'Неделя' },
]
const totalSteps = 3

function clampToRange(value: number, minimum: number, maximum: number, fallback: number) {
  if (!Number.isFinite(value) || value < minimum || value > maximum) {
    return fallback
  }

  return value
}

export function Onboarding({ user, settings, onCompleted }: { user: User; settings: PublicSettings; onCompleted: () => void }) {
  const [step, setStep] = useState(0)
  const [themes, setThemes] = useState<string[]>(user.preferences.themes ?? [])
  const [transport, setTransport] = useState<string[]>(user.preferences.transport_modes ?? [])
  const [homeCity, setHomeCity] = useState(user.home_city_id || settings.home_cities[0]?.id || '')
  const [budget, setBudget] = useState(
    clampToRange(user.preferences.typical_budget, settings.onboarding.budget_min, settings.onboarding.budget_max, settings.onboarding.budget_min),
  )
  const [duration, setDuration] = useState(clampToRange(user.preferences.trip_duration_days, 1, 30, 3))
  const [spontaneity, setSpontaneity] = useState(clampToRange(user.preferences.spontaneity, 1, 5, 3))

  const preferences = useMemo<Preferences>(() => ({
    themes,
    transport_modes: transport,
    max_travel_minutes: clampToRange(user.preferences.max_travel_minutes, 30, 4320, 720),
    typical_budget: budget,
    trip_duration_days: duration,
    spontaneity,
    avoid: user.preferences.avoid ?? [],
  }), [budget, duration, spontaneity, themes, transport, user.preferences])

  const complete = useMutation({
    mutationFn: async () => {
      await api('/profile/preferences', { method: 'PUT', body: JSON.stringify(preferences) })
      await api('/profile/onboarding/complete', { method: 'POST', body: JSON.stringify({ home_city_id: homeCity }) })
    },
    onSuccess: onCompleted,
  })

  const canContinue = step === 0 ? themes.length > 0 : step === 1 ? transport.length > 0 : Boolean(homeCity)

  return (
    <main className="onboarding-page">
      <header>
        <Logo />
        <span>Шаг {step + 1} из {totalSteps}</span>
      </header>
      <div
        className="onboarding-progress"
        role="progressbar"
        aria-valuemin={1}
        aria-valuemax={totalSteps}
        aria-valuenow={step + 1}
        aria-label="Прогресс настройки"
      >
        <span style={{ width: `${((step + 1) / totalSteps) * 100}%` }} />
      </div>
      <section className="onboarding-card">
        {step === 0 && (
          <ChoiceStep
            title="Что вам интересно?"
            subtitle="Выберите всё, что подходит — подберём города под ваши интересы"
            options={settings.onboarding.themes}
            selected={themes}
            onChange={setThemes}
          />
        )}
        {step === 1 && (
          <ChoiceStep
            title="Как любите путешествовать?"
            subtitle="Будем искать билеты только на выбранном транспорте"
            options={settings.onboarding.transport_modes}
            selected={transport}
            onChange={setTransport}
          />
        )}
        {step === 2 && (
          <div className="onboarding-fields">
            <div className="step-heading">
              <span className="step-icon"><MapPin aria-hidden="true" /></span>
              <h1>Настроим поездки</h1>
              <p>Это можно изменить в профиле в любой момент</p>
            </div>
            <label>
              Домашний город
              <select value={homeCity} onChange={(event) => setHomeCity(event.target.value)}>
                {settings.home_cities.map((city) => (
                  <option value={city.id} key={city.id}>{city.name} · {city.region}</option>
                ))}
              </select>
            </label>
            <label>
              Обычный бюджет <strong>{budget.toLocaleString('ru-RU')} ₽</strong>
              <input
                type="range"
                min={settings.onboarding.budget_min}
                max={settings.onboarding.budget_max}
                step={settings.onboarding.budget_step}
                value={budget}
                onChange={(event) => setBudget(Number(event.target.value))}
              />
            </label>
            <label>
              Длительность
              <select value={duration} onChange={(event) => setDuration(Number(event.target.value))}>
                {durationOptions.map((option) => (
                  <option value={option.value} key={option.value}>{option.label}</option>
                ))}
              </select>
            </label>
            <label>
              Планирование <strong>{spontaneityLabels[spontaneity - 1]}</strong>
              <input
                type="range"
                min={1}
                max={5}
                step={1}
                value={spontaneity}
                onChange={(event) => setSpontaneity(Number(event.target.value))}
              />
            </label>
          </div>
        )}
        {complete.error && <div className="form-error" role="alert">{complete.error.message}</div>}
        <footer>
          <button type="button" className="ghost-button" disabled={step === 0} onClick={() => setStep((value) => value - 1)}>
            <ArrowLeft aria-hidden="true" /> Назад
          </button>
          {step < totalSteps - 1 ? (
            <button type="button" className="primary-button" disabled={!canContinue} onClick={() => setStep((value) => value + 1)}>
              Дальше <ArrowRight aria-hidden="true" />
            </button>
          ) : (
            <button type="button" className="primary-button" disabled={!canContinue || complete.isPending} onClick={() => complete.mutate()}>
              {complete.isPending ? 'Сохраняем…' : <><Check aria-hidden="true" /> Готово</>}
            </button>
          )}
        </footer>
      </section>
    </main>
  )
}

function ChoiceStep({
  title,
  subtitle,
  options,
  selected,
  onChange,
}: {
  title: string
  subtitle: string
  options: PublicSettings['onboarding']['themes']
  selected: string[]
  onChange: (value: string[]) => void
}) {
  return (
    <div>
      <div className="step-heading">
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      <div className="choice-grid">
        {options.map((option) => {
          const active = selected.includes(option.code)

          return (
            <button
              type="button"
              key={option.code}
              className={active ? 'active' : ''}
              aria-pressed={active}
              onClick={() => onChange(active ? selected.filter((item) => item !== option.code) : [...selected, option.code])}
            >
              <span className="choice-icon"><Glyph code={option.icon} /></span>
              <strong>{option.label}</strong>
              {option.description && <small>{option.description}</small>}
              {active && <Check aria-hidden="true" />}
            </button>
          )
        })}
      </div>
    </div>
  )
}

import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, CalendarDays, Check, LoaderCircle, Minus, Plus, Search, Sparkles, Ticket, X } from 'lucide-react'
import { api } from '../../api'
import { CityPhoto } from '../../components/CityPhoto'
import { ErrorState } from '../../components/States'
import { errorMessage, formatDuration, formatMoney, transportLabel } from '../../shared/format'
import { useSheet } from '../../shared/useSheet'
import { isTerminalStatus, useRecommendationStream } from '../../shared/useRecommendation'
import { useApp } from '../../state'
import type { PublicSettings, Recommendation, RecommendationOption, Trip, User } from '../../types'

const dayInMilliseconds = 86_400_000
const maximumTravellers = 8
const promptLimit = 500
const leadDays = 7
const quickPrompts = [
  { label: 'Природа', prompt: 'Хочу природу и спокойный отдых' },
  { label: 'Необычный город', prompt: 'Хочу открыть необычный город' },
  { label: 'Гастрономия', prompt: 'Хочу попробовать местную кухню' },
]

const stageLabels: Record<string, string> = {
  guardrails: 'Проверяем ограничения запроса',
  ai_classification: 'Искусственный интеллект разбирает ваш запрос',
  ai_search_plan: 'Искусственный интеллект готовит план поиска',
  mcp_transport: 'Проверяем транспорт, цены и длительность',
  backend_scoring: 'Считаем итоговый рейтинг вариантов',
  ai_explanation: 'Искусственный интеллект объясняет найденные варианты',
  finalize: 'Сохраняем рекомендации и актуальные цены',
}

function toDateInput(value: Date) {
  return value.toISOString().slice(0, 10)
}

// Названия внешних моделей и сервисов пользователю не показываем.
function stageLabel(stage: { code: string; label: string }) {
  return stageLabels[stage.code] ?? stage.label.replace(/deepseek/gi, 'Искусственный интеллект')
}

export function SearchSheet({ user, settings }: { user: User; settings: PublicSettings }) {
  const open = useApp((state) => state.searchOpen)
  const setOpen = useApp((state) => state.setSearchOpen)
  const planning = useApp((state) => state.planningTarget)
  const setPlanning = useApp((state) => state.setPlanningTarget)
  const setActiveTrip = useApp((state) => state.setActiveTrip)
  const setView = useApp((state) => state.setView)
  const setToast = useApp((state) => state.setToast)
  const queryClient = useQueryClient()

  const today = useMemo(() => new Date(), [])
  const minimumDate = toDateInput(today)
  const budgetLimits = settings.onboarding

  const [dateFrom, setDateFrom] = useState(() => toDateInput(new Date(today.getTime() + leadDays * dayInMilliseconds)))
  const [dateTo, setDateTo] = useState(() => {
    const nights = Math.max(2, user.preferences.trip_duration_days || 3)

    return toDateInput(new Date(today.getTime() + (leadDays + nights) * dayInMilliseconds))
  })
  const [budget, setBudget] = useState(user.preferences.typical_budget || budgetLimits.budget_min)
  const [modes, setModes] = useState<string[]>(user.preferences.transport_modes ?? [])
  const [directOnly, setDirectOnly] = useState(false)
  const [adults, setAdults] = useState(1)
  const [prompt, setPrompt] = useState('')
  const [recommendationID, setRecommendationID] = useState('')

  useEffect(() => {
    if (!planning?.event) {
      return
    }

    const start = new Date(planning.event.starts_at)
    const end = new Date(planning.event.ends_at)
    setDateFrom(toDateInput(new Date(Math.max(today.getTime(), start.getTime() - dayInMilliseconds))))
    setDateTo(toDateInput(new Date(end.getTime() + dayInMilliseconds)))
  }, [planning, today])

  const create = useMutation({
    mutationFn: () => {
      const body = {
        origin_city_id: user.home_city_id,
        destination_city_id: planning?.territory.id ?? '',
        event_id: planning?.event?.id ?? '',
        date_from: dateFrom,
        date_to: dateTo,
        adults,
        children: 0,
        budget,
        currency: 'RUB',
        transport_modes: modes,
        max_travel_minutes: user.preferences.max_travel_minutes || 1440,
        direct_only: directOnly,
        prompt,
      }
      const path = planning?.event ? `/events/${planning.event.id}/plan-trip` : '/recommendations'

      return api<Recommendation>(path, { method: 'POST', body: JSON.stringify(body) })
    },
    onSuccess: (result) => setRecommendationID(result.id),
  })

  const stream = useRecommendationStream(recommendationID)
  const result = stream.data
  const busy = create.isPending || Boolean(recommendationID && !isTerminalStatus(result?.status))

  const close = () => {
    setOpen(false)
    setPlanning(null)
    setRecommendationID('')
    create.reset()
  }

  const sheet = useSheet<HTMLElement>(close, open && !busy)
  const budgetOutOfRange = budget < budgetLimits.budget_min || budget > budgetLimits.budget_max
  const datesInvalid = !dateFrom || !dateTo || dateTo < dateFrom
  const submitBlocked = datesInvalid || budgetOutOfRange || modes.length === 0 || create.isPending

  if (!open) {
    return null
  }

  return (
    <div
      className="sheet-backdrop search-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !busy) {
          close()
        }
      }}
    >
      <section className="search-sheet" role="dialog" aria-modal="true" aria-label="Подбор поездки" tabIndex={-1} ref={sheet}>
        <header>
          <button className="icon-button" onClick={close} disabled={busy} aria-label={recommendationID ? 'Назад к параметрам' : 'Закрыть подбор'}>
            {recommendationID ? <ArrowLeft aria-hidden="true" /> : <X aria-hidden="true" />}
          </button>
          <div>
            <span>{planning?.event ? 'Поездка на событие' : planning ? planning.territory.name : 'Новая поездка'}</span>
            <h2>{recommendationID ? 'Подходящие варианты' : 'Куда дальше?'}</h2>
          </div>
        </header>

        {!recommendationID ? (
          <div className="search-form">
            {planning?.event && (
              <div className="selected-event">
                <CalendarDays aria-hidden="true" />
                <div><span>Вы выбрали</span><strong>{planning.event.title}</strong></div>
              </div>
            )}

            <div className="date-row">
              <label>
                Туда
                <input
                  type="date"
                  min={minimumDate}
                  value={dateFrom}
                  onChange={(event) => {
                    setDateFrom(event.target.value)
                    if (dateTo < event.target.value) {
                      setDateTo(event.target.value)
                    }
                  }}
                />
              </label>
              <span aria-hidden="true">→</span>
              <label>
                Обратно
                <input type="date" min={dateFrom || minimumDate} value={dateTo} onChange={(event) => setDateTo(event.target.value)} />
              </label>
            </div>

            <div className="form-row">
              <label>
                Путешественники
                <div className="stepper">
                  <button type="button" onClick={() => setAdults((value) => Math.max(1, value - 1))} aria-label="Меньше путешественников">
                    <Minus aria-hidden="true" />
                  </button>
                  <span aria-live="polite">{adults}</span>
                  <button
                    type="button"
                    onClick={() => setAdults((value) => Math.min(maximumTravellers, value + 1))}
                    aria-label="Больше путешественников"
                  >
                    <Plus aria-hidden="true" />
                  </button>
                </div>
              </label>
              <label>
                Бюджет на человека
                <input
                  inputMode="numeric"
                  type="number"
                  min={budgetLimits.budget_min}
                  max={budgetLimits.budget_max}
                  step={budgetLimits.budget_step}
                  value={budget}
                  onChange={(event) => setBudget(Number(event.target.value))}
                />
                <span className="input-suffix" aria-hidden="true">₽</span>
              </label>
            </div>

            {budgetOutOfRange && (
              <p className="field-hint">
                Бюджет от {budgetLimits.budget_min.toLocaleString('ru-RU')} до {budgetLimits.budget_max.toLocaleString('ru-RU')} ₽
              </p>
            )}

            <fieldset>
              <legend>Транспорт</legend>
              <div className="filter-chips">
                {settings.onboarding.transport_modes.map((mode) => {
                  const active = modes.includes(mode.code)

                  return (
                    <button
                      type="button"
                      key={mode.code}
                      className={active ? 'active' : ''}
                      aria-pressed={active}
                      onClick={() => setModes((value) => (active ? value.filter((item) => item !== mode.code) : [...value, mode.code]))}
                    >
                      {mode.label}
                      {active && <Check aria-hidden="true" />}
                    </button>
                  )
                })}
              </div>
            </fieldset>

            <div className="quick-chips">
              <button type="button" className={directOnly ? 'active' : ''} aria-pressed={directOnly} onClick={() => setDirectOnly((value) => !value)}>
                Без пересадок
              </button>
              {quickPrompts.map((item) => (
                <button type="button" key={item.label} onClick={() => setPrompt(item.prompt)}>{item.label}</button>
              ))}
            </div>

            <label className="prompt-field">
              <span>
                <Sparkles aria-hidden="true" /> Что хочется от поездки
                <em>{prompt.length}/{promptLimit}</em>
              </span>
              <textarea
                maxLength={promptLimit}
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="Например: тихий город, красивая архитектура и хорошая кухня"
              />
            </label>

            {create.error && <div className="form-error" role="alert">{errorMessage(create.error)}</div>}

            <button className="primary-button full-width search-submit" disabled={submitBlocked} onClick={() => create.mutate()}>
              {create.isPending ? <LoaderCircle className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />} Найти варианты
            </button>
            <p className="field-hint">Ничего не бронируем автоматически — сначала покажем варианты и цены.</p>
          </div>
        ) : (
          <div className="search-results">
            {busy && <SearchProgress stage={result?.stage} stages={settings.recommendation_stages} />}
            {stream.error && (
              <ErrorState
                message={stream.error}
                retry={() => {
                  setRecommendationID('')
                  create.reset()
                }}
              />
            )}
            {result && (result.status === 'failed' || result.status === 'blocked') && (
              <ErrorState
                message={failureText(result.failure_code)}
                retry={() => {
                  setRecommendationID('')
                  create.reset()
                }}
              />
            )}
            {result && (result.status === 'completed' || result.status === 'partial') && (
              <>
                {result.status === 'partial' && (
                  <div className="partial-notice">Показали только проверенные варианты: часть направлений не ответила вовремя.</div>
                )}
                {result.options.length === 0 ? (
                  <ErrorState
                    message="Под ваши условия ничего не нашлось. Попробуйте изменить даты, бюджет или транспорт."
                    retry={() => {
                      setRecommendationID('')
                      create.reset()
                    }}
                  />
                ) : (
                  <div className="result-list">
                    {result.options.map((option) => (
                      <ResultCard
                        key={option.id}
                        option={option}
                        recommendationID={result.id}
                        onSelected={(trip) => {
                          setActiveTrip(trip)
                          setToast('Поездка добавлена в «Поездки»')
                          queryClient.invalidateQueries({ queryKey: ['trips'] })
                          queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] })
                          close()
                          setView('trips')
                        }}
                      />
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </section>
    </div>
  )
}

function SearchProgress({ stage, stages }: { stage?: string; stages: PublicSettings['recommendation_stages'] }) {
  const current = stages.findIndex((item) => item.code === stage || item.label === stage)
  const active = Math.max(0, current)

  return (
    <div className="search-progress" role="status" aria-live="polite">
      <div className="ai-loader"><span /><Sparkles aria-hidden="true" /></div>
      <strong>{stages[active] ? stageLabel(stages[active]) : 'Подбираем маршрут'}</strong>
      <div aria-hidden="true">
        {stages.map((item, index) => <span key={item.code} className={index <= active ? 'active' : ''} />)}
      </div>
      <small>Шаг {active + 1} из {stages.length} · сверяем интересы, города и актуальные билеты</small>
    </div>
  )
}

function ResultCard({
  option,
  recommendationID,
  onSelected,
}: {
  option: RecommendationOption
  recommendationID: string
  onSelected: (trip: Trip) => void
}) {
  const select = useMutation({
    mutationFn: () => api<{ trip: Trip }>(`/recommendations/${recommendationID}/select`, {
      method: 'POST',
      body: JSON.stringify({ option_id: option.id }),
    }),
    onSuccess: (data) => onSelected(data.trip),
  })

  return (
    <article className="result-card">
      <div className="city-photo-frame">
        <CityPhoto name={option.city_name} size={640} />
      </div>
      <div className="result-top">
        <div className="rank-badge" aria-hidden="true">#{option.rank}</div>
        <div><h3>{option.city_name}</h3><span>{option.region}</span></div>
        <strong aria-label={`Совпадение ${option.score} процентов`}>{option.score}%</strong>
      </div>
      <p>{option.reason}</p>
      <div className="why-now"><Sparkles aria-hidden="true" /> {option.why_now}</div>
      <div className="result-details">
        <span><Ticket aria-hidden="true" /> <strong>{formatMoney(option.price_amount, option.currency)}</strong></span>
        <span>{transportLabel(option.transport_mode)} · {formatDuration(option.duration_minutes)}</span>
        <span>+{option.reward} баллов</span>
      </div>
      {select.error && <div className="form-error" role="alert">{errorMessage(select.error)}</div>}
      <button className="primary-button full-width" disabled={select.isPending} onClick={() => select.mutate()}>
        {select.isPending ? 'Добавляем…' : <>Выбрать <ArrowRight aria-hidden="true" /></>}
      </button>
    </article>
  )
}

function failureText(code?: string) {
  const messages: Record<string, string> = {
    AI_CLASSIFICATION_FAILED: 'Сервис рекомендаций временно не отвечает. Попробуйте ещё раз через минуту.',
    AI_SEARCH_PLAN_FAILED: 'Не удалось подготовить поиск. Попробуйте изменить пожелание.',
    AI_SEARCH_PLAN_EMPTY: 'Под ваши условия не нашлось направлений. Попробуйте расширить бюджет или даты.',
    AI_EXPLANATION_FAILED: 'Варианты нашлись, но описание подготовить не удалось. Повторите поиск.',
    CANDIDATE_GENERATION_FAILED: 'Не удалось подобрать города. Попробуйте ещё раз.',
    CANDIDATE_GENERATION_EMPTY: 'Подходящих городов не нашлось. Измените интересы или выберите другой город.',
    WORKFLOW_CONFIGURATION_FAILED: 'Подбор временно недоступен. Попробуйте позже.',
    WORKFLOW_CONFIGURATION_INVALID: 'Подбор временно недоступен. Попробуйте позже.',
    RECOMMENDATION_SAVE_FAILED: 'Варианты нашлись, но сохранить их не удалось. Повторите поиск.',
    BACKEND_SCORING_FAILED: 'Не удалось рассчитать рейтинг вариантов. Повторите поиск.',
    RECOMMENDATION_WORKFLOW_PANIC: 'Подбор прервался на нашей стороне. Запустите поиск заново.',
    MCP_TRANSPORT_SEARCH_FAILED: 'Не удалось получить актуальные билеты. Попробуйте позже.',
    MCP_TRANSPORT_SEARCH_EMPTY: 'На выбранные даты билетов не нашлось. Попробуйте другие даты.',
    PROMPT_NOT_TRAVEL_RELATED: 'Опишите пожелание к поездке — так подбор будет точнее.',
    PROMPT_CONTAINS_PII: 'Уберите из пожелания контакты и личные данные.',
    PROMPT_INJECTION_DETECTED: 'В пожелании есть служебные инструкции — переформулируйте запрос.',
    POLITICAL_REQUEST_BLOCKED: 'Помогаем только с выбором и проверкой путешествий.',
    STAGE_UPDATE_FAILED: 'Подбор прервался на нашей стороне. Запустите поиск заново.',
  }

  return messages[code ?? ''] ?? 'Не удалось подобрать варианты. Попробуйте изменить параметры.'
}

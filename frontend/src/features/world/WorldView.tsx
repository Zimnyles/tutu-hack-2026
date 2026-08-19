import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Coins, Compass, Flame, LocateFixed, MapPin, Sparkles, TrendingUp, X } from 'lucide-react'
import { api } from '../../api'
import { useApp } from '../../state'
import { CityPhoto } from '../../components/CityPhoto'
import type { Bootstrap, PopularEventsResponse, Recommendation, RecommendationOption, Territory } from '../../types'
import { formatDate, formatDuration, formatMoney } from '../../shared/format'
import { useMediaQuery } from '../../shared/useMediaQuery'
import { isTerminalStatus, useRecommendationStream } from '../../shared/useRecommendation'
import { TerritorySheet } from './TerritorySheet'

const Globe = lazy(() => import('../../components/Globe').then((module) => ({ default: module.Globe })))

const stateLabels: Record<Territory['state'], string> = {
  locked: 'Не открыто',
  suggested: 'Совет',
  planned: 'Запланировано',
  arrived: 'Открыто',
}

function unavailableReason(code?: string) {
  if (code === 'MCP_TRANSPORT_SEARCH_EMPTY') {
    return 'Не нашли билеты под ваши фильтры на ближайшие даты. Попробуйте поднять бюджет или добавить транспорт в настройках.'
  }

  if (code === 'CANDIDATE_GENERATION_EMPTY') {
    return 'Города по вашим интересам уже открыты. Добавьте новые интересы в настройках профиля.'
  }

  return 'Обновите подбор или задайте параметры вручную — ваши настройки уже подставлены.'
}

export function WorldView({ data }: { data: Bootstrap }) {
  const queryClient = useQueryClient()
  const selected = useApp((state) => state.selected)
  const setSelected = useApp((state) => state.setSelected)
  const setPlanningTarget = useApp((state) => state.setPlanningTarget)
  const setSearchOpen = useApp((state) => state.setSearchOpen)
  const [listOpen, setListOpen] = useState(false)
  const reduceMotion = useMediaQuery('(prefers-reduced-motion: reduce)')

  const arrived = useMemo(
    () => data.territories.filter((city) => city.state === 'arrived').length,
    [data.territories],
  )
  const percent = data.territories.length ? Math.round((arrived / data.territories.length) * 100) : 0

  const promoByCity = useMemo(
    () => Object.fromEntries(data.territories.map((city) => [city.id, city.promo_percent])),
    [data.territories],
  )

  const personal = data.personal_recommendation
  const streamID = personal && !isTerminalStatus(personal.status) ? personal.id : ''
  const stream = useRecommendationStream(streamID)
  const recommendation = stream.data ?? personal

  useEffect(() => {
    if (stream.data && isTerminalStatus(stream.data.status)) {
      queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] })
    }
  }, [queryClient, stream.data])

  const refresh = useMutation({
    mutationFn: () => api<Recommendation>('/recommendations/personal/refresh', { method: 'POST' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] }),
  })

  const handleSelect = useCallback((territory: Territory) => setSelected(territory), [setSelected])

  return (
    <div className="world-screen">
      <section className="world-map-panel">
        <header className="mobile-topbar">
          <img className="mobile-brand" src="/brand/tutu-white.svg" alt="Туту" />
          <span className="mobile-balance"><Coins aria-hidden="true" />{data.balance}</span>
        </header>
        <div className="world-status">
          <div>
            <span>Ваш мир</span>
            <strong>{percent}%</strong>
            <small>{arrived} из {data.territories.length} городов</small>
          </div>
          <div className="status-balance">
            <Coins aria-hidden="true" />
            <span>{data.balance} баллов</span>
          </div>
        </div>
        <Suspense fallback={<div className="globe-skeleton"><span /></div>}>
          <Globe
            territories={data.territories}
            onSelect={handleSelect}
            homeCityID={data.user.home_city_id}
            reduceMotion={reduceMotion}
          />
        </Suspense>
        <div className="map-controls">
          <button onClick={() => setListOpen((value) => !value)} aria-expanded={listOpen} aria-controls="city-list">
            <MapPin aria-hidden="true" /> <span>Города</span>
          </button>
          <button
            onClick={() => {
              const home = data.territories.find((city) => city.id === data.user.home_city_id)
              if (home) {
                setSelected(home)
              }
            }}
            aria-label="Открыть домашний город"
          >
            <LocateFixed aria-hidden="true" />
          </button>
        </div>
        {listOpen && (
          <div className="city-list-popover" id="city-list">
            <div>
              <strong>Города</strong>
              <button onClick={() => setListOpen(false)} aria-label="Закрыть список городов"><X aria-hidden="true" /></button>
            </div>
            {data.territories.map((city) => (
              <button
                key={city.id}
                onClick={() => {
                  setSelected(city)
                  setListOpen(false)
                }}
              >
                <span className={`state-dot ${city.popular_event ? 'has-events' : city.state}`} aria-hidden="true" />
                <span>
                  <strong>{city.name}</strong>
                  <small>{[city.region, ...city.badges.slice(0, 2)].join(' · ')}</small>
                </span>
                <em className={city.popular_event ? 'has-events' : undefined}>
                  {city.popular_event ? 'Популярное событие' : stateLabels[city.state]}
                </em>
              </button>
            ))}
          </div>
        )}
        <button className="floating-search" onClick={() => setSearchOpen(true)}>
          <Compass aria-hidden="true" /><span>Куда дальше?</span>
        </button>
      </section>

      <aside className="discovery-panel">
        <div className="panel-heading">
          <div><span className="eyebrow">Подобрано для вас</span><h1>Следующее открытие</h1></div>
          <Sparkles aria-hidden="true" />
        </div>
        <button className="panel-search" onClick={() => setSearchOpen(true)}>
          <Compass aria-hidden="true" /> <span>Подобрать поездку</span>
        </button>
        <PersonalRecommendation
          recommendation={recommendation}
          promoByCity={promoByCity}
          refreshing={refresh.isPending}
          onRefresh={() => refresh.mutate()}
          onOpenSearch={() => setSearchOpen(true)}
          onOpenOption={(option) => {
            const city = data.territories.find((item) => item.id === option.city_id)
            if (city) {
              setSelected(city)
              setPlanningTarget({ territory: city })
            }
          }}
        />
        <PopularEvents
          onOpenCity={(cityID) => {
            const city = data.territories.find((item) => item.id === cityID)
            if (city) {
              setSelected(city)
            }
          }}
        />
        <div className="season-mini">
          <div>
            <TrendingUp aria-hidden="true" />
            <span><strong>{data.season.user_score}</strong> очков сезона</span>
          </div>
          <div className="progress-track">
            <span style={{ width: `${Math.min(100, (data.season.user_score / Math.max(1, data.season.next_league_score)) * 100)}%` }} />
          </div>
          <small>{data.season.league} · до следующей лиги {Math.max(0, data.season.next_league_score - data.season.user_score)}</small>
        </div>
      </aside>

      {selected && <TerritorySheet territory={selected} onClose={() => setSelected(null)} />}
    </div>
  )
}

function PopularEvents({ onOpenCity }: { onOpenCity: (cityID: string) => void }) {
  const popular = useQuery({
    queryKey: ['popular-events'],
    queryFn: () => api<PopularEventsResponse>('/events/popular'),
    refetchInterval: (query) => (query.state.data?.discovering ? 8000 : false),
    staleTime: 5 * 60 * 1000,
  })

  const items = popular.data?.items ?? []

  if (items.length === 0 && !popular.data?.discovering) {
    return null
  }

  return (
    <section className="popular-events" aria-label="Популярные события России">
      <header>
        <Flame aria-hidden="true" />
        <strong>Популярно в России</strong>
      </header>
      {items.length === 0 ? (
        <p className="popular-events-empty" role="status" aria-live="polite">
          Нейросеть подбирает главные события месяца.
        </p>
      ) : (
        <ul>
          {items.slice(0, 6).map((event) => (
            <li key={event.id}>
              <button onClick={() => onOpenCity(event.city_id)}>
                <span className="popular-date">{formatDate(event.starts_at)}</span>
                <span>
                  <strong>{event.title}</strong>
                  <small>{event.city_name}</small>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function PersonalRecommendation({
  recommendation,
  promoByCity,
  refreshing,
  onRefresh,
  onOpenSearch,
  onOpenOption,
}: {
  recommendation?: Recommendation | null
  promoByCity: Record<string, number>
  refreshing: boolean
  onRefresh: () => void
  onOpenSearch: () => void
  onOpenOption: (option: RecommendationOption) => void
}) {
  if (refreshing || !recommendation || !isTerminalStatus(recommendation.status)) {
    return (
      <div className="recommendation-loading" role="status" aria-live="polite">
        <span className="search-orbit" aria-hidden="true" />
        <strong>Ищем подходящие маршруты</strong>
        <small>Учитываем ваши интересы и проверяем билеты. Это занимает до минуты.</small>
      </div>
    )
  }

  if (recommendation.options.length === 0) {
    return (
      <div className="recommendation-unavailable">
        <strong>Пока нет готовых вариантов</strong>
        <p>{unavailableReason(recommendation.failure_code)}</p>
        <button className="secondary-button" onClick={onRefresh}>Обновить подбор</button>
        <button className="ghost-button" onClick={onOpenSearch}>Задать параметры вручную</button>
      </div>
    )
  }

  return (
    <div className="recommendation-stack">
      {recommendation.options.map((option, index) => (
        <RecommendationCard
          key={option.id}
          option={option}
          featured={index === 0}
          promoPercent={promoByCity[option.city_id] ?? 0}
          onOpen={() => onOpenOption(option)}
        />
      ))}
    </div>
  )
}

function RecommendationCard({
  option,
  featured,
  promoPercent,
  onOpen,
}: {
  option: RecommendationOption
  featured: boolean
  promoPercent: number
  onOpen: () => void
}) {
  return (
    <article className={`recommendation-card ${featured ? 'featured' : ''}`}>
      <div className="city-photo-frame">
        <CityPhoto name={option.city_name} size={640} />
      </div>
      <div className="recommendation-rank">{featured ? 'Лучший вариант' : `№ ${option.rank}`}</div>
      <div className="recommendation-city">
        <div><strong>{option.city_name}</strong><span>{option.region}</span></div>
        <span className="score" aria-label={`Совпадение ${option.score} процентов`}>{option.score}%</span>
      </div>
      <p>{option.reason}</p>
      <div className="recommendation-meta">
        <span>{formatMoney(option.price_amount, option.currency)}</span>
        <span>{formatDuration(option.duration_minutes)}</span>
        <span>+{option.reward} баллов</span>
        {promoPercent > 0 && <span className="promo-chip">промокод −{promoPercent}%</span>}
      </div>
      <button onClick={onOpen}>Посмотреть {option.city_name} <span aria-hidden="true">→</span></button>
    </article>
  )
}

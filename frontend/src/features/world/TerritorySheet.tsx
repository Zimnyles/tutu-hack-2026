import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowRight, Clock, ExternalLink, MapPin, Sparkles, TicketPercent, TrainFront, Users, X } from 'lucide-react'
import { api } from '../../api'
import { CityBadges } from '../../components/CityBadges'
import { CityPhoto } from '../../components/CityPhoto'
import { EmptyState, ErrorState, LoadingState } from '../../components/States'
import { contentLabel, eventAvailability, formatDate, formatMoney } from '../../shared/format'
import { useSheet } from '../../shared/useSheet'
import { useApp } from '../../state'
import type { CityEventsResponse, TravelCohort, Territory } from '../../types'

type Tab = 'overview' | 'events' | 'route' | 'community'

const tabs: Array<[Tab, string]> = [
  ['events', 'События'],
  ['overview', 'Обзор'],
  ['route', 'Маршрут'],
  ['community', 'Люди'],
]

const stateLabels: Record<Territory['state'], string> = {
  locked: 'Можно открыть',
  suggested: 'Рекомендуем',
  planned: 'В планах',
  arrived: 'Открыто',
}

export function TerritorySheet({ territory, onClose }: { territory: Territory; onClose: () => void }) {
  const [tab, setTab] = useState<Tab>('events')
  const setPlanningTarget = useApp((state) => state.setPlanningTarget)
  const sheet = useSheet<HTMLElement>(onClose)

  const events = useQuery({
    queryKey: ['events', territory.id],
    queryFn: () => api<CityEventsResponse>(`/territories/${territory.id}/events`),
    enabled: tab === 'events',
    refetchInterval: (query) => (query.state.data?.discovering ? 5000 : false),
  })
  const discovering = Boolean(events.data?.discovering)
  const cohort = useQuery({
    queryKey: ['cohort', territory.id],
    queryFn: () => api<{ cohort: TravelCohort }>(`/travel-cohorts/${territory.id}`),
    enabled: tab === 'community',
  })

  return (
    <div
      className="sheet-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <section className="territory-sheet" role="dialog" aria-modal="true" aria-label={territory.name} tabIndex={-1} ref={sheet}>
        <div className="sheet-handle" aria-hidden="true" />
        <header className={`territory-hero tone-${territory.image_tone}`}>
          <CityPhoto
            name={territory.name}
            className="territory-hero-photo"
            overlayClassName="territory-hero-scrim"
            size={1200}
          />
          <button className="sheet-close" onClick={onClose} aria-label="Закрыть город"><X aria-hidden="true" /></button>
          <div className="territory-state">{stateLabels[territory.state]}</div>
          <div>
            <span>{territory.region}</span>
            <h2>{territory.name}</h2>
            <div className="territory-tags">
              {(territory.badges.length > 0 ? territory.badges.slice(0, 3) : territory.tags.slice(0, 3).map(contentLabel))
                .map((tag) => <span key={tag}>{tag}</span>)}
            </div>
          </div>
        </header>

        <div className="sheet-tabs" role="tablist" aria-label="Разделы города">
          {tabs.map(([id, label]) => (
            <button
              key={id}
              role="tab"
              id={`tab-${id}`}
              aria-selected={tab === id}
              aria-controls={`panel-${id}`}
              className={tab === id ? 'active' : ''}
              onClick={() => setTab(id)}
            >
              {label}
            </button>
          ))}
        </div>

        <div className="sheet-content" role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`}>
          {tab === 'overview' && (
            <div className="city-overview">
              <p>{territory.description}</p>
              {territory.badges.length > 0 && (
                <section className="badge-block" aria-label="Чем известен город">
                  <h3>Чем известен город</h3>
                  <CityBadges badges={territory.badges} />
                </section>
              )}
              <div className="city-facts">
                <div><Sparkles aria-hidden="true" /><span><strong>+{territory.reward}</strong> баллов за открытие</span></div>
                <div><MapPin aria-hidden="true" /><span><strong>{territory.rarity}/10</strong> редкость города</span></div>
              </div>
              {territory.promo_percent > 0 && (
                <div className="promo-reward">
                  <TicketPercent aria-hidden="true" />
                  <div>
                    <strong>Промокод −{territory.promo_percent}% на билеты Туту</strong>
                    <small>Придёт вместе с баллами, когда отметите, что приехали. Промокод ждёт в профиле 90 дней.</small>
                  </div>
                </div>
              )}
              <button className="primary-button full-width" onClick={() => setPlanningTarget({ territory })}>
                Собрать поездку <ArrowRight aria-hidden="true" />
              </button>
            </div>
          )}

          {tab === 'events' && (
            <div>
              {events.isLoading && <LoadingState label="Загружаем афишу" />}
              {events.isError && <ErrorState message={events.error.message} retry={() => events.refetch()} />}
              {discovering && (
                <div className="events-searching" role="status" aria-live="polite">
                  <span className="search-orbit" aria-hidden="true" />
                  <strong>Ищем мероприятия в {territory.name}</strong>
                  <small>Нейросеть смотрит афишу площадок и билетных сервисов. Это занимает до минуты.</small>
                </div>
              )}
              {!discovering && events.data?.items.length === 0 && (
                <EmptyState title="Событий пока нет" text="Попробуйте выбрать другие даты позже" />
              )}
              {events.data?.items.map((event) => (
                <article className="event-card" key={event.id}>
                  <div className="event-date">
                    <strong>{formatDate(event.starts_at, { day: 'numeric' })}</strong>
                    <span>{formatDate(event.starts_at, { month: 'short' })}</span>
                  </div>
                  <div className="event-body">
                    <div className="event-badges">
                      <span>{contentLabel(event.category)}</span>
                      {eventAvailability(event.availability) && event.availability !== 'available' && (
                        <span className="warning">{eventAvailability(event.availability)}</span>
                      )}
                    </div>
                    <h3>{event.title}</h3>
                    <p>
                      <Clock aria-hidden="true" /> {formatDate(event.starts_at, { hour: '2-digit', minute: '2-digit' })} · {event.venue_name}
                    </p>
                    {event.source_url && (
                      <a className="event-source" href={event.source_url} target="_blank" rel="noreferrer noopener">
                        Источник <ExternalLink aria-hidden="true" />
                      </a>
                    )}
                    <div>
                      <strong>{formatMoney(event.price_from, event.currency)}</strong>
                      <button onClick={() => setPlanningTarget({ territory, event })}>Собрать поездку</button>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          )}

          {tab === 'route' && (
            <div className="route-tab">
              <TrainFront aria-hidden="true" />
              <h3>Маршрут до {territory.name}</h3>
              <p>Проверим актуальные билеты и покажем лучшие варианты по времени и цене.</p>
              <button className="primary-button full-width" onClick={() => setPlanningTarget({ territory })}>Найти билеты</button>
            </div>
          )}

          {tab === 'community' && (
            <div>
              {cohort.isLoading && <LoadingState />}
              {cohort.isError && <ErrorState message={cohort.error.message} retry={() => cohort.refetch()} />}
              {cohort.data && (
                <div className="cohort-card">
                  <Users aria-hidden="true" />
                  {cohort.data.cohort.visible ? (
                    <>
                      <strong>{cohort.data.cohort.count} исследователей</strong>
                      <p>{cohort.data.cohort.from_guild} из вашей гильдии собираются сюда в ближайшее время.</p>
                    </>
                  ) : (
                    <>
                      <strong>Небольшая группа</strong>
                      <p>{cohort.data.cohort.message || 'Данные появятся, когда наберётся безопасное число участников.'}</p>
                    </>
                  )}
                  <span>Личные данные других пользователей не показываем</span>
                </div>
              )}
            </div>
          )}
        </div>
      </section>
    </div>
  )
}

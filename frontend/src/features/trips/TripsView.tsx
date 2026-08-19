import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarDays, CheckCircle2, ExternalLink, MapPin, Navigation, Ticket } from 'lucide-react'
import { api } from '../../api'
import { EmptyState, ErrorState, LoadingState } from '../../components/States'
import { errorMessage, formatDate, formatMoney, transportLabel } from '../../shared/format'
import { useApp } from '../../state'
import type { Trip } from '../../types'

const statusLabels: Record<string, string> = {
  planned: 'Запланировано',
  checkout_created: 'Билеты выбраны',
  departed: 'В пути',
  arrived: 'Открыто',
  cancelled: 'Отменено',
}

export function TripsView() {
  const setSearchOpen = useApp((state) => state.setSearchOpen)
  const trips = useQuery({ queryKey: ['trips'], queryFn: () => api<{ items: Trip[] }>('/trips') })

  return (
    <div className="content-page">
      <header className="page-header">
        <div><span className="eyebrow">Ваши маршруты</span><h1>Поездки</h1></div>
        <button className="primary-button desktop-action" onClick={() => setSearchOpen(true)}>Новая поездка</button>
      </header>
      {trips.isLoading && <LoadingState label="Загружаем поездки" />}
      {trips.isError && <ErrorState message={trips.error.message} retry={() => trips.refetch()} />}
      {trips.data?.items.length === 0 && <EmptyState title="Поездок пока нет" text="Подберите направление — оно появится здесь" />}
      {trips.data && trips.data.items.length > 0 && (
        <div className="trips-grid">
          {trips.data.items.map((trip) => <TripCard key={trip.id} trip={trip} />)}
        </div>
      )}
      <button className="mobile-fab" onClick={() => setSearchOpen(true)}>+ Новая поездка</button>
    </div>
  )
}

function TripCard({ trip }: { trip: Trip }) {
  const queryClient = useQueryClient()
  const setToast = useApp((state) => state.setToast)
  const [confirmingArrival, setConfirmingArrival] = useState(false)
  const arrivalKey = useMemo(() => `${trip.id}:${crypto.randomUUID()}`, [trip.id])

  const checkout = useMutation({
    mutationFn: () => api<{ checkout_url: string }>(`/trips/${trip.id}/checkout`, { method: 'POST' }),
    onSuccess: ({ checkout_url }) => {
      queryClient.invalidateQueries({ queryKey: ['trips'] })

      if (!checkout_url) {
        return
      }

      const opened = window.open(checkout_url, '_blank', 'noopener,noreferrer')
      if (!opened) {
        window.location.assign(checkout_url)
      }
    },
  })

  const arrival = useMutation({
    mutationFn: () => api(`/trips/${trip.id}/simulate-arrival`, {
      method: 'POST',
      headers: { 'Idempotency-Key': arrivalKey },
    }),
    onSuccess: () => {
      setConfirmingArrival(false)
      setToast('Город открыт — награда начислена')
      queryClient.invalidateQueries({ queryKey: ['trips'] })
      queryClient.invalidateQueries({ queryKey: ['world-bootstrap'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      queryClient.invalidateQueries({ queryKey: ['achievements'] })
      queryClient.invalidateQueries({ queryKey: ['promo-codes'] })
    },
  })

  const arrived = trip.status === 'arrived'

  return (
    <article className={`trip-card ${arrived ? 'arrived' : ''}`}>
      <div className="trip-visual" aria-hidden="true">
        <span>{arrived ? <CheckCircle2 /> : <Navigation />}</span>
        <div className="route-line"><i /><i /></div>
      </div>
      <div className="trip-card-body">
        <header className="trip-card-head">
          <div>
            <h2>{trip.option.city_name}</h2>
            <p>{trip.option.region}</p>
          </div>
          <span className={`trip-status ${trip.status}`}>{statusLabels[trip.status] ?? trip.status}</span>
        </header>
        <div className="trip-info">
          <span><CalendarDays aria-hidden="true" /> {formatDate(trip.depart_at)} — {formatDate(trip.arrive_at)}</span>
          <span><Ticket aria-hidden="true" /> {transportLabel(trip.option.transport_mode)}</span>
        </div>
        <div className="trip-price">
          <strong>{formatMoney(trip.option.price_amount, trip.option.currency)}</strong>
          <small>{arrived ? 'поездка завершена' : 'за выбранный вариант'}</small>
        </div>
        {checkout.error && <div className="form-error" role="alert">{errorMessage(checkout.error)}</div>}
        {arrival.error && <div className="form-error" role="alert">{errorMessage(arrival.error)}</div>}
        {confirmingArrival ? (
          <div className="confirm-box">
            <p>Отметить город открытым? Действие необратимо: начислим баллы и закрасим территорию.</p>
            <div className="trip-actions">
              <button className="primary-button" onClick={() => arrival.mutate()} disabled={arrival.isPending}>
                {arrival.isPending ? 'Отмечаем…' : 'Да, я приехал'}
              </button>
              <button className="ghost-button" onClick={() => setConfirmingArrival(false)} disabled={arrival.isPending}>
                Отмена
              </button>
            </div>
          </div>
        ) : (
          !arrived && (
            <div className="trip-actions">
              <button className="primary-button" onClick={() => checkout.mutate()} disabled={checkout.isPending}>
                {checkout.isPending ? 'Готовим…' : <>К билетам на Туту <ExternalLink aria-hidden="true" /></>}
              </button>
              <button className="ghost-button" onClick={() => setConfirmingArrival(true)}>
                <MapPin aria-hidden="true" /> Я приехал
              </button>
            </div>
          )
        )}
        {!arrived && !confirmingArrival && (
          <p className="field-hint">Билеты оформляются на сайте Туту, откроется в новой вкладке.</p>
        )}
      </div>
    </article>
  )
}

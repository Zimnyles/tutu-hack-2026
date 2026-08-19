import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, ArrowLeft, CheckCircle2, Database, Play, RefreshCw, Search, Users, XCircle } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { ErrorState, LoadingState } from '../../components/States'
import { errorMessage, formatDate } from '../../shared/format'
import type { AdminAuditEntry, AdminOverview, AdminSimulation, AdminUser, DemoScenario, User } from '../../types'

const minimumReasonLength = 12
const maximumReasonLength = 500

const actionLabels: Record<string, string> = {
  demo_sync_history: 'Синхронизировать demo-историю',
  recommendation_complete: 'Завершить подбор рекомендаций',
  trip_checkout_created: 'Отметить оформление билетов',
  trip_departed: 'Отметить отправление',
  trip_arrived: 'Отметить прибытие',
  trip_cancelled: 'Отменить поездку',
  event_set_availability: 'Изменить доступность события',
  event_cancel: 'Отменить событие',
  cohort_set_demo_size: 'Задать размер demo-группы',
  guild_join: 'Добавить в гильдию',
  leaderboard_rebuild: 'Пересобрать лидерборд',
  outbox_process: 'Обработать очередь событий',
  demo_profile_reset: 'Сбросить demo-профиль',
}

export function AdminView({ user }: { user: User }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState('')
  const [selectedScenario, setSelectedScenario] = useState('')
  const [action, setAction] = useState('')
  const [reason, setReason] = useState('Проверка пользовательского сценария')

  const overview = useQuery({ queryKey: ['admin-overview'], queryFn: () => api<AdminOverview>('/admin/overview') })
  const users = useQuery({ queryKey: ['admin-users', query], queryFn: () => api<{ items: AdminUser[] }>(`/admin/users?query=${encodeURIComponent(query)}`) })
  const scenarios = useQuery({ queryKey: ['admin-scenarios'], queryFn: () => api<{ items: DemoScenario[] }>('/admin/scenarios') })
  const audit = useQuery({ queryKey: ['admin-audit'], queryFn: () => api<{ items: AdminAuditEntry[] }>('/admin/audit-log') })

  const availableActions = useMemo(() => overview.data?.available_actions ?? [], [overview.data])
  const selectedAction = action || availableActions[0] || ''

  const execute = useMutation({
    mutationFn: () => api<AdminSimulation>('/admin/simulations', {
      method: 'POST',
      headers: { 'Idempotency-Key': crypto.randomUUID() },
      body: JSON.stringify({
        action_code: selectedAction,
        target_type: 'user',
        target_id: selectedUser,
        demo_scenario_id: selectedScenario,
        reason,
        parameters: {},
      }),
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-audit'] })
      queryClient.invalidateQueries({ queryKey: ['admin-overview'] })
    },
  })

  const reasonTooShort = reason.trim().length < minimumReasonLength
  const submitBlocked = !selectedUser || !selectedAction || reasonTooShort || execute.isPending

  return (
    <main className="admin-page">
      <header className="admin-topbar">
        <button onClick={() => navigate('/')}><ArrowLeft aria-hidden="true" /> <span>В приложение</span></button>
        <div><span>Системное управление</span><strong>Открывай</strong></div>
        <span className="admin-user" aria-hidden="true">{user.display_name.slice(0, 1).toUpperCase()}</span>
      </header>

      <div className="admin-content">
        <div className="page-header">
          <div><span className="eyebrow">Управление системой</span><h1>Симулятор действий</h1></div>
          <button
            className="secondary-button"
            onClick={() => {
              overview.refetch()
              audit.refetch()
            }}
          >
            <RefreshCw aria-hidden="true" /> Обновить
          </button>
        </div>

        {overview.isLoading && <LoadingState />}
        {overview.isError && <ErrorState message={overview.error.message} retry={() => overview.refetch()} />}
        {overview.data && (
          <div className="admin-metrics">
            <Metric icon={Database} label="База данных" value={overview.data.database_ready ? 'Готова' : 'Недоступна'} good={overview.data.database_ready} />
            <Metric icon={Users} label="Пользователи" value={String(overview.data.demo_users)} />
            <Metric icon={Activity} label="В очереди" value={String(overview.data.pending_outbox)} />
            <Metric icon={XCircle} label="Ошибки" value={String(overview.data.failed_actions)} good={overview.data.failed_actions === 0} />
          </div>
        )}

        {overview.data && !overview.data.simulator_enabled && (
          <div className="form-error" role="alert">Симулятор выключен в конфигурации: действия выполняться не будут.</div>
        )}

        <div className="admin-grid">
          <section className="surface admin-command">
            <h2>Новое действие</h2>
            <label>
              Действие
              <select value={selectedAction} onChange={(event) => setAction(event.target.value)}>
                {availableActions.map((code) => (
                  <option key={code} value={code}>{actionLabels[code] ?? code}</option>
                ))}
              </select>
            </label>
            <label>
              Пользователь
              <div className="search-input">
                <Search aria-hidden="true" />
                <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Имя или email" />
              </div>
              <select value={selectedUser} onChange={(event) => setSelectedUser(event.target.value)}>
                <option value="">Выберите пользователя</option>
                {users.data?.items.map((item) => (
                  <option key={item.id} value={item.id}>{item.display_name} · {item.email}</option>
                ))}
              </select>
            </label>
            <label>
              Сценарий <small>необязательно</small>
              <select value={selectedScenario} onChange={(event) => setSelectedScenario(event.target.value)}>
                <option value="">Без сценария</option>
                {scenarios.data?.items.filter((item) => item.enabled).map((item) => (
                  <option key={item.id} value={item.id}>{item.name}</option>
                ))}
              </select>
            </label>
            <label>
              Причина <em>{reason.trim().length}/{maximumReasonLength}</em>
              <textarea
                value={reason}
                maxLength={maximumReasonLength}
                onChange={(event) => setReason(event.target.value)}
              />
            </label>
            {reasonTooShort && <p className="field-hint">Опишите причину минимум {minimumReasonLength} символами — она попадёт в журнал.</p>}
            {execute.error && <div className="form-error" role="alert">{errorMessage(execute.error)}</div>}
            {execute.data && <div className="success-notice" role="status"><CheckCircle2 aria-hidden="true" /> Команда принята: {execute.data.status}</div>}
            <button className="primary-button full-width" disabled={submitBlocked} onClick={() => execute.mutate()}>
              <Play aria-hidden="true" /> Выполнить
            </button>
          </section>

          <section className="surface audit-section">
            <h2>Журнал действий</h2>
            {audit.isLoading && <LoadingState />}
            {audit.isError && <ErrorState message={audit.error.message} retry={() => audit.refetch()} />}
            {audit.data?.items.length === 0 && <p className="field-hint">Действий пока не было.</p>}
            {audit.data?.items.map((entry) => (
              <div className="audit-row" key={entry.id}>
                <span className={`audit-status ${entry.outcome}`} aria-hidden="true" />
                <div>
                  <strong>{actionLabels[entry.action_code] ?? entry.action_code}</strong>
                  <small>{entry.reason_code || entry.target_type}</small>
                </div>
                <time>{formatDate(entry.created_at, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</time>
              </div>
            ))}
          </section>
        </div>
      </div>
    </main>
  )
}

function Metric({ icon: Icon, label, value, good }: { icon: typeof Database; label: string; value: string; good?: boolean }) {
  return (
    <div className="admin-metric">
      <span className={good ? 'good' : ''}><Icon aria-hidden="true" /></span>
      <div><small>{label}</small><strong>{value}</strong></div>
    </div>
  )
}

import { Fragment, useState, type CSSProperties } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Award, Crown, Globe, Shield, Sparkles, Trophy, Users, type LucideIcon } from 'lucide-react'
import { api } from '../../api'
import { EmptyState, ErrorState, LoadingState } from '../../components/States'
import { errorMessage, formatAgo, percentage } from '../../shared/format'
import type { Guild, Leaderboard, Season } from '../../types'

type LeaderboardScope = 'league' | 'guild' | 'global'
type LeaderboardPeriod = 'month' | 'season'

const podiumSize = 3
const podiumOrder = [2, 1, 3]
const scopeTabs: Array<[LeaderboardScope, string, LucideIcon]> = [
  ['league', 'Лига', Trophy],
  ['guild', 'Гильдия', Shield],
  ['global', 'Весь мир', Globe],
]
const periodTabs: Array<[LeaderboardPeriod, string]> = [
  ['month', 'Месяц'],
  ['season', 'Сезон'],
]

export function CommunityView({ season }: { season: Season }) {
  const guild = useQuery({ queryKey: ['guild'], queryFn: () => api<{ guild: Guild }>('/guild') })
  const toNextLeague = Math.max(0, season.next_league_score - season.user_score)

  return (
    <div className="content-page community-page">
      <header className="page-header">
        <div><span className="eyebrow">{season.month_title}</span><h1>Сезон и гильдия</h1></div>
        <div className="league-pill"><Trophy aria-hidden="true" /> {season.league}</div>
      </header>

      <section className="season-hero">
        <div>
          <span>Ваш результат</span>
          <strong>{season.user_score.toLocaleString('ru-RU')}</strong>
          <small>Вы выше {season.percentile}% участников</small>
        </div>
        <div
          className="season-ring"
          style={{ '--progress': `${percentage(season.user_score, season.next_league_score) * 3.6}deg` } as CSSProperties}
        >
          <span>
            <Crown aria-hidden="true" />
            <strong>{toNextLeague.toLocaleString('ru-RU')}</strong>
            <small>до лиги</small>
          </span>
        </div>
      </section>

      <div className="community-grid">
        <section className="surface guild-section">
          {guild.isLoading && <LoadingState />}
          {guild.isError && <ErrorState message={guild.error.message} retry={() => guild.refetch()} />}
          {guild.data && <GuildCard guild={guild.data.guild} />}
        </section>

        <LeaderboardSection season={season} />
      </div>
    </div>
  )
}

function LeaderboardSection({ season }: { season: Season }) {
  const [scope, setScope] = useState<LeaderboardScope>('league')
  const [period, setPeriod] = useState<LeaderboardPeriod>('month')
  const leaderboard = useQuery({
    queryKey: ['leaderboard', scope, period],
    queryFn: () => api<Leaderboard>(`/leaderboard?scope=${scope}&period=${period}`),
    placeholderData: (previous) => previous,
  })

  const items = leaderboard.data?.items ?? []
  const podium = items.filter((item) => item.rank <= podiumSize)
  const rest = items.filter((item) => item.rank > podiumSize)
  const me = items.find((item) => item.me)
  const chaser = me ? items.filter((item) => item.rank < me.rank).at(-1) : undefined
  const generatedAt = leaderboard.data?.generated_at

  return (
    <section className="surface leaderboard-section">
      <div className="section-title">
        <div>
          <span className="section-icon orange"><Award aria-hidden="true" /></span>
          <div>
            <h2>Рейтинг</h2>
            <p>{generatedAt ? `обновлён ${formatAgo(generatedAt)}` : 'считаем позиции'}</p>
          </div>
        </div>
      </div>

      <div className="leaderboard-filters">
        <div className="scope-tabs" role="group" aria-label="Масштаб рейтинга">
          {scopeTabs.map(([code, label, Icon]) => (
            <button
              key={code}
              className={scope === code ? 'active' : ''}
              aria-pressed={scope === code}
              onClick={() => setScope(code)}
            >
              <Icon aria-hidden="true" /> <span>{label}</span>
            </button>
          ))}
        </div>
        <div className="period-switch" role="group" aria-label="Период рейтинга">
          {periodTabs.map(([code, label]) => (
            <button
              key={code}
              className={period === code ? 'active' : ''}
              aria-pressed={period === code}
              onClick={() => setPeriod(code)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {leaderboard.isLoading && <LoadingState />}
      {leaderboard.isError && <ErrorState message={leaderboard.error.message} retry={() => leaderboard.refetch()} />}
      {leaderboard.data && items.length === 0 && (
        <EmptyState title="Рейтинг ещё пуст" text="Позиции появятся, когда участники наберут первые очки в этом периоде" />
      )}

      {items.length > 0 && (
        <>
          {podium.length > 0 && (
            <ol className="leaderboard-podium">
              {podiumOrder
                .map((rank) => podium.find((item) => item.rank === rank))
                .filter((item) => item !== undefined)
                .map((item) => (
                  <li key={item.rank} className={`place-${item.rank} ${item.me ? 'me' : ''}`}>
                    <span className="podium-avatar" aria-hidden="true">{item.nickname.slice(0, 1).toUpperCase()}</span>
                    <strong>{item.nickname}</strong>
                    <em>{item.score.toLocaleString('ru-RU')}</em>
                    <span className="podium-place">{item.rank === 1 ? <Crown aria-hidden="true" /> : item.rank}</span>
                  </li>
                ))}
            </ol>
          )}

          {rest.length > 0 && (
            <ol className="leaderboard-list">
              {rest.map((item, index) => (
                <Fragment key={`${item.rank}-${item.nickname}`}>
                  {index === 0 && item.rank > podiumSize + 1 && <li className="leader-gap" aria-hidden="true">···</li>}
                  {index > 0 && item.rank - rest[index - 1].rank > 1 && (
                    <li className="leader-gap" aria-hidden="true">···</li>
                  )}
                  <li className={item.me ? 'me' : ''}>
                    <span className="leader-rank">{item.rank}</span>
                    <span className="leader-avatar" aria-hidden="true">{item.nickname.slice(0, 1).toUpperCase()}</span>
                    <strong>{item.nickname}</strong>
                    <em>{item.score.toLocaleString('ru-RU')}</em>
                  </li>
                </Fragment>
              ))}
            </ol>
          )}

          {me && (
            <div className="leaderboard-me">
              <div>
                <span>Ваше место</span>
                <strong>#{me.rank}</strong>
              </div>
              <p>
                {chaser
                  ? `До «${chaser.nickname}» ${Math.max(0, chaser.score - me.score).toLocaleString('ru-RU')} очков`
                  : 'Вы возглавляете рейтинг — держите отрыв'}
              </p>
              <small>{me.score.toLocaleString('ru-RU')} очков · лига {season.league}</small>
            </div>
          )}
        </>
      )}
    </section>
  )
}

function GuildCard({ guild }: { guild: Guild }) {
  const queryClient = useQueryClient()
  const membership = useMutation({
    mutationFn: () => (guild.user_member
      ? api('/guild/leave', { method: 'POST' })
      : api('/guild/join', { method: 'POST', body: JSON.stringify({ guild_id: guild.id }) })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['guild'] }),
  })

  return (
    <>
      <div className="section-title">
        <div>
          <span className="guild-emblem"><Shield aria-hidden="true" /></span>
          <div><span>Гильдия города</span><h2>{guild.name}</h2></div>
        </div>
        <button onClick={() => membership.mutate()} disabled={membership.isPending}>
          {guild.user_member ? 'Выйти' : 'Вступить'}
        </button>
      </div>
      {membership.error && <div className="form-error" role="alert">{errorMessage(membership.error)}</div>}

      <div className="guild-stats">
        <span><Users aria-hidden="true" /> <strong>{guild.members}</strong> участников</span>
        <span><Trophy aria-hidden="true" /> <strong>#{guild.rank}</strong> место</span>
      </div>

      <div className="challenge-card">
        <div>
          <Sparkles aria-hidden="true" />
          <span><small>Задание месяца</small><strong>{guild.challenge.title}</strong></span>
        </div>
        <p>{guild.challenge.description}</p>
        <div className="progress-track">
          <span style={{ width: `${percentage(guild.challenge.progress, guild.challenge.target)}%` }} />
        </div>
        <small>{guild.challenge.progress.toLocaleString('ru-RU')} из {guild.challenge.target.toLocaleString('ru-RU')}</small>
      </div>

      {guild.user_member && (
        <div className="contribution">
          <span>Ваш вклад</span>
          <strong>+{guild.user_contribution} очков</strong>
        </div>
      )}

      <div className="guild-feed">
        {guild.feed.slice(0, 4).map((item) => (
          <div key={item.id}>
            <span className="feed-dot" aria-hidden="true" />
            <p>{item.text}<small>{item.ago}</small></p>
            <strong>+{item.points}</strong>
          </div>
        ))}
      </div>
    </>
  )
}

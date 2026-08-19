const money = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 })

export function formatMoney(value: number, currency = 'RUB') {
  if (!value) return 'Бесплатно'
  return `${money.format(value)} ${currency === 'RUB' ? '₽' : currency}`
}

export function formatDuration(minutes: number) {
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return rest ? `${hours} ч ${rest} мин` : `${hours} ч`
}

export function formatDate(value: string, options?: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat('ru-RU', options ?? { day: 'numeric', month: 'short' }).format(new Date(value))
}

export function formatAgo(value: string, now = Date.now()) {
  const minutes = Math.max(0, Math.round((now - new Date(value).getTime()) / 60_000))
  if (minutes < 1) return 'только что'
  if (minutes < 60) return `${minutes} мин назад`

  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours} ч назад`

  return formatDate(value, { day: 'numeric', month: 'short' })
}

export function percentage(value: number, max: number) {
  if (max <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((value / max) * 100)))
}

export function transportLabel(mode: string) {
  return ({
    railway: 'Поезд', etrain: 'Электричка', bus: 'Автобус', avia: 'Самолёт',
  } as Record<string, string>)[mode] ?? mode
}

export function visibilityLabel(value: string) {
  return ({
    private: 'Только я',
    aggregate: 'Обезличенно для гильдии',
  } as Record<string, string>)[value] ?? value
}

export function eventAvailability(value: string) {
  return ({ available: 'Есть места', limited: 'Мало мест', sold_out: 'Продано' } as Record<string, string>)[value] ?? value
}

export function contentLabel(value: string) {
  return ({
    nature: 'Природа', history: 'История', unusual: 'Необычное', culture: 'Культура',
    food: 'Гастрономия', architecture: 'Архитектура', active: 'Активный отдых',
    festival: 'Фестиваль', exhibition: 'Выставка', theatre: 'Театр', concert: 'Концерт',
  } as Record<string, string>)[value] ?? value
}

export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Не удалось выполнить действие'
}

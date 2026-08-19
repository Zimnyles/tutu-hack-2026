import { describe, expect, it } from 'vitest'
import { contentLabel, eventAvailability, formatAgo, formatDuration, formatMoney, percentage, transportLabel, visibilityLabel } from './format'

describe('formatters', () => {
  it('formats route values for the Russian UI', () => {
    expect(formatMoney(12500)).toContain('12')
    expect(formatMoney(0)).toBe('Бесплатно')
    expect(formatDuration(155)).toBe('2 ч 35 мин')
    expect(transportLabel('railway')).toBe('Поезд')
    expect(transportLabel('avia')).toBe('Самолёт')
    expect(transportLabel('etrain')).toBe('Электричка')
  })

  it('keeps progress inside valid bounds', () => {
    expect(percentage(12, 10)).toBe(100)
    expect(percentage(-1, 10)).toBe(0)
    expect(percentage(5, 0)).toBe(0)
  })

  it('tells how long ago a snapshot was built', () => {
    const now = new Date('2026-08-19T12:00:00Z').getTime()
    expect(formatAgo('2026-08-19T11:59:40Z', now)).toBe('только что')
    expect(formatAgo('2026-08-19T11:25:00Z', now)).toBe('35 мин назад')
    expect(formatAgo('2026-08-19T06:00:00Z', now)).toBe('6 ч назад')
    expect(formatAgo('2026-08-14T12:00:00Z', now)).toContain('14')
  })

  it('shows domain codes as clear labels', () => {
    expect(contentLabel('nature')).toBe('Природа')
    expect(eventAvailability('limited')).toBe('Мало мест')
    expect(visibilityLabel('aggregate')).toBe('Обезличенно для гильдии')
  })
})

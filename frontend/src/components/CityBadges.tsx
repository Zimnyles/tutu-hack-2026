import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { Glyph } from '../shared/icons'
import type { BadgeDefinition, PublicSettings } from '../types'

export function useBadgeCatalog() {
  const settings = useQuery({
    queryKey: ['public-settings'],
    queryFn: () => api<PublicSettings>('/config'),
    staleTime: 10 * 60 * 1000,
  })

  const catalog = new Map<string, BadgeDefinition>()
  for (const badge of settings.data?.badges ?? []) {
    catalog.set(badge.label, badge)
  }

  return catalog
}

export function CityBadges({
  badges,
  limit,
  variant = 'light',
}: {
  badges: string[]
  limit?: number
  variant?: 'light' | 'solid'
}) {
  const catalog = useBadgeCatalog()
  const visible = limit ? badges.slice(0, limit) : badges

  if (visible.length === 0) {
    return null
  }

  return (
    <ul className={`city-badges ${variant}`}>
      {visible.map((badge) => (
        <li key={badge} title={catalog.get(badge)?.group}>
          <Glyph code={catalog.get(badge)?.icon} />
          {badge}
        </li>
      ))}
    </ul>
  )
}

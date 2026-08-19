export type Preferences = {
  themes: string[]
  transport_modes: string[]
  max_travel_minutes: number
  typical_budget: number
  trip_duration_days: number
  spontaneity: number
  avoid: string[]
}

export type User = {
  id: string
  email: string
  display_name: string
  home_city_id: string
  role: string
  demo: boolean
  onboarding_completed: boolean
  preferences: Preferences
  travel_visibility: string
}

export type TerritoryState = 'locked' | 'suggested' | 'planned' | 'arrived'

export type Territory = {
  id: string
  name: string
  region: string
  latitude: number
  longitude: number
  state: TerritoryState
  level: number
  tags: string[]
  rarity: number
  reward: number
  promo_percent: number
  description: string
  image_tone: string
  upcoming_events: number
  next_event_at?: string
  popular_event: boolean
}

export type PromoCode = {
  id: string
  code: string
  city_id: string
  city_name: string
  discount_percent: number
  status: 'active' | 'used' | 'expired'
  reason_code: string
  issued_at: string
  expires_at: string
}

export type CityEvent = {
  id: string
  city_id: string
  external_id: string
  title: string
  description: string
  category: string
  venue_name: string
  starts_at: string
  ends_at: string
  price_from: number
  currency: string
  age_rating: string
  availability: string
  status: string
  source: string
  trust_status: string
  updated_at: string
  demo: boolean
  city_name?: string
  source_url?: string
}

export type CityEventsResponse = {
  items: CityEvent[]
  discovering: boolean
  catalog: { mode: string; refreshed_at: string; stale: boolean }
}

export type PopularEventsResponse = {
  items: CityEvent[]
  discovering: boolean
  refreshed_at: string
}

export type RecommendationOption = {
  id: string
  city_id: string
  city_name: string
  region: string
  rank: number
  score: number
  reason: string
  why_now: string
  price_amount: number
  currency: string
  duration_minutes: number
  transport_mode: string
  territory_gain_km2: number
  reward: number
  special_offer: boolean
  valid_until: string
  event_id?: string
}

export type Recommendation = {
  id: string
  kind: 'personal' | 'prompt' | 'event'
  status: 'queued' | 'processing' | 'completed' | 'partial' | 'blocked' | 'failed'
  stage: string
  origin_city_id: string
  destination_city_id?: string
  event_id?: string
  date_from: string
  date_to: string
  adults: number
  children: number
  budget: number
  currency: string
  transport_modes: string[]
  max_travel_minutes: number
  direct_only: boolean
  options: RecommendationOption[]
  created_at: string
  completed_at?: string
  failure_code?: string
  demo_fallback: boolean
}

export type Trip = {
  id: string
  option: RecommendationOption
  event_id?: string
  status: string
  checkout_url?: string
  depart_at: string
  arrive_at: string
  created_at: string
}

export type Achievement = {
  id: string
  title: string
  description: string
  icon: string
  unlocked: boolean
  progress: number
  target: number
}

export type LedgerEntry = {
  id: string
  amount: number
  reason_code: string
  reference_type: string
  reference_id: string
  created_at: string
}

export type Guild = {
  id: string
  name: string
  city_id: string
  emblem: string
  level: number
  members: number
  season_score: number
  rank: number
  user_member: boolean
  user_contribution: number
  challenge: {
    title: string
    description: string
    progress: number
    target: number
  }
  feed: Array<{ id: string; text: string; ago: string; points: number }>
}

export type Season = {
  id: string
  name: string
  month_title: string
  status: string
  user_score: number
  league: string
  percentile: number
  next_league_score: number
  starts_at: string
  ends_at: string
}

export type LeaderboardItem = {
  rank: number
  nickname: string
  score: number
  me: boolean
}

export type Leaderboard = {
  scope: string
  period: string
  generated_at: string
  items: LeaderboardItem[]
}

export type TravelCohort = {
  visible: boolean
  count?: number
  from_guild?: number
  threshold: number
  message?: string
  window: string
  privacy: string
  demo: boolean
}

export type Bootstrap = {
  user: User
  territories: Territory[]
  balance: number
  demo_mode: boolean
  season: Season
  personal_recommendation?: Recommendation
}

export type OnboardingOption = {
  code: string
  label: string
  icon?: string
  description?: string
}

export type PublicSettings = {
  onboarding: {
    themes: OnboardingOption[]
    transport_modes: OnboardingOption[]
    travel_time_options: number[]
    budget_min: number
    budget_max: number
    budget_step: number
  }
  recommendation_stages: Array<{ code: string; label: string }>
  privacy_threshold: number
  home_cities: Array<{ id: string; name: string; region: string }>
}

export type ProfileResponse = {
  user: User
  progress: { opened: number; total: number; world_percent: number }
  balance: number
}

export type AdminOverview = {
  database_ready: boolean
  demo_users: number
  pending_outbox: number
  failed_actions: number
  simulator_enabled: boolean
  available_actions: string[]
}

export type AdminUser = {
  id: string
  email: string
  display_name: string
  onboarding_completed: boolean
  visits: number
  trips: number
  reward_balance: number
}

export type DemoScenario = {
  id: string
  code: string
  name: string
  description: string
  fixture_version: string
  enabled: boolean
}

export type AdminAuditEntry = {
  id: string
  action_code: string
  target_type: string
  target_id?: string
  outcome: string
  reason_code: string
  metadata: Record<string, unknown>
  created_at: string
}

export type AdminSimulation = {
  id: string
  action_code: string
  target_type: string
  target_id?: string
  status: string
  result_summary: Record<string, unknown>
  created_at: string
  completed_at?: string
}

export type PlanningTarget = {
  territory: Territory
  event?: CityEvent
}

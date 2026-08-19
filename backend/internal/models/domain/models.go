package domain

import (
	"encoding/json"
	"time"
)

type User struct {
	ID                  string      `json:"id"`
	Email               string      `json:"email"`
	DisplayName         string      `json:"display_name"`
	HomeCityID          string      `json:"home_city_id"`
	Role                string      `json:"role"`
	Demo                bool        `json:"demo"`
	OnboardingCompleted bool        `json:"onboarding_completed"`
	Preferences         Preferences `json:"preferences"`
	TravelVisibility    string      `json:"travel_visibility"`
}

type Preferences struct {
	Themes           []string `json:"themes"`
	TransportModes   []string `json:"transport_modes"`
	MaxTravelMinutes int      `json:"max_travel_minutes"`
	TypicalBudget    int      `json:"typical_budget"`
	TripDurationDays int      `json:"trip_duration_days"`
	Spontaneity      int      `json:"spontaneity"`
	Avoid            []string `json:"avoid"`
}

type Territory struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Region             string   `json:"region"`
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	State              string   `json:"state"`
	Level              int      `json:"level"`
	Tags               []string `json:"tags"`
	Rarity             int      `json:"rarity"`
	Reward             int      `json:"reward"`
	PromoPercent       int      `json:"promo_percent"`
	Description        string   `json:"description"`
	ImageTone          string   `json:"image_tone"`
	UpcomingEvents     int      `json:"upcoming_events"`
	NextEventAt        string   `json:"next_event_at,omitempty"`
	PopularEvent       bool     `json:"popular_event"`
	SeasonalFit        float64  `json:"-"`
	CommercialPriority float64  `json:"-"`
}

type Event struct {
	ID           string    `json:"id"`
	CityID       string    `json:"city_id"`
	ExternalID   string    `json:"external_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Venue        string    `json:"venue_name"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	PriceFrom    int       `json:"price_from"`
	Currency     string    `json:"currency"`
	AgeRating    string    `json:"age_rating"`
	Availability string    `json:"availability"`
	Status       string    `json:"status"`
	Source       string    `json:"source"`
	TrustStatus  string    `json:"trust_status"`
	UpdatedAt    time.Time `json:"updated_at"`
	Demo         bool      `json:"demo"`
	CityName     string    `json:"city_name,omitempty"`
	SourceURL    string    `json:"source_url,omitempty"`
}

type DiscoveredEvent struct {
	CityID      string
	CityName    string
	Title       string
	Category    string
	Venue       string
	Description string
	SourceURL   string
	StartsAt    time.Time
	EndsAt      time.Time
	PriceFrom   int
	Currency    string
	Rank        int
}

type EventDiscoveryState struct {
	Scope       string    `json:"scope"`
	ScopeKey    string    `json:"scope_key"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	RefreshedAt time.Time `json:"refreshed_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	FailureCode string    `json:"failure_code,omitempty"`
}

type EventEnrichment struct {
	EventID     string `json:"event_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type RecommendationRequest struct {
	ID               string                 `json:"id"`
	UserID           string                 `json:"-"`
	Kind             string                 `json:"kind"`
	Status           string                 `json:"status"`
	Stage            string                 `json:"stage"`
	OriginCityID     string                 `json:"origin_city_id"`
	DestinationID    string                 `json:"destination_city_id,omitempty"`
	EventID          string                 `json:"event_id,omitempty"`
	DateFrom         string                 `json:"date_from"`
	DateTo           string                 `json:"date_to"`
	Adults           int                    `json:"adults"`
	Children         int                    `json:"children"`
	Budget           int                    `json:"budget"`
	Currency         string                 `json:"currency"`
	TransportModes   []string               `json:"transport_modes"`
	MaxTravelMinutes int                    `json:"max_travel_minutes"`
	DirectOnly       bool                   `json:"direct_only"`
	Prompt           string                 `json:"-"`
	Options          []RecommendationOption `json:"options"`
	CreatedAt        time.Time              `json:"created_at"`
	CompletedAt      *time.Time             `json:"completed_at,omitempty"`
	DemoFallback     bool                   `json:"demo_fallback"`
	FailureCode      string                 `json:"failure_code,omitempty"`
}

type RecommendationOption struct {
	ID              string          `json:"id"`
	CityID          string          `json:"city_id"`
	CityName        string          `json:"city_name"`
	Region          string          `json:"region"`
	Rank            int             `json:"rank"`
	Score           int             `json:"score"`
	Reason          string          `json:"reason"`
	WhyNow          string          `json:"why_now"`
	Price           int             `json:"price_amount"`
	Currency        string          `json:"currency"`
	DurationMinutes int             `json:"duration_minutes"`
	Transport       string          `json:"transport_mode"`
	TerritoryGain   int             `json:"territory_gain_km2"`
	Reward          int             `json:"reward"`
	Special         bool            `json:"special_offer"`
	ValidUntil      string          `json:"valid_until"`
	EventID         string          `json:"event_id,omitempty"`
	OfferSnapshot   json.RawMessage `json:"-"`
	CheckoutRef     json.RawMessage `json:"-"`
}

type TransportOffer struct {
	CityID          string          `json:"city_id"`
	CityName        string          `json:"city_name"`
	Transport       string          `json:"transport_mode"`
	Price           int             `json:"price_amount"`
	Currency        string          `json:"currency"`
	DurationMinutes int             `json:"duration_minutes"`
	DepartureAt     string          `json:"departure_at"`
	ArrivalAt       string          `json:"arrival_at"`
	CheckoutRef     json.RawMessage `json:"-"`
	Snapshot        json.RawMessage `json:"-"`
	Source          string          `json:"source"`
}

type Trip struct {
	ID          string               `json:"id"`
	UserID      string               `json:"-"`
	Option      RecommendationOption `json:"option"`
	EventID     string               `json:"event_id,omitempty"`
	Status      string               `json:"status"`
	CheckoutURL string               `json:"checkout_url,omitempty"`
	DepartAt    time.Time            `json:"depart_at"`
	ArriveAt    time.Time            `json:"arrive_at"`
	CreatedAt   time.Time            `json:"created_at"`
}

type LedgerEntry struct {
	ID             string    `json:"id"`
	Amount         int       `json:"amount"`
	ReasonCode     string    `json:"reason_code"`
	ReferenceType  string    `json:"reference_type"`
	ReferenceID    string    `json:"reference_id"`
	IdempotencyKey string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

type Achievement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Unlocked    bool   `json:"unlocked"`
	Progress    int    `json:"progress"`
	Target      int    `json:"target"`
}

type PromoCode struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	CityID          string    `json:"city_id"`
	CityName        string    `json:"city_name"`
	DiscountPercent int       `json:"discount_percent"`
	Status          string    `json:"status"`
	ReasonCode      string    `json:"reason_code"`
	IssuedAt        time.Time `json:"issued_at"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Guild struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	CityID           string          `json:"city_id"`
	Emblem           string          `json:"emblem"`
	Level            int             `json:"level"`
	Members          int             `json:"members"`
	SeasonScore      int             `json:"season_score"`
	Rank             int             `json:"rank"`
	Challenge        GuildChallenge  `json:"challenge"`
	UserMember       bool            `json:"user_member"`
	UserContribution int             `json:"user_contribution"`
	Feed             []GuildFeedItem `json:"feed"`
}

type GuildChallenge struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Progress    int    `json:"progress"`
	Target      int    `json:"target"`
}

type GuildFeedItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Ago    string `json:"ago"`
	Points int    `json:"points"`
}

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Code + ": " + e.Message
}

type PublicSettings struct {
	Onboarding           map[string]any       `json:"onboarding"`
	RecommendationStages []WorkflowStage      `json:"recommendation_stages"`
	PrivacyThreshold     int                  `json:"privacy_threshold"`
	HomeCities           []TerritoryReference `json:"home_cities"`
}

type WorkflowStage struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type RecommendationSettings struct {
	CandidateSearchLimit int `json:"candidate_search_limit"`
	OfferPageSize        int `json:"offer_page_size"`
	MaximumRankedOptions int `json:"maximum_ranked_options"`
	OfferValidityMinutes int `json:"offer_validity_minutes"`
}

type PersonalRecommendationSettings struct {
	LeadDays            int    `json:"lead_days"`
	DefaultDurationDays int    `json:"default_duration_days"`
	MaximumDurationDays int    `json:"maximum_duration_days"`
	FreshnessHours      int    `json:"freshness_hours"`
	Adults              int    `json:"adults"`
	Currency            string `json:"currency"`
}

type ScoringWeights struct {
	PreferenceMatch    float64 `json:"preference_match"`
	PriceValue         float64 `json:"price_value"`
	MapGain            float64 `json:"map_gain"`
	Novelty            float64 `json:"novelty"`
	SeasonalFit        float64 `json:"seasonal_fit"`
	EventFit           float64 `json:"event_fit"`
	TravelFriction     float64 `json:"travel_friction"`
	CommercialPriority float64 `json:"commercial_priority"`
	RarityCeiling      float64 `json:"rarity_ceiling"`
}

type TravelSearchPlan struct {
	CandidateIDs   []string `json:"candidate_ids"`
	TransportModes []string `json:"transport_modes"`
}

type TravelIntent struct {
	Themes           []string `json:"themes"`
	TransportModes   []string `json:"transport_modes"`
	TripDurationDays int      `json:"trip_duration_days"`
	BudgetLevel      string   `json:"budget_level"`
	Avoid            []string `json:"avoid"`
}

type AIRequestAnalysis struct {
	TravelRelated   bool         `json:"travel_related"`
	ContainsPII     bool         `json:"contains_pii"`
	PromptInjection bool         `json:"prompt_injection"`
	Political       bool         `json:"political"`
	Intent          TravelIntent `json:"travel_intent"`
}

type AISystemPrompts struct {
	RequestAnalysis           string
	TravelSearchPlan          string
	RecommendationExplanation string
	EventEnrichment           string
}

type TransportSearchIssue struct {
	CityID string `json:"city_id"`
	Code   string `json:"code"`
}

type TransportSearchResult struct {
	Offers []TransportOffer       `json:"offers"`
	Issues []TransportSearchIssue `json:"issues,omitempty"`
}

type ScoredTravelOption struct {
	Candidate Territory      `json:"candidate"`
	Offer     TransportOffer `json:"offer"`
	Score     int            `json:"backend_score"`
}

type TerritoryReference struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type Season struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	MonthTitle      string    `json:"month_title"`
	Status          string    `json:"status"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	RulesVersion    string    `json:"rules_version"`
	UserScore       int       `json:"user_score"`
	League          string    `json:"league"`
	Percentile      int       `json:"percentile"`
	NextLeagueScore int       `json:"next_league_score"`
}

type Leaderboard struct {
	Scope       string            `json:"scope"`
	Period      string            `json:"period"`
	GeneratedAt time.Time         `json:"generated_at"`
	Items       []LeaderboardItem `json:"items"`
}

type LeaderboardItem struct {
	Rank     int    `json:"rank"`
	Nickname string `json:"nickname"`
	Score    int    `json:"score"`
	Me       bool   `json:"me"`
}

type TravelCohort struct {
	Visible   bool   `json:"visible"`
	Count     int    `json:"count,omitempty"`
	FromGuild int    `json:"from_guild,omitempty"`
	Threshold int    `json:"threshold"`
	Message   string `json:"message,omitempty"`
	Window    string `json:"window"`
	Privacy   string `json:"privacy"`
	Demo      bool   `json:"demo"`
}

type AdminOverview struct {
	DatabaseReady    bool     `json:"database_ready"`
	DemoUsers        int      `json:"demo_users"`
	PendingOutbox    int      `json:"pending_outbox"`
	FailedActions    int      `json:"failed_actions"`
	SimulatorEnabled bool     `json:"simulator_enabled"`
	AvailableActions []string `json:"available_actions"`
}

type AdminUserSummary struct {
	ID                  string `json:"id"`
	Email               string `json:"email"`
	DisplayName         string `json:"display_name"`
	OnboardingCompleted bool   `json:"onboarding_completed"`
	Visits              int    `json:"visits"`
	Trips               int    `json:"trips"`
	RewardBalance       int    `json:"reward_balance"`
}

type DemoScenario struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	FixtureVersion string `json:"fixture_version"`
	Enabled        bool   `json:"enabled"`
}

type AdminSimulationCommand struct {
	ActorUserID    string         `json:"-"`
	ActionCode     string         `json:"action_code"`
	TargetType     string         `json:"target_type"`
	TargetID       string         `json:"target_id"`
	DemoScenarioID string         `json:"demo_scenario_id"`
	IdempotencyKey string         `json:"-"`
	Reason         string         `json:"reason"`
	Parameters     map[string]any `json:"parameters"`
	RequestID      string         `json:"-"`
	TraceID        string         `json:"-"`
}

type AdminSimulation struct {
	ID            string         `json:"id"`
	ActionCode    string         `json:"action_code"`
	TargetType    string         `json:"target_type"`
	TargetID      string         `json:"target_id,omitempty"`
	Status        string         `json:"status"`
	ResultSummary map[string]any `json:"result_summary"`
	CreatedAt     time.Time      `json:"created_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

type AdminAuditEntry struct {
	ID         string         `json:"id"`
	ActionCode string         `json:"action_code"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id,omitempty"`
	Outcome    string         `json:"outcome"`
	ReasonCode string         `json:"reason_code"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

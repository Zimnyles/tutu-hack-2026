package dto

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"` //nolint:gosec // Transport DTO; the value is immediately hashed by the auth service.
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` //nolint:gosec // Transport DTO; the value is never persisted or logged.
}

type HomeCityRequest struct {
	HomeCityID string `json:"home_city_id"`
}

type VisibilityRequest struct {
	Visibility string `json:"visibility"`
}

type PreferencesRequest struct {
	Themes           []string `json:"themes"`
	TransportModes   []string `json:"transport_modes"`
	MaxTravelMinutes int      `json:"max_travel_minutes"`
	TypicalBudget    int      `json:"typical_budget"`
	TripDurationDays int      `json:"trip_duration_days"`
	Spontaneity      int      `json:"spontaneity"`
	Avoid            []string `json:"avoid"`
}

type SearchRequest struct {
	OriginCityID     string   `json:"origin_city_id"`
	DestinationID    string   `json:"destination_city_id"`
	EventID          string   `json:"event_id"`
	DateFrom         string   `json:"date_from"`
	DateTo           string   `json:"date_to"`
	Adults           int      `json:"adults"`
	Children         int      `json:"children"`
	Budget           int      `json:"budget"`
	Currency         string   `json:"currency"`
	TransportModes   []string `json:"transport_modes"`
	MaxTravelMinutes int      `json:"max_travel_minutes"`
	DirectOnly       bool     `json:"direct_only"`
	Prompt           string   `json:"prompt"`
}

type SelectOptionRequest struct {
	OptionID string `json:"option_id"`
}

type JoinGuildRequest struct {
	GuildID string `json:"guild_id"`
}

type AdminExecuteRequest struct {
	ActionCode     string         `json:"action_code"`
	TargetType     string         `json:"target_type"`
	TargetID       string         `json:"target_id"`
	DemoScenarioID string         `json:"demo_scenario_id"`
	Reason         string         `json:"reason"`
	Parameters     map[string]any `json:"parameters"`
}

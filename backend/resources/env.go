package resources

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPPort             = "8080"
	defaultRedisAddress         = "localhost:6379"
	defaultShutdownPeriod       = 15 * time.Second
	defaultSessionLifetime      = 7 * 24 * time.Hour
	defaultRecommendationHourly = 20
	defaultRecommendationDaily  = 100
	defaultLoginAttemptsPerHour = 10
	defaultRequestsPerMinute    = 120
	defaultDatabaseConnections  = 20
	minimumSecretLength         = 32
	minimumSessionLifetime      = time.Hour
	maximumSessionLifetime      = 90 * 24 * time.Hour

	defaultEventDiscoveryTTL          = 24 * time.Hour
	defaultEventDiscoveryTimeout      = 5 * time.Minute
	defaultEventDiscoveryBackoff      = time.Hour
	defaultEventDiscoveryCityLimit    = 10
	defaultEventDiscoveryPopularLimit = 12
	defaultEventDiscoveryCityPool     = 120
	defaultEventDiscoveryPrewarm      = 250
	defaultEventDiscoveryWindowDays   = 60
	defaultEventDiscoveryConcurrency  = 3

	minimumEventDiscoveryTTL     = time.Hour
	minimumEventDiscoveryTimeout = time.Minute
	maximumEventDiscoveryLimit   = 40
)

var (
	ErrUnsafeProductionConfig       = errors.New("unsafe production configuration")
	ErrIncompleteDemoConfiguration  = errors.New("demo account configuration is incomplete")
	ErrMissingDatabaseConfiguration = errors.New("DATABASE_URL is required")
	ErrInvalidConfiguration         = errors.New("invalid configuration value")
)

type Env struct {
	Environment                   string
	HTTPPort                      string
	DatabaseURL                   string
	RedisAddress                  string
	RedisPassword                 string
	PublicBaseURL                 string
	AllowedOrigins                []string
	TrustedProxies                []string
	SessionSecret                 string
	DeepSeekAPIKey                string
	DeepSeekBaseURL               string
	DeepSeekModel                 string
	DeepSeekSearchModel           string
	DeepSeekRequestAnalysisPrompt string
	DeepSeekTravelSearchPrompt    string
	DeepSeekExplanationPrompt     string
	DeepSeekEventEnrichmentPrompt string
	TutuMCPURL                    string
	CheckoutAllowedHosts          []string
	DemoMode                      bool
	AdminSimulatorEnabled         bool
	DemoUserEmail                 string
	DemoUserPassword              string
	DemoUserName                  string
	DemoAdminEmail                string
	DemoAdminPassword             string
	DemoAdminName                 string
	ShutdownPeriod                time.Duration
	SessionLifetime               time.Duration
	EventDiscoveryTTL             time.Duration
	EventDiscoveryTimeout         time.Duration
	EventDiscoveryRetryBackoff    time.Duration
	RecommendationsPerHour        int
	RecommendationsPerDay         int
	LoginAttemptsPerHour          int
	RequestsPerMinute             int
	DatabaseMaxConnections        int
	EventDiscoveryEnabled         bool
	EventDiscoveryCityLimit       int
	EventDiscoveryPopularLimit    int
	EventDiscoveryCityPool        int
	EventDiscoveryPrewarmCities   int
	EventDiscoveryWindowDays      int
	EventDiscoveryConcurrency     int
	LogLevel                      slog.Level
}

func LoadEnv() (*Env, error) {
	environment := valueOrDefault("APP_ENV", "local")
	production := environment == "production"
	publicBaseURL := valueOrDefault("PUBLIC_BASE_URL", "http://localhost:5173")

	env := &Env{
		Environment:                   environment,
		HTTPPort:                      valueOrDefault("APP_PORT", defaultHTTPPort),
		DatabaseURL:                   os.Getenv("DATABASE_URL"),
		RedisAddress:                  valueOrDefault("REDIS_ADDR", defaultRedisAddress),
		RedisPassword:                 os.Getenv("REDIS_PASSWORD"),
		PublicBaseURL:                 publicBaseURL,
		AllowedOrigins:                listValue("ALLOWED_ORIGINS", []string{publicBaseURL}),
		TrustedProxies:                listValue("TRUSTED_PROXIES", nil),
		SessionSecret:                 os.Getenv("SESSION_SECRET"),
		DeepSeekAPIKey:                os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekBaseURL:               valueOrDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:                 valueOrDefault("DEEPSEEK_MODEL", "deepseek-chat"),
		DeepSeekSearchModel:           valueOrDefault("DEEPSEEK_SEARCH_MODEL", "deepseek-v4-flash"),
		DeepSeekRequestAnalysisPrompt: os.Getenv("DEEPSEEK_SYSTEM_PROMPT_REQUEST_ANALYSIS"),
		DeepSeekTravelSearchPrompt:    os.Getenv("DEEPSEEK_SYSTEM_PROMPT_TRAVEL_SEARCH"),
		DeepSeekExplanationPrompt:     os.Getenv("DEEPSEEK_SYSTEM_PROMPT_EXPLANATION"),
		DeepSeekEventEnrichmentPrompt: os.Getenv("DEEPSEEK_SYSTEM_PROMPT_EVENT_ENRICHMENT"),
		TutuMCPURL:                    valueOrDefault("TUTU_MCP_URL", "https://mcp.tutu.ru/mcp"),
		CheckoutAllowedHosts:          listValue("CHECKOUT_ALLOWED_HOSTS", []string{"tutu.ru", "*.tutu.ru"}),
		DemoMode:                      boolValue("DEMO_MODE", !production),
		AdminSimulatorEnabled:         boolValue("ADMIN_SIMULATOR_ENABLED", !production),
		DemoUserEmail:                 os.Getenv("DEMO_USER_EMAIL"),
		DemoUserPassword:              os.Getenv("DEMO_USER_PASSWORD"),
		DemoUserName:                  os.Getenv("DEMO_USER_NAME"),
		DemoAdminEmail:                os.Getenv("DEMO_ADMIN_EMAIL"),
		DemoAdminPassword:             os.Getenv("DEMO_ADMIN_PASSWORD"),
		DemoAdminName:                 os.Getenv("DEMO_ADMIN_NAME"),
		ShutdownPeriod:                durationValue("SHUTDOWN_PERIOD", defaultShutdownPeriod),
		SessionLifetime:               durationValue("SESSION_LIFETIME", defaultSessionLifetime),
		EventDiscoveryTTL:             durationValue("EVENT_DISCOVERY_TTL", defaultEventDiscoveryTTL),
		EventDiscoveryTimeout:         durationValue("EVENT_DISCOVERY_TIMEOUT", defaultEventDiscoveryTimeout),
		EventDiscoveryRetryBackoff:    durationValue("EVENT_DISCOVERY_RETRY_BACKOFF", defaultEventDiscoveryBackoff),
		RecommendationsPerHour:        intValue("RECOMMENDATIONS_PER_HOUR", defaultRecommendationHourly),
		RecommendationsPerDay:         intValue("RECOMMENDATIONS_PER_DAY", defaultRecommendationDaily),
		LoginAttemptsPerHour:          intValue("LOGIN_ATTEMPTS_PER_HOUR", defaultLoginAttemptsPerHour),
		RequestsPerMinute:             intValue("REQUESTS_PER_MINUTE", defaultRequestsPerMinute),
		DatabaseMaxConnections:        intValue("DATABASE_MAX_CONNECTIONS", defaultDatabaseConnections),
		EventDiscoveryEnabled:         boolValue("EVENT_DISCOVERY_ENABLED", true),
		EventDiscoveryCityLimit:       intValue("EVENT_DISCOVERY_CITY_LIMIT", defaultEventDiscoveryCityLimit),
		EventDiscoveryPopularLimit:    intValue("EVENT_DISCOVERY_POPULAR_LIMIT", defaultEventDiscoveryPopularLimit),
		EventDiscoveryCityPool:        intValue("EVENT_DISCOVERY_CITY_POOL", defaultEventDiscoveryCityPool),
		EventDiscoveryPrewarmCities:   intValue("EVENT_DISCOVERY_PREWARM_CITIES", defaultEventDiscoveryPrewarm),
		EventDiscoveryWindowDays:      intValue("EVENT_DISCOVERY_WINDOW_DAYS", defaultEventDiscoveryWindowDays),
		EventDiscoveryConcurrency:     intValue("EVENT_DISCOVERY_CONCURRENCY", defaultEventDiscoveryConcurrency),
		LogLevel:                      logLevelValue("LOG_LEVEL", slog.LevelInfo),
	}

	if err := env.validate(); err != nil {
		return nil, err
	}

	return env, nil
}

func (e *Env) IsProduction() bool {
	return e.Environment == "production"
}

func (e *Env) validate() error {
	if e.DatabaseURL == "" {
		return ErrMissingDatabaseConfiguration
	}

	if err := e.validateLimits(); err != nil {
		return err
	}

	if err := validateHTTPURL("PUBLIC_BASE_URL", e.PublicBaseURL); err != nil {
		return err
	}

	if err := validateHTTPURL("TUTU_MCP_URL", e.TutuMCPURL); err != nil {
		return err
	}

	if err := validateHTTPURL("DEEPSEEK_BASE_URL", e.DeepSeekBaseURL); err != nil {
		return err
	}

	if len(e.CheckoutAllowedHosts) == 0 {
		return fmt.Errorf("CHECKOUT_ALLOWED_HOSTS is empty: %w", ErrInvalidConfiguration)
	}

	if e.DemoMode && (e.DemoUserEmail == "" ||
		e.DemoUserPassword == "" ||
		e.DemoUserName == "" ||
		e.DemoAdminEmail == "" ||
		e.DemoAdminPassword == "" ||
		e.DemoAdminName == "") {
		return ErrIncompleteDemoConfiguration
	}

	if !e.IsProduction() {
		return nil
	}

	if e.DemoMode || e.AdminSimulatorEnabled {
		return fmt.Errorf("demo features in production: %w", ErrUnsafeProductionConfig)
	}

	if e.DeepSeekAPIKey == "" || e.DeepSeekAPIKey == "replace-me" {
		return fmt.Errorf("DeepSeek key: %w", ErrUnsafeProductionConfig)
	}

	if len([]rune(e.SessionSecret)) < minimumSecretLength {
		return fmt.Errorf("SESSION_SECRET must be at least %d characters: %w",
			minimumSecretLength, ErrUnsafeProductionConfig)
	}

	if !strings.HasPrefix(e.PublicBaseURL, "https://") {
		return fmt.Errorf("PUBLIC_BASE_URL must use https: %w", ErrUnsafeProductionConfig)
	}

	return nil
}

func (e *Env) validateLimits() error {
	if e.SessionLifetime < minimumSessionLifetime || e.SessionLifetime > maximumSessionLifetime {
		return fmt.Errorf("SESSION_LIFETIME out of range: %w", ErrInvalidConfiguration)
	}

	if e.ShutdownPeriod <= 0 || e.ShutdownPeriod > time.Minute {
		return fmt.Errorf("SHUTDOWN_PERIOD out of range: %w", ErrInvalidConfiguration)
	}

	positives := map[string]int{
		"RECOMMENDATIONS_PER_HOUR":    e.RecommendationsPerHour,
		"RECOMMENDATIONS_PER_DAY":     e.RecommendationsPerDay,
		"LOGIN_ATTEMPTS_PER_HOUR":     e.LoginAttemptsPerHour,
		"REQUESTS_PER_MINUTE":         e.RequestsPerMinute,
		"DATABASE_MAX_CONNECTIONS":    e.DatabaseMaxConnections,
		"EVENT_DISCOVERY_CITY_POOL":   e.EventDiscoveryCityPool,
		"EVENT_DISCOVERY_WINDOW_DAYS": e.EventDiscoveryWindowDays,
		"EVENT_DISCOVERY_CONCURRENCY": e.EventDiscoveryConcurrency,
	}
	for key, value := range positives {
		if value <= 0 {
			return fmt.Errorf("%s must be positive: %w", key, ErrInvalidConfiguration)
		}
	}

	discoveryLimits := map[string]int{
		"EVENT_DISCOVERY_CITY_LIMIT":    e.EventDiscoveryCityLimit,
		"EVENT_DISCOVERY_POPULAR_LIMIT": e.EventDiscoveryPopularLimit,
	}
	for key, value := range discoveryLimits {
		if value <= 0 || value > maximumEventDiscoveryLimit {
			return fmt.Errorf("%s out of range: %w", key, ErrInvalidConfiguration)
		}
	}

	if e.EventDiscoveryTTL < minimumEventDiscoveryTTL {
		return fmt.Errorf("EVENT_DISCOVERY_TTL is too short: %w", ErrInvalidConfiguration)
	}

	if e.EventDiscoveryTimeout < minimumEventDiscoveryTimeout {
		return fmt.Errorf("EVENT_DISCOVERY_TIMEOUT is too short: %w", ErrInvalidConfiguration)
	}

	if e.EventDiscoveryRetryBackoff <= 0 {
		return fmt.Errorf("EVENT_DISCOVERY_RETRY_BACKOFF must be positive: %w", ErrInvalidConfiguration)
	}

	if e.RecommendationsPerDay < e.RecommendationsPerHour {
		return fmt.Errorf(
			"RECOMMENDATIONS_PER_DAY must not be lower than RECOMMENDATIONS_PER_HOUR: %w",
			ErrInvalidConfiguration,
		)
	}

	return nil
}

func validateHTTPURL(key string, rawValue string) error {
	parsed, err := url.Parse(rawValue)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", key, ErrInvalidConfiguration)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https: %w", key, ErrInvalidConfiguration)
	}

	if parsed.Host == "" {
		return fmt.Errorf("%s has no host: %w", key, ErrInvalidConfiguration)
	}

	return nil
}

func valueOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func boolValue(key string, fallback bool) bool {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return fallback
	}

	return value
}

func intValue(key string, fallback int) int {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return fallback
	}

	return value
}

func durationValue(key string, fallback time.Duration) time.Duration {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil {
		return fallback
	}

	return value
}

func logLevelValue(key string, fallback slog.Level) slog.Level {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(rawValue)); err != nil {
		return fallback
	}

	return level
}

func listValue(key string, fallback []string) []string {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}

	parts := strings.Split(rawValue, ",")
	values := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	if len(values) == 0 {
		return fallback
	}

	return values
}

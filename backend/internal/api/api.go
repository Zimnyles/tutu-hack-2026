package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	tutu_checkout "github.com/tutu-hack/openworld/infra/checkout/tutumcp"
	"github.com/tutu-hack/openworld/infra/clients/deepseek"
	"github.com/tutu-hack/openworld/infra/ratelimit"
	adminsim_storage "github.com/tutu-hack/openworld/infra/storage/adminsim"
	auth_storage "github.com/tutu-hack/openworld/infra/storage/auth"
	community_storage "github.com/tutu-hack/openworld/infra/storage/community"
	events_storage "github.com/tutu-hack/openworld/infra/storage/events"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	profile_storage "github.com/tutu-hack/openworld/infra/storage/profile"
	recommendations_storage "github.com/tutu-hack/openworld/infra/storage/recommendations"
	rewards_storage "github.com/tutu-hack/openworld/infra/storage/rewards"
	trips_storage "github.com/tutu-hack/openworld/infra/storage/trips"
	world_storage "github.com/tutu-hack/openworld/infra/storage/world"
	tutu_transport "github.com/tutu-hack/openworld/infra/transport/tutumcp"
	"github.com/tutu-hack/openworld/infra/tutumcp"
	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	http_errors "github.com/tutu-hack/openworld/internal/errors/http"
	adminsim_handler "github.com/tutu-hack/openworld/internal/handlers/adminsim"
	auth_handler "github.com/tutu-hack/openworld/internal/handlers/auth"
	community_handler "github.com/tutu-hack/openworld/internal/handlers/community"
	events_handler "github.com/tutu-hack/openworld/internal/handlers/events"
	health_handler "github.com/tutu-hack/openworld/internal/handlers/health"
	profile_handler "github.com/tutu-hack/openworld/internal/handlers/profile"
	recommendations_handler "github.com/tutu-hack/openworld/internal/handlers/recommendations"
	rewards_handler "github.com/tutu-hack/openworld/internal/handlers/rewards"
	trips_handler "github.com/tutu-hack/openworld/internal/handlers/trips"
	world_handler "github.com/tutu-hack/openworld/internal/handlers/world"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/internal/router"
	"github.com/tutu-hack/openworld/resources"
	adminsim_service "github.com/tutu-hack/openworld/services/adminsim"
	auth_service "github.com/tutu-hack/openworld/services/auth"
	community_service "github.com/tutu-hack/openworld/services/community"
	events_service "github.com/tutu-hack/openworld/services/events"
	profile_service "github.com/tutu-hack/openworld/services/profile"
	recommendation_service "github.com/tutu-hack/openworld/services/recommendation"
	rewards_service "github.com/tutu-hack/openworld/services/rewards"
	scoring_service "github.com/tutu-hack/openworld/services/scoring"
	trips_service "github.com/tutu-hack/openworld/services/trips"
	world_service "github.com/tutu-hack/openworld/services/world"
)

type API struct {
	resources *resources.Resources
}

const (
	apiBodyLimit       = 1 << 20
	apiReadTimeout     = 10 * time.Second
	apiWriteTimeout    = 55 * time.Second
	apiIdleTimeout     = 75 * time.Second
	mcpRequestTimeout  = 45 * time.Second
	mcpMaxResponseSize = 2 << 20

	popularRefreshInterval = 30 * time.Minute
	prewarmInterval        = 10 * time.Minute
	minimumPromptRunes     = 100
	maximumPromptRunes     = 8_000
)

func New(resources *resources.Resources) *API {
	return &API{resources: resources}
}

func (a *API) Start(ctx context.Context) error {
	environment := a.resources.Env
	responder := http_errors.NewResponder(a.resources.Logger)
	fiberConfig := fiber.Config{
		AppName:               "Открывай API",
		BodyLimit:             apiBodyLimit,
		ReadTimeout:           apiReadTimeout,
		WriteTimeout:          apiWriteTimeout,
		IdleTimeout:           apiIdleTimeout,
		JSONDecoder:           decodeJSON,
		ErrorHandler:          responder.Handle,
		DisableStartupMessage: environment.IsProduction(),
	}

	if len(environment.TrustedProxies) > 0 {
		fiberConfig.EnableTrustedProxyCheck = true
		fiberConfig.TrustedProxies = environment.TrustedProxies
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedFor
	}

	app := fiber.New(fiberConfig)

	authRepository := auth_storage.NewRepository(a.resources.Database, environment.SessionLifetime)
	profileRepository := profile_storage.NewRepository(a.resources.Database)
	worldRepository := world_storage.NewRepository(a.resources.Database)
	eventRepository := events_storage.NewRepository(a.resources.Database)
	recommendationRepository := recommendations_storage.NewRepository(a.resources.Database)
	tripRepository := trips_storage.NewRepository(a.resources.Database)
	rewardRepository := rewards_storage.NewRepository(a.resources.Database)
	communityRepository := community_storage.NewRepository(a.resources.Database)
	adminRepository := adminsim_storage.NewRepository(a.resources.Database)

	aiPrompts, err := worldRepository.AISystemPrompts(ctx)
	if err != nil {
		return err
	}

	aiPrompts = applyAIPromptOverrides(aiPrompts, a.resources.Env)
	if err := validateAIPrompts(aiPrompts); err != nil {
		return err
	}

	transactionManager := postgres.NewTransactionManager(a.resources.Database)
	attemptLimiter := ratelimit.New(a.resources.Redis)

	tutuClient, err := tutumcp.New(
		tutumcp.WithEndpoint(a.resources.Env.TutuMCPURL),
		tutumcp.WithClientInfo("openworld", "1.0.0"),
		tutumcp.WithTimeout(mcpRequestTimeout),
		tutumcp.WithLogger(a.resources.Logger),
		tutumcp.WithMaxResponseBytes(mcpMaxResponseSize),
	)
	if err != nil {
		return err
	}

	defer func(closeContext context.Context) {
		_ = tutuClient.Close(closeContext)
	}(ctx)

	authService := auth_service.New(authRepository, authRepository, transactionManager)
	rewardService := rewards_service.New(rewardRepository)
	worldService := world_service.New(worldRepository, rewardService)
	communityService := community_service.New(
		communityRepository,
		communityRepository,
		communityRepository,
	)
	transportProvider := tutu_transport.NewProvider(a.resources.Database, tutuClient, a.resources.Logger)
	recommendationRanker := deepseek.NewRecommendationRanker(
		a.resources.Env.DeepSeekAPIKey,
		a.resources.Env.DeepSeekBaseURL,
		a.resources.Env.DeepSeekModel,
		aiPrompts,
	)
	recommendationScorer := scoring_service.New(worldRepository)
	recommendationService := recommendation_service.New(recommendation_service.Dependencies{
		Repository:    recommendationRepository,
		Settings:      worldRepository,
		Analyzer:      recommendationRanker,
		SearchPlanner: recommendationRanker,
		TravelSearch:  transportProvider,
		Scorer:        recommendationScorer,
		Explainer:     recommendationRanker,
		Limiter:       attemptLimiter,
		Logger:        a.resources.Logger,
		Quotas: recommendation_service.Quotas{
			PerHour: environment.RecommendationsPerHour,
			PerDay:  environment.RecommendationsPerDay,
		},
	})
	profileService := profile_service.New(
		profileRepository,
		recommendationService,
		a.resources.Logger,
	)
	eventEnricher := deepseek.NewEventEnricher(
		a.resources.Env.DeepSeekAPIKey,
		a.resources.Env.DeepSeekBaseURL,
		a.resources.Env.DeepSeekModel,
		aiPrompts.EventEnrichment,
	)
	eventDiscoverer := deepseek.NewEventDiscoverer(
		a.resources.Env.DeepSeekAPIKey,
		a.resources.Env.DeepSeekBaseURL,
		a.resources.Env.DeepSeekSearchModel,
		environment.EventDiscoveryTimeout,
	)
	eventService := events_service.New(events_service.Dependencies{
		Repository: eventRepository,
		Enricher:   eventEnricher,
		Discoverer: eventDiscoverer,
		Logger:     a.resources.Logger,
		Config: events_service.Config{
			DiscoveryEnabled: environment.EventDiscoveryEnabled,
			CacheTTL:         environment.EventDiscoveryTTL,
			FailureBackoff:   environment.EventDiscoveryRetryBackoff,
			RunTimeout:       environment.EventDiscoveryTimeout,
			CityLimit:        environment.EventDiscoveryCityLimit,
			PopularLimit:     environment.EventDiscoveryPopularLimit,
			PopularCityPool:  environment.EventDiscoveryCityPool,
			PrewarmCities:    environment.EventDiscoveryPrewarmCities,
			WindowDays:       environment.EventDiscoveryWindowDays,
			Concurrency:      environment.EventDiscoveryConcurrency,
		},
	})
	checkoutCreator := tutu_checkout.NewCreator(tutuClient, environment.CheckoutAllowedHosts)
	tripService := trips_service.New(tripRepository, checkoutCreator, tripRepository)
	adminService := adminsim_service.New(
		adminRepository,
		adminRepository,
		a.resources.Env.AdminSimulatorEnabled,
	)

	middleware := middlewares.New(
		a.resources.Logger,
		authService,
		middlewares.Config{
			AllowedOrigins:    environment.AllowedOrigins,
			SecureCookies:     environment.IsProduction(),
			SessionLifetime:   environment.SessionLifetime,
			RequestsPerMinute: environment.RequestsPerMinute,
		},
	)
	middleware.SetGlobal(app)

	handlers := router.Handlers{
		Health: health_handler.NewHandler(a.resources, a.resources.Env.DemoMode),
		Auth: auth_handler.NewHandler(
			authService,
			middleware,
			attemptLimiter,
			a.resources.Logger,
			environment.LoginAttemptsPerHour,
		),
		Profile:         profile_handler.NewHandler(profileService, worldService),
		World:           world_handler.NewHandler(worldService, communityService, recommendationService, a.resources.Env.DemoMode),
		Events:          events_handler.NewHandler(eventService, worldService, recommendationService),
		Recommendations: recommendations_handler.NewHandler(recommendationService),
		Trips:           trips_handler.NewHandler(tripService),
		Rewards:         rewards_handler.NewHandler(rewardService),
		Community:       community_handler.NewHandler(communityService),
		AdminSimulation: adminsim_handler.NewHandler(adminService),
	}
	router.New(app, middleware, handlers).Register()

	if environment.EventDiscoveryEnabled {
		go watchPopularEvents(ctx, eventService, popularRefreshInterval)
		go watchCityPrewarm(ctx, eventService, prewarmInterval)
	}

	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- app.Listen(":" + environment.HTTPPort)
	}()

	select {
	case <-ctx.Done():
		return app.ShutdownWithTimeout(environment.ShutdownPeriod)
	case err := <-errorChannel:
		return err
	}
}

func watchPopularEvents(
	ctx context.Context,
	service *events_service.Service,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	service.RefreshPopular(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.RefreshPopular(ctx)
		}
	}
}

func watchCityPrewarm(
	ctx context.Context,
	service *events_service.Service,
	interval time.Duration,
) {
	for {
		service.PrewarmCities(ctx)

		timer := time.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()

			return
		case <-timer.C:
		}
	}
}

func applyAIPromptOverrides(
	prompts domain.AISystemPrompts,
	environment *resources.Env,
) domain.AISystemPrompts {
	if value := strings.TrimSpace(environment.DeepSeekRequestAnalysisPrompt); value != "" {
		prompts.RequestAnalysis = value
	}

	if value := strings.TrimSpace(environment.DeepSeekTravelSearchPrompt); value != "" {
		prompts.TravelSearchPlan = value
	}

	if value := strings.TrimSpace(environment.DeepSeekExplanationPrompt); value != "" {
		prompts.RecommendationExplanation = value
	}

	if value := strings.TrimSpace(environment.DeepSeekEventEnrichmentPrompt); value != "" {
		prompts.EventEnrichment = value
	}

	return prompts
}

func validateAIPrompts(prompts domain.AISystemPrompts) error {
	values := []string{
		prompts.RequestAnalysis,
		prompts.TravelSearchPlan,
		prompts.RecommendationExplanation,
		prompts.EventEnrichment,
	}
	for _, value := range values {
		length := len([]rune(strings.TrimSpace(value)))
		if length < minimumPromptRunes || length > maximumPromptRunes {
			return ai_errors.ErrPromptConfiguration
		}
	}

	return nil
}

func decodeJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(value); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fiber.ErrBadRequest
	}

	return nil
}

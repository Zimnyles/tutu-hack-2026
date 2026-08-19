package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/tutu-hack/openworld/internal/middlewares"
)

type Handlers struct {
	Health          HealthHandler
	Auth            AuthHandler
	Profile         ProfileHandler
	World           WorldHandler
	Events          EventsHandler
	Recommendations RecommendationsHandler
	Trips           TripsHandler
	Rewards         RewardsHandler
	Community       CommunityHandler
	AdminSimulation AdminSimulationHandler
}

type Router struct {
	app        *fiber.App
	middleware *middlewares.Middleware
	handlers   Handlers
}

func New(
	app *fiber.App,
	middleware *middlewares.Middleware,
	handlers Handlers,
) *Router {
	return &Router{app: app, middleware: middleware, handlers: handlers}
}

func (r *Router) Register() {
	r.app.Get("/healthz", r.handlers.Health.Live)
	r.app.Get("/readyz", r.handlers.Health.Ready)

	v1 := r.app.Group("/api/v1")
	v1.Get("/config", r.handlers.World.Settings)
	v1.Post("/auth/register", r.handlers.Auth.Register)
	v1.Post("/auth/login", r.handlers.Auth.Login)
	v1.Get("/auth/me", r.middleware.Authenticate, r.handlers.Auth.Me)
	v1.Post(
		"/auth/logout",
		r.middleware.Authenticate,
		r.middleware.VerifyCSRF,
		r.handlers.Auth.Logout,
	)

	private := v1.Group("", r.middleware.Authenticate, r.middleware.VerifyCSRF)
	r.registerProfile(private)
	r.registerWorld(private)
	r.registerEvents(private)
	r.registerRecommendations(private)
	r.registerTrips(private)
	r.registerRewards(private)
	r.registerCommunity(private)
	r.registerAdminSimulation(private)
}

func (r *Router) registerProfile(private fiber.Router) {
	private.Get("/profile", r.handlers.Profile.Get)
	private.Put("/profile/preferences", r.handlers.Profile.SavePreferences)
	private.Post("/profile/onboarding/complete", r.handlers.Profile.CompleteOnboarding)
	private.Put("/profile/home-city", r.handlers.Profile.SetHomeCity)
	private.Put("/profile/travel-visibility", r.handlers.Profile.SetVisibility)
	private.Post("/integrations/tutu/demo-sync", r.handlers.World.DemoSync)
}

func (r *Router) registerWorld(private fiber.Router) {
	private.Get("/world/bootstrap", r.handlers.World.Bootstrap)
	private.Get("/world/progress", r.handlers.World.Progress)
	private.Get("/territories", r.handlers.World.Territories)
	private.Get("/territories/:id", r.handlers.World.Territory)
}

func (r *Router) registerEvents(private fiber.Router) {
	private.Get("/territories/:id/events", r.handlers.Events.List)
	private.Post("/territories/:id/events/refresh", r.handlers.Events.Refresh)
	private.Get("/events/popular", r.handlers.Events.Popular)
	private.Get("/events/:id", r.handlers.Events.Get)
	private.Post("/events/:id/plan-trip", r.handlers.Events.PlanTrip)
}

func (r *Router) registerRecommendations(private fiber.Router) {
	private.Post("/recommendations", r.handlers.Recommendations.Create)
	private.Get("/recommendations/personal", r.handlers.Recommendations.Personal)
	private.Post("/recommendations/personal/refresh", r.handlers.Recommendations.RefreshPersonal)
	private.Get("/recommendations/:id/events", r.handlers.Recommendations.Events)
	private.Get("/recommendations/:id", r.handlers.Recommendations.Get)
	private.Post("/recommendations/:id/select", r.handlers.Trips.SelectOption)
}

func (r *Router) registerTrips(private fiber.Router) {
	private.Get("/trips", r.handlers.Trips.List)
	private.Post("/trips/:id/checkout", r.handlers.Trips.Checkout)
	private.Post("/trips/:id/simulate-arrival", r.handlers.Trips.Arrive)
}

func (r *Router) registerRewards(private fiber.Router) {
	private.Get("/rewards/ledger", r.handlers.Rewards.Ledger)
	private.Get("/rewards/promo-codes", r.handlers.Rewards.PromoCodes)
	private.Get("/achievements", r.handlers.Rewards.Achievements)
}

func (r *Router) registerCommunity(private fiber.Router) {
	private.Get("/season/current", r.handlers.Community.Season)
	private.Get("/leaderboard", r.handlers.Community.Leaderboard)
	private.Get("/guild", r.handlers.Community.Guild)
	private.Post("/guild/join", r.handlers.Community.JoinGuild)
	private.Post("/guild/leave", r.handlers.Community.LeaveGuild)
	private.Get("/travel-cohorts/:territoryId", r.handlers.Community.Cohort)
}

func (r *Router) registerAdminSimulation(private fiber.Router) {
	admin := private.Group("/admin")
	admin.Get("/overview", r.handlers.AdminSimulation.Overview)
	admin.Get("/users", r.handlers.AdminSimulation.Users)
	admin.Get("/scenarios", r.handlers.AdminSimulation.Scenarios)
	admin.Post("/simulations", r.handlers.AdminSimulation.Execute)
	admin.Get("/simulations/:id", r.handlers.AdminSimulation.Simulation)
	admin.Get("/audit-log", r.handlers.AdminSimulation.AuditLog)
}

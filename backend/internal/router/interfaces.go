package router

import "github.com/gofiber/fiber/v2"

type HealthHandler interface {
	Live(*fiber.Ctx) error
	Ready(*fiber.Ctx) error
}

type AuthHandler interface {
	Register(*fiber.Ctx) error
	Login(*fiber.Ctx) error
	Me(*fiber.Ctx) error
	Logout(*fiber.Ctx) error
}

type ProfileHandler interface {
	Get(*fiber.Ctx) error
	SavePreferences(*fiber.Ctx) error
	CompleteOnboarding(*fiber.Ctx) error
	SetHomeCity(*fiber.Ctx) error
	SetVisibility(*fiber.Ctx) error
}

type WorldHandler interface {
	Settings(*fiber.Ctx) error
	Bootstrap(*fiber.Ctx) error
	Territories(*fiber.Ctx) error
	Territory(*fiber.Ctx) error
	Progress(*fiber.Ctx) error
	DemoSync(*fiber.Ctx) error
}

type EventsHandler interface {
	List(*fiber.Ctx) error
	Popular(*fiber.Ctx) error
	Get(*fiber.Ctx) error
	Refresh(*fiber.Ctx) error
	PlanTrip(*fiber.Ctx) error
}

type RecommendationsHandler interface {
	Create(*fiber.Ctx) error
	Get(*fiber.Ctx) error
	Personal(*fiber.Ctx) error
	RefreshPersonal(*fiber.Ctx) error
	Events(*fiber.Ctx) error
}

type TripsHandler interface {
	SelectOption(*fiber.Ctx) error
	List(*fiber.Ctx) error
	Checkout(*fiber.Ctx) error
	Arrive(*fiber.Ctx) error
}

type RewardsHandler interface {
	Ledger(*fiber.Ctx) error
	Achievements(*fiber.Ctx) error
	PromoCodes(*fiber.Ctx) error
}

type CommunityHandler interface {
	Season(*fiber.Ctx) error
	Leaderboard(*fiber.Ctx) error
	Guild(*fiber.Ctx) error
	JoinGuild(*fiber.Ctx) error
	LeaveGuild(*fiber.Ctx) error
	Cohort(*fiber.Ctx) error
}

type AdminSimulationHandler interface {
	Overview(*fiber.Ctx) error
	Users(*fiber.Ctx) error
	Scenarios(*fiber.Ctx) error
	Execute(*fiber.Ctx) error
	Simulation(*fiber.Ctx) error
	AuditLog(*fiber.Ctx) error
}

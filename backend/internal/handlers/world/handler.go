package world_handler

import (
	"github.com/gofiber/fiber/v2"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	"github.com/tutu-hack/openworld/internal/handlers/httpx"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Handler struct {
	world           WorldService
	seasons         SeasonService
	recommendations PersonalRecommendationService
	demoMode        bool
}

const percentageBase = 100

func NewHandler(
	world WorldService,
	seasons SeasonService,
	recommendations PersonalRecommendationService,
	demoMode bool,
) *Handler {
	return &Handler{world: world, seasons: seasons, recommendations: recommendations, demoMode: demoMode}
}

func (h *Handler) Settings(c *fiber.Ctx) error {
	settings, err := h.world.Settings(c.UserContext())
	if err != nil {
		return err
	}

	return c.JSON(settings)
}

func (h *Handler) Bootstrap(c *fiber.Ctx) error {
	user := middlewares.User(c)

	territories, balance, err := h.world.Bootstrap(c.UserContext(), user)
	if err != nil {
		return err
	}

	season, err := h.seasons.CurrentSeason(c.UserContext(), user.ID)
	if err != nil {
		return err
	}

	personalRecommendation, found, err := h.recommendations.Latest(c.UserContext(), user.ID, "personal")
	if err != nil {
		return err
	}

	if !found && user.OnboardingCompleted {
		personalRecommendation, err = h.recommendations.CreatePersonalized(c.UserContext(), user)
		if err != nil {
			return err
		}

		found = true
	}

	if found && user.OnboardingCompleted && exhaustedRecommendation(personalRecommendation) {
		if restarted, restartErr := h.recommendations.CreatePersonalized(c.UserContext(), user); restartErr == nil {
			personalRecommendation = restarted
		}
	}

	return c.JSON(fiber.Map{
		"user":                    user,
		"territories":             territories,
		"balance":                 balance,
		"season":                  season,
		"personal_recommendation": personalRecommendation,
		"demo_mode":               h.demoMode,
	})
}

func exhaustedRecommendation(recommendation domain.RecommendationRequest) bool {
	switch recommendation.Status {
	case "failed", "blocked":
		return true
	case "completed", "partial":
		return len(recommendation.Options) == 0
	default:
		return false
	}
}

func (h *Handler) Territories(c *fiber.Ctx) error {
	user := middlewares.User(c)

	territories, _, err := h.world.Bootstrap(c.UserContext(), user)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": territories})
}

func (h *Handler) Territory(c *fiber.Ctx) error {
	territoryID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	territory, found, err := h.world.Territory(
		c.UserContext(),
		middlewares.User(c).ID,
		territoryID,
	)
	if err != nil {
		return err
	}

	if !found {
		return common_errors.ErrNotFound
	}

	return c.JSON(fiber.Map{"territory": territory})
}

func (h *Handler) Progress(c *fiber.Ctx) error {
	user := middlewares.User(c)

	territories, _, err := h.world.Bootstrap(c.UserContext(), user)
	if err != nil {
		return err
	}

	opened := 0

	for _, territory := range territories {
		if territory.State == "arrived" {
			opened++
		}
	}

	percentage := 0.0
	if len(territories) > 0 {
		percentage = float64(opened) / float64(len(territories)) * percentageBase
	}

	return c.JSON(fiber.Map{
		"opened":        opened,
		"total":         len(territories),
		"world_percent": percentage,
	})
}

func (h *Handler) DemoSync(c *fiber.Ctx) error {
	territories, err := h.world.SyncDemoHistory(
		c.UserContext(),
		middlewares.User(c).ID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"demo":        true,
		"source":      "prepared_fixture",
		"territories": territories,
	})
}

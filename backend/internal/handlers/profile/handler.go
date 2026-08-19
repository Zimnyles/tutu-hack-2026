package profile_handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/internal/models/dto"
)

type Handler struct {
	service ProfileService
	world   WorldReader
}

const percentageBase = 100

func NewHandler(service ProfileService, world WorldReader) *Handler {
	return &Handler{service: service, world: world}
}

func (h *Handler) Get(c *fiber.Ctx) error {
	user := middlewares.User(c)

	territories, balance, err := h.world.Bootstrap(c.UserContext(), user)
	if err != nil {
		return err
	}

	opened := 0

	for _, territory := range territories {
		if territory.State == "arrived" {
			opened++
		}
	}

	worldPercent := 0.0
	if len(territories) > 0 {
		worldPercent = float64(opened) / float64(len(territories)) * percentageBase
	}

	return c.JSON(fiber.Map{
		"user": user,
		"progress": fiber.Map{
			"opened":        opened,
			"total":         len(territories),
			"world_percent": worldPercent,
		},
		"balance": balance,
	})
}

func (h *Handler) SavePreferences(c *fiber.Ctx) error {
	var request dto.PreferencesRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	user, err := h.service.SavePreferences(
		c.UserContext(),
		middlewares.User(c),
		domain.Preferences{
			Themes:           request.Themes,
			TransportModes:   request.TransportModes,
			MaxTravelMinutes: request.MaxTravelMinutes,
			TypicalBudget:    request.TypicalBudget,
			TripDurationDays: request.TripDurationDays,
			Spontaneity:      request.Spontaneity,
			Avoid:            request.Avoid,
		},
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"user": user})
}

func (h *Handler) CompleteOnboarding(c *fiber.Ctx) error {
	var request dto.HomeCityRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	user, err := h.service.CompleteOnboarding(
		c.UserContext(),
		middlewares.User(c),
		request.HomeCityID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"user": user})
}

func (h *Handler) SetHomeCity(c *fiber.Ctx) error {
	var request dto.HomeCityRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	user, err := h.service.SetHomeCity(
		c.UserContext(),
		middlewares.User(c),
		request.HomeCityID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"user": user})
}

func (h *Handler) SetVisibility(c *fiber.Ctx) error {
	var request dto.VisibilityRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	user, err := h.service.SetTravelVisibility(
		c.UserContext(),
		middlewares.User(c),
		request.Visibility,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"user": user})
}

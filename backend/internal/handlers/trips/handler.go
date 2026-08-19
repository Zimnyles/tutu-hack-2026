package trips_handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	trips_errors "github.com/tutu-hack/openworld/internal/errors/trips"
	"github.com/tutu-hack/openworld/internal/handlers/httpx"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/dto"
)

const (
	minimumIdempotencyKeyLength = 8
	maximumIdempotencyKeyLength = 128
)

type Handler struct {
	service TripService
}

func NewHandler(service TripService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SelectOption(c *fiber.Ctx) error {
	var request dto.SelectOptionRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	recommendationID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	optionID, err := httpx.Identifier(request.OptionID)
	if err != nil {
		return err
	}

	trip, err := h.service.SelectOption(
		c.UserContext(),
		middlewares.User(c).ID,
		recommendationID,
		optionID,
	)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"trip": trip})
}

func (h *Handler) List(c *fiber.Ctx) error {
	trips, err := h.service.List(c.UserContext(), middlewares.User(c).ID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": trips})
}

func (h *Handler) Checkout(c *fiber.Ctx) error {
	tripID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	trip, checkoutKind, demo, err := h.service.CreateCheckout(
		c.UserContext(),
		middlewares.User(c).ID,
		tripID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"trip":          trip,
		"checkout_url":  trip.CheckoutURL,
		"checkout_kind": checkoutKind,
		"demo":          demo,
	})
}

func (h *Handler) Arrive(c *fiber.Ctx) error {
	tripID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	idempotencyKey := strings.TrimSpace(c.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		return trips_errors.ErrMissingIdempotencyKey
	}

	if len(idempotencyKey) < minimumIdempotencyKeyLength ||
		len(idempotencyKey) > maximumIdempotencyKeyLength {
		return httpx.ValidationError("Ключ идемпотентности должен содержать от 8 до 128 символов")
	}

	trip, reward, replayed, err := h.service.SimulateArrival(
		c.UserContext(),
		middlewares.User(c).ID,
		tripID,
		idempotencyKey,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"trip":     trip,
		"reward":   reward,
		"replayed": replayed,
	})
}

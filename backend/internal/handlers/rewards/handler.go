package rewards_handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/tutu-hack/openworld/internal/middlewares"
)

type Handler struct {
	service RewardService
}

func NewHandler(service RewardService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Ledger(c *fiber.Ctx) error {
	balance, entries, err := h.service.Ledger(
		c.UserContext(),
		middlewares.User(c).ID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"balance": balance, "items": entries})
}

func (h *Handler) Achievements(c *fiber.Ctx) error {
	items, err := h.service.Achievements(
		c.UserContext(),
		middlewares.User(c).ID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": items})
}

func (h *Handler) PromoCodes(c *fiber.Ctx) error {
	items, err := h.service.PromoCodes(
		c.UserContext(),
		middlewares.User(c).ID,
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": items})
}

package health_handler

import "github.com/gofiber/fiber/v2"

type Handler struct {
	checker  Checker
	demoMode bool
}

func NewHandler(checker Checker, demoMode bool) *Handler {
	return &Handler{checker: checker, demoMode: demoMode}
}

func (h *Handler) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "demo_mode": h.demoMode})
}

func (h *Handler) Ready(c *fiber.Ctx) error {
	if err := h.checker.Check(c.UserContext()); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"status": "ready", "demo_mode": h.demoMode})
}

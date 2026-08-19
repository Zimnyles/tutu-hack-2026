package adminsim_handler

import (
	"github.com/gofiber/fiber/v2"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/internal/models/dto"
)

type Handler struct {
	service AdminSimulationService
}

func NewHandler(service AdminSimulationService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Overview(c *fiber.Ctx) error {
	overview, err := h.service.Overview(c.UserContext(), middlewares.User(c))
	if err != nil {
		return err
	}

	return c.JSON(overview)
}

func (h *Handler) Users(c *fiber.Ctx) error {
	users, err := h.service.Users(
		c.UserContext(),
		middlewares.User(c),
		c.Query("query"),
	)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": users})
}

func (h *Handler) Scenarios(c *fiber.Ctx) error {
	scenarios, err := h.service.Scenarios(c.UserContext(), middlewares.User(c))
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": scenarios})
}

func (h *Handler) Execute(c *fiber.Ctx) error {
	var request dto.AdminExecuteRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	simulation, err := h.service.Execute(
		c.UserContext(),
		middlewares.User(c),
		domain.AdminSimulationCommand{
			ActionCode:     request.ActionCode,
			TargetType:     request.TargetType,
			TargetID:       request.TargetID,
			DemoScenarioID: request.DemoScenarioID,
			IdempotencyKey: c.Get("Idempotency-Key"),
			Reason:         request.Reason,
			Parameters:     request.Parameters,
			RequestID:      c.GetRespHeader(fiber.HeaderXRequestID),
		},
	)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(simulation)
}

func (h *Handler) Simulation(c *fiber.Ctx) error {
	simulation, found, err := h.service.Simulation(
		c.UserContext(),
		middlewares.User(c),
		c.Params("id"),
	)
	if err != nil {
		return err
	}

	if !found {
		return common_errors.ErrNotFound
	}

	return c.JSON(simulation)
}

func (h *Handler) AuditLog(c *fiber.Ctx) error {
	entries, err := h.service.AuditLog(c.UserContext(), middlewares.User(c))
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": entries})
}

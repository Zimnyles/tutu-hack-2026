package community_handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/tutu-hack/openworld/internal/handlers/httpx"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/dto"
)

type Handler struct {
	service CommunityService
}

func NewHandler(service CommunityService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Season(c *fiber.Ctx) error {
	season, err := h.service.CurrentSeason(c.UserContext(), middlewares.User(c).ID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"season": season})
}

func (h *Handler) Leaderboard(c *fiber.Ctx) error {
	leaderboard, err := h.service.Leaderboard(
		c.UserContext(),
		c.Query("scope", "league"),
		c.Query("period", "month"),
	)
	if err != nil {
		return err
	}

	return c.JSON(leaderboard)
}

func (h *Handler) Guild(c *fiber.Ctx) error {
	guild, err := h.service.Guild(c.UserContext(), middlewares.User(c).ID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"guild": guild})
}

func (h *Handler) JoinGuild(c *fiber.Ctx) error {
	var request dto.JoinGuildRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	guildID, err := httpx.Identifier(request.GuildID)
	if err != nil {
		return err
	}

	guild, err := h.service.JoinGuild(c.UserContext(), middlewares.User(c).ID, guildID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"guild": guild})
}

func (h *Handler) LeaveGuild(c *fiber.Ctx) error {
	if err := h.service.LeaveGuild(c.UserContext(), middlewares.User(c).ID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Cohort(c *fiber.Ctx) error {
	territoryID, err := httpx.RequiredIdentifier(c, "territoryId")
	if err != nil {
		return err
	}

	cohort, err := h.service.TravelCohort(c.UserContext(), middlewares.User(c).ID, territoryID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"cohort": cohort})
}

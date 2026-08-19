package events_handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	events_errors "github.com/tutu-hack/openworld/internal/errors/events"
	"github.com/tutu-hack/openworld/internal/handlers/httpx"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/dto"
	"github.com/tutu-hack/openworld/services/events"
	"github.com/tutu-hack/openworld/services/recommendation"
)

type Handler struct {
	events          EventService
	world           WorldReader
	recommendations RecommendationCreator
}

const (
	defaultEventWindowMonths = 3
	maximumEventPrice        = 5_000_000
	maximumFilterLength      = 64
)

func NewHandler(
	eventService EventService,
	world WorldReader,
	recommendations RecommendationCreator,
) *Handler {
	return &Handler{
		events:          eventService,
		world:           world,
		recommendations: recommendations,
	}
}

func (h *Handler) List(c *fiber.Ctx) error {
	territoryID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	priceMax, err := strconv.Atoi(c.Query("price_max", "0"))
	if err != nil || priceMax < 0 || priceMax > maximumEventPrice {
		return httpx.ValidationError("Проверьте максимальную цену")
	}

	filters, err := eventFilters(c, priceMax)
	if err != nil {
		return err
	}

	city, found, err := h.world.Territory(c.UserContext(), middlewares.User(c).ID, territoryID)
	if err != nil {
		return err
	}

	if !found {
		return common_errors.ErrNotFound
	}

	listing, err := h.events.ListCity(c.UserContext(), city, filters)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"items":       listing.Items,
		"discovering": listing.Discovering,
		"catalog": fiber.Map{
			"mode":         "ai_web_search",
			"refreshed_at": listing.RefreshedAt,
			"stale":        listing.Discovering,
		},
	})
}

func (h *Handler) Popular(c *fiber.Ctx) error {
	listing, err := h.events.Popular(c.UserContext())
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"items":        listing.Items,
		"discovering":  listing.Discovering,
		"refreshed_at": listing.RefreshedAt,
	})
}

func eventFilters(c *fiber.Ctx, priceMax int) (events.Filters, error) {
	category, err := optionalFilter(c.Query("category"))
	if err != nil {
		return events.Filters{}, err
	}

	ageRating, err := optionalFilter(c.Query("age_rating"))
	if err != nil {
		return events.Filters{}, err
	}

	availability, err := optionalFilter(c.Query("availability"))
	if err != nil {
		return events.Filters{}, err
	}

	dateFrom, err := optionalDate(c.Query("date_from"))
	if err != nil {
		return events.Filters{}, err
	}

	dateTo, err := optionalDate(c.Query("date_to"))
	if err != nil {
		return events.Filters{}, err
	}

	if !dateFrom.IsZero() && !dateTo.IsZero() && dateTo.Before(dateFrom) {
		return events.Filters{}, httpx.ValidationError("Дата окончания раньше даты начала")
	}

	return events.Filters{
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		Category:     category,
		PriceMax:     priceMax,
		AgeRating:    ageRating,
		Availability: availability,
	}, nil
}

func (h *Handler) Get(c *fiber.Ctx) error {
	eventID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	event, found, err := h.events.Get(c.UserContext(), eventID)
	if err != nil {
		return err
	}

	if !found {
		return events_errors.ErrNotFound
	}

	return c.JSON(fiber.Map{"event": event})
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	user := middlewares.User(c)

	territoryID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	city, found, err := h.world.Territory(c.UserContext(), user.ID, territoryID)
	if err != nil {
		return err
	}

	if !found {
		return common_errors.ErrNotFound
	}

	dateFrom, err := optionalDate(c.Query("date_from"))
	if err != nil {
		return err
	}

	dateTo, err := optionalDate(c.Query("date_to"))
	if err != nil {
		return err
	}

	if dateFrom.IsZero() {
		dateFrom = time.Now().UTC()
	}

	if dateTo.IsZero() {
		dateTo = dateFrom.AddDate(0, defaultEventWindowMonths, 0)
	}

	items, err := h.events.Refresh(c.UserContext(), city, dateFrom, dateTo)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": items, "source": "database", "enriched_by": "deepseek"})
}

func (h *Handler) PlanTrip(c *fiber.Ctx) error {
	eventID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	event, found, err := h.events.Get(c.UserContext(), eventID)
	if err != nil {
		return err
	}

	if !found {
		return events_errors.ErrNotFound
	}

	if event.EndsAt.Before(time.Now()) || event.Status == "cancelled" {
		return events_errors.ErrUnavailable
	}

	var request dto.SearchRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	if request.EventID != "" && request.EventID != event.ID {
		return httpx.ValidationError("Событие в запросе не совпадает с выбранным")
	}

	if request.DestinationID != "" && request.DestinationID != event.CityID {
		return httpx.ValidationError("Город события не совпадает с выбранным")
	}

	if request.DateFrom == "" {
		request.DateFrom = event.StartsAt.AddDate(0, 0, -1).Format(time.DateOnly)
	}

	if request.DateTo == "" {
		request.DateTo = event.EndsAt.AddDate(0, 0, 1).Format(time.DateOnly)
	}

	result, err := h.recommendations.Create(
		c.UserContext(),
		middlewares.User(c),
		recommendation.Input{
			Kind:             "event",
			OriginCityID:     request.OriginCityID,
			DestinationID:    event.CityID,
			EventID:          event.ID,
			DateFrom:         request.DateFrom,
			DateTo:           request.DateTo,
			Adults:           request.Adults,
			Children:         request.Children,
			Budget:           request.Budget,
			Currency:         request.Currency,
			TransportModes:   request.TransportModes,
			MaxTravelMinutes: request.MaxTravelMinutes,
			DirectOnly:       request.DirectOnly,
			Prompt:           request.Prompt,
			RequestID:        c.GetRespHeader(fiber.HeaderXRequestID),
		},
	)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(result)
}

func optionalDate(rawValue string) (time.Time, error) {
	if strings.TrimSpace(rawValue) == "" {
		return time.Time{}, nil
	}

	return httpx.Date(rawValue)
}

func optionalFilter(rawValue string) (string, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return "", nil
	}

	return httpx.BoundedText(trimmed, 1, maximumFilterLength, "Слишком длинное значение фильтра")
}

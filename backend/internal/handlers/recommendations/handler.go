package recommendations_handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	"github.com/tutu-hack/openworld/internal/handlers/httpx"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/dto"
	"github.com/tutu-hack/openworld/services/recommendation"
)

const (
	recommendationSSEPollInterval = 500 * time.Millisecond
	recommendationSSELifetime     = 50 * time.Second
)

type Handler struct {
	service RecommendationService
}

func NewHandler(service RecommendationService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var request dto.SearchRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	result, err := h.service.Create(
		c.UserContext(),
		middlewares.User(c),
		recommendation.Input{
			OriginCityID:     request.OriginCityID,
			DestinationID:    request.DestinationID,
			EventID:          request.EventID,
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

func (h *Handler) Get(c *fiber.Ctx) error {
	recommendationID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	result, found, err := h.service.Get(c.UserContext(), middlewares.User(c).ID, recommendationID)
	if err != nil {
		return err
	}

	if !found {
		return common_errors.ErrNotFound
	}

	return c.JSON(result)
}

func (h *Handler) Personal(c *fiber.Ctx) error {
	result, found, err := h.service.Latest(
		c.UserContext(),
		middlewares.User(c).ID,
		"personal",
	)
	if err != nil {
		return err
	}

	if !found {
		return common_errors.ErrNotFound
	}

	return c.JSON(result)
}

func (h *Handler) RefreshPersonal(c *fiber.Ctx) error {
	result, err := h.service.RebuildPersonalized(c.UserContext(), middlewares.User(c))
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(result)
}

func (h *Handler) Events(c *fiber.Ctx) error {
	userID := middlewares.User(c).ID

	recommendationID, err := httpx.RequiredIdentifier(c, "id")
	if err != nil {
		return err
	}

	if _, found, err := h.service.Get(c.UserContext(), userID, recommendationID); err != nil {
		return err
	} else if !found {
		return common_errors.ErrNotFound
	}

	streamContext, cancel := context.WithTimeout(
		context.WithoutCancel(c.UserContext()),
		recommendationSSELifetime,
	)

	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Context().SetBodyStreamWriter(func(writer *bufio.Writer) {
		defer cancel()
		h.streamRecommendation(streamContext, writer, userID, recommendationID)
	})

	return nil
}

func (h *Handler) streamRecommendation(
	ctx context.Context,
	writer *bufio.Writer,
	userID string,
	recommendationID string,
) {
	ticker := time.NewTicker(recommendationSSEPollInterval)
	defer ticker.Stop()

	lastState := ""

	for {
		current, found, err := h.service.Get(ctx, userID, recommendationID)
		if err != nil || !found {
			_ = writeSSE(
				writer,
				"error",
				time.Now().UnixNano(),
				map[string]string{"code": "RECOMMENDATION_STREAM_FAILED"},
			)

			return
		}

		state := current.Status + "|" + current.Stage
		if state != lastState {
			if err := writeSSE(writer, "recommendation", time.Now().UnixNano(), current); err != nil {
				return
			}

			lastState = state
		}

		if terminalRecommendationStatus(current.Status) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func writeSSE(writer *bufio.Writer, event string, id int64, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode recommendation SSE payload: %w", err)
	}

	if _, err := fmt.Fprintf(writer, "id: %d\nevent: %s\ndata: %s\n\n", id, event, encoded); err != nil {
		return fmt.Errorf("write recommendation SSE payload: %w", err)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush recommendation SSE payload: %w", err)
	}

	return nil
}

func terminalRecommendationStatus(status string) bool {
	return status == "completed" || status == "partial" || status == "blocked" || status == "failed"
}

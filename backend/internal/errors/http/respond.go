package http_errors

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	adminsim_errors "github.com/tutu-hack/openworld/internal/errors/adminsim"
	auth_errors "github.com/tutu-hack/openworld/internal/errors/auth"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	events_errors "github.com/tutu-hack/openworld/internal/errors/events"
	mcp_errors "github.com/tutu-hack/openworld/internal/errors/mcp"
	recommendation_errors "github.com/tutu-hack/openworld/internal/errors/recommendation"
	trips_errors "github.com/tutu-hack/openworld/internal/errors/trips"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Responder struct {
	logger *slog.Logger
}

func NewResponder(logger *slog.Logger) *Responder {
	return &Responder{logger: logger}
}

func (r *Responder) Handle(c *fiber.Ctx, err error) error {
	status, code, message := responseFor(err)
	if status >= fiber.StatusInternalServerError {
		r.logger.Error(
			"http request failed",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"error", err,
		)
	}

	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":       code,
			"message":    message,
			"request_id": c.GetRespHeader(fiber.HeaderXRequestID),
			"details":    []string{},
		},
	})
}

func responseFor(err error) (int, string, string) {
	var appError *domain.AppError
	if errors.As(err, &appError) {
		return appError.Status, appError.Code, appError.Message
	}

	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		return fiberError.Code, "INVALID_INPUT", "Проверьте данные запроса"
	}

	switch {
	case errors.Is(err, auth_errors.ErrCredentials):
		return fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Неверный email или пароль"
	case errors.Is(err, adminsim_errors.ErrAccessDenied),
		errors.Is(err, adminsim_errors.ErrSimulatorDisabled):
		return fiber.StatusForbidden, "ACCESS_DENIED", "Недостаточно прав для этого действия"
	case errors.Is(err, adminsim_errors.ErrActionNotAllowed):
		return fiber.StatusBadRequest, "ACTION_NOT_ALLOWED", "Это действие недоступно в симуляторе"
	case errors.Is(err, adminsim_errors.ErrInvalidReason):
		return fiber.StatusBadRequest, "INVALID_REASON", "Укажите понятную причину запуска симуляции"
	case errors.Is(err, adminsim_errors.ErrTargetNotDemo):
		return fiber.StatusConflict, "TARGET_NOT_DEMO", "Симулятор работает только с demo-данными"
	case errors.Is(err, adminsim_errors.ErrInvalidIdempotencyKey):
		return fiber.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Для операции нужен корректный Idempotency-Key"
	case errors.Is(err, adminsim_errors.ErrInvalidTarget),
		errors.Is(err, adminsim_errors.ErrInvalidParameters),
		errors.Is(err, adminsim_errors.ErrInvalidQuery):
		return fiber.StatusBadRequest, "INVALID_INPUT", "Проверьте параметры симуляции"
	case errors.Is(err, events_errors.ErrNotFound):
		return fiber.StatusNotFound, "EVENT_NOT_FOUND", "Событие не найдено"
	case errors.Is(err, events_errors.ErrUnavailable):
		return fiber.StatusConflict, "EVENT_UNAVAILABLE", "Событие сейчас недоступно"
	case errors.Is(err, recommendation_errors.ErrExpired):
		return fiber.StatusConflict, "RECOMMENDATION_EXPIRED", "Подборка устарела — соберите новую"
	case errors.Is(err, recommendation_errors.ErrNoDestinations):
		return fiber.StatusUnprocessableEntity, "NO_DESTINATIONS", "Не нашли подходящих направлений"
	case errors.Is(err, trips_errors.ErrTripNotFound), errors.Is(err, common_errors.ErrNotFound):
		return fiber.StatusNotFound, "NOT_FOUND", "Запись не найдена"
	case errors.Is(err, trips_errors.ErrMissingIdempotencyKey):
		return fiber.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Для операции нужен Idempotency-Key"
	case errors.Is(err, trips_errors.ErrInvalidState), errors.Is(err, common_errors.ErrInvalidState):
		return fiber.StatusConflict, "INVALID_STATE", "Действие недоступно в текущем состоянии"
	case errors.Is(err, common_errors.ErrConflict):
		return fiber.StatusConflict, "CONFLICT", "Данные уже были изменены"
	case errors.Is(err, mcp_errors.ErrOriginNotFound):
		return fiber.StatusUnprocessableEntity, "ORIGIN_NOT_FOUND", "Не нашли город отправления"
	case errors.Is(err, mcp_errors.ErrNoOffers):
		return fiber.StatusUnprocessableEntity, "NO_OFFERS", "Не нашли билетов на выбранные даты"
	case errors.Is(err, mcp_errors.ErrCheckoutReferenceMissing),
		errors.Is(err, mcp_errors.ErrCheckoutURLNotAllowed):
		return fiber.StatusBadGateway, "CHECKOUT_UNAVAILABLE", "Оформление сейчас недоступно, попробуйте позже"
	case errors.Is(err, context.DeadlineExceeded):
		return fiber.StatusGatewayTimeout, "UPSTREAM_TIMEOUT", "Сервис отвечает слишком долго"
	case errors.Is(err, context.Canceled):
		return fiber.StatusRequestTimeout, "REQUEST_CANCELLED", "Запрос был отменён"
	default:
		return fiber.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось выполнить запрос"
	}
}

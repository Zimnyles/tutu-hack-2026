package auth_handler

import (
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	auth_errors "github.com/tutu-hack/openworld/internal/errors/auth"
	"github.com/tutu-hack/openworld/internal/middlewares"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/internal/models/dto"
)

const (
	minimumPasswordLength    = 8
	maximumPasswordLength    = 128
	minimumDisplayNameLength = 2
	maximumDisplayNameLength = 64
	maximumEmailLength       = 254
	attemptWindow            = time.Hour
	loginScope               = "login"
	registerScope            = "register"
)

type Handler struct {
	service       AuthService
	cookies       CookieManager
	limiter       AttemptLimiter
	logger        *slog.Logger
	attemptsLimit int
}

func NewHandler(
	service AuthService,
	cookies CookieManager,
	limiter AttemptLimiter,
	logger *slog.Logger,
	attemptsLimit int,
) *Handler {
	return &Handler{
		service:       service,
		cookies:       cookies,
		limiter:       limiter,
		logger:        logger,
		attemptsLimit: attemptsLimit,
	}
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var request dto.RegisterRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		return err
	}

	if err := validatePassword(request.Password); err != nil {
		return err
	}

	displayName, err := normalizeDisplayName(request.DisplayName)
	if err != nil {
		return err
	}

	if err := h.checkAttempts(c, registerScope, c.IP()); err != nil {
		return err
	}

	user, sessionToken, err := h.service.Register(
		c.UserContext(),
		email,
		request.Password,
		displayName,
	)
	if err != nil {
		return err
	}

	if err := h.cookies.SetSessionCookies(c, sessionToken); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": user})
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var request dto.LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.ErrBadRequest
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		return auth_errors.ErrCredentials
	}

	if err := h.checkAttempts(c, loginScope, email); err != nil {
		return err
	}

	if err := h.checkAttempts(c, loginScope, c.IP()); err != nil {
		return err
	}

	user, sessionToken, authenticated, err := h.service.Login(
		c.UserContext(),
		email,
		request.Password,
	)
	if err != nil {
		return err
	}

	if !authenticated {
		return auth_errors.ErrCredentials
	}

	h.resetAttempts(c, email)

	if err := h.cookies.SetSessionCookies(c, sessionToken); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"user": user})
}

func (h *Handler) Me(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"user": middlewares.User(c)})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	if err := h.service.Logout(c.UserContext(), c.Cookies("ow_session")); err != nil {
		return err
	}

	h.cookies.ClearSessionCookies(c)

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) checkAttempts(c *fiber.Ctx, scope string, identity string) error {
	allowed, err := h.limiter.Allow(
		c.UserContext(),
		scope,
		identity,
		h.attemptsLimit,
		attemptWindow,
	)
	if err != nil {
		h.logger.Warn("attempt limiter unavailable", "scope", scope, "error", err)
	}

	if !allowed {
		return &domain.AppError{
			Code:    "RATE_LIMITED",
			Message: "Слишком много попыток, попробуйте позже",
			Status:  fiber.StatusTooManyRequests,
		}
	}

	return nil
}

func (h *Handler) resetAttempts(c *fiber.Ctx, email string) {
	if err := h.limiter.Reset(c.UserContext(), loginScope, email, attemptWindow); err != nil {
		h.logger.Warn("attempt limiter reset failed", "error", err)
	}
}

func normalizeEmail(rawEmail string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(rawEmail))
	if trimmed == "" || len(trimmed) > maximumEmailLength {
		return "", validationError("Укажите корректный email")
	}

	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address != trimmed {
		return "", validationError("Укажите корректный email")
	}

	return trimmed, nil
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < minimumPasswordLength || length > maximumPasswordLength {
		return validationError("Пароль должен содержать от 8 до 128 символов")
	}

	if strings.TrimSpace(password) == "" {
		return validationError("Пароль не может состоять из пробелов")
	}

	return nil
}

func normalizeDisplayName(rawName string) (string, error) {
	trimmed := strings.TrimSpace(rawName)
	length := utf8.RuneCountInString(trimmed)

	if length < minimumDisplayNameLength || length > maximumDisplayNameLength {
		return "", validationError("Имя должно содержать от 2 до 64 символов")
	}

	return trimmed, nil
}

func validationError(message string) error {
	return &domain.AppError{
		Code:    "VALIDATION_ERROR",
		Message: message,
		Status:  fiber.StatusUnprocessableEntity,
	}
}

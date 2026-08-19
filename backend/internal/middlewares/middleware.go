package middlewares

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	userLocalKey   = "authenticated_user"
	sessionCookie  = "ow_session"
	csrfCookie     = "ow_csrf"
	csrfHeader     = "X-CSRF-Token"
	csrfTokenBytes = 16
	hstsMaxAge     = "max-age=31536000; includeSubDomains"
)

type Config struct {
	AllowedOrigins    []string
	SecureCookies     bool
	SessionLifetime   time.Duration
	RequestsPerMinute int
}

type Middleware struct {
	logger   *slog.Logger
	sessions SessionResolver
	config   Config
}

func New(logger *slog.Logger, sessions SessionResolver, config Config) *Middleware {
	return &Middleware{logger: logger, sessions: sessions, config: config}
}

func (m *Middleware) SetGlobal(app *fiber.App) {
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(m.securityHeaders())
	app.Use(m.requestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(m.config.AllowedOrigins, ","),
		AllowCredentials: true,
		AllowHeaders:     "Content-Type, X-CSRF-Token, Idempotency-Key",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        m.config.RequestsPerMinute,
		Expiration: time.Minute,
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/healthz" || c.Path() == "/readyz"
		},
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(_ *fiber.Ctx) error {
			return &domain.AppError{
				Code:    "RATE_LIMITED",
				Message: "Слишком много запросов, попробуйте позже",
				Status:  fiber.StatusTooManyRequests,
			}
		},
	}))
}

func (m *Middleware) Authenticate(c *fiber.Ctx) error {
	rawToken := c.Cookies(sessionCookie)

	user, found, err := m.sessions.ResolveSession(c.UserContext(), rawToken)
	if err != nil {
		return err
	}

	if !found {
		return &domain.AppError{
			Code:    "AUTH_REQUIRED",
			Message: "Войдите, чтобы продолжить",
			Status:  fiber.StatusUnauthorized,
		}
	}

	c.Locals(userLocalKey, user)

	return c.Next()
}

func (m *Middleware) VerifyCSRF(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodGet ||
		c.Method() == fiber.MethodHead ||
		c.Method() == fiber.MethodOptions {
		return c.Next()
	}

	requestToken := c.Get(csrfHeader)
	cookieToken := c.Cookies(csrfCookie)

	if requestToken == "" || cookieToken == "" ||
		subtle.ConstantTimeCompare([]byte(requestToken), []byte(cookieToken)) != 1 {
		return &domain.AppError{
			Code:    "INVALID_CSRF_TOKEN",
			Message: "Обновите страницу и повторите действие",
			Status:  fiber.StatusForbidden,
		}
	}

	return c.Next()
}

func (m *Middleware) SetSessionCookies(c *fiber.Ctx, sessionToken string) error {
	csrfToken, err := randomToken(csrfTokenBytes)
	if err != nil {
		return err
	}

	maxAge := int(m.config.SessionLifetime.Seconds())
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookie,
		Value:    sessionToken,
		HTTPOnly: true,
		Secure:   m.config.SecureCookies,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
		MaxAge:   maxAge,
	})
	c.Cookie(&fiber.Cookie{
		Name:     csrfCookie,
		Value:    csrfToken,
		HTTPOnly: false,
		Secure:   m.config.SecureCookies,
		SameSite: fiber.CookieSameSiteLaxMode,
		Path:     "/",
		MaxAge:   maxAge,
	})

	return nil
}

func (m *Middleware) ClearSessionCookies(c *fiber.Ctx) {
	expiration := time.Now().Add(-time.Hour)

	for _, name := range []string{sessionCookie, csrfCookie} {
		c.Cookie(&fiber.Cookie{
			Name:     name,
			Value:    "",
			HTTPOnly: name == sessionCookie,
			Secure:   m.config.SecureCookies,
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
			Expires:  expiration,
			MaxAge:   -1,
		})
	}
}

func User(c *fiber.Ctx) domain.User {
	user, _ := c.Locals(userLocalKey).(domain.User)

	return user
}

func (m *Middleware) securityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Set("Cross-Origin-Resource-Policy", "same-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		if m.config.SecureCookies {
			c.Set("Strict-Transport-Security", hstsMaxAge)
		}

		return c.Next()
	}
}

func (m *Middleware) requestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		err := c.Next()
		m.logger.Info(
			"http request",
			"request_id", c.GetRespHeader(fiber.HeaderXRequestID),
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration", time.Since(startedAt),
		)

		return err
	}
}

func randomToken(byteLength int) (string, error) {
	buffer := make([]byte, byteLength)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return strings.ToLower(hex.EncodeToString(buffer)), nil
}

package auth_handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type AuthService interface {
	Register(context.Context, string, string, string) (domain.User, string, error)
	Login(context.Context, string, string) (domain.User, string, bool, error)
	Logout(context.Context, string) error
}

type CookieManager interface {
	SetSessionCookies(*fiber.Ctx, string) error
	ClearSessionCookies(*fiber.Ctx)
}

type AttemptLimiter interface {
	Allow(ctx context.Context, scope string, identity string, limit int, window time.Duration) (bool, error)
	Reset(ctx context.Context, scope string, identity string, window time.Duration) error
}

package middlewares

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type SessionResolver interface {
	ResolveSession(context.Context, string) (domain.User, bool, error)
}

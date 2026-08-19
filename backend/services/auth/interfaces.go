package auth

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository interface {
	Register(context.Context, string, string, string) (domain.User, error)
	Authenticate(context.Context, string, string) (domain.User, bool, error)
}

type SessionRepository interface {
	NewSession(context.Context, string) (string, error)
	Session(context.Context, string) (domain.User, bool, error)
	RevokeSession(context.Context, string) error
}

type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

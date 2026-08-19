package profile_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type ProfileService interface {
	SavePreferences(context.Context, domain.User, domain.Preferences) (domain.User, error)
	CompleteOnboarding(context.Context, domain.User, string) (domain.User, error)
	SetHomeCity(context.Context, domain.User, string) (domain.User, error)
	SetTravelVisibility(context.Context, domain.User, string) (domain.User, error)
}

type WorldReader interface {
	Bootstrap(context.Context, domain.User) ([]domain.Territory, int, error)
}

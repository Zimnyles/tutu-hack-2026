package profile

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository interface {
	SaveUser(context.Context, domain.User) error
	CityExists(context.Context, string) (bool, error)
}

type PersonalRecommendationStarter interface {
	CreatePersonalized(context.Context, domain.User) (domain.RecommendationRequest, error)
	RebuildPersonalized(context.Context, domain.User) (domain.RecommendationRequest, error)
}

type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

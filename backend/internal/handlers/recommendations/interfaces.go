package recommendations_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/services/recommendation"
)

type RecommendationService interface {
	Create(context.Context, domain.User, recommendation.Input) (domain.RecommendationRequest, error)
	Get(context.Context, string, string) (domain.RecommendationRequest, bool, error)
	Latest(context.Context, string, string) (domain.RecommendationRequest, bool, error)
	RebuildPersonalized(context.Context, domain.User) (domain.RecommendationRequest, error)
}

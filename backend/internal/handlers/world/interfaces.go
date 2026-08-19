package world_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type WorldService interface {
	Settings(context.Context) (domain.PublicSettings, error)
	Bootstrap(context.Context, domain.User) ([]domain.Territory, int, error)
	Territory(context.Context, string, string) (domain.Territory, bool, error)
	SyncDemoHistory(context.Context, string) ([]domain.Territory, error)
}

type SeasonService interface {
	CurrentSeason(context.Context, string) (domain.Season, error)
}

type PersonalRecommendationService interface {
	Latest(context.Context, string, string) (domain.RecommendationRequest, bool, error)
	CreatePersonalized(context.Context, domain.User) (domain.RecommendationRequest, error)
}

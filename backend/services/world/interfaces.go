package world

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository interface {
	Settings(context.Context) (domain.PublicSettings, error)
	TerritoriesFor(context.Context, string) ([]domain.Territory, error)
	Territory(context.Context, string, string) (domain.Territory, bool, error)
	DemoSync(context.Context, string) ([]domain.Territory, error)
}

type RewardBalance interface {
	Balance(context.Context, string) (int, error)
}

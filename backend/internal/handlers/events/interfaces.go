package events_handler

import (
	"context"
	"time"

	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/services/events"
	"github.com/tutu-hack/openworld/services/recommendation"
)

type EventService interface {
	ListCity(context.Context, domain.Territory, events.Filters) (events.Listing, error)
	Popular(context.Context) (events.Listing, error)
	Get(context.Context, string) (domain.Event, bool, error)
	Refresh(context.Context, domain.Territory, time.Time, time.Time) ([]domain.Event, error)
}

type WorldReader interface {
	Territory(context.Context, string, string) (domain.Territory, bool, error)
}

type RecommendationCreator interface {
	Create(context.Context, domain.User, recommendation.Input) (domain.RecommendationRequest, error)
}

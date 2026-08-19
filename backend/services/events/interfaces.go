package events

import (
	"context"
	"time"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository interface {
	List(context.Context, string, Filters) ([]domain.Event, error)
	Get(context.Context, string) (domain.Event, bool, error)
	SaveAIEnrichment(context.Context, string, []domain.EventEnrichment) error
	SaveCityDiscovery(context.Context, string, []domain.DiscoveredEvent, time.Time) error
	SavePopularDiscovery(context.Context, []domain.DiscoveredEvent, time.Time) error
	PopularEvents(context.Context, int) ([]domain.Event, error)
	DiscoveryCities(context.Context, int) ([]domain.Territory, error)
	DiscoveryState(context.Context, string, string) (domain.EventDiscoveryState, bool, error)
	ClaimDiscovery(context.Context, string, string, time.Time) (bool, error)
	CompleteDiscovery(context.Context, string, string, int, time.Time) error
	FailDiscovery(context.Context, string, string, string, time.Time) error
}

type EventEnricher interface {
	Enrich(context.Context, domain.Territory, []domain.Event) ([]domain.EventEnrichment, error)
}

type EventDiscoverer interface {
	DiscoverCity(
		context.Context,
		domain.Territory,
		time.Time,
		time.Time,
		int,
	) ([]domain.DiscoveredEvent, error)
	DiscoverPopular(
		context.Context,
		[]domain.Territory,
		time.Time,
		time.Time,
		int,
	) ([]domain.DiscoveredEvent, error)
}

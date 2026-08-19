package trips_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type TripService interface {
	SelectOption(context.Context, string, string, string) (domain.Trip, error)
	List(context.Context, string) ([]domain.Trip, error)
	CreateCheckout(context.Context, string, string) (domain.Trip, string, bool, error)
	SimulateArrival(context.Context, string, string, string) (domain.Trip, domain.LedgerEntry, bool, error)
}

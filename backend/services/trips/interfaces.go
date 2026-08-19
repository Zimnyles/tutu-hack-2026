package trips

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository interface {
	CreateFromRecommendation(context.Context, string, string, string) (domain.Trip, error)
	List(context.Context, string) ([]domain.Trip, error)
	Get(context.Context, string, string) (domain.Trip, bool, error)
	SaveCheckout(context.Context, string, string, string) (domain.Trip, error)
}

type CheckoutCreator interface {
	Create(context.Context, domain.Trip) (string, string, bool, error)
}

type ArrivalProcessor interface {
	Arrive(context.Context, string, string, string) (domain.Trip, domain.LedgerEntry, bool, error)
}

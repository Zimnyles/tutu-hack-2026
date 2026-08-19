package rewards

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository interface {
	Ledger(context.Context, string) ([]domain.LedgerEntry, error)
	Balance(context.Context, string) (int, error)
	Achievements(context.Context, string) ([]domain.Achievement, error)
	PromoCodes(context.Context, string) ([]domain.PromoCode, error)
}

package rewards_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type RewardService interface {
	Ledger(context.Context, string) (int, []domain.LedgerEntry, error)
	Achievements(context.Context, string) ([]domain.Achievement, error)
	PromoCodes(context.Context, string) ([]domain.PromoCode, error)
}

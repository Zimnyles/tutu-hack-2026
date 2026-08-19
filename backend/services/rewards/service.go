package rewards

import (
	"context"
	"fmt"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Balance(ctx context.Context, userID string) (int, error) {
	balance, err := s.repository.Balance(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("calculate reward balance: %w", err)
	}

	return balance, nil
}

func (s *Service) Ledger(
	ctx context.Context,
	userID string,
) (int, []domain.LedgerEntry, error) {
	balance, err := s.repository.Balance(ctx, userID)
	if err != nil {
		return 0, nil, fmt.Errorf("calculate reward balance: %w", err)
	}

	entries, err := s.repository.Ledger(ctx, userID)
	if err != nil {
		return 0, nil, fmt.Errorf("load reward ledger: %w", err)
	}

	return balance, entries, nil
}

func (s *Service) Achievements(
	ctx context.Context,
	userID string,
) ([]domain.Achievement, error) {
	achievements, err := s.repository.Achievements(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load achievements: %w", err)
	}

	return achievements, nil
}

func (s *Service) PromoCodes(
	ctx context.Context,
	userID string,
) ([]domain.PromoCode, error) {
	codes, err := s.repository.PromoCodes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load promo codes: %w", err)
	}

	return codes, nil
}

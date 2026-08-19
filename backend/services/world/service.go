package world

import (
	"context"
	"fmt"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Service struct {
	repository Repository
	rewards    RewardBalance
}

func New(repository Repository, rewards RewardBalance) *Service {
	return &Service{
		repository: repository,
		rewards:    rewards,
	}
}

func (s *Service) Settings(ctx context.Context) (domain.PublicSettings, error) {
	settings, err := s.repository.Settings(ctx)
	if err != nil {
		return domain.PublicSettings{}, fmt.Errorf("load public settings: %w", err)
	}

	return settings, nil
}

func (s *Service) Bootstrap(
	ctx context.Context,
	user domain.User,
) ([]domain.Territory, int, error) {
	territories, err := s.repository.TerritoriesFor(ctx, user.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("load world territories: %w", err)
	}

	balance, err := s.rewards.Balance(ctx, user.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("load reward balance: %w", err)
	}

	return territories, balance, nil
}

func (s *Service) Territory(
	ctx context.Context,
	userID string,
	territoryID string,
) (domain.Territory, bool, error) {
	territory, found, err := s.repository.Territory(ctx, userID, territoryID)
	if err != nil {
		return domain.Territory{}, false, fmt.Errorf("load territory: %w", err)
	}

	return territory, found, nil
}

func (s *Service) SyncDemoHistory(
	ctx context.Context,
	userID string,
) ([]domain.Territory, error) {
	territories, err := s.repository.DemoSync(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sync demo history: %w", err)
	}

	return territories, nil
}

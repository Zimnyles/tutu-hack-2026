package community

import (
	"context"
	"fmt"

	community_errors "github.com/tutu-hack/openworld/internal/errors/community"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

var (
	allowedLeaderboardScopes = map[string]struct{}{ //nolint:gochecknoglobals // static allowlist.
		"league": {},
		"guild":  {},
		"global": {},
	}
	allowedLeaderboardPeriods = map[string]struct{}{ //nolint:gochecknoglobals // static allowlist.
		"week":   {},
		"month":  {},
		"season": {},
	}
)

type Service struct {
	seasons SeasonRepository
	guilds  GuildRepository
	cohorts CohortRepository
}

func New(
	seasons SeasonRepository,
	guilds GuildRepository,
	cohorts CohortRepository,
) *Service {
	return &Service{
		seasons: seasons,
		guilds:  guilds,
		cohorts: cohorts,
	}
}

func (s *Service) CurrentSeason(
	ctx context.Context,
	userID string,
) (domain.Season, error) {
	season, err := s.seasons.CurrentSeason(ctx, userID)
	if err != nil {
		return domain.Season{}, fmt.Errorf("load current season: %w", err)
	}

	return season, nil
}

func (s *Service) Leaderboard(
	ctx context.Context,
	scope string,
	period string,
) (domain.Leaderboard, error) {
	if _, allowed := allowedLeaderboardScopes[scope]; !allowed {
		return domain.Leaderboard{}, community_errors.ErrUnknownLeaderboardScope
	}

	if _, allowed := allowedLeaderboardPeriods[period]; !allowed {
		return domain.Leaderboard{}, community_errors.ErrUnknownLeaderboardPeriod
	}

	leaderboard, err := s.seasons.Leaderboard(ctx, scope, period)
	if err != nil {
		return domain.Leaderboard{}, fmt.Errorf("load leaderboard: %w", err)
	}

	return leaderboard, nil
}

func (s *Service) Guild(
	ctx context.Context,
	userID string,
) (domain.Guild, error) {
	guild, err := s.guilds.SuggestedGuild(ctx, userID)
	if err != nil {
		return domain.Guild{}, fmt.Errorf("load suggested guild: %w", err)
	}

	return guild, nil
}

func (s *Service) JoinGuild(
	ctx context.Context,
	userID string,
	guildID string,
) (domain.Guild, error) {
	if err := s.guilds.JoinGuild(ctx, userID, guildID); err != nil {
		return domain.Guild{}, fmt.Errorf("join guild: %w", err)
	}

	guild, err := s.guilds.SuggestedGuild(ctx, userID)
	if err != nil {
		return domain.Guild{}, fmt.Errorf("load joined guild: %w", err)
	}

	return guild, nil
}

func (s *Service) LeaveGuild(ctx context.Context, userID string) error {
	if err := s.guilds.LeaveGuild(ctx, userID); err != nil {
		return fmt.Errorf("leave guild: %w", err)
	}

	return nil
}

func (s *Service) TravelCohort(
	ctx context.Context,
	userID string,
	territoryID string,
) (domain.TravelCohort, error) {
	cohort, err := s.cohorts.TravelCohort(ctx, userID, territoryID)
	if err != nil {
		return domain.TravelCohort{}, fmt.Errorf("load travel cohort: %w", err)
	}

	return cohort, nil
}

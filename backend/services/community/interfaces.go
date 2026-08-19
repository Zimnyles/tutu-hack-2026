package community

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type SeasonRepository interface {
	CurrentSeason(context.Context, string) (domain.Season, error)
	Leaderboard(context.Context, string, string) (domain.Leaderboard, error)
}

type GuildRepository interface {
	SuggestedGuild(context.Context, string) (domain.Guild, error)
	JoinGuild(context.Context, string, string) error
	LeaveGuild(context.Context, string) error
}

type CohortRepository interface {
	TravelCohort(context.Context, string, string) (domain.TravelCohort, error)
}

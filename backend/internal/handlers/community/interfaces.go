package community_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type CommunityService interface {
	CurrentSeason(context.Context, string) (domain.Season, error)
	Leaderboard(context.Context, string, string) (domain.Leaderboard, error)
	Guild(context.Context, string) (domain.Guild, error)
	JoinGuild(context.Context, string, string) (domain.Guild, error)
	LeaveGuild(context.Context, string) error
	TravelCohort(context.Context, string, string) (domain.TravelCohort, error)
}

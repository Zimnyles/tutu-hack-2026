package recommendation

import (
	"context"
	"time"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type QuotaLimiter interface {
	Allow(ctx context.Context, scope string, identity string, limit int, window time.Duration) (bool, error)
}

type Repository interface {
	Create(context.Context, domain.RecommendationRequest, string, string) error
	Get(context.Context, string, string) (domain.RecommendationRequest, bool, error)
	Latest(context.Context, string, string) (domain.RecommendationRequest, bool, error)
	SetStage(context.Context, string, string) error
	Candidates(context.Context, domain.User, domain.RecommendationRequest) ([]domain.Territory, error)
	Complete(context.Context, string, []domain.RecommendationOption, string) error
	Fail(context.Context, string, string) error
	Block(context.Context, string, string) error
}

type RequestAnalyzer interface {
	Analyze(context.Context, string, []string) (domain.AIRequestAnalysis, error)
}

type WorkflowSettings interface {
	RecommendationStages(context.Context) ([]domain.WorkflowStage, error)
	RecommendationSettings(context.Context) (domain.RecommendationSettings, error)
	PersonalRecommendationSettings(context.Context) (domain.PersonalRecommendationSettings, error)
}

type SearchPlanner interface {
	PlanSearch(
		context.Context,
		domain.User,
		domain.RecommendationRequest,
		[]domain.Territory,
		domain.RecommendationSettings,
	) (domain.TravelSearchPlan, error)
}

type TravelSearch interface {
	Search(
		context.Context,
		domain.RecommendationRequest,
		[]domain.Territory,
		domain.RecommendationSettings,
	) (domain.TransportSearchResult, error)
}

type RecommendationScorer interface {
	Score(
		context.Context,
		domain.User,
		domain.RecommendationRequest,
		[]domain.Territory,
		[]domain.TransportOffer,
		int,
	) ([]domain.ScoredTravelOption, error)
}

type RecommendationExplainer interface {
	Explain(
		context.Context,
		domain.User,
		domain.RecommendationRequest,
		[]domain.ScoredTravelOption,
		domain.RecommendationSettings,
	) ([]domain.RecommendationOption, error)
}

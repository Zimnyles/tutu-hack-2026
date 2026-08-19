package recommendation

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type workflowRepositoryStub struct {
	mu              sync.Mutex
	created         domain.RecommendationRequest
	completionKind  string
	completionReady chan struct{}
}

func (r *workflowRepositoryStub) Create(_ context.Context, value domain.RecommendationRequest, _, _ string) error {
	r.mu.Lock()
	r.created = value
	r.mu.Unlock()

	return nil
}

func (r *workflowRepositoryStub) Get(context.Context, string, string) (domain.RecommendationRequest, bool, error) {
	return domain.RecommendationRequest{}, false, nil
}

func (r *workflowRepositoryStub) Latest(context.Context, string, string) (domain.RecommendationRequest, bool, error) {
	return domain.RecommendationRequest{}, false, nil
}

func (r *workflowRepositoryStub) SetStage(context.Context, string, string) error { return nil }

func (r *workflowRepositoryStub) Candidates(context.Context, domain.User, domain.RecommendationRequest) ([]domain.Territory, error) {
	return []domain.Territory{{ID: "city", Name: "Город", Tags: []string{"nature"}, Rarity: 3, Reward: 100}}, nil
}

func (r *workflowRepositoryStub) Complete(_ context.Context, _ string, _ []domain.RecommendationOption, status string) error {
	r.mu.Lock()
	r.completionKind = status
	r.mu.Unlock()
	close(r.completionReady)

	return nil
}

func (r *workflowRepositoryStub) Fail(context.Context, string, string) error  { return nil }
func (r *workflowRepositoryStub) Block(context.Context, string, string) error { return nil }

type workflowSettingsStub struct{}

func (workflowSettingsStub) RecommendationStages(context.Context) ([]domain.WorkflowStage, error) {
	return []domain.WorkflowStage{
		{Label: "guardrails"},
		{Label: "classification"},
		{Label: "plan"},
		{Label: "mcp"},
		{Label: "scoring"},
		{Label: "explanation"},
		{Label: "finalize"},
	}, nil
}

func (workflowSettingsStub) RecommendationSettings(context.Context) (domain.RecommendationSettings, error) {
	return domain.RecommendationSettings{
		CandidateSearchLimit: 3, OfferPageSize: 3, MaximumRankedOptions: 3, OfferValidityMinutes: 15,
	}, nil
}

func (workflowSettingsStub) PersonalRecommendationSettings(context.Context) (domain.PersonalRecommendationSettings, error) {
	return domain.PersonalRecommendationSettings{
		LeadDays: 7, DefaultDurationDays: 2, MaximumDurationDays: 14,
		FreshnessHours: 24, Adults: 1, Currency: "RUB",
	}, nil
}

type analyzerStub struct{ calls int }

func (a *analyzerStub) Analyze(context.Context, string, []string) (domain.AIRequestAnalysis, error) {
	a.calls++

	return domain.AIRequestAnalysis{TravelRelated: true}, nil
}

type plannerStub struct{}

func (plannerStub) PlanSearch(
	context.Context,
	domain.User,
	domain.RecommendationRequest,
	[]domain.Territory,
	domain.RecommendationSettings,
) (domain.TravelSearchPlan, error) {
	return domain.TravelSearchPlan{CandidateIDs: []string{"city"}, TransportModes: []string{"railway"}}, nil
}

type travelSearchStub struct{ partial bool }

func (s travelSearchStub) Search(
	context.Context,
	domain.RecommendationRequest,
	[]domain.Territory,
	domain.RecommendationSettings,
) (domain.TransportSearchResult, error) {
	result := domain.TransportSearchResult{Offers: []domain.TransportOffer{{
		CityID: "city", Price: 5_000, Currency: "RUB", DurationMinutes: 180, Transport: "railway",
		Snapshot: json.RawMessage(`{"offer":"verified"}`), CheckoutRef: json.RawMessage(`{"ref":"opaque"}`),
	}}}
	if s.partial {
		result.Issues = []domain.TransportSearchIssue{{CityID: "another", Code: "MCP_SEARCH_FAILED"}}
	}

	return result, nil
}

type scorerStub struct{}

func (scorerStub) Score(
	_ context.Context,
	_ domain.User,
	_ domain.RecommendationRequest,
	candidates []domain.Territory,
	offers []domain.TransportOffer,
	_ int,
) ([]domain.ScoredTravelOption, error) {
	return []domain.ScoredTravelOption{{Candidate: candidates[0], Offer: offers[0], Score: 87}}, nil
}

type explainerStub struct{}

func (explainerStub) Explain(
	_ context.Context,
	_ domain.User,
	_ domain.RecommendationRequest,
	scored []domain.ScoredTravelOption,
	_ domain.RecommendationSettings,
) ([]domain.RecommendationOption, error) {
	return []domain.RecommendationOption{{
		ID: "option", CityID: scored[0].Candidate.ID, Score: scored[0].Score,
		Reason: "Совпадает с интересами", WhyNow: "Есть проверенный билет",
	}}, nil
}

type quotaLimiterStub struct{}

func (quotaLimiterStub) Allow(context.Context, string, string, int, time.Duration) (bool, error) {
	return true, nil
}

func TestCreatePersonalizedIsIndependentFromPromptAndCanCompletePartially(t *testing.T) {
	repository := &workflowRepositoryStub{completionReady: make(chan struct{})}
	analyzer := &analyzerStub{}
	service := New(Dependencies{
		Repository:    repository,
		Settings:      workflowSettingsStub{},
		Analyzer:      analyzer,
		SearchPlanner: plannerStub{},
		TravelSearch:  travelSearchStub{partial: true},
		Scorer:        scorerStub{},
		Explainer:     explainerStub{},
		Limiter:       quotaLimiterStub{},
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Quotas:        Quotas{PerHour: 10, PerDay: 10},
	})
	user := domain.User{
		ID: "user", HomeCityID: uuid.NewString(), OnboardingCompleted: true,
		Preferences: domain.Preferences{
			Themes: []string{"nature"}, TransportModes: []string{"railway"},
			MaxTravelMinutes: 600, TypicalBudget: 20_000, TripDurationDays: 3,
		},
	}

	created, err := service.CreatePersonalized(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if created.Kind != "personal" || created.Prompt != "" {
		t.Fatalf("personal workflow was mixed with prompt workflow: %#v", created)
	}
	select {
	case <-repository.completionReady:
	case <-time.After(time.Second):
		t.Fatal("personal recommendation workflow did not complete")
	}
	if analyzer.calls != 0 {
		t.Fatalf("personal workflow must use stored preferences, analyzer calls: %d", analyzer.calls)
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.completionKind != "partial" {
		t.Fatalf("expected partial result from partial MCP search, got %q", repository.completionKind)
	}
}

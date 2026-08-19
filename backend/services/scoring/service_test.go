package scoring

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	scoring_errors "github.com/tutu-hack/openworld/internal/errors/scoring"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type settingsStub struct {
	weights domain.ScoringWeights
	err     error
}

func (s settingsStub) ScoringWeights(context.Context) (domain.ScoringWeights, error) {
	return s.weights, s.err
}

func TestScoreUsesBackendWeightsAndMandatoryConstraints(t *testing.T) {
	service := New(settingsStub{weights: testWeights()})
	user := domain.User{Preferences: domain.Preferences{Themes: []string{"architecture"}, MaxTravelMinutes: 600}}
	recommendation := domain.RecommendationRequest{
		Budget: 20_000, Currency: "RUB", TransportModes: []string{"railway"}, MaxTravelMinutes: 600,
	}
	candidates := []domain.Territory{
		{ID: "matching", Tags: []string{"architecture"}, Rarity: 5, SeasonalFit: 1},
		{ID: "other", Tags: []string{"nature"}, Rarity: 1, SeasonalFit: 0.2},
		{ID: "over-budget", Tags: []string{"architecture"}, Rarity: 5, SeasonalFit: 1},
	}
	offers := []domain.TransportOffer{
		testOffer("matching", 8_000, 240),
		testOffer("other", 10_000, 300),
		testOffer("over-budget", 25_000, 240),
	}

	result, err := service.Score(context.Background(), user, recommendation, candidates, offers, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 eligible offers, got %d", len(result))
	}
	if result[0].Candidate.ID != "matching" || result[0].Score <= result[1].Score {
		t.Fatalf("unexpected backend order: %#v", result)
	}
}

func TestScoreRejectsInvalidConfiguration(t *testing.T) {
	service := New(settingsStub{})
	_, err := service.Score(context.Background(), domain.User{}, domain.RecommendationRequest{}, nil, nil, 3)
	if !errors.Is(err, scoring_errors.ErrInvalidConfiguration) {
		t.Fatalf("expected invalid configuration, got %v", err)
	}
}

func testWeights() domain.ScoringWeights {
	return domain.ScoringWeights{
		PreferenceMatch: 0.25, PriceValue: 0.18, MapGain: 0.14, Novelty: 0.13,
		SeasonalFit: 0.1, EventFit: 0.1, TravelFriction: 0.05,
		CommercialPriority: 0.05, RarityCeiling: 5,
	}
}

func testOffer(cityID string, price int, duration int) domain.TransportOffer {
	return domain.TransportOffer{
		CityID: cityID, Transport: "railway", Price: price, Currency: "RUB", DurationMinutes: duration,
		Snapshot: json.RawMessage(`{"offer_id":"verified"}`), CheckoutRef: json.RawMessage(`{"ref":"opaque"}`),
	}
}

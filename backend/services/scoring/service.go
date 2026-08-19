package scoring

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	scoring_errors "github.com/tutu-hack/openworld/internal/errors/scoring"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Service struct {
	settings Settings
}

const scorePercentageBase = 100

func New(settings Settings) *Service {
	return &Service{settings: settings}
}

func (s *Service) Score(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
	candidates []domain.Territory,
	offers []domain.TransportOffer,
	limit int,
) ([]domain.ScoredTravelOption, error) {
	weights, err := s.settings.ScoringWeights(ctx)
	if err != nil {
		return nil, fmt.Errorf("load scoring weights: %w", err)
	}

	if !validWeights(weights) || limit <= 0 {
		return nil, scoring_errors.ErrInvalidConfiguration
	}

	candidatesByID := make(map[string]domain.Territory, len(candidates))
	for _, candidate := range candidates {
		candidatesByID[candidate.ID] = candidate
	}

	result := make([]domain.ScoredTravelOption, 0, len(offers))

	for _, offer := range offers {
		candidate, found := candidatesByID[offer.CityID]
		if !found || !validOffer(recommendation, offer) {
			continue
		}

		result = append(result, domain.ScoredTravelOption{
			Candidate: candidate,
			Offer:     offer,
			Score:     calculateScore(user, recommendation, candidate, offer, weights),
		})
	}

	sort.SliceStable(result, func(left int, right int) bool {
		if result[left].Score == result[right].Score {
			return result[left].Offer.Price < result[right].Offer.Price
		}

		return result[left].Score > result[right].Score
	})

	if len(result) > limit {
		result = result[:limit]
	}

	if len(result) == 0 {
		return nil, scoring_errors.ErrNoEligibleOffers
	}

	return result, nil
}

func calculateScore(
	user domain.User,
	recommendation domain.RecommendationRequest,
	candidate domain.Territory,
	offer domain.TransportOffer,
	weights domain.ScoringWeights,
) int {
	preferenceMatch := overlapRatio(user.Preferences.Themes, candidate.Tags)
	if badgeMatch := badgeMatchRatio(
		domain.BadgesForThemes(user.Preferences.Themes),
		candidate.Badges,
	); badgeMatch > preferenceMatch {
		preferenceMatch = badgeMatch
	}

	priceValue := 0.0
	if recommendation.Budget > 0 {
		priceValue = clamp01(1 - float64(offer.Price)/float64(recommendation.Budget))
	}

	mapGain := clamp01(float64(candidate.Rarity) / weights.RarityCeiling)
	seasonalFit := clamp01(candidate.SeasonalFit)

	eventFit := 0.0
	if recommendation.EventID != "" && recommendation.DestinationID == candidate.ID {
		eventFit = 1
	}

	frictionCeiling := recommendation.MaxTravelMinutes
	if frictionCeiling <= 0 {
		frictionCeiling = user.Preferences.MaxTravelMinutes
	}

	travelFriction := 1.0
	if frictionCeiling > 0 {
		travelFriction = clamp01(float64(offer.DurationMinutes) / float64(frictionCeiling))
	}

	positiveWeight := weights.PreferenceMatch + weights.PriceValue + weights.MapGain +
		weights.Novelty + weights.SeasonalFit + weights.EventFit + weights.CommercialPriority
	raw := preferenceMatch*weights.PreferenceMatch +
		priceValue*weights.PriceValue +
		mapGain*weights.MapGain +
		weights.Novelty +
		seasonalFit*weights.SeasonalFit +
		eventFit*weights.EventFit +
		clamp01(candidate.CommercialPriority)*weights.CommercialPriority -
		travelFriction*weights.TravelFriction

	return int(math.Round(clamp01(raw/positiveWeight) * scorePercentageBase))
}

func validOffer(recommendation domain.RecommendationRequest, offer domain.TransportOffer) bool {
	if offer.Price < 0 || offer.Price > recommendation.Budget ||
		!strings.EqualFold(offer.Currency, recommendation.Currency) ||
		offer.DurationMinutes <= 0 || len(offer.Snapshot) == 0 || len(offer.CheckoutRef) == 0 {
		return false
	}

	for _, allowedMode := range recommendation.TransportModes {
		if offer.Transport == allowedMode {
			return true
		}
	}

	return false
}

func validWeights(weights domain.ScoringWeights) bool {
	values := []float64{
		weights.PreferenceMatch,
		weights.PriceValue,
		weights.MapGain,
		weights.Novelty,
		weights.SeasonalFit,
		weights.EventFit,
		weights.TravelFriction,
		weights.CommercialPriority,
	}
	positiveTotal := 0.0

	for _, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}

		positiveTotal += value
	}

	return positiveTotal > 0 && weights.RarityCeiling > 0
}

func overlapRatio(preferences []string, tags []string) float64 {
	if len(preferences) == 0 {
		return 0
	}

	tagSet := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tagSet[tag] = struct{}{}
	}

	matches := 0

	for _, preference := range preferences {
		if _, found := tagSet[preference]; found {
			matches++
		}
	}

	return clamp01(float64(matches) / float64(len(preferences)))
}

func badgeMatchRatio(wanted []string, badges []string) float64 {
	if len(wanted) == 0 || len(badges) == 0 {
		return 0
	}

	badgeSet := make(map[string]struct{}, len(badges))
	for _, badge := range badges {
		badgeSet[badge] = struct{}{}
	}

	matches := 0

	for _, badge := range wanted {
		if _, found := badgeSet[badge]; found {
			matches++
		}
	}

	divisor := len(badges)
	if len(wanted) < divisor {
		divisor = len(wanted)
	}

	return clamp01(float64(matches) / float64(divisor))
}

func clamp01(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

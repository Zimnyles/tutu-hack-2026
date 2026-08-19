package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	maximumExplanationRunes = 500
)

type RecommendationRanker struct {
	transport chatClient
	model     string
	prompts   domain.AISystemPrompts
}

type rankedRecommendation struct {
	CityID string `json:"city_id"`
	Reason string `json:"reason"`
	WhyNow string `json:"why_now"`
}

type rankingEnvelope struct {
	Recommendations []rankedRecommendation `json:"recommendations"`
}

type searchPlanEnvelope struct {
	CandidateIDs   []string `json:"candidate_ids"`
	TransportModes []string `json:"transport_modes"`
}

type analysisEnvelope struct {
	TravelRelated   bool                `json:"travel_related"`
	ContainsPII     bool                `json:"contains_pii"`
	PromptInjection bool                `json:"prompt_injection"`
	Political       bool                `json:"political"`
	Intent          domain.TravelIntent `json:"travel_intent"`
}

func NewRecommendationRanker(
	apiKey string,
	baseURL string,
	model string,
	prompts domain.AISystemPrompts,
) *RecommendationRanker {
	return &RecommendationRanker{
		transport: newChatClient(apiKey, baseURL),
		model:     model,
		prompts:   prompts,
	}
}

func (r *RecommendationRanker) Analyze(
	ctx context.Context,
	prompt string,
	allowedModes []string,
) (domain.AIRequestAnalysis, error) {
	if r.transport.apiKey == "" {
		return domain.AIRequestAnalysis{}, ai_errors.ErrNotConfigured
	}

	inputJSON, err := json.Marshal(struct {
		Prompt       string   `json:"prompt"`
		AllowedModes []string `json:"allowed_transport_modes"`
	}{Prompt: strings.TrimSpace(prompt), AllowedModes: allowedModes})
	if err != nil {
		return domain.AIRequestAnalysis{}, fmt.Errorf("encode AI request analysis input: %w", err)
	}

	payload := chatRequest{
		Model: r.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: r.prompts.RequestAnalysis,
			},
			{Role: "user", Content: string(inputJSON)},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
		Temperature:    0,
	}

	content, err := r.chat(ctx, payload)
	if err != nil {
		return domain.AIRequestAnalysis{}, err
	}

	var envelope analysisEnvelope
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		return domain.AIRequestAnalysis{}, fmt.Errorf("decode DeepSeek request analysis: %w", err)
	}

	if !validIntent(envelope.Intent, allowedModes) {
		return domain.AIRequestAnalysis{}, ai_errors.ErrInvalidAnalysis
	}

	return domain.AIRequestAnalysis{
		TravelRelated:   envelope.TravelRelated,
		ContainsPII:     envelope.ContainsPII,
		PromptInjection: envelope.PromptInjection,
		Political:       envelope.Political,
		Intent:          envelope.Intent,
	}, nil
}

func (r *RecommendationRanker) PlanSearch(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
	candidates []domain.Territory,
	settings domain.RecommendationSettings,
) (domain.TravelSearchPlan, error) {
	if r.transport.apiKey == "" {
		return domain.TravelSearchPlan{}, ai_errors.ErrNotConfigured
	}

	input := struct {
		Preferences      domain.Preferences `json:"preferences"`
		Dates            [2]string          `json:"dates"`
		Budget           int                `json:"budget"`
		Prompt           string             `json:"prompt,omitempty"`
		AllowedModes     []string           `json:"allowed_transport_modes"`
		Candidates       []domain.Territory `json:"candidates"`
		MaximumCityCount int                `json:"maximum_city_count"`
	}{
		Preferences:      user.Preferences,
		Dates:            [2]string{recommendation.DateFrom, recommendation.DateTo},
		Budget:           recommendation.Budget,
		Prompt:           recommendation.Prompt,
		AllowedModes:     recommendation.TransportModes,
		Candidates:       candidates,
		MaximumCityCount: settings.CandidateSearchLimit,
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return domain.TravelSearchPlan{}, fmt.Errorf("encode AI search planning input: %w", err)
	}

	payload := chatRequest{
		Model: r.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: r.prompts.TravelSearchPlan,
			},
			{Role: "user", Content: string(inputJSON)},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
		Temperature:    discoveryTemperature,
	}

	content, err := r.chat(ctx, payload)
	if err != nil {
		return domain.TravelSearchPlan{}, err
	}

	var envelope searchPlanEnvelope
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		return domain.TravelSearchPlan{}, fmt.Errorf("decode DeepSeek search plan: %w", err)
	}

	if err := validateSearchPlanEnvelope(envelope, candidates, recommendation.TransportModes, settings); err != nil {
		return domain.TravelSearchPlan{}, err
	}

	return domain.TravelSearchPlan{
		CandidateIDs:   envelope.CandidateIDs,
		TransportModes: envelope.TransportModes,
	}, nil
}

func (r *RecommendationRanker) Explain(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
	scored []domain.ScoredTravelOption,
	settings domain.RecommendationSettings,
) ([]domain.RecommendationOption, error) {
	if r.transport.apiKey == "" {
		return nil, ai_errors.ErrNotConfigured
	}

	input := struct {
		Preferences domain.Preferences          `json:"preferences"`
		DateFrom    string                      `json:"date_from"`
		DateTo      string                      `json:"date_to"`
		Budget      int                         `json:"budget"`
		Prompt      string                      `json:"prompt,omitempty"`
		Options     []domain.ScoredTravelOption `json:"backend_ranked_options"`
	}{
		Preferences: user.Preferences,
		DateFrom:    recommendation.DateFrom,
		DateTo:      recommendation.DateTo,
		Budget:      recommendation.Budget,
		Prompt:      recommendation.Prompt,
		Options:     scored,
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode recommendation ranking input: %w", err)
	}

	payload := chatRequest{
		Model: r.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: r.prompts.RecommendationExplanation,
			},
			{Role: "user", Content: string(inputJSON)},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
		Temperature:    discoveryTemperature,
	}

	content, err := r.chat(ctx, payload)
	if err != nil {
		return nil, err
	}

	var envelope rankingEnvelope
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		return nil, fmt.Errorf("decode DeepSeek ranking JSON: %w", err)
	}

	return joinRankedOptions(
		envelope.Recommendations,
		scored,
		recommendation,
		settings,
	)
}

func (r *RecommendationRanker) chat(
	ctx context.Context,
	payload chatRequest,
) (string, error) {
	payload.MaxTokens = maximumCompletionTokens

	return r.transport.complete(ctx, payload)
}

func joinRankedOptions(
	ranked []rankedRecommendation,
	scored []domain.ScoredTravelOption,
	recommendation domain.RecommendationRequest,
	settings domain.RecommendationSettings,
) ([]domain.RecommendationOption, error) {
	if len(ranked) == 0 || len(ranked) != len(scored) || len(ranked) > settings.MaximumRankedOptions {
		return nil, ai_errors.ErrInvalidRanking
	}

	explanationsByCityID := make(map[string]rankedRecommendation, len(ranked))
	for _, item := range ranked {
		if _, duplicate := explanationsByCityID[item.CityID]; duplicate ||
			!validExplanation(item.Reason) || !validExplanation(item.WhyNow) {
			return nil, ai_errors.ErrInvalidRanking
		}

		explanationsByCityID[item.CityID] = item
	}

	options := make([]domain.RecommendationOption, 0, len(scored))

	for index, item := range scored {
		explanation, found := explanationsByCityID[item.Candidate.ID]
		if !found {
			return nil, ai_errors.ErrInvalidRanking
		}

		if item.Score < 0 || item.Score > 100 ||
			len(item.Offer.Snapshot) == 0 || len(item.Offer.CheckoutRef) == 0 {
			return nil, ai_errors.ErrInvalidRanking
		}

		options = append(options, domain.RecommendationOption{
			ID:              uuid.NewSHA1(uuid.NameSpaceOID, []byte(recommendation.ID+item.Candidate.ID)).String(),
			CityID:          item.Candidate.ID,
			CityName:        item.Candidate.Name,
			Region:          item.Candidate.Region,
			Rank:            index + 1,
			Score:           item.Score,
			Reason:          explanation.Reason,
			WhyNow:          explanation.WhyNow,
			Price:           item.Offer.Price,
			Currency:        item.Offer.Currency,
			DurationMinutes: item.Offer.DurationMinutes,
			Transport:       item.Offer.Transport,
			TerritoryGain:   item.Candidate.Rarity,
			Reward:          item.Candidate.Reward,
			Special:         false,
			ValidUntil: time.Now().UTC().Add(
				time.Duration(settings.OfferValidityMinutes) * time.Minute,
			).Format(time.RFC3339),
			EventID:       recommendation.EventID,
			OfferSnapshot: item.Offer.Snapshot,
			CheckoutRef:   item.Offer.CheckoutRef,
		})
	}

	return options, nil
}

func validateSearchPlanEnvelope(
	plan searchPlanEnvelope,
	candidates []domain.Territory,
	allowedModes []string,
	settings domain.RecommendationSettings,
) error {
	if len(plan.CandidateIDs) == 0 {
		return ai_errors.ErrEmptySearchPlan
	}

	if len(plan.CandidateIDs) > settings.CandidateSearchLimit {
		return ai_errors.ErrInvalidSearchPlan
	}

	allowedCandidates := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		allowedCandidates[candidate.ID] = struct{}{}
	}

	seenCandidates := make(map[string]struct{}, len(plan.CandidateIDs))

	for _, candidateID := range plan.CandidateIDs {
		if _, allowed := allowedCandidates[candidateID]; !allowed {
			return ai_errors.ErrInvalidSearchPlan
		}

		if _, duplicate := seenCandidates[candidateID]; duplicate {
			return ai_errors.ErrInvalidSearchPlan
		}

		seenCandidates[candidateID] = struct{}{}
	}

	allowedTransportModes := make(map[string]struct{}, len(allowedModes))
	for _, mode := range allowedModes {
		allowedTransportModes[mode] = struct{}{}
	}

	for _, mode := range plan.TransportModes {
		if _, allowed := allowedTransportModes[mode]; !allowed {
			return ai_errors.ErrInvalidSearchPlan
		}
	}

	return nil
}

func validExplanation(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len([]rune(trimmed)) > maximumExplanationRunes {
		return false
	}

	if strings.Contains(trimmed, "<") || strings.Contains(trimmed, ">") {
		return false
	}

	lowercase := strings.ToLower(trimmed)

	return !strings.Contains(lowercase, "http://") &&
		!strings.Contains(lowercase, "https://") &&
		!strings.Contains(lowercase, "www.")
}

func validIntent(intent domain.TravelIntent, allowedModes []string) bool {
	if intent.TripDurationDays < 0 || intent.TripDurationDays > 60 || len(intent.Themes) > 20 || len(intent.Avoid) > 20 {
		return false
	}

	allowedBudgetLevels := map[string]struct{}{"": {}, "low": {}, "medium": {}, "high": {}}
	if _, found := allowedBudgetLevels[intent.BudgetLevel]; !found {
		return false
	}

	allowedModeSet := make(map[string]struct{}, len(allowedModes))
	for _, mode := range allowedModes {
		allowedModeSet[mode] = struct{}{}
	}

	for _, mode := range intent.TransportModes {
		if _, found := allowedModeSet[mode]; !found {
			return false
		}
	}

	for _, value := range append(append([]string(nil), intent.Themes...), intent.Avoid...) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || len([]rune(trimmed)) > 80 || strings.ContainsAny(trimmed, "<>/\\") {
			return false
		}
	}

	return true
}

package recommendation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	mcp_errors "github.com/tutu-hack/openworld/internal/errors/mcp"
	recommendation_errors "github.com/tutu-hack/openworld/internal/errors/recommendation"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/internal/security"
)

const (
	workflowTimeout        = 45 * time.Second
	requiredWorkflowStages = 7
	dayDuration            = 24 * time.Hour
	stageAIClassification  = 1
	stageAISearchPlan      = 2
	stageMCPTransport      = 3
	stageBackendScoring    = 4
	stageAIExplanation     = 5
	stageFinalize          = 6
)

type Input struct {
	Kind             string
	OriginCityID     string
	DestinationID    string
	EventID          string
	DateFrom         string
	DateTo           string
	Adults           int
	Children         int
	Budget           int
	Currency         string
	TransportModes   []string
	MaxTravelMinutes int
	DirectOnly       bool
	Prompt           string
	RequestID        string
	TraceID          string
}

type Quotas struct {
	PerHour int
	PerDay  int
}

type Service struct {
	repository    Repository
	settings      WorkflowSettings
	analyzer      RequestAnalyzer
	searchPlanner SearchPlanner
	travelSearch  TravelSearch
	scorer        RecommendationScorer
	explainer     RecommendationExplainer
	limiter       QuotaLimiter
	logger        *slog.Logger
	quotas        Quotas
}

type Dependencies struct {
	Repository    Repository
	Settings      WorkflowSettings
	Analyzer      RequestAnalyzer
	SearchPlanner SearchPlanner
	TravelSearch  TravelSearch
	Scorer        RecommendationScorer
	Explainer     RecommendationExplainer
	Limiter       QuotaLimiter
	Logger        *slog.Logger
	Quotas        Quotas
}

func New(dependencies Dependencies) *Service {
	return &Service{
		repository:    dependencies.Repository,
		settings:      dependencies.Settings,
		analyzer:      dependencies.Analyzer,
		searchPlanner: dependencies.SearchPlanner,
		travelSearch:  dependencies.TravelSearch,
		scorer:        dependencies.Scorer,
		explainer:     dependencies.Explainer,
		limiter:       dependencies.Limiter,
		logger:        dependencies.Logger,
		quotas:        dependencies.Quotas,
	}
}

func (s *Service) ensureQuota(ctx context.Context, userID string) error {
	windows := []struct {
		scope  string
		limit  int
		window time.Duration
	}{
		{scope: "recommendations_hour", limit: s.quotas.PerHour, window: time.Hour},
		{scope: "recommendations_day", limit: s.quotas.PerDay, window: dayDuration},
	}

	for _, quota := range windows {
		allowed, err := s.limiter.Allow(ctx, quota.scope, userID, quota.limit, quota.window)
		if err != nil {
			s.logger.Warn("recommendation quota unavailable", "scope", quota.scope, "error", err)
		}

		if !allowed {
			return &domain.AppError{
				Code:    "RATE_LIMITED",
				Message: "Достигнут лимит запросов рекомендаций, попробуйте позже",
				Status:  http.StatusTooManyRequests,
			}
		}
	}

	return nil
}

func (s *Service) Create(
	ctx context.Context,
	user domain.User,
	input Input,
) (domain.RecommendationRequest, error) {
	if strings.TrimSpace(input.OriginCityID) == "" {
		input.OriginCityID = user.HomeCityID
	}

	input, normalizationError := normalizeInput(input)
	if normalizationError != nil {
		return domain.RecommendationRequest{}, normalizationError
	}

	if validationError := security.ValidateRecommendation(security.RecommendationInput{
		OriginCityID:   input.OriginCityID,
		DateFrom:       input.DateFrom,
		DateTo:         input.DateTo,
		Adults:         input.Adults,
		Children:       input.Children,
		Budget:         input.Budget,
		TransportModes: input.TransportModes,
		Prompt:         input.Prompt,
	}); validationError != nil {
		return domain.RecommendationRequest{}, validationError
	}

	if err := s.ensureQuota(ctx, user.ID); err != nil {
		return domain.RecommendationRequest{}, err
	}

	stages, err := s.settings.RecommendationStages(ctx)
	if err != nil {
		return domain.RecommendationRequest{}, fmt.Errorf("load recommendation stages: %w", err)
	}

	if len(stages) < requiredWorkflowStages {
		return domain.RecommendationRequest{}, fmt.Errorf("recommendation stages: %w", recommendation_errors.ErrWorkflowConfiguration)
	}

	recommendation := domain.RecommendationRequest{
		ID:               uuid.NewString(),
		UserID:           user.ID,
		Kind:             input.Kind,
		Status:           "processing",
		Stage:            stages[0].Code,
		OriginCityID:     input.OriginCityID,
		DestinationID:    input.DestinationID,
		EventID:          input.EventID,
		DateFrom:         input.DateFrom,
		DateTo:           input.DateTo,
		Adults:           input.Adults,
		Children:         input.Children,
		Budget:           input.Budget,
		Currency:         input.Currency,
		TransportModes:   input.TransportModes,
		MaxTravelMinutes: input.MaxTravelMinutes,
		DirectOnly:       input.DirectOnly,
		Prompt:           input.Prompt,
		CreatedAt:        time.Now(),
	}

	promptHash := sha256.Sum256([]byte(input.Prompt))
	if err := s.repository.Create(
		ctx,
		recommendation,
		hex.EncodeToString(promptHash[:]),
		input.RequestID,
	); err != nil {
		return domain.RecommendationRequest{}, fmt.Errorf("create recommendation: %w", err)
	}

	workflowContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), workflowTimeout)
	go func() {
		defer cancel()
		defer s.recoverWorkflow(workflowContext, recommendation.ID)
		s.runWorkflow(workflowContext, user, recommendation, stages)
	}()

	return recommendation, nil
}

func (s *Service) Latest(
	ctx context.Context,
	userID string,
	kind string,
) (domain.RecommendationRequest, bool, error) {
	recommendation, found, err := s.repository.Latest(ctx, userID, kind)
	if err != nil {
		return domain.RecommendationRequest{}, false, fmt.Errorf("get latest %s recommendation: %w", kind, err)
	}

	return recommendation, found, nil
}

func (s *Service) CreatePersonalized(
	ctx context.Context,
	user domain.User,
) (domain.RecommendationRequest, error) {
	return s.createPersonalized(ctx, user, false)
}

func (s *Service) RebuildPersonalized(
	ctx context.Context,
	user domain.User,
) (domain.RecommendationRequest, error) {
	return s.createPersonalized(ctx, user, true)
}

func (s *Service) createPersonalized(
	ctx context.Context,
	user domain.User,
	force bool,
) (domain.RecommendationRequest, error) {
	if !user.OnboardingCompleted || user.HomeCityID == "" {
		return domain.RecommendationRequest{}, &domain.AppError{
			Code: "ONBOARDING_REQUIRED", Message: "Сначала завершите онбординг", Status: http.StatusConflict,
		}
	}

	settings, err := s.settings.PersonalRecommendationSettings(ctx)
	if err != nil {
		return domain.RecommendationRequest{}, fmt.Errorf("load personal recommendation settings: %w", err)
	}

	if !validPersonalRecommendationSettings(settings) {
		return domain.RecommendationRequest{}, recommendation_errors.ErrInvalidPersonalConfiguration
	}

	latest, found, err := s.repository.Latest(ctx, user.ID, "personal")
	if err != nil {
		return domain.RecommendationRequest{}, fmt.Errorf("get current personal recommendation: %w", err)
	}

	if !force && found && time.Since(latest.CreatedAt) < time.Duration(settings.FreshnessHours)*time.Hour &&
		(latest.Status == "processing" || latest.Status == "completed" || latest.Status == "partial") {
		return latest, nil
	}

	durationDays := user.Preferences.TripDurationDays
	if durationDays <= 0 {
		durationDays = settings.DefaultDurationDays
	}

	if durationDays > settings.MaximumDurationDays {
		durationDays = settings.MaximumDurationDays
	}

	dateFrom := time.Now().UTC().AddDate(0, 0, settings.LeadDays)
	dateTo := dateFrom.AddDate(0, 0, durationDays)

	return s.Create(ctx, user, Input{
		Kind:             "personal",
		OriginCityID:     user.HomeCityID,
		DateFrom:         dateFrom.Format(time.DateOnly),
		DateTo:           dateTo.Format(time.DateOnly),
		Adults:           settings.Adults,
		Budget:           user.Preferences.TypicalBudget,
		Currency:         settings.Currency,
		TransportModes:   user.Preferences.TransportModes,
		MaxTravelMinutes: user.Preferences.MaxTravelMinutes,
	})
}

func (s *Service) Get(
	ctx context.Context,
	userID string,
	recommendationID string,
) (domain.RecommendationRequest, bool, error) {
	recommendation, found, err := s.repository.Get(ctx, recommendationID, userID)
	if err != nil {
		return domain.RecommendationRequest{}, false, fmt.Errorf("get recommendation: %w", err)
	}

	return recommendation, found, nil
}

func (s *Service) runWorkflow(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
	stages []domain.WorkflowStage,
) {
	if !s.analyzeWorkflowRequest(ctx, &user, &recommendation, stages) {
		return
	}

	settings, candidates, planned := s.planWorkflowSearch(ctx, user, &recommendation, stages)
	if !planned {
		return
	}

	searchResult, searched := s.searchWorkflowTransport(ctx, recommendation, candidates, settings, stages)
	if !searched {
		return
	}

	scored, scoredSuccessfully := s.scoreWorkflowOptions(
		ctx,
		user,
		recommendation,
		candidates,
		searchResult.Offers,
		settings,
		stages,
	)
	if !scoredSuccessfully {
		return
	}

	options, explained := s.explainWorkflowOptions(ctx, user, recommendation, scored, settings, stages)
	if !explained || !s.updateWorkflowStage(ctx, recommendation.ID, stages, stageFinalize) {
		return
	}

	status := "completed"
	if len(searchResult.Issues) > 0 {
		status = "partial"
	}

	if err := s.repository.Complete(ctx, recommendation.ID, options, status); err != nil {
		s.failWorkflow(ctx, recommendation.ID, "RECOMMENDATION_SAVE_FAILED", err)
	}
}

func (s *Service) analyzeWorkflowRequest(
	ctx context.Context,
	user *domain.User,
	recommendation *domain.RecommendationRequest,
	stages []domain.WorkflowStage,
) bool {
	if !s.updateWorkflowStage(ctx, recommendation.ID, stages, stageAIClassification) {
		return false
	}

	if recommendation.Prompt == "" {
		return true
	}

	analysis, err := s.analyzer.Analyze(ctx, recommendation.Prompt, recommendation.TransportModes)
	if err != nil {
		s.failWorkflow(ctx, recommendation.ID, "AI_CLASSIFICATION_FAILED", err)

		return false
	}

	if blockCode := analysisBlockCode(analysis); blockCode != "" {
		_ = s.repository.Block(ctx, recommendation.ID, blockCode)

		return false
	}

	applyTravelIntent(user, recommendation, analysis.Intent)

	return true
}

func (s *Service) planWorkflowSearch(
	ctx context.Context,
	user domain.User,
	recommendation *domain.RecommendationRequest,
	stages []domain.WorkflowStage,
) (domain.RecommendationSettings, []domain.Territory, bool) {
	candidates, err := s.repository.Candidates(ctx, user, *recommendation)
	if err != nil {
		s.failWorkflow(ctx, recommendation.ID, "CANDIDATE_GENERATION_FAILED", err)

		return domain.RecommendationSettings{}, nil, false
	}

	if len(candidates) == 0 {
		s.failWorkflow(ctx, recommendation.ID, "CANDIDATE_GENERATION_EMPTY", recommendation_errors.ErrNoDestinations)

		return domain.RecommendationSettings{}, nil, false
	}

	settings, err := s.settings.RecommendationSettings(ctx)
	if err != nil {
		s.failWorkflow(ctx, recommendation.ID, "WORKFLOW_CONFIGURATION_FAILED", err)

		return domain.RecommendationSettings{}, nil, false
	}

	if !validRecommendationSettings(settings) {
		s.failWorkflow(
			ctx,
			recommendation.ID,
			"WORKFLOW_CONFIGURATION_INVALID",
			recommendation_errors.ErrWorkflowConfiguration,
		)

		return domain.RecommendationSettings{}, nil, false
	}

	if !s.updateWorkflowStage(ctx, recommendation.ID, stages, stageAISearchPlan) {
		return domain.RecommendationSettings{}, nil, false
	}

	if recommendation.DestinationID != "" {
		return settings, candidates, true
	}

	searchPlan, err := s.searchPlanner.PlanSearch(ctx, user, *recommendation, candidates, settings)
	if err != nil {
		reason := "AI_SEARCH_PLAN_FAILED"
		if errors.Is(err, ai_errors.ErrEmptySearchPlan) {
			reason = "AI_SEARCH_PLAN_EMPTY"
		}

		s.failWorkflow(ctx, recommendation.ID, reason, err)

		return domain.RecommendationSettings{}, nil, false
	}

	selectedCandidates := validateSearchPlan(searchPlan, candidates)
	if len(selectedCandidates) == 0 {
		s.failWorkflow(ctx, recommendation.ID, "AI_SEARCH_PLAN_EMPTY", recommendation_errors.ErrNoDestinations)

		return domain.RecommendationSettings{}, nil, false
	}

	if len(recommendation.TransportModes) == 0 {
		recommendation.TransportModes = validateTransportModes(
			searchPlan.TransportModes,
			supportedTransportModes,
		)
	}

	return settings, selectedCandidates, true
}

func (s *Service) searchWorkflowTransport(
	ctx context.Context,
	recommendation domain.RecommendationRequest,
	candidates []domain.Territory,
	settings domain.RecommendationSettings,
	stages []domain.WorkflowStage,
) (domain.TransportSearchResult, bool) {
	if !s.updateWorkflowStage(ctx, recommendation.ID, stages, stageMCPTransport) {
		return domain.TransportSearchResult{}, false
	}

	result, err := s.travelSearch.Search(ctx, recommendation, candidates, settings)
	if err != nil {
		reason := "MCP_TRANSPORT_SEARCH_FAILED"
		if errors.Is(err, mcp_errors.ErrNoOffers) {
			reason = "MCP_TRANSPORT_SEARCH_EMPTY"
		}

		s.failWorkflow(ctx, recommendation.ID, reason, err)

		return domain.TransportSearchResult{}, false
	}

	if len(result.Offers) == 0 {
		s.failWorkflow(ctx, recommendation.ID, "MCP_TRANSPORT_SEARCH_EMPTY", mcp_errors.ErrNoOffers)

		return domain.TransportSearchResult{}, false
	}

	return result, true
}

func (s *Service) scoreWorkflowOptions(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
	candidates []domain.Territory,
	offers []domain.TransportOffer,
	settings domain.RecommendationSettings,
	stages []domain.WorkflowStage,
) ([]domain.ScoredTravelOption, bool) {
	if !s.updateWorkflowStage(ctx, recommendation.ID, stages, stageBackendScoring) {
		return nil, false
	}

	scored, err := s.scorer.Score(
		ctx,
		user,
		recommendation,
		candidates,
		offers,
		settings.MaximumRankedOptions,
	)
	if err != nil || len(scored) == 0 {
		s.failWorkflow(ctx, recommendation.ID, "BACKEND_SCORING_FAILED", err)

		return nil, false
	}

	return scored, true
}

func (s *Service) explainWorkflowOptions(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
	scored []domain.ScoredTravelOption,
	settings domain.RecommendationSettings,
	stages []domain.WorkflowStage,
) ([]domain.RecommendationOption, bool) {
	if !s.updateWorkflowStage(ctx, recommendation.ID, stages, stageAIExplanation) {
		return nil, false
	}

	options, err := s.explainer.Explain(ctx, user, recommendation, scored, settings)
	if err != nil || len(options) == 0 {
		s.failWorkflow(ctx, recommendation.ID, "AI_EXPLANATION_FAILED", err)

		return nil, false
	}

	return options, true
}

func (s *Service) updateWorkflowStage(
	ctx context.Context,
	recommendationID string,
	stages []domain.WorkflowStage,
	index int,
) bool {
	if index >= len(stages) {
		s.failWorkflow(ctx, recommendationID, "STAGE_UPDATE_FAILED", recommendation_errors.ErrWorkflowConfiguration)

		return false
	}

	if err := s.repository.SetStage(ctx, recommendationID, stages[index].Code); err != nil {
		s.failWorkflow(ctx, recommendationID, "STAGE_UPDATE_FAILED", err)

		return false
	}

	return true
}

func analysisBlockCode(analysis domain.AIRequestAnalysis) string {
	switch {
	case analysis.ContainsPII:
		return "PROMPT_CONTAINS_PII"
	case analysis.PromptInjection:
		return "PROMPT_INJECTION_DETECTED"
	case analysis.Political:
		return "POLITICAL_REQUEST_BLOCKED"
	case !analysis.TravelRelated:
		return "PROMPT_NOT_TRAVEL_RELATED"
	default:
		return ""
	}
}

func applyTravelIntent(
	user *domain.User,
	recommendation *domain.RecommendationRequest,
	intent domain.TravelIntent,
) {
	if len(intent.Themes) > 0 {
		user.Preferences.Themes = intent.Themes
	}

	if selectedModes := validateTransportModes(intent.TransportModes, recommendation.TransportModes); len(selectedModes) > 0 {
		recommendation.TransportModes = selectedModes
	}
}

func validateSearchPlan(
	plan domain.TravelSearchPlan,
	candidates []domain.Territory,
) []domain.Territory {
	allowed := make(map[string]domain.Territory, len(candidates))
	for _, candidate := range candidates {
		allowed[candidate.ID] = candidate
	}

	selected := make([]domain.Territory, 0, len(plan.CandidateIDs))
	seen := make(map[string]struct{}, len(plan.CandidateIDs))

	for _, candidateID := range plan.CandidateIDs {
		candidate, found := allowed[candidateID]
		if !found {
			continue
		}

		if _, duplicate := seen[candidateID]; duplicate {
			continue
		}

		seen[candidateID] = struct{}{}

		selected = append(selected, candidate)
	}

	return selected
}

func validateTransportModes(planned []string, allowed []string) []string {
	allowlist := make(map[string]struct{}, len(allowed))
	for _, mode := range allowed {
		allowlist[mode] = struct{}{}
	}

	selected := make([]string, 0, len(planned))
	seen := make(map[string]struct{}, len(planned))

	for _, mode := range planned {
		if _, allowedMode := allowlist[mode]; !allowedMode {
			continue
		}

		if _, duplicate := seen[mode]; duplicate {
			continue
		}

		seen[mode] = struct{}{}

		selected = append(selected, mode)
	}

	return selected
}

func validRecommendationSettings(settings domain.RecommendationSettings) bool {
	return settings.CandidateSearchLimit > 0 &&
		settings.OfferPageSize > 0 &&
		settings.MaximumRankedOptions > 0 &&
		settings.OfferValidityMinutes > 0
}

func validPersonalRecommendationSettings(settings domain.PersonalRecommendationSettings) bool {
	return settings.LeadDays >= 0 &&
		settings.DefaultDurationDays > 0 &&
		settings.MaximumDurationDays >= settings.DefaultDurationDays &&
		settings.FreshnessHours > 0 &&
		settings.Adults > 0 &&
		settings.Currency != ""
}

func (s *Service) failWorkflow(ctx context.Context, recommendationID string, reason string, cause error) {
	s.logger.Error(
		"recommendation workflow failed",
		"recommendation_id", recommendationID,
		"reason", reason,
		"error", cause,
	)

	if err := s.repository.Fail(ctx, recommendationID, reason); err != nil {
		s.logger.Error(
			"mark recommendation as failed",
			"recommendation_id", recommendationID,
			"reason", reason,
			"error", err,
		)
	}
}

func (s *Service) recoverWorkflow(ctx context.Context, recommendationID string) {
	recovered := recover()
	if recovered == nil {
		return
	}

	s.logger.Error(
		"recommendation workflow panicked",
		"recommendation_id", recommendationID,
		"panic", recovered,
		"stack", string(debug.Stack()),
	)

	s.failWorkflow(
		ctx,
		recommendationID,
		"RECOMMENDATION_WORKFLOW_PANIC",
		fmt.Errorf("%w: %v", recommendation_errors.ErrWorkflowPanic, recovered),
	)
}

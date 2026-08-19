package ai_errors

import "errors"

var (
	ErrNotConfigured       = errors.New("AI provider is not configured")
	ErrUnexpectedStatus    = errors.New("AI provider returned an unexpected status")
	ErrMissingChoice       = errors.New("AI provider response has no choices")
	ErrInvalidEventPayload = errors.New("AI event payload is invalid")
	ErrInvalidSearchPlan   = errors.New("AI search plan is invalid")
	ErrEmptySearchPlan     = errors.New("AI search plan has no destinations")
	ErrInvalidRanking      = errors.New("AI recommendation ranking is invalid")
	ErrInvalidAnalysis     = errors.New("AI request analysis is invalid")
	ErrPromptConfiguration = errors.New("AI system prompt configuration is invalid")
	ErrTemporaryFailure    = errors.New("AI provider is temporarily unavailable")
	ErrTruncatedCompletion = errors.New("AI provider response was truncated")
	ErrRedirectNotAllowed  = errors.New("AI provider redirect is not allowed")
	ErrInvalidDiscovery    = errors.New("AI event discovery payload is invalid")
	ErrEmptyDiscovery      = errors.New("AI event discovery returned no events")
)

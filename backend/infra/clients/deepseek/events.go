package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	requestTimeout          = 25 * time.Second
	responseBodyLimit       = 1 << 20
	discoveryTemperature    = 0.4
	maximumEventCount       = 60
	maximumDescriptionRunes = 700
	maximumCompletionTokens = 4096
)

type EventEnricher struct {
	transport    chatClient
	model        string
	systemPrompt string
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

type enrichmentEnvelope struct {
	Events []domain.EventEnrichment `json:"events"`
}

func NewEventEnricher(apiKey string, baseURL string, model string, systemPrompt string) *EventEnricher {
	return &EventEnricher{
		transport:    newChatClient(apiKey, baseURL),
		model:        model,
		systemPrompt: systemPrompt,
	}
}

func (e *EventEnricher) Enrich(
	ctx context.Context,
	city domain.Territory,
	events []domain.Event,
) ([]domain.EventEnrichment, error) {
	if e.transport.apiKey == "" {
		return nil, ai_errors.ErrNotConfigured
	}

	if len(events) == 0 || len(events) > maximumEventCount {
		return nil, ai_errors.ErrInvalidEventPayload
	}

	input := struct {
		CityName string         `json:"city_name"`
		Region   string         `json:"region"`
		Events   []domain.Event `json:"events"`
	}{CityName: city.Name, Region: city.Region, Events: events}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode event enrichment input: %w", err)
	}

	payload := chatRequest{
		Model: e.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: e.systemPrompt,
			},
			{Role: "user", Content: string(inputJSON)},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
		Temperature:    discoveryTemperature,
		MaxTokens:      maximumCompletionTokens,
	}

	content, err := e.transport.complete(ctx, payload)
	if err != nil {
		return nil, err
	}

	var envelope enrichmentEnvelope
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		return nil, fmt.Errorf("decode DeepSeek event enrichment JSON: %w", err)
	}

	if err := validateEnrichments(envelope.Events, events); err != nil {
		return nil, err
	}

	return envelope.Events, nil
}

func validateEnrichments(enrichments []domain.EventEnrichment, source []domain.Event) error {
	if len(enrichments) != len(source) {
		return ai_errors.ErrInvalidEventPayload
	}

	allowedIDs := make(map[string]struct{}, len(source))
	for _, event := range source {
		allowedIDs[event.ID] = struct{}{}
	}

	seen := make(map[string]struct{}, len(enrichments))

	for _, enrichment := range enrichments {
		if _, allowed := allowedIDs[enrichment.EventID]; !allowed {
			return ai_errors.ErrInvalidEventPayload
		}

		if _, duplicate := seen[enrichment.EventID]; duplicate {
			return ai_errors.ErrInvalidEventPayload
		}

		seen[enrichment.EventID] = struct{}{}

		if !validCategory(enrichment.Category) || !validEventDescription(enrichment.Description) {
			return ai_errors.ErrInvalidEventPayload
		}
	}

	return nil
}

func validCategory(value string) bool {
	trimmed := strings.TrimSpace(value)

	return trimmed != "" && len([]rune(trimmed)) <= 80 && !strings.ContainsAny(trimmed, "<>/\\")
}

func validEventDescription(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len([]rune(trimmed)) > maximumDescriptionRunes ||
		strings.ContainsAny(trimmed, "<>") {
		return false
	}

	lowercase := strings.ToLower(trimmed)

	return !strings.Contains(lowercase, "http://") &&
		!strings.Contains(lowercase, "https://") &&
		!strings.Contains(lowercase, "www.")
}

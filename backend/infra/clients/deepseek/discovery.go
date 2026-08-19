package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	discoveryCityTokens     = 6000
	discoveryTopTokens      = 9000
	discoveryCityToolCalls  = 1
	discoveryTopToolCalls   = 3
	discoveryDefaultHours   = 3
	maximumDiscoveredEvents = 40
	maximumTitleRunes       = 160
	maximumVenueRunes       = 160
	minimumTitleRunes       = 3
	maximumDiscoveredPrice  = 1_000_000
	discoveryTrailingDays   = 7
)

var discoveryCategories = map[string]string{
	"festival":   "festival",
	"фестиваль":  "festival",
	"concert":    "concert",
	"концерт":    "concert",
	"музыка":     "concert",
	"exhibition": "exhibition",
	"выставка":   "exhibition",
	"museum":     "exhibition",
	"theatre":    "theatre",
	"theater":    "theatre",
	"театр":      "theatre",
	"спектакль":  "theatre",
	"sport":      "sport",
	"спорт":      "sport",
	"матч":       "sport",
	"food":       "food",
	"еда":        "food",
	"гастро":     "food",
}

type EventDiscoverer struct {
	transport searchClient
	model     string
}

type discoveryEnvelope struct {
	Events []discoveredEventPayload `json:"events"`
}

type discoveredEventPayload struct {
	City        string `json:"city"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Venue       string `json:"venue"`
	Description string `json:"description"`
	SourceURL   string `json:"source_url"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	PriceFrom   int    `json:"price_from"`
}

func NewEventDiscoverer(
	apiKey string,
	baseURL string,
	model string,
	timeout time.Duration,
) *EventDiscoverer {
	return &EventDiscoverer{
		transport: newSearchClient(apiKey, baseURL, timeout),
		model:     strings.TrimSpace(model),
	}
}

func (d *EventDiscoverer) DiscoverCity(
	ctx context.Context,
	city domain.Territory,
	dateFrom time.Time,
	dateTo time.Time,
	limit int,
) ([]domain.DiscoveredEvent, error) {
	input := fmt.Sprintf(
		"Афиша города %s, %s. Период: %s — %s. Нужно до %d событий для туриста.\n%s",
		city.Name,
		city.Region,
		dateFrom.Format(time.DateOnly),
		dateTo.Format(time.DateOnly),
		limit,
		discoveryFormat(false),
	)

	events, err := d.discover(ctx, input, dateFrom, dateTo, limit, discoveryCityToolCalls, discoveryCityTokens)
	if err != nil {
		return nil, err
	}

	for index := range events {
		events[index].CityID = city.ID
		events[index].CityName = city.Name
	}

	return events, nil
}

func (d *EventDiscoverer) DiscoverPopular(
	ctx context.Context,
	cities []domain.Territory,
	dateFrom time.Time,
	dateTo time.Time,
	limit int,
) ([]domain.DiscoveredEvent, error) {
	if len(cities) == 0 {
		return nil, ai_errors.ErrEmptyDiscovery
	}

	input := fmt.Sprintf(
		"Главные события России, период %s — %s. Нужно %d крупных событий федерального уровня "+
			"из разных городов, максимум два события на город.\n"+
			"В поле city — город в именительном падеже, например «Казань».\n%s",
		dateFrom.Format(time.DateOnly),
		dateTo.Format(time.DateOnly),
		limit,
		discoveryFormat(true),
	)

	events, err := d.discover(ctx, input, dateFrom, dateTo, limit, discoveryTopToolCalls, discoveryTopTokens)
	if err != nil {
		return nil, err
	}

	matched := make([]domain.DiscoveredEvent, 0, len(events))
	index := cityIndex(cities)

	for _, event := range events {
		city, found := index[normalizeCityKey(event.CityName)]
		if !found {
			continue
		}

		event.CityID = city.ID
		event.CityName = city.Name
		event.Rank = len(matched) + 1
		matched = append(matched, event)
	}

	if len(matched) == 0 {
		return nil, ai_errors.ErrEmptyDiscovery
	}

	return matched, nil
}

func (d *EventDiscoverer) discover(
	ctx context.Context,
	input string,
	dateFrom time.Time,
	dateTo time.Time,
	limit int,
	toolCalls int,
	outputTokens int,
) ([]domain.DiscoveredEvent, error) {
	content, err := d.transport.search(ctx, searchRequest{
		Model: d.model,
		Instructions: fmt.Sprintf(
			"Составь афишу по результатам поиска. Сделай %d поиск(а) и сразу верни ответ. "+
				"Не проверяй события отдельными запросами, не рассуждай, не пиши текст вокруг JSON. "+
				"Бери только события из результатов поиска, ничего не придумывай. "+
				"Ответ — один JSON-объект.",
			toolCalls,
		),
		Input:           input,
		MaxOutputTokens: outputTokens,
		MaxToolCalls:    toolCalls,
	})
	if err != nil {
		return nil, fmt.Errorf("discover events: %w", err)
	}

	var envelope discoveryEnvelope
	if err := json.Unmarshal([]byte(discoveryJSON(content)), &envelope); err != nil {
		return nil, fmt.Errorf("%w: %w", ai_errors.ErrInvalidDiscovery, err)
	}

	events := normalizeDiscovered(envelope.Events, dateFrom, dateTo, limit)
	if len(events) == 0 {
		return nil, ai_errors.ErrEmptyDiscovery
	}

	return events, nil
}

func normalizeDiscovered(
	payloads []discoveredEventPayload,
	dateFrom time.Time,
	dateTo time.Time,
	limit int,
) []domain.DiscoveredEvent {
	if limit <= 0 || limit > maximumDiscoveredEvents {
		limit = maximumDiscoveredEvents
	}

	events := make([]domain.DiscoveredEvent, 0, limit)
	seen := make(map[string]struct{}, limit)

	for _, payload := range payloads {
		event, valid := normalizeDiscoveredEvent(payload, dateFrom, dateTo)
		if !valid {
			continue
		}

		key := strings.ToLower(event.Title) + "|" + event.StartsAt.Format(time.DateOnly)
		if _, duplicate := seen[key]; duplicate {
			continue
		}

		seen[key] = struct{}{}
		event.Rank = len(events) + 1
		events = append(events, event)

		if len(events) == limit {
			break
		}
	}

	return events
}

func normalizeDiscoveredEvent(
	payload discoveredEventPayload,
	dateFrom time.Time,
	dateTo time.Time,
) (domain.DiscoveredEvent, bool) {
	title := boundedText(payload.Title, maximumTitleRunes)
	if len([]rune(title)) < minimumTitleRunes || containsMarkup(title) {
		return domain.DiscoveredEvent{}, false
	}

	startsAt, valid := parseDiscoveredTime(payload.StartsAt)
	if !valid {
		return domain.DiscoveredEvent{}, false
	}

	windowStart := dateFrom.AddDate(0, 0, -1)
	windowEnd := dateTo.AddDate(0, 0, discoveryTrailingDays)

	if startsAt.Before(windowStart) || startsAt.After(windowEnd) {
		return domain.DiscoveredEvent{}, false
	}

	endsAt, hasEnd := parseDiscoveredTime(payload.EndsAt)
	if !hasEnd || endsAt.Before(startsAt) {
		endsAt = startsAt.Add(discoveryDefaultHours * time.Hour)
	}

	if endsAt.After(windowEnd.AddDate(0, 0, discoveryTrailingDays)) {
		endsAt = startsAt.Add(discoveryDefaultHours * time.Hour)
	}

	price := payload.PriceFrom
	if price < 0 || price > maximumDiscoveredPrice {
		price = 0
	}

	description := boundedText(payload.Description, maximumDescriptionRunes)
	if containsMarkup(description) {
		description = ""
	}

	return domain.DiscoveredEvent{
		CityName:    strings.TrimSpace(payload.City),
		Title:       title,
		Category:    normalizeCategory(payload.Category),
		Venue:       boundedText(payload.Venue, maximumVenueRunes),
		Description: description,
		SourceURL:   normalizeSourceURL(payload.SourceURL),
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		PriceFrom:   price,
		Currency:    "RUB",
	}, true
}

func discoveryFormat(includeCity bool) string {
	cityField := ""
	if includeCity {
		cityField = `"city":"Казань",`
	}

	return "Формат ответа:\n" +
		`{"events":[{` + cityField +
		`"title":"Название","category":"concert|festival|exhibition|theatre|sport|food|other",` +
		`"venue":"Площадка","description":"Одно предложение до 140 символов",` +
		`"starts_at":"2026-09-05T19:00:00+03:00","ends_at":"","price_from":1500,"source_url":""}]}` + "\n" +
		"price_from в рублях, 0 если бесплатно. ends_at и source_url можно оставить пустыми."
}

func discoveryJSON(content string) string {
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')

	if start < 0 || end <= start {
		return content
	}

	return content[start : end+1]
}

func cityIndex(cities []domain.Territory) map[string]domain.Territory {
	index := make(map[string]domain.Territory, len(cities))

	for _, city := range cities {
		index[normalizeCityKey(city.Name)] = city
	}

	return index
}

func normalizeCityKey(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "ё", "е")))
}

func normalizeCategory(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "other"
	}

	if category, found := discoveryCategories[normalized]; found {
		return category
	}

	for keyword, category := range discoveryCategories {
		if strings.Contains(normalized, keyword) {
			return category
		}
	}

	return "other"
}

func normalizeSourceURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}

	return parsed.String()
}

func parseDiscoveredTime(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		time.DateOnly,
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}

func boundedText(value string, limit int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")

	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}

	return strings.TrimSpace(string(runes[:limit]))
}

func containsMarkup(value string) bool {
	lowered := strings.ToLower(value)

	return strings.Contains(lowered, "<") ||
		strings.Contains(lowered, "http://") ||
		strings.Contains(lowered, "https://")
}

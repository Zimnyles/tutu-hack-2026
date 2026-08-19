package profile

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	maximumThemes           = 12
	maximumAvoid            = 12
	maximumTransportModes   = 8
	maximumTravelMinutes    = 3 * 24 * 60
	minimumTravelMinutes    = 30
	maximumTypicalBudget    = 2_000_000
	minimumTypicalBudget    = 1_000
	maximumTripDurationDays = 30
	minimumSpontaneity      = 1
	maximumSpontaneity      = 5
	maximumTagLength        = 32
)

var (
	allowedThemes = map[string]struct{}{
		"nature":       {},
		"architecture": {},
		"food":         {},
		"history":      {},
		"events":       {},
		"calm":         {},
		"active":       {},
		"unusual":      {},
	}
	allowedTransportModes = map[string]struct{}{
		"railway": {},
		"bus":     {},
		"avia":    {},
		"etrain":  {},
	}
	allowedVisibility = map[string]struct{}{
		"private":   {},
		"aggregate": {},
	}
)

func normalizePreferences(preferences domain.Preferences) (domain.Preferences, error) {
	themes, err := normalizeTags(preferences.Themes, maximumThemes, allowedThemes)
	if err != nil {
		return domain.Preferences{}, invalidInput("Проверьте выбранные интересы")
	}

	modes, err := normalizeTags(preferences.TransportModes, maximumTransportModes, allowedTransportModes)
	if err != nil {
		return domain.Preferences{}, invalidInput("Проверьте выбранный транспорт")
	}

	if len(modes) == 0 {
		return domain.Preferences{}, invalidInput("Выберите хотя бы один вид транспорта")
	}

	avoid, err := normalizeTags(preferences.Avoid, maximumAvoid, nil)
	if err != nil {
		return domain.Preferences{}, invalidInput("Проверьте список ограничений")
	}

	if preferences.MaxTravelMinutes < minimumTravelMinutes ||
		preferences.MaxTravelMinutes > maximumTravelMinutes {
		return domain.Preferences{}, invalidInput("Проверьте максимальное время в пути")
	}

	if preferences.TypicalBudget < minimumTypicalBudget ||
		preferences.TypicalBudget > maximumTypicalBudget {
		return domain.Preferences{}, invalidInput("Проверьте типичный бюджет поездки")
	}

	if preferences.TripDurationDays < 1 || preferences.TripDurationDays > maximumTripDurationDays {
		return domain.Preferences{}, invalidInput("Проверьте длительность поездки")
	}

	if preferences.Spontaneity < minimumSpontaneity || preferences.Spontaneity > maximumSpontaneity {
		return domain.Preferences{}, invalidInput("Спонтанность должна быть от 1 до 5")
	}

	preferences.Themes = themes
	preferences.TransportModes = modes
	preferences.Avoid = avoid

	return preferences, nil
}

func normalizeTags(
	values []string,
	maximum int,
	allowed map[string]struct{},
) ([]string, error) {
	if len(values) > maximum {
		return nil, invalidInput("Слишком много значений")
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		tag := strings.ToLower(strings.TrimSpace(value))
		if tag == "" {
			continue
		}

		if len(tag) > maximumTagLength {
			return nil, invalidInput("Слишком длинное значение")
		}

		if allowed != nil {
			if _, permitted := allowed[tag]; !permitted {
				return nil, invalidInput("Недопустимое значение")
			}
		}

		if _, duplicate := seen[tag]; duplicate {
			continue
		}

		seen[tag] = struct{}{}

		normalized = append(normalized, tag)
	}

	return normalized, nil
}

func normalizeVisibility(visibility string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(visibility))
	if _, allowed := allowedVisibility[trimmed]; !allowed {
		return "", invalidInput("Недопустимый режим приватности")
	}

	return trimmed, nil
}

func normalizeHomeCity(homeCityID string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(homeCityID))
	if err != nil {
		return "", invalidInput("Выберите домашний город")
	}

	return parsed.String(), nil
}

func invalidInput(message string) error {
	return &domain.AppError{
		Code:    "INVALID_INPUT",
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

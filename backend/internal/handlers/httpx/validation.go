package httpx

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const DateLayout = "2006-01-02"

func ValidationError(message string) error {
	return &domain.AppError{
		Code:    "VALIDATION_ERROR",
		Message: message,
		Status:  fiber.StatusUnprocessableEntity,
	}
}

func RequiredIdentifier(c *fiber.Ctx, name string) (string, error) {
	return Identifier(c.Params(name))
}

func Identifier(rawValue string) (string, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return "", ValidationError("Не указан идентификатор")
	}

	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return "", ValidationError("Некорректный идентификатор")
	}

	return parsed.String(), nil
}

func OptionalIdentifier(rawValue string) (string, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return "", nil
	}

	return Identifier(trimmed)
}

func Date(rawValue string) (time.Time, error) {
	parsed, err := time.Parse(DateLayout, strings.TrimSpace(rawValue))
	if err != nil {
		return time.Time{}, ValidationError("Дата должна быть в формате ГГГГ-ММ-ДД")
	}

	return parsed, nil
}

func BoundedText(rawValue string, minimum int, maximum int, message string) (string, error) {
	trimmed := strings.TrimSpace(rawValue)
	length := utf8.RuneCountInString(trimmed)

	if length < minimum || length > maximum {
		return "", ValidationError(message)
	}

	return trimmed, nil
}

func BoundedInteger(value int, minimum int, maximum int, message string) error {
	if value < minimum || value > maximum {
		return ValidationError(message)
	}

	return nil
}

func BoundedList(values []string, maximum int, message string) ([]string, error) {
	if len(values) > maximum {
		return nil, ValidationError(message)
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}

		if _, duplicate := seen[trimmed]; duplicate {
			continue
		}

		seen[trimmed] = struct{}{}

		normalized = append(normalized, trimmed)
	}

	return normalized, nil
}

func OneOf(value string, allowed []string, message string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if trimmed == candidate {
			return trimmed, nil
		}
	}

	return "", ValidationError(message)
}

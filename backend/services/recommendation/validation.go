package recommendation

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	maximumTravelHorizonDays = 365
	maximumTripLengthDays    = 30
	maximumTransportModes    = 8
	maximumAdults            = 8
	maximumChildren          = 7
	maximumTravelMinutes     = 3 * 24 * 60
	maximumBudgetAmount      = 2_000_000
	defaultCurrency          = "RUB"
)

var (
	allowedKinds = map[string]struct{}{
		"personal": {},
		"prompt":   {},
		"event":    {},
	}
	allowedCurrencies = map[string]struct{}{
		"RUB": {},
	}
	supportedTransportModes = []string{"railway", "bus", "avia", "etrain"}
	allowedTransportModes   = transportModeSet(supportedTransportModes)
)

func transportModeSet(modes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(modes))
	for _, mode := range modes {
		set[mode] = struct{}{}
	}

	return set
}

func normalizeInput(input Input) (Input, *domain.AppError) {
	normalized := input

	if normalized.Kind == "" {
		normalized.Kind = "prompt"
	}

	if _, allowed := allowedKinds[normalized.Kind]; !allowed {
		return Input{}, invalidInput("Неизвестный тип рекомендации")
	}

	identifiers := []*string{
		&normalized.OriginCityID,
		&normalized.DestinationID,
		&normalized.EventID,
	}
	for index, identifier := range identifiers {
		value, err := normalizeIdentifier(*identifier, index == 0)
		if err != nil {
			return Input{}, err
		}

		*identifier = value
	}

	if normalized.DestinationID != "" && normalized.DestinationID == normalized.OriginCityID {
		return Input{}, invalidInput("Город прибытия совпадает с домашним городом")
	}

	dateFrom, dateTo, err := normalizeDateRange(normalized.DateFrom, normalized.DateTo)
	if err != nil {
		return Input{}, err
	}

	normalized.DateFrom = dateFrom.Format(time.DateOnly)
	normalized.DateTo = dateTo.Format(time.DateOnly)

	if normalized.Adults < 1 || normalized.Adults > maximumAdults {
		return Input{}, invalidInput("Проверьте количество взрослых")
	}

	if normalized.Children < 0 || normalized.Children > maximumChildren {
		return Input{}, invalidInput("Проверьте количество детей")
	}

	if normalized.Budget < 0 || normalized.Budget > maximumBudgetAmount {
		return Input{}, invalidInput("Проверьте бюджет поездки")
	}

	if normalized.MaxTravelMinutes < 0 || normalized.MaxTravelMinutes > maximumTravelMinutes {
		return Input{}, invalidInput("Проверьте максимальное время в пути")
	}

	currency := strings.ToUpper(strings.TrimSpace(normalized.Currency))
	if currency == "" {
		currency = defaultCurrency
	}

	if _, allowed := allowedCurrencies[currency]; !allowed {
		return Input{}, invalidInput("Поддерживается только рубль")
	}

	normalized.Currency = currency

	modes, err := normalizeTransportModes(normalized.TransportModes)
	if err != nil {
		return Input{}, err
	}

	normalized.TransportModes = modes

	return normalized, nil
}

func normalizeIdentifier(rawValue string, required bool) (string, *domain.AppError) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		if required {
			return "", invalidInput("Укажите город отправления")
		}

		return "", nil
	}

	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return "", invalidInput("Некорректный идентификатор")
	}

	return parsed.String(), nil
}

func normalizeDateRange(rawFrom string, rawTo string) (time.Time, time.Time, *domain.AppError) {
	dateFrom, fromError := time.Parse(time.DateOnly, strings.TrimSpace(rawFrom))
	dateTo, toError := time.Parse(time.DateOnly, strings.TrimSpace(rawTo))

	if fromError != nil || toError != nil {
		return time.Time{}, time.Time{}, invalidInput("Даты должны быть в формате ГГГГ-ММ-ДД")
	}

	today := time.Now().UTC().Truncate(dayDuration)
	if dateFrom.Before(today) || dateTo.Before(dateFrom) {
		return time.Time{}, time.Time{}, invalidInput("Проверьте диапазон дат поездки")
	}

	if dateFrom.After(today.AddDate(0, 0, maximumTravelHorizonDays)) {
		return time.Time{}, time.Time{}, invalidInput("Поездку можно планировать не дальше чем на год")
	}

	if dateTo.Sub(dateFrom) > maximumTripLengthDays*dayDuration {
		return time.Time{}, time.Time{}, invalidInput("Длительность поездки не может превышать 30 дней")
	}

	return dateFrom, dateTo, nil
}

func normalizeTransportModes(values []string) ([]string, *domain.AppError) {
	if len(values) > maximumTransportModes {
		return nil, invalidInput("Слишком много видов транспорта")
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		mode := strings.ToLower(strings.TrimSpace(value))
		if mode == "" {
			continue
		}

		if _, allowed := allowedTransportModes[mode]; !allowed {
			return nil, invalidInput("Неизвестный вид транспорта")
		}

		if _, duplicate := seen[mode]; duplicate {
			continue
		}

		seen[mode] = struct{}{}

		normalized = append(normalized, mode)
	}

	if len(normalized) == 0 {
		return nil, invalidInput("Выберите хотя бы один вид транспорта")
	}

	return normalized, nil
}

func invalidInput(message string) *domain.AppError {
	return &domain.AppError{
		Code:    "INVALID_INPUT",
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

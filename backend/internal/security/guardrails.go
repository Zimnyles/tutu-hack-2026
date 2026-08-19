package security

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	maximumPromptLength = 1500
	maximumTravelParty  = 8
	minimumBudget       = 1000
	maximumBudget       = 2_000_000
	maximumTransports   = 8
	maximumRepeats      = 40
)

var (
	emailPattern     = regexp.MustCompile(`(?i)[A-Z0-9._%+-]+\s*(?:@|\(at\)|\[at\])\s*[A-Z0-9.-]+\.[A-Z]{2,}`)
	phonePattern     = regexp.MustCompile(`(?:\+?7|8)[\s()-]*\d{3}[\s()-]*\d{3}[\s-]*\d{2}[\s-]*\d{2}`)
	cardPattern      = regexp.MustCompile(`(?:\d[ -]?){13,19}`)
	passportPattern  = regexp.MustCompile(`(?i)(паспорт|снилс|инн|passport)\D{0,20}\d[\d\s-]{5,}`)
	secretPattern    = regexp.MustCompile(`(?i)(api[_-]?key|bearer|token|secret|password|пароль)\s*[:=]\s*\S{8,}`)
	injectionPattern = regexp.MustCompile(
		`(?i)(ignore|disregard|forget|override|игнорируй|забудь|отмени|перепиши).{0,40}` +
			`(instruction|prompt|rule|инструкц|правил|указан|промпт)|` +
			`system prompt|системн.{0,10}промпт|developer message|developer mode|` +
			`you are now|ты теперь|act as|веди себя как|jailbreak|dan mode|` +
			`(покажи|reveal|print|выведи).{0,30}(prompt|промпт|инструкц|instruction)|` +
			`вызови.{0,20}(url|tool|инструмент)|base64|` +
			`<\s*/?\s*system\s*>|<\|.{0,20}\|>|\[\[.{0,20}\]\]`,
	)
	politicsPattern  = regexp.MustCompile(`(?i)(агитац|политик|парт(ия|ии)|выборы|президент).{0,80}(оцени|лучше|хуже|голос|поддерж)`)
	nonTravelPattern = regexp.MustCompile(`(?i)^(напиши код|реши уравнение|сочини|переведи|кто ты|какой курс валют)`)
)

type RecommendationInput struct {
	OriginCityID   string   `json:"origin_city_id"`
	DateFrom       string   `json:"date_from"`
	DateTo         string   `json:"date_to"`
	Adults         int      `json:"adults"`
	Children       int      `json:"children"`
	Budget         int      `json:"budget"`
	TransportModes []string `json:"transport_modes"`
	Prompt         string   `json:"prompt"`
}

func ValidateRecommendation(v RecommendationInput) *domain.AppError {
	if invalidRecommendationFields(v) {
		return &domain.AppError{
			Code:    "INVALID_INPUT",
			Message: "Проверьте город отправления, даты, пассажиров, бюджет и транспорт",
			Status:  http.StatusBadRequest,
		}
	}

	if utf8.RuneCountInString(strings.TrimSpace(v.Prompt)) > maximumPromptLength {
		return &domain.AppError{
			Code:    "INVALID_INPUT",
			Message: "Пожелание должно быть короче 1 500 символов",
			Status:  http.StatusBadRequest,
		}
	}

	prompt := NormalizePrompt(v.Prompt)

	if hasExcessiveRepetition(prompt) {
		return &domain.AppError{
			Code:    "INVALID_INPUT",
			Message: "Пожелание содержит слишком много повторяющихся символов",
			Status:  http.StatusBadRequest,
		}
	}

	if emailPattern.MatchString(prompt) ||
		phonePattern.MatchString(prompt) ||
		cardPattern.MatchString(prompt) ||
		passportPattern.MatchString(prompt) ||
		secretPattern.MatchString(prompt) {
		return &domain.AppError{
			Code:    "PROMPT_CONTAINS_PII",
			Message: "Уберите из пожелания контакты, реквизиты и другие личные данные",
			Status:  http.StatusBadRequest,
		}
	}

	if injectionPattern.MatchString(prompt) {
		return &domain.AppError{
			Code:    "PROMPT_INJECTION_DETECTED",
			Message: "Запрос содержит недопустимые служебные инструкции",
			Status:  http.StatusBadRequest,
		}
	}

	if politicsPattern.MatchString(prompt) {
		return &domain.AppError{
			Code:    "POLITICAL_REQUEST_BLOCKED",
			Message: "Я могу помочь только с выбором и проверкой путешествий",
			Status:  http.StatusBadRequest,
		}
	}

	if nonTravelPattern.MatchString(prompt) {
		return &domain.AppError{
			Code:    "PROMPT_NOT_TRAVEL_RELATED",
			Message: "Запрос должен относиться к планированию путешествия",
			Status:  http.StatusBadRequest,
		}
	}

	return nil
}

func invalidRecommendationFields(value RecommendationInput) bool {
	return value.OriginCityID == "" ||
		value.DateFrom == "" ||
		value.DateTo == "" ||
		value.Adults < 1 ||
		value.Children < 0 ||
		value.Adults+value.Children > maximumTravelParty ||
		value.Budget < minimumBudget ||
		value.Budget > maximumBudget ||
		len(value.TransportModes) == 0 ||
		len(value.TransportModes) > maximumTransports
}

func hasExcessiveRepetition(prompt string) bool {
	var previous rune

	repeats := 0

	for _, symbol := range prompt {
		if symbol == previous {
			repeats++
			if repeats >= maximumRepeats {
				return true
			}

			continue
		}

		previous = symbol
		repeats = 1
	}

	return false
}

func NormalizePrompt(prompt string) string {
	var builder strings.Builder

	builder.Grow(len(prompt))

	for _, symbol := range prompt {
		switch {
		case symbol == '\n' || symbol == '\t' || symbol == '\r':
			builder.WriteRune(' ')
		case unicode.Is(unicode.Cf, symbol) || unicode.Is(unicode.Cc, symbol):
			continue
		case unicode.Is(unicode.Mn, symbol):
			continue
		default:
			builder.WriteRune(symbol)
		}
	}

	return strings.Join(strings.Fields(builder.String()), " ")
}

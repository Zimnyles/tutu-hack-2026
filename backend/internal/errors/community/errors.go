package community_errors

import (
	"errors"
	"net/http"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

var ErrLeagueRulesConfiguration = errors.New("league rules configuration is invalid")

var (
	ErrUnknownLeaderboardScope = &domain.AppError{
		Code:    "INVALID_INPUT",
		Message: "Неизвестная область рейтинга",
		Status:  http.StatusBadRequest,
	}
	ErrUnknownLeaderboardPeriod = &domain.AppError{
		Code:    "INVALID_INPUT",
		Message: "Неизвестный период рейтинга",
		Status:  http.StatusBadRequest,
	}
)

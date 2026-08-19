package scoring_errors

import "errors"

var (
	ErrInvalidConfiguration = errors.New("scoring configuration is invalid")
	ErrNoEligibleOffers     = errors.New("no offers satisfy mandatory recommendation constraints")
)

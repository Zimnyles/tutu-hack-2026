package trips_errors

import "errors"

var (
	ErrTripNotFound          = errors.New("trip not found")
	ErrInvalidState          = errors.New("trip state transition is not allowed")
	ErrMissingIdempotencyKey = errors.New("idempotency key is required")
)

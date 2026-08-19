package adminsim_errors

import "errors"

var (
	ErrAccessDenied      = errors.New("admin access denied")
	ErrSimulatorDisabled = errors.New("admin simulator is disabled")
	ErrActionNotAllowed  = errors.New("admin action is not allowed")
	ErrInvalidReason     = errors.New("admin reason is too short")
	ErrTargetNotDemo     = errors.New("admin target is not a demo record")

	ErrInvalidIdempotencyKey = errors.New("admin idempotency key is invalid")
	ErrInvalidTarget         = errors.New("admin target is invalid")
	ErrInvalidParameters     = errors.New("admin parameters are invalid")
	ErrInvalidQuery          = errors.New("admin search query is invalid")
)

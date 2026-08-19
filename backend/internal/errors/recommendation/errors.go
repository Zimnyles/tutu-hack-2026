package recommendation_errors

import "errors"

var (
	ErrWorkflowConfiguration        = errors.New("workflow configuration is empty")
	ErrExpired                      = errors.New("recommendation has expired")
	ErrNoDestinations               = errors.New("no destinations found")
	ErrUnsupportedCompletionStatus  = errors.New("unsupported recommendation completion status")
	ErrInvalidPersonalConfiguration = errors.New("personal recommendation configuration is invalid")
	ErrWorkflowPanic                = errors.New("recommendation workflow panicked")
)

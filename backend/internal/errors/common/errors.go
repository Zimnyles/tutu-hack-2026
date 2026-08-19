package common_errors

import "errors"

var (
	ErrNotFound     = errors.New("record not found")
	ErrInvalidState = errors.New("invalid domain state")
	ErrConflict     = errors.New("record conflict")
)

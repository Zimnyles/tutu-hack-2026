package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	uniqueViolationCode       = "23505"
	serializationFailureCode  = "40001"
	deadlockDetectedCode      = "40P01"
	checkViolationCode        = "23514"
	foreignKeyViolationCode   = "23503"
	invalidTextRepresentation = "22P02"
)

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func IsSerializationFailure(err error) bool {
	return hasCode(err, serializationFailureCode) || hasCode(err, deadlockDetectedCode)
}

func IsConstraintViolation(err error) bool {
	return hasCode(err, checkViolationCode) || hasCode(err, foreignKeyViolationCode)
}

func IsInvalidValue(err error) bool {
	return hasCode(err, invalidTextRepresentation)
}

func hasCode(err error, code string) bool {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return false
	}

	return postgresError.Code == code
}

func IsUniqueViolation(err error) bool {
	return hasCode(err, uniqueViolationCode)
}

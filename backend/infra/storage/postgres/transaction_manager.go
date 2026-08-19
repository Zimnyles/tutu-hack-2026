package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	serializableAttempts = 3
	serializableBackoff  = 20 * time.Millisecond
)

var ErrTransactionPanic = errors.New("transaction panicked")

type TransactionManager struct {
	database *pgxpool.Pool
}

func NewTransactionManager(database *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{database: database}
}

func (m *TransactionManager) WithinTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	return m.run(ctx, pgx.TxOptions{}, operation)
}

func (m *TransactionManager) WithinSerializableTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}

	var lastError error

	for attempt := 1; attempt <= serializableAttempts; attempt++ {
		lastError = m.run(ctx, options, operation)
		if lastError == nil {
			return nil
		}

		if !IsSerializationFailure(lastError) {
			return lastError
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(serializableBackoff * time.Duration(attempt)):
		}
	}

	return lastError
}

func (m *TransactionManager) run(
	ctx context.Context,
	options pgx.TxOptions,
	operation func(context.Context) error,
) (err error) {
	transaction, beginError := m.database.BeginTx(ctx, options)
	if beginError != nil {
		return fmt.Errorf("begin transaction: %w", beginError)
	}

	committed := false

	defer func() {
		recovered := recover()

		if !committed {
			rollbackContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*5)
			defer cancel()

			_ = transaction.Rollback(rollbackContext)
		}

		if recovered != nil {
			err = fmt.Errorf("%w: %v", ErrTransactionPanic, recovered)
		}
	}()

	if err = operation(ContextWithTransaction(ctx, transaction)); err != nil {
		return err
	}

	if err = transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	committed = true

	return nil
}

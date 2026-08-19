package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type transactionContextKey struct{}

func ExecutorFromContext(ctx context.Context, pool *pgxpool.Pool) Executor {
	transaction, ok := ctx.Value(transactionContextKey{}).(pgx.Tx)
	if ok {
		return transaction
	}

	return pool
}

func ContextWithTransaction(ctx context.Context, transaction pgx.Tx) context.Context {
	return context.WithValue(ctx, transactionContextKey{}, transaction)
}

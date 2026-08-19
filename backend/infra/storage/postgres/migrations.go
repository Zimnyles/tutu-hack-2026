package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockKey int64 = 8_713_402_596_114_233

//go:embed migrations/*.sql
var migrationFiles embed.FS //nolint:gochecknoglobals // go:embed requires a package variable.

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("list embedded migrations: %w", err)
	}

	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		if err := applyMigration(ctx, pool, entry.Name()); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, version string) error {
	body, err := migrationFiles.ReadFile("migrations/" + version)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", version, err)
	}

	transaction, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if _, err := transaction.Exec(
		ctx,
		`SELECT pg_advisory_xact_lock($1)`,
		migrationLockKey,
	); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	var alreadyApplied bool
	if err := transaction.QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
		version,
	).Scan(&alreadyApplied); err != nil {
		return fmt.Errorf("check migration %s: %w", version, err)
	}

	if alreadyApplied {
		return nil
	}

	if _, err := transaction.Exec(ctx, string(body)); err != nil {
		return fmt.Errorf("execute migration %s: %w", version, err)
	}

	if _, err := transaction.Exec(
		ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`,
		version,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}

	return nil
}

package bootstrap_storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/internal/security"
)

type Repository struct {
	database *pgxpool.Pool
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

func (r *Repository) EnsureDemoAccount(
	ctx context.Context,
	email string,
	password string,
	displayName string,
	role string,
) error {
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	if _, err := r.database.Exec(ctx, `
		INSERT INTO users (email, password_hash, display_name, home_city_id, role, is_demo)
		SELECT $1, $2, $3, id, $4, TRUE
		FROM territories
		WHERE slug = 'yekaterinburg'
		ON CONFLICT (email) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			role = EXCLUDED.role,
			is_demo = TRUE
	`, email, passwordHash, displayName, role); err != nil {
		return fmt.Errorf("ensure demo account: %w", err)
	}

	if _, err := r.database.Exec(ctx, `
		INSERT INTO user_preferences (
			user_id,
			themes,
			transport_modes,
			max_travel_minutes,
			typical_budget,
			trip_duration_days,
			spontaneity
		)
		SELECT
			id,
			ARRAY['architecture','food','history'],
			ARRAY['railway','bus'],
			480,
			30000,
			2,
			4
		FROM users
		WHERE email = $1
		ON CONFLICT (user_id) DO NOTHING
	`, email); err != nil {
		return fmt.Errorf("ensure demo preferences: %w", err)
	}

	return nil
}

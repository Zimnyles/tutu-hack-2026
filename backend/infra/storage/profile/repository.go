package profile_storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository struct {
	database *pgxpool.Pool
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

func (r *Repository) CityExists(ctx context.Context, cityID string) (bool, error) {
	executor := postgres.ExecutorFromContext(ctx, r.database)

	var exists bool
	if err := executor.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM territories WHERE id = NULLIF($1, '')::uuid AND active
		)
	`, cityID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check home city: %w", err)
	}

	return exists, nil
}

func (r *Repository) SaveUser(ctx context.Context, user domain.User) error {
	executor := postgres.ExecutorFromContext(ctx, r.database)

	if _, err := executor.Exec(ctx, `
		UPDATE users
		SET
			home_city_id = NULLIF($2, '')::uuid,
			onboarding_completed_at = CASE
				WHEN $3 THEN COALESCE(onboarding_completed_at, now())
				ELSE NULL
			END,
			travel_visibility = $4,
			updated_at = now()
		WHERE id = $1
	`, user.ID, user.HomeCityID, user.OnboardingCompleted, user.TravelVisibility); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	preferences := user.Preferences
	if _, err := executor.Exec(ctx, `
		INSERT INTO user_preferences (
			user_id,
			themes,
			transport_modes,
			max_travel_minutes,
			typical_budget,
			trip_duration_days,
			spontaneity,
			avoid
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			themes = EXCLUDED.themes,
			transport_modes = EXCLUDED.transport_modes,
			max_travel_minutes = EXCLUDED.max_travel_minutes,
			typical_budget = EXCLUDED.typical_budget,
			trip_duration_days = EXCLUDED.trip_duration_days,
			spontaneity = EXCLUDED.spontaneity,
			avoid = EXCLUDED.avoid,
			updated_at = now()
	`,
		user.ID,
		preferences.Themes,
		preferences.TransportModes,
		preferences.MaxTravelMinutes,
		preferences.TypicalBudget,
		preferences.TripDurationDays,
		preferences.Spontaneity,
		preferences.Avoid,
	); err != nil {
		return fmt.Errorf("update user preferences: %w", err)
	}

	return nil
}

package rewards_storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository struct {
	database *pgxpool.Pool
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

func (r *Repository) Balance(ctx context.Context, userID string) (int, error) {
	var balance int
	if err := r.database.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::int
		FROM reward_ledger
		WHERE user_id = $1
	`, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("query reward balance: %w", err)
	}

	return balance, nil
}

func (r *Repository) Ledger(
	ctx context.Context,
	userID string,
) ([]domain.LedgerEntry, error) {
	rows, err := r.database.Query(ctx, `
		SELECT id, amount, reason_code, reference_type, reference_id, created_at
		FROM reward_ledger
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query reward ledger: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.LedgerEntry, 0)

	for rows.Next() {
		var entry domain.LedgerEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.Amount,
			&entry.ReasonCode,
			&entry.ReferenceType,
			&entry.ReferenceID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reward ledger entry: %w", err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reward ledger: %w", err)
	}

	return entries, nil
}

func (r *Repository) Achievements(
	ctx context.Context,
	userID string,
) ([]domain.Achievement, error) {
	rows, err := r.database.Query(ctx, achievementProgressQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query achievements: %w", err)
	}
	defer rows.Close()

	achievements := make([]domain.Achievement, 0)

	for rows.Next() {
		var achievement domain.Achievement
		if err := rows.Scan(
			&achievement.ID,
			&achievement.Title,
			&achievement.Description,
			&achievement.Icon,
			&achievement.Target,
			&achievement.Progress,
			&achievement.Unlocked,
		); err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}

		achievements = append(achievements, achievement)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate achievements: %w", err)
	}

	return achievements, nil
}

func (r *Repository) PromoCodes(
	ctx context.Context,
	userID string,
) ([]domain.PromoCode, error) {
	rows, err := r.database.Query(ctx, `
		SELECT
			promo.id,
			promo.code,
			territory.id,
			territory.name,
			promo.discount_percent,
			CASE
				WHEN promo.status = 'active' AND promo.expires_at <= now() THEN 'expired'
				ELSE promo.status
			END,
			promo.reason_code,
			promo.issued_at,
			promo.expires_at
		FROM user_promo_codes promo
		JOIN territories territory ON territory.id = promo.territory_id
		WHERE promo.user_id = $1
		ORDER BY promo.issued_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query promo codes: %w", err)
	}
	defer rows.Close()

	codes := make([]domain.PromoCode, 0)

	for rows.Next() {
		var code domain.PromoCode
		if err := rows.Scan(
			&code.ID,
			&code.Code,
			&code.CityID,
			&code.CityName,
			&code.DiscountPercent,
			&code.Status,
			&code.ReasonCode,
			&code.IssuedAt,
			&code.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan promo code: %w", err)
		}

		codes = append(codes, code)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promo codes: %w", err)
	}

	return codes, nil
}

const achievementProgressQuery = `
	WITH user_metrics AS (
		SELECT
			(SELECT COUNT(DISTINCT territory_id) FROM user_visits WHERE user_id = $1) AS unique_cities,
			(SELECT COUNT(DISTINCT territory.region)
			 FROM user_visits visit
			 JOIN territories territory ON territory.id = visit.territory_id
			 WHERE visit.user_id = $1) AS unique_regions,
			(SELECT COALESCE(MAX(level), 0) FROM user_visits WHERE user_id = $1) AS maximum_level,
			(SELECT COUNT(*)
			 FROM trips trip
			 JOIN recommendation_options option ON option.id = trip.recommendation_option_id
			 WHERE trip.user_id = $1 AND trip.status = 'arrived'
			   AND option.transport_mode IN ('railway', 'bus')) AS eco_trips,
			(SELECT COUNT(*) FROM trips WHERE user_id = $1 AND event_id IS NOT NULL) AS event_trips,
			(SELECT COUNT(*) FROM guild_memberships WHERE user_id = $1 AND left_at IS NULL) AS guild_join,
			(SELECT COALESCE(SUM(points), 0) FROM season_score_ledger WHERE user_id = $1) AS season_points
	), progress AS (
		SELECT
			achievement.*,
			CASE achievement.condition ->> 'metric'
				WHEN 'unique_cities' THEN metrics.unique_cities
				WHEN 'unique_regions' THEN metrics.unique_regions
				WHEN 'territory_level' THEN metrics.maximum_level
				WHEN 'eco_trips' THEN metrics.eco_trips
				WHEN 'ground_cities' THEN metrics.eco_trips
				WHEN 'event_trips' THEN metrics.event_trips
				WHEN 'guild_join' THEN metrics.guild_join
				WHEN 'guild_points' THEN metrics.season_points
				WHEN 'season_points' THEN metrics.season_points
				WHEN 'trips_by_mode' THEN (
					SELECT COUNT(*)
					FROM trips trip
					JOIN recommendation_options option ON option.id = trip.recommendation_option_id
					WHERE trip.user_id = $1
					  AND trip.status = 'arrived'
					  AND option.transport_mode = achievement.condition ->> 'mode'
				)
				WHEN 'tag' THEN (
					SELECT COUNT(DISTINCT visit.territory_id)
					FROM user_visits visit
					JOIN territories territory ON territory.id = visit.territory_id
					WHERE visit.user_id = $1
					  AND territory.tags @> ARRAY[achievement.condition ->> 'tag']
				)
				ELSE 0
			END::int AS current_progress
		FROM achievements achievement
		CROSS JOIN user_metrics metrics
		WHERE achievement.active
	)
	SELECT
		progress.id,
		progress.title,
		progress.description,
		progress.icon,
		progress.target,
		LEAST(progress.current_progress, progress.target),
		user_achievement.user_id IS NOT NULL OR progress.current_progress >= progress.target
	FROM progress
	LEFT JOIN user_achievements user_achievement
		ON user_achievement.user_id = $1
		AND user_achievement.achievement_id = progress.id
	ORDER BY progress.sort_order, progress.title`

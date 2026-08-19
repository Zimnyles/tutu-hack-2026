package world_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	ai_errors "github.com/tutu-hack/openworld/internal/errors/ai"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository struct {
	database *pgxpool.Pool
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

type rowScanner interface {
	Scan(...any) error
}

func (r *Repository) Settings(ctx context.Context) (domain.PublicSettings, error) {
	settings := domain.PublicSettings{}

	var onboardingJSON []byte

	var stagesJSON []byte

	var thresholdJSON []byte

	const query = `
		SELECT
			(SELECT value FROM app_settings WHERE key = 'onboarding'),
			(SELECT value FROM app_settings WHERE key = 'recommendation_stages'),
			(SELECT value FROM app_settings WHERE key = 'privacy_threshold')`

	if err := r.database.QueryRow(ctx, query).Scan(
		&onboardingJSON,
		&stagesJSON,
		&thresholdJSON,
	); err != nil {
		return domain.PublicSettings{}, fmt.Errorf("query public settings: %w", err)
	}

	if err := json.Unmarshal(onboardingJSON, &settings.Onboarding); err != nil {
		return domain.PublicSettings{}, fmt.Errorf("decode onboarding settings: %w", err)
	}

	if err := json.Unmarshal(stagesJSON, &settings.RecommendationStages); err != nil {
		return domain.PublicSettings{}, fmt.Errorf("decode recommendation stages: %w", err)
	}

	if err := json.Unmarshal(thresholdJSON, &settings.PrivacyThreshold); err != nil {
		return domain.PublicSettings{}, fmt.Errorf("decode privacy threshold: %w", err)
	}

	homeCities, err := r.homeCities(ctx)
	if err != nil {
		return domain.PublicSettings{}, err
	}

	settings.HomeCities = homeCities

	return settings, nil
}

func (r *Repository) TerritoriesFor(
	ctx context.Context,
	userID string,
) ([]domain.Territory, error) {
	const query = `
		SELECT
			territory.id,
			territory.name,
			territory.region,
			ST_Y(territory.centroid::geometry),
			ST_X(territory.centroid::geometry),
			CASE
				WHEN visit.id IS NOT NULL THEN 'arrived'
				WHEN planned.city_id IS NOT NULL THEN 'planned'
				WHEN territory.tags && preferences.themes THEN 'suggested'
				ELSE 'locked'
			END,
			COALESCE(visit.level, 0),
			territory.tags,
			territory.rarity,
			territory.reward,
			territory.promo_percent,
			territory.description,
			territory.image_tone,
			COALESCE(upcoming.total, 0),
			COALESCE(to_char(upcoming.next_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
			COALESCE(upcoming.popular, FALSE),
			territory.seasonal_fit,
			territory.commercial_priority
		FROM territories territory
		JOIN user_preferences preferences ON preferences.user_id = $1
		LEFT JOIN LATERAL (
			SELECT id, level
			FROM user_visits
			WHERE user_id = $1 AND territory_id = territory.id
			ORDER BY level DESC
			LIMIT 1
		) visit ON TRUE
		LEFT JOIN LATERAL (
			SELECT option.city_id
			FROM trips trip
			JOIN recommendation_options option ON option.id = trip.recommendation_option_id
			WHERE trip.user_id = $1
			  AND option.city_id = territory.id
			  AND trip.status IN ('planned', 'checkout_created', 'departed')
			LIMIT 1
		) planned ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				count(*) AS total,
				min(event.starts_at) AS next_at,
				bool_or(event.popularity_rank IS NOT NULL) AS popular
			FROM events event
			WHERE event.city_id = territory.id
			  AND event.status = 'active'
			  AND event.trust_status IN ('verified', 'ai_web_search')
			  AND event.availability IN ('available', 'limited')
			  AND event.starts_at >= now()
			  AND event.starts_at < now() + interval '30 days'
			  AND event.expires_at > now()
		) upcoming ON TRUE
		WHERE territory.active
		ORDER BY territory.name`

	rows, err := r.database.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query territories: %w", err)
	}
	defer rows.Close()

	territories := make([]domain.Territory, 0)

	for rows.Next() {
		territory, err := scanTerritory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan territory: %w", err)
		}

		territories = append(territories, territory)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate territories: %w", err)
	}

	return territories, nil
}

func (r *Repository) Territory(
	ctx context.Context,
	userID string,
	territoryID string,
) (domain.Territory, bool, error) {
	const query = `
		SELECT
			territory.id,
			territory.name,
			territory.region,
			ST_Y(territory.centroid::geometry),
			ST_X(territory.centroid::geometry),
			CASE
				WHEN visit.id IS NOT NULL THEN 'arrived'
				WHEN territory.tags && preferences.themes THEN 'suggested'
				ELSE 'locked'
			END,
			COALESCE(visit.level, 0),
			territory.tags,
			territory.rarity,
			territory.reward,
			territory.promo_percent,
			territory.description,
			territory.image_tone,
			COALESCE(upcoming.total, 0),
			COALESCE(to_char(upcoming.next_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
			COALESCE(upcoming.popular, FALSE),
			territory.seasonal_fit,
			territory.commercial_priority
		FROM territories territory
		JOIN user_preferences preferences ON preferences.user_id = $1
		LEFT JOIN LATERAL (
			SELECT id, level
			FROM user_visits
			WHERE user_id = $1 AND territory_id = territory.id
			ORDER BY level DESC
			LIMIT 1
		) visit ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				count(*) AS total,
				min(event.starts_at) AS next_at,
				bool_or(event.popularity_rank IS NOT NULL) AS popular
			FROM events event
			WHERE event.city_id = territory.id
			  AND event.status = 'active'
			  AND event.trust_status IN ('verified', 'ai_web_search')
			  AND event.availability IN ('available', 'limited')
			  AND event.starts_at >= now()
			  AND event.starts_at < now() + interval '30 days'
			  AND event.expires_at > now()
		) upcoming ON TRUE
		WHERE territory.id = $2 AND territory.active`

	territory, err := scanTerritory(r.database.QueryRow(ctx, query, userID, territoryID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Territory{}, false, nil
	}

	if err != nil {
		return domain.Territory{}, false, fmt.Errorf("query territory: %w", err)
	}

	return territory, true, nil
}

func (r *Repository) DemoSync(
	ctx context.Context,
	userID string,
) ([]domain.Territory, error) {
	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin demo sync: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	const insertVisits = `
		INSERT INTO user_visits (
			user_id,
			territory_id,
			source,
			level,
			visited_at,
			evidence_ref
		)
		SELECT $1, home_city_id, 'demo_sync', 1, now(), 'DEMO-HOME'
		FROM users
		WHERE id = $1 AND home_city_id IS NOT NULL
		UNION ALL
		SELECT $1, destination_city_id, 'demo_sync', 1, arrived_at, external_order_ref
		FROM demo_travel_history
		WHERE demo_profile = 'default'
		ON CONFLICT (user_id, territory_id, source) DO NOTHING`

	if _, err := transaction.Exec(ctx, insertVisits, userID); err != nil {
		return nil, fmt.Errorf("insert demo visits: %w", err)
	}

	const insertReward = `
		INSERT INTO reward_ledger (
			user_id,
			amount,
			reason_code,
			reference_type,
			reference_id,
			idempotency_key
		)
		VALUES ($1, 1280, 'DEMO_HISTORY_SYNC', 'user', $1, 'demo-sync:' || $1)
		ON CONFLICT DO NOTHING`

	if _, err := transaction.Exec(ctx, insertReward, userID); err != nil {
		return nil, fmt.Errorf("insert demo sync reward: %w", err)
	}

	const insertPromoCodes = `
		INSERT INTO user_promo_codes (
			user_id,
			territory_id,
			code,
			discount_percent,
			idempotency_key
		)
		SELECT
			$1,
			territory.id,
			'TUTU' || territory.promo_percent || '-' || upper(substr(md5($1::text || territory.id::text), 1, 6)),
			territory.promo_percent,
			'visit:' || $1 || ':' || territory.id
		FROM user_visits visit
		JOIN territories territory ON territory.id = visit.territory_id
		WHERE visit.user_id = $1 AND territory.promo_percent > 0
		ON CONFLICT DO NOTHING`

	if _, err := transaction.Exec(ctx, insertPromoCodes, userID); err != nil {
		return nil, fmt.Errorf("insert demo sync promo codes: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit demo sync: %w", err)
	}

	allTerritories, err := r.TerritoriesFor(ctx, userID)
	if err != nil {
		return nil, err
	}

	opened := make([]domain.Territory, 0)

	for _, territory := range allTerritories {
		if territory.State == "arrived" {
			opened = append(opened, territory)
		}
	}

	return opened, nil
}

func (r *Repository) RecommendationStages(
	ctx context.Context,
) ([]domain.WorkflowStage, error) {
	var value []byte
	if err := r.database.QueryRow(
		ctx,
		`SELECT value FROM app_settings WHERE key = 'recommendation_stages'`,
	).Scan(&value); err != nil {
		return nil, fmt.Errorf("query recommendation stages: %w", err)
	}

	var stages []domain.WorkflowStage
	if err := json.Unmarshal(value, &stages); err != nil {
		return nil, fmt.Errorf("decode recommendation stages: %w", err)
	}

	return stages, nil
}

func (r *Repository) RecommendationSettings(
	ctx context.Context,
) (domain.RecommendationSettings, error) {
	var value []byte
	if err := r.database.QueryRow(
		ctx,
		`SELECT value FROM app_settings WHERE key = 'ai_recommendation'`,
	).Scan(&value); err != nil {
		return domain.RecommendationSettings{}, fmt.Errorf("query recommendation settings: %w", err)
	}

	var settings domain.RecommendationSettings
	if err := json.Unmarshal(value, &settings); err != nil {
		return domain.RecommendationSettings{}, fmt.Errorf("decode recommendation settings: %w", err)
	}

	return settings, nil
}

func (r *Repository) ScoringWeights(ctx context.Context) (domain.ScoringWeights, error) {
	var value []byte
	if err := r.database.QueryRow(
		ctx,
		`SELECT value FROM app_settings WHERE key = 'scoring_weights'`,
	).Scan(&value); err != nil {
		return domain.ScoringWeights{}, fmt.Errorf("query scoring weights: %w", err)
	}

	var weights domain.ScoringWeights
	if err := json.Unmarshal(value, &weights); err != nil {
		return domain.ScoringWeights{}, fmt.Errorf("decode scoring weights: %w", err)
	}

	return weights, nil
}

func (r *Repository) PersonalRecommendationSettings(
	ctx context.Context,
) (domain.PersonalRecommendationSettings, error) {
	var value []byte
	if err := r.database.QueryRow(
		ctx,
		`SELECT value FROM app_settings WHERE key = 'personal_recommendation'`,
	).Scan(&value); err != nil {
		return domain.PersonalRecommendationSettings{}, fmt.Errorf("query personal recommendation settings: %w", err)
	}

	var settings domain.PersonalRecommendationSettings
	if err := json.Unmarshal(value, &settings); err != nil {
		return domain.PersonalRecommendationSettings{}, fmt.Errorf("decode personal recommendation settings: %w", err)
	}

	return settings, nil
}

func (r *Repository) AISystemPrompts(ctx context.Context) (domain.AISystemPrompts, error) {
	rows, err := r.database.Query(ctx, `
		SELECT code, content
		FROM ai_system_prompts
		WHERE active AND code = ANY($1::text[])
	`, []string{
		"request_analysis",
		"travel_search_plan",
		"recommendation_explanation",
		"event_enrichment",
	})
	if err != nil {
		return domain.AISystemPrompts{}, fmt.Errorf("query AI system prompts: %w", err)
	}
	defer rows.Close()

	var prompts domain.AISystemPrompts

	for rows.Next() {
		var code string

		var content string
		if err := rows.Scan(&code, &content); err != nil {
			return domain.AISystemPrompts{}, fmt.Errorf("scan AI system prompt: %w", err)
		}

		switch code {
		case "request_analysis":
			prompts.RequestAnalysis = content
		case "travel_search_plan":
			prompts.TravelSearchPlan = content
		case "recommendation_explanation":
			prompts.RecommendationExplanation = content
		case "event_enrichment":
			prompts.EventEnrichment = content
		}
	}

	if err := rows.Err(); err != nil {
		return domain.AISystemPrompts{}, fmt.Errorf("iterate AI system prompts: %w", err)
	}

	if prompts.RequestAnalysis == "" || prompts.TravelSearchPlan == "" ||
		prompts.RecommendationExplanation == "" || prompts.EventEnrichment == "" {
		return domain.AISystemPrompts{}, ai_errors.ErrPromptConfiguration
	}

	return prompts, nil
}

func (r *Repository) homeCities(
	ctx context.Context,
) ([]domain.TerritoryReference, error) {
	rows, err := r.database.Query(
		ctx,
		`SELECT id, name, region FROM territories WHERE active ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query home cities: %w", err)
	}
	defer rows.Close()

	cities := make([]domain.TerritoryReference, 0)

	for rows.Next() {
		var city domain.TerritoryReference
		if err := rows.Scan(&city.ID, &city.Name, &city.Region); err != nil {
			return nil, fmt.Errorf("scan home city: %w", err)
		}

		cities = append(cities, city)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate home cities: %w", err)
	}

	return cities, nil
}

func scanTerritory(row rowScanner) (domain.Territory, error) {
	var territory domain.Territory
	err := row.Scan(
		&territory.ID,
		&territory.Name,
		&territory.Region,
		&territory.Latitude,
		&territory.Longitude,
		&territory.State,
		&territory.Level,
		&territory.Tags,
		&territory.Rarity,
		&territory.Reward,
		&territory.PromoPercent,
		&territory.Description,
		&territory.ImageTone,
		&territory.UpcomingEvents,
		&territory.NextEventAt,
		&territory.PopularEvent,
		&territory.SeasonalFit,
		&territory.CommercialPriority,
	)

	return territory, err
}

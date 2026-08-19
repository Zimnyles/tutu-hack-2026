package recommendations_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	recommendation_errors "github.com/tutu-hack/openworld/internal/errors/recommendation"
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

type storedOffer struct {
	Offer       json.RawMessage `json:"offer"`
	CheckoutRef json.RawMessage `json:"checkout_ref"`
}

const (
	maximumCandidateCount = 20
	maximumOptionCount    = 3
)

func (r *Repository) Create(
	ctx context.Context,
	recommendation domain.RecommendationRequest,
	promptHash string,
	requestID string,
) error {
	const query = `
		INSERT INTO recommendation_requests (
			id,
			user_id,
			origin_city_id,
			destination_city_id,
			event_id,
			date_from,
			date_to,
			adults,
			children,
			budget,
			currency,
			transport_modes,
			max_travel_minutes,
			direct_only,
			prompt_hash,
			request_kind,
			status,
			stage_code,
			request_id,
			created_at
		)
		VALUES (
			$1,
			$2,
			$3,
			NULLIF($4, '')::uuid,
			NULLIF($5, '')::uuid,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15,
			$16,
			$17,
			$18,
			NULLIF($19, '')::uuid,
			$20
		)`

	_, err := r.database.Exec(
		ctx,
		query,
		recommendation.ID,
		recommendation.UserID,
		recommendation.OriginCityID,
		recommendation.DestinationID,
		recommendation.EventID,
		recommendation.DateFrom,
		recommendation.DateTo,
		recommendation.Adults,
		recommendation.Children,
		recommendation.Budget,
		recommendation.Currency,
		recommendation.TransportModes,
		recommendation.MaxTravelMinutes,
		recommendation.DirectOnly,
		promptHash,
		recommendation.Kind,
		recommendation.Status,
		recommendation.Stage,
		requestID,
		recommendation.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert recommendation request: %w", err)
	}

	return nil
}

func (r *Repository) Get(
	ctx context.Context,
	recommendationID string,
	userID string,
) (domain.RecommendationRequest, bool, error) {
	const query = `
		SELECT
			id,
			user_id,
			request_kind,
			status,
			stage_code,
			origin_city_id,
			COALESCE(destination_city_id::text, ''),
			COALESCE(event_id::text, ''),
			date_from::text,
			date_to::text,
			adults,
			children,
			budget,
			currency,
			transport_modes,
			max_travel_minutes,
			direct_only,
			created_at,
			completed_at,
			is_demo_fallback,
			COALESCE(guardrail_reason, '')
		FROM recommendation_requests
		WHERE id = $1 AND user_id = $2`

	var recommendation domain.RecommendationRequest

	err := r.database.QueryRow(ctx, query, recommendationID, userID).Scan(
		&recommendation.ID,
		&recommendation.UserID,
		&recommendation.Kind,
		&recommendation.Status,
		&recommendation.Stage,
		&recommendation.OriginCityID,
		&recommendation.DestinationID,
		&recommendation.EventID,
		&recommendation.DateFrom,
		&recommendation.DateTo,
		&recommendation.Adults,
		&recommendation.Children,
		&recommendation.Budget,
		&recommendation.Currency,
		&recommendation.TransportModes,
		&recommendation.MaxTravelMinutes,
		&recommendation.DirectOnly,
		&recommendation.CreatedAt,
		&recommendation.CompletedAt,
		&recommendation.DemoFallback,
		&recommendation.FailureCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecommendationRequest{}, false, nil
	}

	if err != nil {
		return domain.RecommendationRequest{}, false, fmt.Errorf("query recommendation request: %w", err)
	}

	options, err := r.options(ctx, recommendation.ID)
	if err != nil {
		return domain.RecommendationRequest{}, false, err
	}

	recommendation.Options = options

	return recommendation, true, nil
}

func (r *Repository) Latest(
	ctx context.Context,
	userID string,
	kind string,
) (domain.RecommendationRequest, bool, error) {
	var recommendationID string

	err := r.database.QueryRow(ctx, `
		SELECT id
		FROM recommendation_requests
		WHERE user_id = $1 AND request_kind = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, kind).Scan(&recommendationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecommendationRequest{}, false, nil
	}

	if err != nil {
		return domain.RecommendationRequest{}, false, fmt.Errorf("query latest recommendation: %w", err)
	}

	return r.Get(ctx, recommendationID, userID)
}

func (r *Repository) SetStage(
	ctx context.Context,
	recommendationID string,
	stage string,
) error {
	const query = `
		UPDATE recommendation_requests
		SET stage_code = $2
		WHERE id = $1 AND status = 'processing'`

	commandTag, err := r.database.Exec(ctx, query, recommendationID, stage)
	if err != nil {
		return fmt.Errorf("update recommendation stage: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return common_errors.ErrInvalidState
	}

	return nil
}

func (r *Repository) Candidates(
	ctx context.Context,
	user domain.User,
	recommendation domain.RecommendationRequest,
) ([]domain.Territory, error) {
	const query = `
		SELECT
			territory.id,
			territory.name,
			territory.region,
			ST_Y(territory.centroid::geometry),
			ST_X(territory.centroid::geometry),
			'suggested',
			0,
			territory.tags,
			territory.badges,
			territory.rarity,
			territory.reward,
			territory.description,
			territory.image_tone,
			territory.seasonal_fit,
			territory.commercial_priority
		FROM territories territory
		WHERE territory.active
		  AND ($2 = '' OR territory.id = NULLIF($2, '')::uuid)
		  AND (
			$2 <> ''
			OR (
				territory.id IS DISTINCT FROM NULLIF($5, '')::uuid
				AND NOT EXISTS (
					SELECT 1
					FROM user_visits visit
					WHERE visit.user_id = $1 AND visit.territory_id = territory.id
				)
			)
		  )
		ORDER BY
			cardinality(ARRAY(
				SELECT unnest(territory.badges)
				INTERSECT
				SELECT unnest($4::text[])
			)) DESC,
			cardinality(ARRAY(
				SELECT unnest(territory.tags)
				INTERSECT
				SELECT unnest($3::text[])
			)) DESC,
			territory.rarity DESC,
			territory.name
		LIMIT 20`

	rows, err := r.database.Query(
		ctx,
		query,
		user.ID,
		recommendation.DestinationID,
		user.Preferences.Themes,
		domain.BadgesForThemes(user.Preferences.Themes),
		recommendation.OriginCityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query recommendation candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]domain.Territory, 0, maximumCandidateCount)

	for rows.Next() {
		candidate, err := scanTerritory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recommendation candidate: %w", err)
		}

		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation candidates: %w", err)
	}

	return candidates, nil
}

func (r *Repository) Complete(
	ctx context.Context,
	recommendationID string,
	options []domain.RecommendationOption,
	status string,
) error {
	if status != "completed" && status != "partial" {
		return fmt.Errorf("%w: %s", recommendation_errors.ErrUnsupportedCompletionStatus, status)
	}

	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recommendation completion: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	const insertOption = `
		INSERT INTO recommendation_options (
			id, request_id, city_id, event_id, rank, score, reason, why_now,
			price_amount, currency, duration_minutes, transport_mode,
			territory_gain_km2, reward, special_offer, offer_snapshot, expires_at
		)
		VALUES (
			$1, $2, $3, NULLIF($4, '')::uuid, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (id) DO NOTHING`

	for _, option := range options {
		validUntil, err := time.Parse(time.RFC3339, option.ValidUntil)
		if err != nil {
			return fmt.Errorf("parse recommendation option expiration: %w", err)
		}

		offerSnapshot, err := json.Marshal(storedOffer{
			Offer:       option.OfferSnapshot,
			CheckoutRef: option.CheckoutRef,
		})
		if err != nil {
			return fmt.Errorf("encode recommendation offer snapshot: %w", err)
		}

		if _, err := transaction.Exec(
			ctx,
			insertOption,
			option.ID,
			recommendationID,
			option.CityID,
			option.EventID,
			option.Rank,
			option.Score,
			option.Reason,
			option.WhyNow,
			option.Price,
			option.Currency,
			option.DurationMinutes,
			option.Transport,
			option.TerritoryGain,
			option.Reward,
			option.Special,
			string(offerSnapshot),
			validUntil,
		); err != nil {
			return fmt.Errorf("insert recommendation option: %w", err)
		}
	}

	const updateRequest = `
		UPDATE recommendation_requests
		SET
			status = CASE WHEN $2 = 0 THEN 'failed' ELSE $3 END,
			is_demo_fallback = FALSE,
			completed_at = now()
		WHERE id = $1`

	if _, err := transaction.Exec(
		ctx,
		updateRequest,
		recommendationID,
		len(options),
		status,
	); err != nil {
		return fmt.Errorf("complete recommendation request: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit recommendation completion: %w", err)
	}

	return nil
}

func (r *Repository) Fail(
	ctx context.Context,
	recommendationID string,
	reason string,
) error {
	const query = `
		UPDATE recommendation_requests
		SET status = 'failed', guardrail_reason = $2, completed_at = now()
		WHERE id = $1`

	if _, err := r.database.Exec(ctx, query, recommendationID, reason); err != nil {
		return fmt.Errorf("fail recommendation request: %w", err)
	}

	return nil
}

func (r *Repository) Block(
	ctx context.Context,
	recommendationID string,
	reason string,
) error {
	const query = `
		UPDATE recommendation_requests
		SET status = 'blocked', guardrail_reason = $2, completed_at = now()
		WHERE id = $1 AND status = 'processing'`

	commandTag, err := r.database.Exec(ctx, query, recommendationID, reason)
	if err != nil {
		return fmt.Errorf("block recommendation request: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return common_errors.ErrInvalidState
	}

	return nil
}

func (r *Repository) options(
	ctx context.Context,
	recommendationID string,
) ([]domain.RecommendationOption, error) {
	const query = `
		SELECT
			option.id,
			option.city_id,
			territory.name,
			territory.region,
			option.rank,
			option.score,
			option.reason,
			option.why_now,
			option.price_amount,
			option.currency,
			option.duration_minutes,
			option.transport_mode,
			option.territory_gain_km2,
			option.reward,
			option.special_offer,
			option.expires_at,
			COALESCE(option.event_id::text, ''),
			option.offer_snapshot
		FROM recommendation_options option
		JOIN territories territory ON territory.id = option.city_id
		WHERE option.request_id = $1
		ORDER BY option.rank`

	rows, err := r.database.Query(ctx, query, recommendationID)
	if err != nil {
		return nil, fmt.Errorf("query recommendation options: %w", err)
	}
	defer rows.Close()

	options := make([]domain.RecommendationOption, 0, maximumOptionCount)

	for rows.Next() {
		var option domain.RecommendationOption

		var expiresAt time.Time

		var rawStoredOffer []byte

		err := rows.Scan(
			&option.ID,
			&option.CityID,
			&option.CityName,
			&option.Region,
			&option.Rank,
			&option.Score,
			&option.Reason,
			&option.WhyNow,
			&option.Price,
			&option.Currency,
			&option.DurationMinutes,
			&option.Transport,
			&option.TerritoryGain,
			&option.Reward,
			&option.Special,
			&expiresAt,
			&option.EventID,
			&rawStoredOffer,
		)
		if err != nil {
			return nil, fmt.Errorf("scan recommendation option: %w", err)
		}

		var persistedOffer storedOffer
		if err := json.Unmarshal(rawStoredOffer, &persistedOffer); err != nil {
			return nil, fmt.Errorf("decode recommendation offer snapshot: %w", err)
		}

		option.ValidUntil = expiresAt.Format(time.RFC3339)
		option.OfferSnapshot = persistedOffer.Offer
		option.CheckoutRef = persistedOffer.CheckoutRef
		options = append(options, option)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation options: %w", err)
	}

	return options, nil
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
		&territory.Badges,
		&territory.Rarity,
		&territory.Reward,
		&territory.Description,
		&territory.ImageTone,
		&territory.SeasonalFit,
		&territory.CommercialPriority,
	)

	return territory, err
}

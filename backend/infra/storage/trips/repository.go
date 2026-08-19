package trips_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	arrivalAttempts     = 3
	arrivalRetryBackoff = 20 * time.Millisecond
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

func (r *Repository) CreateFromRecommendation(
	ctx context.Context,
	userID string,
	recommendationID string,
	optionID string,
) (domain.Trip, error) {
	const query = `
		INSERT INTO trips (
			user_id,
			recommendation_option_id,
			event_id,
			status,
			depart_at,
			arrive_at,
			is_demo
		)
		SELECT
			request.user_id,
			option.id,
			option.event_id,
			'planned',
			request.date_from::timestamptz + interval '6 hours',
			request.date_from::timestamptz + interval '6 hours' + (option.duration_minutes || ' minutes')::interval,
			request.is_demo_fallback
		FROM recommendation_requests request
		JOIN recommendation_options option ON option.request_id = request.id
		WHERE request.id = $1
		  AND request.user_id = $2
		  AND option.id = $3
		  AND option.expires_at > now()
		RETURNING id`

	var tripID string

	err := r.database.QueryRow(
		ctx,
		query,
		recommendationID,
		userID,
		optionID,
	).Scan(&tripID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Trip{}, common_errors.ErrNotFound
	case postgres.IsUniqueViolation(err):
		return r.byOption(ctx, userID, optionID)
	case err != nil:
		return domain.Trip{}, fmt.Errorf("insert trip: %w", err)
	}

	trip, found, err := r.Get(ctx, tripID, userID)
	if err != nil {
		return domain.Trip{}, err
	}

	if !found {
		return domain.Trip{}, common_errors.ErrNotFound
	}

	return trip, nil
}

func (r *Repository) byOption(
	ctx context.Context,
	userID string,
	optionID string,
) (domain.Trip, error) {
	trip, err := scanTrip(r.database.QueryRow(
		ctx,
		tripSelect+` WHERE trip.user_id = $1 AND trip.recommendation_option_id = $2`,
		userID,
		optionID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Trip{}, common_errors.ErrNotFound
	}

	if err != nil {
		return domain.Trip{}, fmt.Errorf("query trip by option: %w", err)
	}

	return trip, nil
}

func (r *Repository) List(
	ctx context.Context,
	userID string,
) ([]domain.Trip, error) {
	rows, err := r.database.Query(ctx, tripSelect+`
		WHERE trip.user_id = $1
		ORDER BY trip.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query trips: %w", err)
	}
	defer rows.Close()

	trips := make([]domain.Trip, 0)

	for rows.Next() {
		trip, err := scanTrip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan trip: %w", err)
		}

		trips = append(trips, trip)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trips: %w", err)
	}

	return trips, nil
}

func (r *Repository) Get(
	ctx context.Context,
	tripID string,
	userID string,
) (domain.Trip, bool, error) {
	trip, err := scanTrip(r.database.QueryRow(
		ctx,
		tripSelect+` WHERE trip.id = $1 AND trip.user_id = $2`,
		tripID,
		userID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Trip{}, false, nil
	}

	if err != nil {
		return domain.Trip{}, false, fmt.Errorf("query trip: %w", err)
	}

	return trip, true, nil
}

func (r *Repository) SaveCheckout(
	ctx context.Context,
	tripID string,
	userID string,
	checkoutURL string,
) (domain.Trip, error) {
	const query = `
		UPDATE trips
		SET status = 'checkout_created', checkout_url = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status IN ('planned', 'checkout_created')`

	commandTag, err := r.database.Exec(ctx, query, tripID, userID, checkoutURL)
	if err != nil {
		return domain.Trip{}, fmt.Errorf("update trip checkout: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return domain.Trip{}, common_errors.ErrInvalidState
	}

	trip, found, err := r.Get(ctx, tripID, userID)
	if err != nil {
		return domain.Trip{}, err
	}

	if !found {
		return domain.Trip{}, common_errors.ErrNotFound
	}

	return trip, nil
}

func (r *Repository) Arrive(
	ctx context.Context,
	tripID string,
	userID string,
	idempotencyKey string,
) (domain.Trip, domain.LedgerEntry, bool, error) {
	var (
		trip      domain.Trip
		reward    domain.LedgerEntry
		replayed  bool
		lastError error
	)

	for attempt := 1; attempt <= arrivalAttempts; attempt++ {
		trip, reward, replayed, lastError = r.arriveOnce(ctx, tripID, userID, idempotencyKey)
		if lastError == nil || !postgres.IsSerializationFailure(lastError) {
			return trip, reward, replayed, lastError
		}

		select {
		case <-ctx.Done():
			return domain.Trip{}, domain.LedgerEntry{}, false, ctx.Err()
		case <-time.After(arrivalRetryBackoff * time.Duration(attempt)):
		}
	}

	return trip, reward, replayed, lastError
}

func (r *Repository) arriveOnce(
	ctx context.Context,
	tripID string,
	userID string,
	idempotencyKey string,
) (domain.Trip, domain.LedgerEntry, bool, error) {
	transaction, err := r.database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("begin arrival transaction: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	const lockTrip = `
		SELECT trip.status, option.city_id, option.reward, option.transport_mode
		FROM trips trip
		JOIN recommendation_options option ON option.id = trip.recommendation_option_id
		WHERE trip.id = $1 AND trip.user_id = $2
		FOR UPDATE`

	var status string

	var cityID string

	var rewardAmount int

	var transportMode string

	err = transaction.QueryRow(ctx, lockTrip, tripID, userID).Scan(
		&status,
		&cityID,
		&rewardAmount,
		&transportMode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Trip{}, domain.LedgerEntry{}, false, common_errors.ErrNotFound
	}

	if err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("lock arrival trip: %w", err)
	}

	if status == "arrived" {
		reward, err := rewardByTrip(ctx, transaction, tripID)
		if err != nil {
			return domain.Trip{}, domain.LedgerEntry{}, false, err
		}

		if err := transaction.Commit(ctx); err != nil {
			return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("commit repeated arrival: %w", err)
		}

		trip, _, err := r.Get(ctx, tripID, userID)

		return trip, reward, true, err
	}

	if status != "planned" && status != "checkout_created" && status != "departed" {
		return domain.Trip{}, domain.LedgerEntry{}, false, common_errors.ErrInvalidState
	}

	if _, err := transaction.Exec(
		ctx,
		`UPDATE trips SET status = 'arrived', updated_at = now() WHERE id = $1`,
		tripID,
	); err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("mark trip arrived: %w", err)
	}

	if _, err := transaction.Exec(ctx, `
		INSERT INTO user_visits (user_id, territory_id, source, level, visited_at, evidence_ref)
		VALUES ($1, $2, 'demo_sync', 1, now(), $3)
		ON CONFLICT (user_id, territory_id, source) DO UPDATE SET
			level = GREATEST(user_visits.level, 1)`, userID, cityID, tripID); err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("insert arrival visit: %w", err)
	}

	var reward domain.LedgerEntry

	const insertReward = `
		INSERT INTO reward_ledger (
			user_id, amount, reason_code, reference_type, reference_id, idempotency_key
		)
		VALUES ($1, $2, 'FIRST_CITY_VISIT', 'trip', $3, $4)
		ON CONFLICT DO NOTHING
		RETURNING id, amount, reason_code, reference_type, reference_id, created_at`

	err = transaction.QueryRow(
		ctx,
		insertReward,
		userID,
		rewardAmount,
		tripID,
		idempotencyKey,
	).Scan(
		&reward.ID,
		&reward.Amount,
		&reward.ReasonCode,
		&reward.ReferenceType,
		&reward.ReferenceID,
		&reward.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		reward, err = rewardByTrip(ctx, transaction, tripID)
	}

	if err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("insert arrival reward: %w", err)
	}

	if err := insertPromoCode(ctx, transaction, userID, tripID, cityID); err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, err
	}

	if err := insertSeasonScore(ctx, transaction, userID, tripID, rewardAmount); err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, err
	}

	if err := insertArrivalOutbox(ctx, transaction, userID, tripID, cityID, transportMode); err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, err
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, fmt.Errorf("commit arrival: %w", err)
	}

	trip, found, err := r.Get(ctx, tripID, userID)
	if err != nil {
		return domain.Trip{}, domain.LedgerEntry{}, false, err
	}

	if !found {
		return domain.Trip{}, domain.LedgerEntry{}, false, common_errors.ErrNotFound
	}

	return trip, reward, false, nil
}

const tripSelect = `
	SELECT
		trip.id,
		trip.user_id,
		trip.status,
		COALESCE(trip.checkout_url, ''),
		trip.depart_at,
		trip.arrive_at,
		trip.created_at,
		COALESCE(trip.event_id::text, ''),
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
	FROM trips trip
	JOIN recommendation_options option ON option.id = trip.recommendation_option_id
	JOIN territories territory ON territory.id = option.city_id`

func scanTrip(row rowScanner) (domain.Trip, error) {
	var trip domain.Trip

	var optionExpiration time.Time

	var rawStoredOffer []byte

	err := row.Scan(
		&trip.ID,
		&trip.UserID,
		&trip.Status,
		&trip.CheckoutURL,
		&trip.DepartAt,
		&trip.ArriveAt,
		&trip.CreatedAt,
		&trip.EventID,
		&trip.Option.ID,
		&trip.Option.CityID,
		&trip.Option.CityName,
		&trip.Option.Region,
		&trip.Option.Rank,
		&trip.Option.Score,
		&trip.Option.Reason,
		&trip.Option.WhyNow,
		&trip.Option.Price,
		&trip.Option.Currency,
		&trip.Option.DurationMinutes,
		&trip.Option.Transport,
		&trip.Option.TerritoryGain,
		&trip.Option.Reward,
		&trip.Option.Special,
		&optionExpiration,
		&trip.Option.EventID,
		&rawStoredOffer,
	)
	if err != nil {
		return domain.Trip{}, err
	}

	var persistedOffer storedOffer
	if err := json.Unmarshal(rawStoredOffer, &persistedOffer); err != nil {
		return domain.Trip{}, fmt.Errorf("decode trip offer snapshot: %w", err)
	}

	trip.Option.ValidUntil = optionExpiration.Format(time.RFC3339)
	trip.Option.OfferSnapshot = persistedOffer.Offer
	trip.Option.CheckoutRef = persistedOffer.CheckoutRef

	return trip, nil
}

func rewardByTrip(
	ctx context.Context,
	transaction pgx.Tx,
	tripID string,
) (domain.LedgerEntry, error) {
	var reward domain.LedgerEntry

	const query = `
		SELECT id, amount, reason_code, reference_type, reference_id, created_at
		FROM reward_ledger
		WHERE reference_type = 'trip' AND reference_id = $1
		ORDER BY created_at
		LIMIT 1`

	err := transaction.QueryRow(ctx, query, tripID).Scan(
		&reward.ID,
		&reward.Amount,
		&reward.ReasonCode,
		&reward.ReferenceType,
		&reward.ReferenceID,
		&reward.CreatedAt,
	)
	if err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("query trip reward: %w", err)
	}

	return reward, nil
}

func insertPromoCode(
	ctx context.Context,
	transaction pgx.Tx,
	userID string,
	tripID string,
	cityID string,
) error {
	const query = `
		INSERT INTO user_promo_codes (
			user_id,
			territory_id,
			trip_id,
			code,
			discount_percent,
			idempotency_key
		)
		SELECT
			$1,
			territory.id,
			$2,
			'TUTU' || territory.promo_percent || '-' || upper(substr(md5($1::text || territory.id::text), 1, 6)),
			territory.promo_percent,
			'visit:' || $1 || ':' || territory.id
		FROM territories territory
		WHERE territory.id = $3 AND territory.promo_percent > 0
		ON CONFLICT DO NOTHING`

	if _, err := transaction.Exec(ctx, query, userID, tripID, cityID); err != nil {
		return fmt.Errorf("insert arrival promo code: %w", err)
	}

	return nil
}

func insertSeasonScore(
	ctx context.Context,
	transaction pgx.Tx,
	userID string,
	tripID string,
	points int,
) error {
	const query = `
		INSERT INTO season_score_ledger (
			season_id,
			user_id,
			guild_id,
			points,
			reason_code,
			reference_type,
			reference_id,
			idempotency_key
		)
		SELECT
			season.id,
			$1,
			membership.guild_id,
			$3,
			'NEW_CITY',
			'trip',
			$2,
			'season-arrival:' || $2
		FROM seasons season
		LEFT JOIN guild_memberships membership
			ON membership.user_id = $1 AND membership.left_at IS NULL
		WHERE season.status = 'active'
		ORDER BY season.starts_at DESC
		LIMIT 1
		ON CONFLICT (idempotency_key) DO NOTHING`

	if _, err := transaction.Exec(ctx, query, userID, tripID, points); err != nil {
		return fmt.Errorf("insert season score: %w", err)
	}

	return nil
}

func insertArrivalOutbox(
	ctx context.Context,
	transaction pgx.Tx,
	userID string,
	tripID string,
	cityID string,
	transportMode string,
) error {
	const query = `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES (
			'trip',
			$1,
			'TripArrived',
			jsonb_build_object(
				'user_id', $2,
				'city_id', $3,
				'transport_mode', $4
			)
		)
		ON CONFLICT DO NOTHING`

	if _, err := transaction.Exec(
		ctx,
		query,
		tripID,
		userID,
		cityID,
		transportMode,
	); err != nil {
		return fmt.Errorf("insert arrival outbox event: %w", err)
	}

	return nil
}

package events_storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	events_errors "github.com/tutu-hack/openworld/internal/errors/events"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/services/events"
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

func (r *Repository) List(
	ctx context.Context,
	cityID string,
	filters events.Filters,
) ([]domain.Event, error) {
	const query = `
		SELECT
			event.id,
			event.city_id,
			event.external_id,
			event.title,
			event.description_plain,
			event.category,
			event.venue_name,
			event.starts_at,
			event.ends_at,
			COALESCE(event.price_from, 0),
			event.currency,
			COALESCE(event.age_rating, ''),
			event.availability,
			event.status,
			source.name,
			event.trust_status,
			event.source_updated_at,
			event.is_demo
		FROM events event
		JOIN event_sources source ON source.id = event.source_id
		WHERE event.city_id = $1
		  AND event.status = 'active'
		  AND event.ends_at > now()
		  AND event.expires_at > now()
		  AND ($2::timestamptz IS NULL OR event.starts_at >= $2)
		  AND ($3::timestamptz IS NULL OR event.starts_at < $3::timestamptz + interval '1 day')
		  AND ($4 = '' OR event.category = $4)
		  AND ($5 = 0 OR COALESCE(event.price_from, 0) <= $5)
		  AND ($6 = '' OR event.age_rating = $6)
		  AND ($7 = '' OR event.availability = $7)
		  AND (
			NOT event.is_demo
			OR NOT EXISTS (
				SELECT 1
				FROM events live
				WHERE live.city_id = $1
				  AND live.source_id = '20000000-0000-0000-0000-000000000003'
				  AND live.status = 'active'
				  AND live.ends_at > now()
				  AND live.expires_at > now()
			)
		  )
		ORDER BY event.starts_at
		LIMIT 60`

	rows, err := r.database.Query(
		ctx,
		query,
		cityID,
		nullTime(filters.DateFrom),
		nullTime(filters.DateTo),
		filters.Category,
		filters.PriceMax,
		filters.AgeRating,
		filters.Availability,
	)
	if err != nil {
		return nil, fmt.Errorf("query city events: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Event, 0)

	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan city event: %w", err)
		}

		items = append(items, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate city events: %w", err)
	}

	return items, nil
}

func (r *Repository) Get(
	ctx context.Context,
	eventID string,
) (domain.Event, bool, error) {
	const query = `
		SELECT
			event.id,
			event.city_id,
			event.external_id,
			event.title,
			event.description_plain,
			event.category,
			event.venue_name,
			event.starts_at,
			event.ends_at,
			COALESCE(event.price_from, 0),
			event.currency,
			COALESCE(event.age_rating, ''),
			event.availability,
			event.status,
			source.name,
			event.trust_status,
			event.source_updated_at,
			event.is_demo
		FROM events event
		JOIN event_sources source ON source.id = event.source_id
		WHERE event.id = $1`

	event, err := scanEvent(r.database.QueryRow(ctx, query, eventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, false, nil
	}

	if err != nil {
		return domain.Event{}, false, fmt.Errorf("query event: %w", err)
	}

	return event, true, nil
}

func (r *Repository) SaveAIEnrichment(
	ctx context.Context,
	cityID string,
	enrichments []domain.EventEnrichment,
) error {
	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin AI event enrichment: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	const updateEvent = `
		UPDATE events
		SET category = $3,
			description_plain = $4,
			provenance = provenance || '{"enriched_by":"deepseek"}'::jsonb,
			updated_at = now()
		WHERE id = $1 AND city_id = $2 AND status = 'active'`

	for _, enrichment := range enrichments {
		commandTag, err := transaction.Exec(
			ctx,
			updateEvent,
			enrichment.EventID,
			cityID,
			enrichment.Category,
			enrichment.Description,
		)
		if err != nil {
			return fmt.Errorf("update AI event enrichment: %w", err)
		}

		if commandTag.RowsAffected() != 1 {
			return events_errors.ErrUnknownEnrichment
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit AI event enrichment: %w", err)
	}

	return nil
}

const webSearchSourceID = "20000000-0000-0000-0000-000000000003"

const upsertDiscoveredEvent = `
	INSERT INTO events (
		city_id, source_id, external_id, title, description_plain, category, venue_name,
		starts_at, ends_at, price_from, currency, availability, status, trust_status,
		provenance, source_url, popularity_rank, source_updated_at, expires_at, is_demo
	)
	VALUES (
		$1, $2, $3, $4, $5, $6, $7,
		$8, $9, $10, $11, 'unknown', 'active', 'ai_web_search',
		$12, $13, $14, now(), $15, FALSE
	)
	ON CONFLICT (source_id, external_id) DO UPDATE SET
		title = EXCLUDED.title,
		description_plain = EXCLUDED.description_plain,
		category = EXCLUDED.category,
		venue_name = EXCLUDED.venue_name,
		starts_at = EXCLUDED.starts_at,
		ends_at = EXCLUDED.ends_at,
		price_from = EXCLUDED.price_from,
		currency = EXCLUDED.currency,
		status = 'active',
		source_url = EXCLUDED.source_url,
		popularity_rank = COALESCE(EXCLUDED.popularity_rank, events.popularity_rank),
		provenance = EXCLUDED.provenance,
		source_updated_at = now(),
		expires_at = EXCLUDED.expires_at,
		updated_at = now()`

func (r *Repository) SaveCityDiscovery(
	ctx context.Context,
	cityID string,
	discovered []domain.DiscoveredEvent,
	expiresAt time.Time,
) error {
	return r.saveDiscovery(ctx, discovered, expiresAt, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE events
			SET status = 'expired', updated_at = now()
			WHERE city_id = $1
			  AND source_id = $2
			  AND status = 'active'
			  AND popularity_rank IS NULL
		`, cityID, webSearchSourceID)
		if err != nil {
			return fmt.Errorf("expire previous city events: %w", err)
		}

		return nil
	})
}

func (r *Repository) SavePopularDiscovery(
	ctx context.Context,
	discovered []domain.DiscoveredEvent,
	expiresAt time.Time,
) error {
	return r.saveDiscovery(ctx, discovered, expiresAt, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE events
			SET popularity_rank = NULL, updated_at = now()
			WHERE source_id = $1 AND popularity_rank IS NOT NULL
		`, webSearchSourceID)
		if err != nil {
			return fmt.Errorf("reset popular events: %w", err)
		}

		return nil
	})
}

func (r *Repository) saveDiscovery(
	ctx context.Context,
	discovered []domain.DiscoveredEvent,
	expiresAt time.Time,
	prepare func(context.Context, pgx.Tx) error,
) error {
	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin event discovery save: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := prepare(ctx, transaction); err != nil {
		return err
	}

	for _, event := range discovered {
		provenance, err := json.Marshal(map[string]any{
			"provider":   "deepseek",
			"mode":       "web_search",
			"source_url": event.SourceURL,
		})
		if err != nil {
			return fmt.Errorf("encode event provenance: %w", err)
		}

		_, err = transaction.Exec(
			ctx,
			upsertDiscoveredEvent,
			event.CityID,
			webSearchSourceID,
			discoveredExternalID(event),
			event.Title,
			event.Description,
			event.Category,
			event.Venue,
			event.StartsAt,
			event.EndsAt,
			event.PriceFrom,
			event.Currency,
			string(provenance),
			nullText(event.SourceURL),
			nullRank(event.Rank),
			expiresAt,
		)
		if err != nil {
			return fmt.Errorf("save discovered event: %w", err)
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit event discovery save: %w", err)
	}

	return nil
}

func (r *Repository) PopularEvents(ctx context.Context, limit int) ([]domain.Event, error) {
	const query = `
		SELECT
			event.id,
			event.city_id,
			event.external_id,
			event.title,
			event.description_plain,
			event.category,
			event.venue_name,
			event.starts_at,
			event.ends_at,
			COALESCE(event.price_from, 0),
			event.currency,
			COALESCE(event.age_rating, ''),
			event.availability,
			event.status,
			source.name,
			event.trust_status,
			event.source_updated_at,
			event.is_demo,
			territory.name,
			COALESCE(event.source_url, '')
		FROM events event
		JOIN event_sources source ON source.id = event.source_id
		JOIN territories territory ON territory.id = event.city_id
		WHERE event.popularity_rank IS NOT NULL
		  AND event.status = 'active'
		  AND event.ends_at > now()
		  AND event.expires_at > now()
		ORDER BY event.popularity_rank
		LIMIT $1`

	rows, err := r.database.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query popular events: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Event, 0)

	for rows.Next() {
		var event domain.Event
		if err := scanEventInto(rows, &event, &event.CityName, &event.SourceURL); err != nil {
			return nil, fmt.Errorf("scan popular event: %w", err)
		}

		items = append(items, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate popular events: %w", err)
	}

	return items, nil
}

func (r *Repository) DiscoveryCities(ctx context.Context, limit int) ([]domain.Territory, error) {
	rows, err := r.database.Query(ctx, `
		SELECT id, name, region
		FROM territories
		WHERE active
		ORDER BY commercial_priority DESC, name
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query discovery cities: %w", err)
	}
	defer rows.Close()

	cities := make([]domain.Territory, 0)

	for rows.Next() {
		var city domain.Territory
		if err := rows.Scan(&city.ID, &city.Name, &city.Region); err != nil {
			return nil, fmt.Errorf("scan discovery city: %w", err)
		}

		cities = append(cities, city)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery cities: %w", err)
	}

	return cities, nil
}

func (r *Repository) DiscoveryState(
	ctx context.Context,
	scope string,
	scopeKey string,
) (domain.EventDiscoveryState, bool, error) {
	const query = `
		SELECT scope, scope_key, status,
			started_at,
			COALESCE(refreshed_at, to_timestamp(0)),
			COALESCE(expires_at, to_timestamp(0)),
			COALESCE(failure_code, '')
		FROM event_discovery_runs
		WHERE scope = $1 AND scope_key = $2`

	var state domain.EventDiscoveryState

	err := r.database.QueryRow(ctx, query, scope, scopeKey).Scan(
		&state.Scope,
		&state.ScopeKey,
		&state.Status,
		&state.StartedAt,
		&state.RefreshedAt,
		&state.ExpiresAt,
		&state.FailureCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EventDiscoveryState{}, false, nil
	}

	if err != nil {
		return domain.EventDiscoveryState{}, false, fmt.Errorf("query discovery state: %w", err)
	}

	return state, true, nil
}

func (r *Repository) ClaimDiscovery(
	ctx context.Context,
	scope string,
	scopeKey string,
	stuckBefore time.Time,
) (bool, error) {
	const query = `
		INSERT INTO event_discovery_runs (scope, scope_key, status, started_at)
		VALUES ($1, $2, 'running', now())
		ON CONFLICT (scope, scope_key) DO UPDATE SET
			status = 'running',
			started_at = now(),
			failure_code = NULL,
			updated_at = now()
		WHERE (
			event_discovery_runs.status <> 'running'
			AND (event_discovery_runs.expires_at IS NULL OR event_discovery_runs.expires_at <= now())
		) OR (
			event_discovery_runs.status = 'running' AND event_discovery_runs.started_at < $3
		)
		RETURNING scope_key`

	var claimed string

	err := r.database.QueryRow(ctx, query, scope, scopeKey, stuckBefore).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("claim event discovery: %w", err)
	}

	return true, nil
}

func (r *Repository) CompleteDiscovery(
	ctx context.Context,
	scope string,
	scopeKey string,
	found int,
	expiresAt time.Time,
) error {
	_, err := r.database.Exec(ctx, `
		UPDATE event_discovery_runs
		SET status = 'ready',
			events_found = $3,
			failure_code = NULL,
			refreshed_at = now(),
			expires_at = $4,
			updated_at = now()
		WHERE scope = $1 AND scope_key = $2
	`, scope, scopeKey, found, expiresAt)
	if err != nil {
		return fmt.Errorf("complete event discovery: %w", err)
	}

	return nil
}

func (r *Repository) FailDiscovery(
	ctx context.Context,
	scope string,
	scopeKey string,
	failureCode string,
	retryAfter time.Time,
) error {
	_, err := r.database.Exec(ctx, `
		UPDATE event_discovery_runs
		SET status = 'failed',
			failure_code = $3,
			expires_at = $4,
			updated_at = now()
		WHERE scope = $1 AND scope_key = $2
	`, scope, scopeKey, failureCode, retryAfter)
	if err != nil {
		return fmt.Errorf("fail event discovery: %w", err)
	}

	return nil
}

func discoveredExternalID(event domain.DiscoveredEvent) string {
	digest := sha256.Sum256([]byte(strings.ToLower(event.Title) + "|" +
		event.StartsAt.Format(time.DateOnly)))

	return event.CityID + ":" + hex.EncodeToString(digest[:])[:16]
}

func nullText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	return value
}

func nullRank(rank int) any {
	if rank <= 0 {
		return nil
	}

	return rank
}

func scanEvent(row rowScanner) (domain.Event, error) {
	var event domain.Event
	if err := scanEventInto(row, &event); err != nil {
		return domain.Event{}, err
	}

	return event, nil
}

func scanEventInto(row rowScanner, event *domain.Event, extra ...any) error {
	targets := []any{
		&event.ID,
		&event.CityID,
		&event.ExternalID,
		&event.Title,
		&event.Description,
		&event.Category,
		&event.Venue,
		&event.StartsAt,
		&event.EndsAt,
		&event.PriceFrom,
		&event.Currency,
		&event.AgeRating,
		&event.Availability,
		&event.Status,
		&event.Source,
		&event.TrustStatus,
		&event.UpdatedAt,
		&event.Demo,
	}

	return row.Scan(append(targets, extra...)...)
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}

	return value
}

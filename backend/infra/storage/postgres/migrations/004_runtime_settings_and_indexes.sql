INSERT INTO app_settings (key, value)
VALUES
('ai_recommendation','{"candidate_search_limit":6,"offer_page_size":5,"maximum_ranked_options":3,"offer_validity_minutes":15,"required_sources":["tutu_mcp","deepseek"],"allow_fallback":false}'::jsonb),
('league_rules','[{"minimum_score":5000,"name":"Легенды","next_score":0,"percentile":3},{"minimum_score":2000,"name":"Первооткрыватели","next_score":5000,"percentile":9},{"minimum_score":1000,"name":"Искатели","next_score":2000,"percentile":18},{"minimum_score":0,"name":"Путники","next_score":1000,"percentile":42}]'::jsonb)
ON CONFLICT (key) DO NOTHING;

ALTER TABLE user_preferences
    ALTER COLUMN transport_modes SET DEFAULT '{railway,bus,avia}';

UPDATE user_preferences
SET transport_modes = '{railway,bus,avia}'
WHERE cardinality(transport_modes) = 0;

ALTER TABLE reward_ledger
    DROP CONSTRAINT IF EXISTS reward_ledger_idempotency_key_key;

CREATE UNIQUE INDEX IF NOT EXISTS reward_ledger_user_idempotency_idx
    ON reward_ledger(user_id, idempotency_key);

CREATE UNIQUE INDEX IF NOT EXISTS reward_ledger_user_reference_idx
    ON reward_ledger(user_id, reference_type, reference_id);

ALTER TABLE admin_simulation_actions
    DROP CONSTRAINT IF EXISTS admin_simulation_actions_idempotency_key_key;

CREATE UNIQUE INDEX IF NOT EXISTS admin_simulation_actions_actor_idempotency_idx
    ON admin_simulation_actions(actor_user_id, idempotency_key);

DELETE FROM trips
WHERE id IN (
    SELECT id
    FROM (
        SELECT
            id,
            row_number() OVER (
                PARTITION BY user_id, recommendation_option_id
                ORDER BY created_at
            ) AS position
        FROM trips
    ) ranked
    WHERE ranked.position > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS trips_user_option_idx
    ON trips(user_id, recommendation_option_id);

CREATE UNIQUE INDEX IF NOT EXISTS outbox_events_aggregate_type_idx
    ON outbox_events(aggregate_type, aggregate_id, event_type);

CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id);

CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS reward_ledger_user_created_idx
    ON reward_ledger(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS season_score_ledger_user_season_idx
    ON season_score_ledger(user_id, season_id);

CREATE INDEX IF NOT EXISTS user_visits_user_territory_idx
    ON user_visits(user_id, territory_id);

CREATE INDEX IF NOT EXISTS trips_user_created_idx
    ON trips(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS guild_feed_guild_created_idx
    ON guild_feed(guild_id, created_at DESC);

CREATE INDEX IF NOT EXISTS guild_memberships_user_idx
    ON guild_memberships(user_id);

CREATE INDEX IF NOT EXISTS travel_cohort_memberships_cohort_idx
    ON travel_cohort_memberships(cohort_id)
    WHERE left_at IS NULL;

CREATE INDEX IF NOT EXISTS outbox_events_pending_idx
    ON outbox_events(available_at)
    WHERE processed_at IS NULL;

CREATE INDEX IF NOT EXISTS recommendation_options_request_idx
    ON recommendation_options(request_id);

CREATE INDEX IF NOT EXISTS territories_active_name_idx
    ON territories(name)
    WHERE active;

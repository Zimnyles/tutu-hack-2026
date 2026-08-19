ALTER TABLE events ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS popularity_rank INT;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_trust_status_check;
ALTER TABLE events ADD CONSTRAINT events_trust_status_check
    CHECK (trust_status IN ('ai_demo', 'ai_unverified', 'ai_web_search', 'verified'));

CREATE INDEX IF NOT EXISTS events_popularity_idx
    ON events(popularity_rank, starts_at)
    WHERE popularity_rank IS NOT NULL;

INSERT INTO event_sources (id, code, name, base_domain, trust_level, last_synced_at, last_sync_status)
VALUES (
    '20000000-0000-0000-0000-000000000003',
    'deepseek_web_search',
    'Поиск в интернете · DeepSeek',
    'api.deepseek.com',
    40,
    now(),
    'ok'
)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, updated_at = now();

CREATE TABLE IF NOT EXISTS event_discovery_runs (
    scope TEXT NOT NULL CHECK (scope IN ('city', 'country')),
    scope_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'ready', 'failed')),
    events_found INT NOT NULL DEFAULT 0,
    failure_code TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    refreshed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, scope_key)
);

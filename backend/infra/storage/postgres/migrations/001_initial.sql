CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    home_city_id UUID,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'demo_admin')),
    is_demo BOOLEAN NOT NULL DEFAULT FALSE,
    onboarding_completed_at TIMESTAMPTZ,
    travel_visibility TEXT NOT NULL DEFAULT 'private' CHECK (travel_visibility IN ('private', 'aggregate')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    themes TEXT[] NOT NULL DEFAULT '{}',
    transport_modes TEXT[] NOT NULL DEFAULT '{}',
    max_travel_minutes INT NOT NULL DEFAULT 480,
    typical_budget INT NOT NULL DEFAULT 30000,
    trip_duration_days INT NOT NULL DEFAULT 2,
    spontaneity SMALLINT NOT NULL DEFAULT 3 CHECK (spontaneity BETWEEN 1 AND 5),
    avoid TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS territories (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'city' CHECK (kind IN ('country', 'region', 'city', 'settlement')),
    parent_id UUID REFERENCES territories(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    region TEXT NOT NULL,
    geometry GEOMETRY,
    centroid GEOGRAPHY(POINT, 4326) NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    rarity SMALLINT NOT NULL DEFAULT 1,
    reward INT NOT NULL DEFAULT 100,
    description TEXT NOT NULL DEFAULT '',
    image_tone TEXT NOT NULL DEFAULT 'lilac',
    seasonal_fit NUMERIC(4,3) NOT NULL DEFAULT 0.5 CHECK (seasonal_fit BETWEEN 0 AND 1),
    commercial_priority NUMERIC(4,3) NOT NULL DEFAULT 0 CHECK (commercial_priority BETWEEN 0 AND 1),
    source_code TEXT NOT NULL DEFAULT 'curated',
    external_id TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE UNIQUE INDEX IF NOT EXISTS territories_source_external_idx ON territories(source_code, external_id) WHERE external_id IS NOT NULL;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_home_city_id_fkey;
ALTER TABLE users
    ADD CONSTRAINT users_home_city_id_fkey FOREIGN KEY (home_city_id) REFERENCES territories(id);

CREATE TABLE IF NOT EXISTS event_sources (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    base_domain TEXT NOT NULL,
    trust_level SMALLINT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    last_synced_at TIMESTAMPTZ,
    last_sync_status TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    city_id UUID NOT NULL REFERENCES territories(id),
    source_id UUID NOT NULL REFERENCES event_sources(id),
    external_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description_plain TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    venue_name TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    price_from INT,
    currency TEXT NOT NULL DEFAULT 'RUB',
    age_rating TEXT,
    ticket_url TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled', 'expired')),
    availability TEXT NOT NULL DEFAULT 'available' CHECK (availability IN ('available', 'limited', 'sold_out', 'unknown')),
    trust_status TEXT NOT NULL DEFAULT 'verified' CHECK (trust_status IN ('ai_demo', 'ai_unverified', 'verified')),
    provenance JSONB NOT NULL DEFAULT '{}',
    source_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_demo BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_id, external_id)
);
CREATE INDEX IF NOT EXISTS events_city_date_idx ON events(city_id, starts_at, status);

CREATE TABLE IF NOT EXISTS event_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    city_id UUID REFERENCES territories(id),
    category TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS demo_travel_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    demo_profile TEXT NOT NULL,
    origin_city_id UUID NOT NULL REFERENCES territories(id),
    destination_city_id UUID NOT NULL REFERENCES territories(id),
    departed_at TIMESTAMPTZ NOT NULL,
    arrived_at TIMESTAMPTZ NOT NULL,
    transport_mode TEXT NOT NULL,
    external_order_ref TEXT
);

CREATE TABLE IF NOT EXISTS user_visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    territory_id UUID NOT NULL REFERENCES territories(id),
    source TEXT NOT NULL CHECK (source IN ('demo_sync', 'tutu', 'geolocation', 'manual_review')),
    level SMALLINT NOT NULL DEFAULT 1,
    visited_at TIMESTAMPTZ NOT NULL,
    evidence_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, territory_id, source)
);

CREATE TABLE IF NOT EXISTS recommendation_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    origin_city_id UUID NOT NULL REFERENCES territories(id),
    destination_city_id UUID REFERENCES territories(id),
    event_id UUID REFERENCES events(id),
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    adults INT NOT NULL,
    children INT NOT NULL,
    budget INT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'RUB',
    transport_modes TEXT[] NOT NULL,
    max_travel_minutes INT NOT NULL,
    direct_only BOOLEAN NOT NULL DEFAULT FALSE,
    prompt_hash TEXT NOT NULL,
    request_kind TEXT NOT NULL DEFAULT 'prompt' CHECK (request_kind IN ('personal', 'prompt', 'event')),
    status TEXT NOT NULL CHECK (status IN ('received', 'blocked', 'processing', 'partial', 'completed', 'failed')),
    stage_code TEXT NOT NULL,
    guardrail_reason TEXT,
    is_demo_fallback BOOLEAN NOT NULL DEFAULT FALSE,
    request_id UUID,
    trace_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS recommendation_requests_user_kind_created_idx
    ON recommendation_requests(user_id, request_kind, created_at DESC);

CREATE TABLE IF NOT EXISTS recommendation_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES recommendation_requests(id) ON DELETE CASCADE,
    city_id UUID NOT NULL REFERENCES territories(id),
    event_id UUID REFERENCES events(id),
    rank SMALLINT NOT NULL,
    score NUMERIC NOT NULL,
    reason TEXT NOT NULL,
    why_now TEXT NOT NULL,
    price_amount INT NOT NULL,
    currency TEXT NOT NULL,
    duration_minutes INT NOT NULL,
    transport_mode TEXT NOT NULL,
    territory_gain_km2 INT NOT NULL,
    reward INT NOT NULL,
    special_offer BOOLEAN NOT NULL DEFAULT FALSE,
    offer_snapshot JSONB NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recommendation_option_id UUID NOT NULL REFERENCES recommendation_options(id),
    event_id UUID REFERENCES events(id),
    status TEXT NOT NULL CHECK (status IN ('planned', 'checkout_created', 'departed', 'arrived', 'cancelled')),
    checkout_url TEXT,
    depart_at TIMESTAMPTZ NOT NULL,
    arrive_at TIMESTAMPTZ NOT NULL,
    is_demo BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reward_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    reason_code TEXT NOT NULL,
    reference_type TEXT NOT NULL,
    reference_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS achievements (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    icon TEXT NOT NULL,
    target INT NOT NULL,
    category TEXT NOT NULL DEFAULT 'exploration',
    condition JSONB NOT NULL DEFAULT '{}',
    reward INT NOT NULL DEFAULT 50,
    sort_order INT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS user_achievements (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id UUID NOT NULL REFERENCES achievements(id),
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, achievement_id)
);

CREATE TABLE IF NOT EXISTS seasons (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    month_title TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    rules_version TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS season_score_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id UUID NOT NULL REFERENCES seasons(id),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id UUID,
    points INT NOT NULL,
    reason_code TEXT NOT NULL,
    reference_type TEXT NOT NULL,
    reference_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS guilds (
    id UUID PRIMARY KEY,
    territory_id UUID NOT NULL UNIQUE REFERENCES territories(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    emblem_asset TEXT NOT NULL,
    level INT NOT NULL DEFAULT 1,
    demo_member_count INT NOT NULL DEFAULT 0,
    demo_base_score INT NOT NULL DEFAULT 0,
    demo_rank INT NOT NULL DEFAULT 1,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE season_score_ledger
    DROP CONSTRAINT IF EXISTS season_score_ledger_guild_id_fkey;
ALTER TABLE season_score_ledger
    ADD CONSTRAINT season_score_ledger_guild_id_fkey FOREIGN KEY (guild_id) REFERENCES guilds(id);

CREATE TABLE IF NOT EXISTS guild_memberships (
    guild_id UUID NOT NULL REFERENCES guilds(id),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    public_nickname_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    PRIMARY KEY (guild_id, user_id, joined_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS one_active_guild_per_user ON guild_memberships(user_id) WHERE left_at IS NULL;

CREATE TABLE IF NOT EXISTS guild_challenges (
    id UUID PRIMARY KEY,
    season_id UUID NOT NULL REFERENCES seasons(id),
    guild_id UUID REFERENCES guilds(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    metric_code TEXT NOT NULL,
    target_value BIGINT NOT NULL,
    demo_base_progress BIGINT NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS guild_feed (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id UUID NOT NULL REFERENCES guilds(id),
    text TEXT NOT NULL,
    points INT NOT NULL DEFAULT 0,
    is_demo BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS travel_cohorts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    destination_city_id UUID NOT NULL REFERENCES territories(id),
    origin_city_id UUID REFERENCES territories(id),
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    demo_aggregate_count INT NOT NULL DEFAULT 0,
    demo_guild_count INT NOT NULL DEFAULT 0,
    UNIQUE (destination_city_id, origin_city_id, window_start, window_end)
);

CREATE TABLE IF NOT EXISTS travel_cohort_memberships (
    cohort_id UUID NOT NULL REFERENCES travel_cohorts(id),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    visibility TEXT NOT NULL CHECK (visibility IN ('private', 'aggregate', 'discoverable')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    UNIQUE (cohort_id, user_id, trip_id)
);

CREATE TABLE IF NOT EXISTS leaderboard_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id UUID NOT NULL REFERENCES seasons(id),
    scope TEXT NOT NULL,
    scope_id UUID,
    period TEXT NOT NULL,
    period_key TEXT NOT NULL,
    payload JSONB NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (season_id, scope, scope_id, period, period_key)
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    attempts INT NOT NULL DEFAULT 0,
    last_error_code TEXT
);

CREATE TABLE IF NOT EXISTS demo_scenarios (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    fixture_version TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    reset_strategy TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_simulation_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action_code TEXT NOT NULL,
    actor_user_id UUID NOT NULL REFERENCES users(id),
    target_type TEXT NOT NULL,
    target_id UUID,
    demo_scenario_id UUID REFERENCES demo_scenarios(id),
    idempotency_key TEXT NOT NULL UNIQUE,
    reason TEXT NOT NULL,
    request_payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL CHECK (status IN ('requested', 'running', 'completed', 'rejected', 'failed')),
    result_summary JSONB NOT NULL DEFAULT '{}',
    request_id UUID,
    trace_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS admin_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID NOT NULL REFERENCES users(id),
    action_code TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'rejected', 'failure')),
    reason_code TEXT NOT NULL,
    simulation_action_id UUID REFERENCES admin_simulation_actions(id),
    request_id UUID,
    trace_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS city_content (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    city_id UUID NOT NULL REFERENCES territories(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL CHECK (content_type IN ('fact', 'tip', 'seasonal', 'food', 'architecture', 'nature')),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    source_code TEXT NOT NULL,
    is_demo BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    UNIQUE (city_id, content_type, title)
);

CREATE TABLE IF NOT EXISTS travel_collections (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    icon TEXT NOT NULL,
    reward INT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS travel_collection_territories (
    collection_id UUID NOT NULL REFERENCES travel_collections(id) ON DELETE CASCADE,
    territory_id UUID NOT NULL REFERENCES territories(id) ON DELETE CASCADE,
    PRIMARY KEY (collection_id, territory_id)
);

INSERT INTO territories (id, name, slug, region, centroid, tags, rarity, reward, description, image_tone)
VALUES
('10000000-0000-0000-0000-000000000001','Екатеринбург','yekaterinburg','Свердловская область',ST_GeogFromText('POINT(60.61 56.84)'),ARRAY['architecture','food','history'],2,120,'Город конструктивизма, уральской кухни и смелых идей.','lilac'),
('10000000-0000-0000-0000-000000000002','Москва','moscow','Москва',ST_GeogFromText('POINT(37.62 55.75)'),ARRAY['architecture','events','food'],2,120,'Столица с маршрутами на любой ритм.','orange'),
('10000000-0000-0000-0000-000000000003','Казань','kazan','Татарстан',ST_GeogFromText('POINT(49.12 55.79)'),ARRAY['architecture','food','history'],3,140,'Переплетение культур на берегу Волги.','green'),
('10000000-0000-0000-0000-000000000004','Уфа','ufa','Башкортостан',ST_GeogFromText('POINT(55.97 54.74)'),ARRAY['food','nature','events'],3,140,'Город на холмах с башкирской кухней и живой культурой.','green'),
('10000000-0000-0000-0000-000000000005','Пермь','perm','Пермский край',ST_GeogFromText('POINT(56.23 58.01)'),ARRAY['art','events','nature'],3,140,'Современное искусство и маршруты вдоль Камы.','lilac'),
('10000000-0000-0000-0000-000000000006','Тюмень','tyumen','Тюменская область',ST_GeogFromText('POINT(65.53 57.15)'),ARRAY['food','spa','architecture'],2,120,'Термальные источники и уютный центр.','orange'),
('10000000-0000-0000-0000-000000000007','Челябинск','chelyabinsk','Челябинская область',ST_GeogFromText('POINT(61.40 55.16)'),ARRAY['nature','sport','history'],2,120,'Ворота к озёрам Южного Урала.','blue'),
('10000000-0000-0000-0000-000000000008','Тобольск','tobolsk','Тюменская область',ST_GeogFromText('POINT(68.25 58.20)'),ARRAY['history','architecture','unusual'],4,160,'Белокаменный кремль и большая сибирская история.','blue'),
('10000000-0000-0000-0000-000000000009','Омск','omsk','Омская область',ST_GeogFromText('POINT(73.37 54.99)'),ARRAY['architecture','history','food'],2,120,'Сибирский город с выразительной архитектурой.','orange'),
('10000000-0000-0000-0000-000000000010','Новосибирск','novosibirsk','Новосибирская область',ST_GeogFromText('POINT(82.92 55.03)'),ARRAY['events','science','food'],3,140,'Наука, театр и энергия большого сибирского города.','lilac'),
('10000000-0000-0000-0000-000000000011','Самара','samara','Самарская область',ST_GeogFromText('POINT(50.15 53.20)'),ARRAY['river','food','architecture'],2,120,'Волга, модерн и длинная набережная.','green'),
('10000000-0000-0000-0000-000000000012','Нижний Новгород','nizhny-novgorod','Нижегородская область',ST_GeogFromText('POINT(44.00 56.33)'),ARRAY['architecture','history','river'],3,140,'Стрелка двух рек и город на перепадах высот.','orange'),
('10000000-0000-0000-0000-000000000013','Санкт-Петербург','saint-petersburg','Санкт-Петербург',ST_GeogFromText('POINT(30.32 59.94)'),ARRAY['architecture','art','food'],3,140,'Архитектура, музеи и белые ночи.','blue'),
('10000000-0000-0000-0000-000000000014','Вологда','vologda','Вологодская область',ST_GeogFromText('POINT(39.89 59.22)'),ARRAY['history','food','calm'],3,140,'Деревянная архитектура и северная кухня.','green'),
('10000000-0000-0000-0000-000000000015','Ярославль','yaroslavl','Ярославская область',ST_GeogFromText('POINT(39.87 57.63)'),ARRAY['history','architecture','river'],2,120,'Исторический центр и волжские виды.','orange'),
('10000000-0000-0000-0000-000000000016','Псков','pskov','Псковская область',ST_GeogFromText('POINT(28.33 57.82)'),ARRAY['history','architecture','calm'],4,160,'Крепостные стены и спокойный ритм.','blue'),
('10000000-0000-0000-0000-000000000017','Калининград','kaliningrad','Калининградская область',ST_GeogFromText('POINT(20.51 54.71)'),ARRAY['architecture','food','sea'],4,160,'Балтийский воздух и необычная городская среда.','lilac'),
('10000000-0000-0000-0000-000000000018','Сочи','sochi','Краснодарский край',ST_GeogFromText('POINT(39.73 43.59)'),ARRAY['sea','nature','active'],2,120,'Море и горы в одной поездке.','green'),
('10000000-0000-0000-0000-000000000019','Волгоград','volgograd','Волгоградская область',ST_GeogFromText('POINT(44.51 48.71)'),ARRAY['history','river','architecture'],3,140,'Большая история на берегу Волги.','orange'),
('10000000-0000-0000-0000-000000000020','Саратов','saratov','Саратовская область',ST_GeogFromText('POINT(46.03 51.53)'),ARRAY['architecture','river','food'],2,120,'Купеческая архитектура и волжские маршруты.','blue'),
('10000000-0000-0000-0000-000000000021','Оренбург','orenburg','Оренбургская область',ST_GeogFromText('POINT(55.10 51.77)'),ARRAY['history','food','steppe'],3,140,'Степные горизонты на границе Европы и Азии.','green'),
('10000000-0000-0000-0000-000000000022','Ижевск','izhevsk','Удмуртия',ST_GeogFromText('POINT(53.20 56.85)'),ARRAY['history','food','events'],2,120,'Удмуртская кухня и городские фестивали.','lilac'),
('10000000-0000-0000-0000-000000000023','Киров','kirov','Кировская область',ST_GeogFromText('POINT(49.67 58.60)'),ARRAY['calm','history','craft'],3,140,'Народные промыслы и тихие улицы.','orange'),
('10000000-0000-0000-0000-000000000024','Кострома','kostroma','Костромская область',ST_GeogFromText('POINT(40.93 57.77)'),ARRAY['history','food','architecture'],3,140,'Торговые ряды и волжская кухня.','green'),
('10000000-0000-0000-0000-000000000025','Суздаль','suzdal','Владимирская область',ST_GeogFromText('POINT(40.45 56.42)'),ARRAY['history','architecture','calm'],4,160,'Небольшой город с большой коллекцией храмов.','blue'),
('10000000-0000-0000-0000-000000000026','Владимир','vladimir','Владимирская область',ST_GeogFromText('POINT(40.41 56.13)'),ARRAY['history','architecture','food'],3,140,'Белокаменные памятники и старые улицы.','orange'),
('10000000-0000-0000-0000-000000000027','Ростов-на-Дону','rostov-on-don','Ростовская область',ST_GeogFromText('POINT(39.71 47.24)'),ARRAY['food','architecture','river'],2,120,'Южный характер, рынки и набережная Дона.','green'),
('10000000-0000-0000-0000-000000000028','Архангельск','arkhangelsk','Архангельская область',ST_GeogFromText('POINT(40.54 64.54)'),ARRAY['north','history','unusual'],4,160,'Поморская культура и Северная Двина.','blue'),
('10000000-0000-0000-0000-000000000029','Мурманск','murmansk','Мурманская область',ST_GeogFromText('POINT(33.08 68.97)'),ARRAY['north','nature','unusual'],4,160,'Арктический город и северное сияние.','lilac'),
('10000000-0000-0000-0000-000000000030','Иркутск','irkutsk','Иркутская область',ST_GeogFromText('POINT(104.28 52.29)'),ARRAY['nature','history','architecture'],4,160,'Деревянное зодчество по дороге к Байкалу.','orange'),
('10000000-0000-0000-0000-000000000031','Владивосток','vladivostok','Приморский край',ST_GeogFromText('POINT(131.89 43.12)'),ARRAY['sea','food','nature'],4,160,'Мосты, сопки и кухня Тихого океана.','blue')
ON CONFLICT (id) DO NOTHING;

INSERT INTO event_sources (id, code, name, base_domain, trust_level, last_synced_at, last_sync_status)
VALUES ('20000000-0000-0000-0000-000000000001','demo_catalog','Demo catalog · Туту','example.invalid',10,now(),'ok')
ON CONFLICT (id) DO NOTHING;

WITH city_set AS (
    SELECT id, name, row_number() OVER (ORDER BY id) AS city_number
    FROM territories
    WHERE slug IN ('ufa','perm','tyumen','chelyabinsk','tobolsk','kazan','samara','moscow','saint-petersburg','nizhny-novgorod')
), fixture AS (
    SELECT city_set.*, series.event_number
    FROM city_set
    CROSS JOIN generate_series(1,3) AS series(event_number)
)
INSERT INTO events (city_id, source_id, external_id, title, description_plain, category, venue_name, starts_at, ends_at, price_from, age_rating, availability, expires_at, is_demo)
SELECT
    fixture.id,
    '20000000-0000-0000-0000-000000000001',
    fixture.id::text || '-' || fixture.event_number,
    fixture.name || ': ' || (ARRAY['городской фестиваль','музыкальный вечер','выставка новых маршрутов'])[fixture.event_number],
    'Подготовленное демонстрационное событие. Актуальность подтверждается локальным каталогом.',
    (ARRAY['festival','concert','exhibition'])[fixture.event_number],
    (ARRAY['Центральная площадь','Городской театр','Музей города'])[fixture.event_number],
    date_trunc('day', now()) + ((fixture.city_number + fixture.event_number + 3) || ' days')::interval + interval '15 hours',
    date_trunc('day', now()) + ((fixture.city_number + fixture.event_number + 3) || ' days')::interval + interval '18 hours',
    (ARRAY[0,900,550])[fixture.event_number],
    (ARRAY['0+','12+','6+'])[fixture.event_number],
    (ARRAY['available','limited','available'])[fixture.event_number],
    now() + interval '60 days',
    TRUE
FROM fixture
ON CONFLICT (source_id, external_id) DO UPDATE SET
    starts_at = EXCLUDED.starts_at,
    ends_at = EXCLUDED.ends_at,
    expires_at = EXCLUDED.expires_at,
    source_updated_at = now();

UPDATE events SET title = 'Уфимский фестиваль вкуса' WHERE external_id = '10000000-0000-0000-0000-000000000004-1';
UPDATE events SET title = 'Вечер башкирской музыки' WHERE external_id = '10000000-0000-0000-0000-000000000004-2';
UPDATE events SET title = 'Городские истории Уфы' WHERE external_id = '10000000-0000-0000-0000-000000000004-3';

INSERT INTO demo_travel_history (demo_profile, origin_city_id, destination_city_id, departed_at, arrived_at, transport_mode, external_order_ref)
SELECT 'default', origin.id, destination.id, now() - interval '420 days', now() - interval '419 days', 'railway', 'DEMO-MSK'
FROM territories origin, territories destination WHERE origin.slug = 'yekaterinburg' AND destination.slug = 'moscow'
UNION ALL
SELECT 'default', origin.id, destination.id, now() - interval '240 days', now() - interval '239 days', 'railway', 'DEMO-KZN'
FROM territories origin, territories destination WHERE origin.slug = 'yekaterinburg' AND destination.slug = 'kazan'
ON CONFLICT DO NOTHING;

INSERT INTO achievements (id, code, title, description, icon, target)
VALUES
('30000000-0000-0000-0000-000000000001','first_step','Первый шаг','Открыть первый новый город','spark',1),
('30000000-0000-0000-0000-000000000002','three_points','Три точки','Открыть три города','route',3),
('30000000-0000-0000-0000-000000000003','rail','По рельсам','Совершить поездку на поезде','train',1),
('30000000-0000-0000-0000-000000000004','new_region','Новая территория','Открыть новый регион','flag',1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO seasons (id, name, month_title, starts_at, ends_at, status, rules_version)
VALUES ('40000000-0000-0000-0000-000000000001','Сезон первооткрывателей','Уральские истории',date_trunc('year',now()),date_trunc('year',now()) + interval '1 year' - interval '1 second','active','2026.1')
ON CONFLICT (id) DO UPDATE SET starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at;

INSERT INTO guilds (id, territory_id, name, slug, emblem_asset, level, demo_member_count, demo_base_score, demo_rank)
SELECT '50000000-0000-0000-0000-000000000001', id, 'Исследователи Екатеринбурга', 'yekaterinburg-explorers', 'diamond', 12, 2846, 184320, 4
FROM territories WHERE slug = 'yekaterinburg'
ON CONFLICT (id) DO NOTHING;

INSERT INTO guild_challenges (id, season_id, guild_id, title, description, metric_code, target_value, demo_base_progress, starts_at, ends_at, status)
VALUES ('51000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000001','Уральский маршрут','Вместе открыть 500 новых городов','new_city',500,364,date_trunc('month',now()),date_trunc('month',now()) + interval '1 month','active')
ON CONFLICT (id) DO UPDATE SET starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at;

INSERT INTO guild_feed (guild_id, text, points, created_at)
SELECT '50000000-0000-0000-0000-000000000001', item.text, item.points, now() - item.age
FROM (VALUES
    ('Участник гильдии открыл Тобольск',120,interval '12 minutes'),
    ('Команда завершила 800-ю поездку на поезде',800,interval '1 hour'),
    ('Участник гильдии открыл Уфу',140,interval '3 hours')
) AS item(text,points,age)
WHERE NOT EXISTS (SELECT 1 FROM guild_feed);

INSERT INTO travel_cohorts (destination_city_id, origin_city_id, window_start, window_end, expires_at, demo_aggregate_count, demo_guild_count)
SELECT destination.id, origin.id, date_trunc('week',now()) + interval '5 days', date_trunc('week',now()) + interval '8 days', date_trunc('week',now()) + interval '9 days', 24, 7
FROM territories destination, territories origin WHERE destination.slug = 'ufa' AND origin.slug = 'yekaterinburg'
ON CONFLICT (destination_city_id, origin_city_id, window_start, window_end) DO UPDATE SET demo_aggregate_count = 24, demo_guild_count = 7;

INSERT INTO leaderboard_snapshots (season_id, scope, period, period_key, payload)
VALUES ('40000000-0000-0000-0000-000000000001','league','month',to_char(now(),'YYYY-MM'),
'[{"rank":1,"nickname":"Полярная звезда","score":2460,"me":false},{"rank":16,"nickname":"Вольный ветер","score":1690,"me":false},{"rank":17,"nickname":"Вы","score":1640,"me":true},{"rank":18,"nickname":"Тихий ход","score":1595,"me":false},{"rank":42,"nickname":"Лесной след","score":1120,"me":false}]'::jsonb)
ON CONFLICT (season_id, scope, scope_id, period, period_key) DO UPDATE SET payload = EXCLUDED.payload, generated_at = now();

INSERT INTO demo_scenarios (id, code, name, description, fixture_version, reset_strategy)
VALUES ('60000000-0000-0000-0000-000000000001','default_journey','Основной demo-маршрут','Онбординг, синхронизация, поездка в Уфу и открытие территории','1.0.0','transactional_profile_reset')
ON CONFLICT (id) DO NOTHING;

INSERT INTO app_settings (key, value)
VALUES
('privacy_threshold','5'::jsonb),
('recommendation_stages','[{"code":"guardrails","label":"Проверяем обязательные ограничения"},{"code":"ai_classification","label":"DeepSeek разбирает интересы и ограничения"},{"code":"ai_search_plan","label":"DeepSeek составляет разрешённый план поиска"},{"code":"mcp_transport","label":"Туту MCP проверяет транспорт, цены и длительность"},{"code":"backend_scoring","label":"Backend рассчитывает итоговый рейтинг"},{"code":"ai_explanation","label":"DeepSeek объясняет подтверждённые варианты"},{"code":"finalize","label":"Сохраняем рекомендации и снимки оферов"}]'::jsonb),
('onboarding','{"themes":[{"code":"nature","label":"Природа","icon":"leaf"},{"code":"architecture","label":"Архитектура","icon":"building"},{"code":"food","label":"Гастрономия","icon":"food"},{"code":"history","label":"История","icon":"diamond"},{"code":"events","label":"События","icon":"spark"},{"code":"calm","label":"Спокойствие","icon":"cloud"},{"code":"active","label":"Активный отдых","icon":"arrow"},{"code":"unusual","label":"Необычные места","icon":"wave"}],"transport_modes":[{"code":"railway","label":"Поезд","description":"Люблю смотреть в окно"},{"code":"bus","label":"Автобус","description":"Для коротких маршрутов"},{"code":"avia","label":"Самолёт","description":"Когда далеко"}],"travel_time_options":[240,480,720,960],"budget_min":10000,"budget_max":80000,"budget_step":5000}'::jsonb),
('scoring_weights','{"preference_match":0.25,"price_value":0.18,"map_gain":0.14,"novelty":0.13,"seasonal_fit":0.10,"event_fit":0.10,"travel_friction":0.05,"commercial_priority":0.05,"rarity_ceiling":5}'::jsonb),
('admin_action_allowlist','["demo_sync_history","recommendation_complete","trip_checkout_created","trip_departed","trip_arrived","trip_cancelled","event_set_availability","event_cancel","cohort_set_demo_size","guild_join","leaderboard_rebuild","outbox_process","demo_profile_reset"]'::jsonb)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

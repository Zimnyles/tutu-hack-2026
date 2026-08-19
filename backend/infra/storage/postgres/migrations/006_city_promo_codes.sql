ALTER TABLE territories
    ADD COLUMN IF NOT EXISTS promo_percent SMALLINT NOT NULL DEFAULT 0
        CHECK (promo_percent BETWEEN 0 AND 100);

UPDATE territories
SET promo_percent = CASE
        WHEN rarity >= 4 THEN 10
        WHEN rarity = 3 THEN 7
        ELSE 5
    END
WHERE promo_percent = 0
  AND (rarity >= 4 OR abs(hashtext(slug)) % 100 < 35);

CREATE TABLE IF NOT EXISTS user_promo_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    territory_id UUID NOT NULL REFERENCES territories(id),
    trip_id UUID REFERENCES trips(id) ON DELETE SET NULL,
    code TEXT NOT NULL UNIQUE,
    discount_percent SMALLINT NOT NULL CHECK (discount_percent BETWEEN 1 AND 100),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired')),
    reason_code TEXT NOT NULL DEFAULT 'CITY_OPENED',
    idempotency_key TEXT NOT NULL UNIQUE,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '90 days'
);

CREATE INDEX IF NOT EXISTS user_promo_codes_user_idx
    ON user_promo_codes(user_id, issued_at DESC);

INSERT INTO user_promo_codes (
    user_id,
    territory_id,
    code,
    discount_percent,
    idempotency_key
)
SELECT
    visit.user_id,
    territory.id,
    'TUTU' || territory.promo_percent || '-' || upper(substr(md5(visit.user_id::text || territory.id::text), 1, 6)),
    territory.promo_percent,
    'visit:' || visit.user_id || ':' || territory.id
FROM user_visits visit
JOIN territories territory ON territory.id = visit.territory_id
WHERE territory.promo_percent > 0
ON CONFLICT DO NOTHING;

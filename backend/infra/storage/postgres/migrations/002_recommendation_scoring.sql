ALTER TABLE territories
    ADD COLUMN IF NOT EXISTS seasonal_fit NUMERIC(4,3) NOT NULL DEFAULT 0.5
        CHECK (seasonal_fit BETWEEN 0 AND 1),
    ADD COLUMN IF NOT EXISTS commercial_priority NUMERIC(4,3) NOT NULL DEFAULT 0
        CHECK (commercial_priority BETWEEN 0 AND 1);

ALTER TABLE recommendation_requests
    ADD COLUMN IF NOT EXISTS request_kind TEXT NOT NULL DEFAULT 'prompt'
        CHECK (request_kind IN ('personal', 'prompt', 'event'));

CREATE INDEX IF NOT EXISTS recommendation_requests_user_kind_created_idx
    ON recommendation_requests(user_id, request_kind, created_at DESC);

INSERT INTO app_settings (key, value)
VALUES (
    'scoring_weights',
    '{"preference_match":0.25,"price_value":0.18,"map_gain":0.14,"novelty":0.13,"seasonal_fit":0.10,"event_fit":0.10,"travel_friction":0.05,"commercial_priority":0.05,"rarity_ceiling":5}'::jsonb
)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now();

INSERT INTO app_settings (key, value)
VALUES (
    'personal_recommendation',
    '{"lead_days":7,"default_duration_days":2,"maximum_duration_days":14,"freshness_hours":24,"adults":1,"currency":"RUB"}'::jsonb
)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now();

INSERT INTO app_settings (key, value)
VALUES (
    'recommendation_stages',
    '[{"code":"guardrails","label":"Проверяем обязательные ограничения"},{"code":"ai_classification","label":"DeepSeek разбирает интересы и ограничения"},{"code":"ai_search_plan","label":"DeepSeek составляет разрешённый план поиска"},{"code":"mcp_transport","label":"Туту MCP проверяет транспорт, цены и длительность"},{"code":"backend_scoring","label":"Backend рассчитывает итоговый рейтинг"},{"code":"ai_explanation","label":"DeepSeek объясняет подтверждённые варианты"},{"code":"finalize","label":"Сохраняем рекомендации и снимки оферов"}]'::jsonb
)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now();

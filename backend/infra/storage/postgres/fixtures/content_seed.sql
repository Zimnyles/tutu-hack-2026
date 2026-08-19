INSERT INTO event_sources (
    id,
    code,
    name,
    base_domain,
    trust_level,
    last_synced_at,
    last_sync_status
)
VALUES (
    '20000000-0000-0000-0000-000000000002',
    'deepseek_ai_demo',
    'AI-афиша',
    'api.deepseek.com',
    5,
    now(),
    'ok'
)
ON CONFLICT (id) DO UPDATE SET
    last_synced_at = now(),
    last_sync_status = 'ok';

WITH event_templates AS (
    SELECT * FROM (VALUES
        (1, 'festival', 'Городской фестиваль «Открытый маршрут»', 'Центральная площадь', 0, '0+', 'available'),
        (2, 'concert', 'Музыкальный вечер «Звук города»', 'Городской театр', 900, '12+', 'limited'),
        (3, 'exhibition', 'Выставка «Город в деталях»', 'Краеведческий музей', 550, '6+', 'available'),
        (4, 'food', 'Гастрономическая неделя локальной кухни', 'Городской рынок', 700, '0+', 'available'),
        (5, 'sport', 'Прогулка-марафон по знаковым местам', 'Набережная', 0, '6+', 'available'),
        (6, 'theatre', 'Театральная премьера «Дорога домой»', 'Драматический театр', 1200, '16+', 'limited')
    ) AS template(number, category, title, venue, price, age_rating, availability)
)
INSERT INTO events (
    city_id,
    source_id,
    external_id,
    title,
    description_plain,
    category,
    venue_name,
    starts_at,
    ends_at,
    price_from,
    age_rating,
    availability,
    trust_status,
    provenance,
    expires_at,
    is_demo
)
SELECT
    territory.id,
    '20000000-0000-0000-0000-000000000002',
    'ai-demo-' || territory.id || '-' || event_template.number,
    territory.name || ': ' || event_template.title,
    'Карточка подготовлена искусственным интеллектом для проверки городского сценария. Перед реальной поездкой уточните программу у организатора.',
    event_template.category,
    event_template.venue,
    date_trunc('day', now()) + ((event_template.number + territory.rarity + 4) || ' days')::interval + interval '15 hours',
    date_trunc('day', now()) + ((event_template.number + territory.rarity + 4) || ' days')::interval + interval '18 hours',
    event_template.price,
    event_template.age_rating,
    event_template.availability,
    'ai_demo',
    jsonb_build_object(
        'provider', 'deepseek',
        'mode', 'offline_fixture',
        'fixture_version', '1.0.0'
    ),
    now() + interval '60 days',
    TRUE
FROM territories territory
CROSS JOIN event_templates event_template
WHERE territory.active
ON CONFLICT (source_id, external_id) DO UPDATE SET
    starts_at = EXCLUDED.starts_at,
    ends_at = EXCLUDED.ends_at,
    expires_at = EXCLUDED.expires_at,
    source_updated_at = now();

INSERT INTO city_content (
    city_id,
    content_type,
    title,
    body,
    source_code,
    sort_order
)
SELECT
    territory.id,
    content.type,
    content.title,
    content.body,
    'deepseek_ai_demo',
    content.sort_order
FROM territories territory
CROSS JOIN LATERAL (
    VALUES
        ('fact', 'Почему стоит открыть', territory.name || ' добавит на карту новый маршрут и поможет лучше почувствовать разнообразие регионов России.', 10),
        ('tip', 'Идея на первый день', 'Начните с прогулки по центру, выберите локальное кафе и оставьте время на одно событие из городской афиши.', 20),
        ('food', 'Попробовать город', 'Ищите блюда и продукты, которыми гордятся местные жители. Спросите о сезонном меню и небольших семейных заведениях.', 30),
        ('seasonal', 'Когда ехать', 'Проверьте прогноз погоды и расписание событий: впечатление от города сильно меняется вместе с сезоном.', 40)
) AS content(type, title, body, sort_order)
ON CONFLICT (city_id, content_type, title) DO NOTHING;

INSERT INTO achievements (
    id,
    code,
    title,
    description,
    icon,
    target,
    category,
    condition,
    reward,
    sort_order
)
VALUES
('30000000-0000-0000-0000-000000000001','first_step','Первый шаг','Открыть первый новый город','spark',1,'start','{"metric":"unique_cities"}',50,10),
('30000000-0000-0000-0000-000000000002','three_points','Три точки','Открыть три города','route',3,'start','{"metric":"unique_cities"}',80,20),
('30000000-0000-0000-0000-000000000003','rail','По рельсам','Совершить поездку на поезде','train',1,'transport','{"metric":"trips_by_mode","mode":"railway"}',70,30),
('30000000-0000-0000-0000-000000000004','new_region','Новая территория','Открыть новый регион','flag',1,'geography','{"metric":"unique_regions"}',100,40),
('30000000-0000-0000-0000-000000000005','five_cities','Пять открытий','Открыть пять городов','map',5,'start','{"metric":"unique_cities"}',120,50),
('30000000-0000-0000-0000-000000000006','ten_cities','Десять городов','Открыть десять городов','map',10,'start','{"metric":"unique_cities"}',200,60),
('30000000-0000-0000-0000-000000000007','neighbor_world','Соседний мир','Открыть город домашнего региона','compass',1,'geography','{"metric":"home_region_city"}',80,70),
('30000000-0000-0000-0000-000000000008','two_capitals','Две столицы','Открыть Москву и Санкт-Петербург','crown',2,'geography','{"metric":"collection","code":"two_capitals"}',200,80),
('30000000-0000-0000-0000-000000000009','beyond_urals','За Уралом','Открыть три города восточнее Урала','mountain',3,'geography','{"metric":"east_cities"}',160,90),
('30000000-0000-0000-0000-000000000010','to_volga','К Волге','Открыть три волжских города','wave',3,'geography','{"metric":"tag","tag":"river"}',160,100),
('30000000-0000-0000-0000-000000000011','russian_north','Русский Север','Открыть два северных города','snow',2,'geography','{"metric":"tag","tag":"north"}',160,110),
('30000000-0000-0000-0000-000000000012','bus_route','Автобусный маршрут','Совершить поездку на автобусе','bus',1,'transport','{"metric":"trips_by_mode","mode":"bus"}',70,120),
('30000000-0000-0000-0000-000000000013','night_express','Ночной экспресс','Прибыть после ночной поездки','moon',1,'transport','{"metric":"night_trip"}',100,130),
('30000000-0000-0000-0000-000000000014','eco_choice','Экологичный выбор','Совершить три поездки на поезде или автобусе','leaf',3,'transport','{"metric":"eco_trips"}',140,140),
('30000000-0000-0000-0000-000000000015','event_trip','Еду на событие','Собрать поездку вокруг события','ticket',1,'events','{"metric":"event_trips"}',80,150),
('30000000-0000-0000-0000-000000000016','festival_season','Фестивальный сезон','Посетить три фестивальных события','party',3,'events','{"metric":"events_by_category","category":"festival"}',160,160),
('30000000-0000-0000-0000-000000000017','museum_day','Музейный день','Собрать поездку к выставке','museum',1,'events','{"metric":"events_by_category","category":"exhibition"}',90,170),
('30000000-0000-0000-0000-000000000018','food_route','Гастрономический маршрут','Открыть три города с гастрономической программой','food',3,'events','{"metric":"events_by_category","category":"food"}',160,180),
('30000000-0000-0000-0000-000000000019','spontaneous','Спонтанное решение','Выбрать предложение менее чем за 48 часов','bolt',1,'style','{"metric":"spontaneous_trip"}',120,190),
('30000000-0000-0000-0000-000000000020','weekend','Выходные в пути','Совершить поездку длительностью до трёх дней','calendar',1,'style','{"metric":"short_trip"}',80,200),
('30000000-0000-0000-0000-000000000021','small_city','Малый город — большая история','Открыть три малых города','gem',3,'style','{"metric":"small_cities"}',180,210),
('30000000-0000-0000-0000-000000000022','no_flight','Без самолёта','Открыть пять городов наземным транспортом','leaf',5,'style','{"metric":"ground_cities"}',180,220),
('30000000-0000-0000-0000-000000000023','deeper','Глубже в город','Открыть второй уровень города','layers',1,'exploration','{"metric":"territory_level","level":2}',130,230),
('30000000-0000-0000-0000-000000000024','territory_master','Мастер территории','Открыть третий уровень города','star',1,'exploration','{"metric":"territory_level","level":3}',240,240),
('30000000-0000-0000-0000-000000000025','guild_join','Вступление в гильдию','Вступить в домашнюю гильдию','users',1,'community','{"metric":"guild_join"}',60,250),
('30000000-0000-0000-0000-000000000026','guild_contribution','Общий вклад','Принести гильдии 500 очков','shield',500,'community','{"metric":"guild_points"}',150,260),
('30000000-0000-0000-0000-000000000027','together','Вместе в путь','Запланировать направление с активным cohort','people',1,'community','{"metric":"cohort_trip"}',100,270),
('30000000-0000-0000-0000-000000000028','first_stage','Первый этап','Получить первые сезонные очки','trophy',1,'season','{"metric":"season_points"}',60,280),
('30000000-0000-0000-0000-000000000029','new_league','В новой лиге','Перейти в следующую сезонную лигу','crown',1,'season','{"metric":"league_up"}',180,290),
('30000000-0000-0000-0000-000000000030','monthly_goal','Месячная цель','Выполнить месячное задание','target',1,'season','{"metric":"monthly_challenge"}',220,300)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon,
    target = EXCLUDED.target,
    category = EXCLUDED.category,
    condition = EXCLUDED.condition,
    reward = EXCLUDED.reward,
    sort_order = EXCLUDED.sort_order;

INSERT INTO travel_collections (id, code, title, description, icon, reward)
VALUES
('70000000-0000-0000-0000-000000000001','two_capitals','Две столицы','Откройте Москву и Санкт-Петербург','crown',300),
('70000000-0000-0000-0000-000000000002','ural_ring','Уральское кольцо','Соберите ключевые города Урала','mountain',420),
('70000000-0000-0000-0000-000000000003','volga_route','Большая Волга','Откройте города вдоль Волги','wave',500),
('70000000-0000-0000-0000-000000000004','russian_north','Русский Север','Доберитесь до северных городов','snow',500),
('70000000-0000-0000-0000-000000000005','golden_weekend','Исторические выходные','Откройте небольшие города с большой историей','museum',360),
('70000000-0000-0000-0000-000000000006','siberian_scale','Сибирский масштаб','Исследуйте города Сибири','compass',600),
('70000000-0000-0000-0000-000000000007','sea_to_sea','От моря до моря','Откройте балтийское, черноморское и тихоокеанское направления','ship',650),
('70000000-0000-0000-0000-000000000008','food_map','Карта вкусов','Соберите гастрономические города','food',400)
ON CONFLICT (id) DO NOTHING;

INSERT INTO travel_collection_territories (collection_id, territory_id)
SELECT collection.id, territory.id
FROM travel_collections collection
JOIN territories territory ON
    (collection.code = 'two_capitals' AND territory.slug IN ('moscow','saint-petersburg')) OR
    (collection.code = 'ural_ring' AND territory.tags && ARRAY['mountain','steppe']) OR
    (collection.code = 'volga_route' AND territory.tags @> ARRAY['river']) OR
    (collection.code = 'russian_north' AND territory.tags @> ARRAY['north']) OR
    (collection.code = 'golden_weekend' AND territory.tags && ARRAY['history','architecture']) OR
    (collection.code = 'siberian_scale' AND ST_X(territory.centroid::geometry) BETWEEN 65 AND 120) OR
    (collection.code = 'sea_to_sea' AND territory.tags @> ARRAY['sea']) OR
    (collection.code = 'food_map' AND territory.tags @> ARRAY['food'])
ON CONFLICT DO NOTHING;

INSERT INTO guilds (
    id,
    territory_id,
    name,
    slug,
    emblem_asset,
    level,
    demo_member_count,
    demo_base_score,
    demo_rank
)
SELECT
    md5('guild:' || territory.id)::uuid,
    territory.id,
    'Исследователи ' || territory.name,
    'guild-' || territory.slug,
    (ARRAY['diamond','compass','mountain','star'])[(territory.rarity % 4) + 1],
    3 + territory.rarity * 2,
    120 + territory.rarity * 173,
    12000 + territory.rarity * 8340,
    territory.rarity + 2
FROM territories territory
WHERE territory.active
ON CONFLICT (territory_id) DO NOTHING;

INSERT INTO guild_challenges (
    id,
    season_id,
    guild_id,
    title,
    description,
    metric_code,
    target_value,
    demo_base_progress,
    starts_at,
    ends_at,
    status
)
SELECT
    md5('challenge:' || guild.id)::uuid,
    '40000000-0000-0000-0000-000000000001',
    guild.id,
    'Открываем Россию вместе',
    'Участники гильдии вместе открывают новые города в этом месяце',
    'new_city',
    500,
    80 + guild.level * 17,
    date_trunc('month', now()),
    date_trunc('month', now()) + interval '1 month',
    'active'
FROM guilds guild
ON CONFLICT (id) DO UPDATE SET
    starts_at = EXCLUDED.starts_at,
    ends_at = EXCLUDED.ends_at;

DELETE FROM leaderboard_snapshots
WHERE season_id = '40000000-0000-0000-0000-000000000001'
  AND scope IN ('league', 'guild', 'global');

INSERT INTO leaderboard_snapshots (season_id, scope, period, period_key, payload)
VALUES
('40000000-0000-0000-0000-000000000001','league','month',to_char(now(),'YYYY-MM'),
'[{"rank":1,"nickname":"Полярная звезда","score":2460,"me":false},{"rank":2,"nickname":"Северный маршрут","score":2285,"me":false},{"rank":3,"nickname":"Ветер странствий","score":2140,"me":false},{"rank":16,"nickname":"Вольный ветер","score":1690,"me":false},{"rank":17,"nickname":"Вы","score":1640,"me":true},{"rank":18,"nickname":"Тихий ход","score":1595,"me":false},{"rank":19,"nickname":"Каменный мост","score":1540,"me":false}]'::jsonb),
('40000000-0000-0000-0000-000000000001','league','season','season-2026',
'[{"rank":1,"nickname":"Полярная звезда","score":11840,"me":false},{"rank":2,"nickname":"Ветер странствий","score":10920,"me":false},{"rank":3,"nickname":"Северный маршрут","score":10375,"me":false},{"rank":22,"nickname":"Городской филин","score":6180,"me":false},{"rank":23,"nickname":"Вы","score":6040,"me":true},{"rank":24,"nickname":"Тихий ход","score":5915,"me":false},{"rank":25,"nickname":"Лесной след","score":5720,"me":false}]'::jsonb),
('40000000-0000-0000-0000-000000000001','guild','month',to_char(now(),'YYYY-MM'),
'[{"rank":1,"nickname":"Хранитель карты","score":2180,"me":false},{"rank":2,"nickname":"Утренний поезд","score":1960,"me":false},{"rank":3,"nickname":"Вы","score":1640,"me":true},{"rank":4,"nickname":"Ночной перрон","score":1480,"me":false},{"rank":5,"nickname":"Северная тропа","score":1325,"me":false}]'::jsonb),
('40000000-0000-0000-0000-000000000001','guild','season','season-2026',
'[{"rank":1,"nickname":"Хранитель карты","score":9240,"me":false},{"rank":2,"nickname":"Утренний поезд","score":7815,"me":false},{"rank":3,"nickname":"Ночной перрон","score":6390,"me":false},{"rank":4,"nickname":"Вы","score":6040,"me":true},{"rank":5,"nickname":"Северная тропа","score":5470,"me":false}]'::jsonb),
('40000000-0000-0000-0000-000000000001','global','month',to_char(now(),'YYYY-MM'),
'[{"rank":1,"nickname":"Атлас Сибири","score":8420,"me":false},{"rank":2,"nickname":"Полярная звезда","score":7965,"me":false},{"rank":3,"nickname":"Тихая гавань","score":7310,"me":false},{"rank":1841,"nickname":"Медный купол","score":1655,"me":false},{"rank":1842,"nickname":"Вы","score":1640,"me":true},{"rank":1843,"nickname":"Дальний путь","score":1628,"me":false}]'::jsonb),
('40000000-0000-0000-0000-000000000001','global','season','season-2026',
'[{"rank":1,"nickname":"Атлас Сибири","score":32750,"me":false},{"rank":2,"nickname":"Полярная звезда","score":29480,"me":false},{"rank":3,"nickname":"Тихая гавань","score":27910,"me":false},{"rank":2710,"nickname":"Медный купол","score":6095,"me":false},{"rank":2711,"nickname":"Вы","score":6040,"me":true},{"rank":2712,"nickname":"Дальний путь","score":5980,"me":false}]'::jsonb);

INSERT INTO app_settings (key, value)
VALUES
('content_targets','{"minimum_cities":200,"ai_events_per_city":6,"minimum_achievements":24,"city_content_cards":4,"travel_collections":8}'::jsonb),
('event_enrichment','{"provider":"deepseek","maximum_items":60,"fact_sources":["trusted_feed","admin_upload","demo_fixture"],"allow_fact_generation":false}'::jsonb),
('ai_recommendation','{"candidate_search_limit":6,"offer_page_size":5,"maximum_ranked_options":3,"offer_validity_minutes":15,"required_sources":["tutu_mcp","deepseek"],"allow_fallback":false}'::jsonb),
('personal_recommendation','{"lead_days":7,"default_duration_days":2,"maximum_duration_days":14,"freshness_hours":24,"adults":1,"currency":"RUB"}'::jsonb),
('scoring_weights','{"preference_match":0.25,"price_value":0.18,"map_gain":0.14,"novelty":0.13,"seasonal_fit":0.10,"event_fit":0.10,"travel_friction":0.05,"commercial_priority":0.05,"rarity_ceiling":5}'::jsonb),
('recommendation_stages','[{"code":"guardrails","label":"Проверяем ограничения запроса"},{"code":"ai_classification","label":"Искусственный интеллект разбирает ваш запрос"},{"code":"ai_search_plan","label":"Искусственный интеллект готовит план поиска"},{"code":"mcp_transport","label":"Проверяем транспорт, цены и длительность"},{"code":"backend_scoring","label":"Считаем итоговый рейтинг вариантов"},{"code":"ai_explanation","label":"Искусственный интеллект объясняет найденные варианты"},{"code":"finalize","label":"Сохраняем рекомендации и актуальные цены"}]'::jsonb),
('league_rules','[{"minimum_score":5000,"name":"Легенды","next_score":0,"percentile":3},{"minimum_score":2000,"name":"Первооткрыватели","next_score":5000,"percentile":9},{"minimum_score":1000,"name":"Искатели","next_score":2000,"percentile":18},{"minimum_score":0,"name":"Путники","next_score":1000,"percentile":42}]'::jsonb)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

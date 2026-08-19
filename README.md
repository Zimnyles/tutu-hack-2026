# Открывай — техническая документация

«Открывай» — сервис, который превращает поездки по России в игру: пользователь открывает города на 3D-глобусе, получает награды и промокоды, а AI подбирает поездки под его интересы и реальные транспортные предложения из Tutu MCP.

Репозиторий содержит всё для запуска целиком: Go-бэкенд, React-PWA, миграции PostgreSQL и демо-данные.

| | |
|---|---|
| Backend | Go 1.25, Fiber v2, pgx v5, Redis |
| Frontend | React 19, TypeScript 5.8, Vite 7, Three.js, PWA |
| Хранилище | PostgreSQL 16 + PostGIS 3.4 |
| Кеш и лимиты | Redis 7 |
| Внешние сервисы | Tutu MCP (транспорт и оформление), DeepSeek (ранжирование, поиск событий) |
| Развёртывание | Docker Compose, multi-stage образы, nginx |

---

## 1. Быстрый старт

### 1.1. Через Docker Compose (рекомендуется)

Нужен только Docker. Правки конфигурации не требуются:

```bash
git clone <repo> && cd tutu_h
cp .env.example .env
make up
```

`make up` — это `docker compose up --build --detach --remove-orphans`. Compose сам подставляет то, что в `.env.example` оставлено пустым: `DATABASE_URL` и `REDIS_ADDR` он задаёт по именам сервисов, а демо-пароли берёт из дефолтов вида `${DEMO_USER_PASSWORD:-openworld-demo-2026}`.

Что поднимется:

| Сервис | Адрес | Описание |
|---|---|---|
| `web` | http://localhost:5173 | nginx с собранным PWA, проксирует `/api` на `api` |
| `api` | http://localhost:8080 | Go API |
| `postgres` | только внутри сети compose | PostGIS 16, том `postgres-data` |
| `redis` | только внутри сети compose | Redis 7 |

Открывайте **http://localhost:5173** и входите демо-аккаунтом:

| Роль | Email | Пароль |
|---|---|---|
| Пользователь | `demo@otkryvai.local` | `openworld-demo-2026` |
| Демо-админ (`/admin`) | `admin@otkryvai.local` | `openworld-admin-2026` |

Чтобы задать свои значения, впишите `DEMO_USER_PASSWORD` и `DEMO_ADMIN_PASSWORD` в `.env` — они переопределят дефолты.

При `DEMO_MODE=true` (по умолчанию) API на старте сам применит миграции, загрузит справочник городов и контент и создаст демо-аккаунты. Отдельных команд для этого запускать не нужно.

`DEEPSEEK_API_KEY` нужен только для AI-функций — подбора поездок и афиши. Без него приложение поднимется, глобус, поездки, награды и сообщество будут работать, а запросы к AI вернут ошибку.

Полезное:

```bash
make logs      # логи всех сервисов
make status    # состояние контейнеров
make down      # остановить
```

### 1.2. Локальная разработка без Docker

Нужны Go 1.25, Node 24 (как в Dockerfile) и pnpm.

В `docker-compose.yml` порты Postgres и Redis наружу не выведены — они нужны только контейнеру `api`, и это правильный дефолт. Для локальной разработки откройте их отдельным override-файлом, который Compose подхватывает автоматически, не меняя основной:

```yaml
# docker-compose.override.yml
services:
  postgres:
    ports: ["5432:5432"]
  redis:
    ports: ["6379:6379"]
```

```bash
# 1. Только инфраструктура
docker compose up -d postgres redis

# 2. Backend (порт 8080)
cd backend
set -a && source ../.env && set +a
export DATABASE_URL='postgres://openworld:openworld@localhost:5432/openworld?sslmode=disable'
export REDIS_ADDR=localhost:6379
export DEMO_USER_PASSWORD=openworld-demo-2026
export DEMO_ADMIN_PASSWORD=openworld-admin-2026
go run ./cmd/api

# 3. Frontend (порт 5173) — в другом терминале
cd frontend
pnpm install
pnpm dev
```

Два экспорта с паролями обязательны: при `DEMO_MODE=true` пустые `DEMO_*_PASSWORD` останавливают старт, а дефолты из `${...:-...}` подставляет compose, и напрямую через `go run` они не действуют.

Vite-дев-сервер проксирует `/api` на `http://localhost:8080`, поэтому CORS в разработке не мешает.

Полный разбор всех переменных окружения — в §12, читать его для запуска не обязательно.

### 1.3. Проверки перед коммитом

```bash
# backend
cd backend && go build ./... && go test ./... && golangci-lint run

# frontend
cd frontend && pnpm typecheck && pnpm lint && pnpm test && pnpm build
```

### 1.4. Первый сценарий для проверки

1. Войти демо-пользователем → пройти онбординг (интересы, транспорт, домашний город).
2. На глобусе выбрать город → откроется карточка с афишей и бейджами.
3. Нажать подбор поездки → в панели пойдут стадии AI-воркфлоу по SSE.
4. Выбрать вариант → создастся поездка со ссылкой на оформление в Tutu.
5. «Симулировать прибытие» → город открывается, начисляется награда и промокод.

---

## 2. Архитектура

### 2.1. Контекст системы

```mermaid
graph LR
    U["Пользователь<br/>браузер / PWA"]
    W["web<br/>nginx + статика"]
    A["api<br/>Go + Fiber"]
    P[("PostgreSQL<br/>+ PostGIS")]
    R[("Redis")]
    T["Tutu MCP<br/>внешний"]
    D["DeepSeek<br/>внешний"]

    U -->|HTTPS| W
    W -->|"/api/* proxy"| A
    A -->|SQL, pgx| P
    A -->|лимиты, кеш| R
    A -->|JSON-RPC 2.0| T
    A -->|HTTP JSON| D
```

Ключевая мысль: браузер никогда не обращается к DeepSeek или Tutu напрямую. Все внешние вызовы идут с бэкенда, ключи и токены не покидают серверную часть.

### 2.2. Слои бэкенда

```mermaid
graph TD
    R["router<br/>internal/router"] --> M["middlewares<br/>request-id, recover, security headers,<br/>logging, CORS, rate limit"]
    M --> H["handlers<br/>internal/handlers/*<br/>валидация, DTO, HTTP-коды"]
    H --> S["services<br/>services/*<br/>бизнес-логика, транзакции, квоты"]
    S --> ST["storage<br/>infra/storage/*<br/>SQL-репозитории"]
    S --> C["clients<br/>infra/clients/deepseek<br/>infra/tutumcp<br/>infra/checkout"]
    S --> RL["ratelimit<br/>infra/ratelimit"]
    ST --> DB[("PostgreSQL")]
    RL --> RD[("Redis")]
```

Правила, которые держат эту структуру:

- **Handler не знает про SQL.** Он валидирует вход, достаёт пользователя из контекста и вызывает сервис.
- **Сервис не знает про HTTP.** Он принимает `context.Context` и доменные типы, возвращает доменные типы или типизированную ошибку из `internal/errors/*`.
- **Зависимости передаются интерфейсами.** У каждого сервиса есть свой `interfaces.go` с минимальным контрактом того, что ему нужно от репозитория и клиента. Поэтому юнит-тесты пишутся на заглушках без базы.
- **Сборка графа зависимостей — в одном месте**, `internal/api/api.go`. Там создаются пул, репозитории, клиенты, сервисы, хендлеры и регистрируются маршруты.

### 2.3. Жизненный цикл процесса

```mermaid
sequenceDiagram
    participant M as main.go
    participant Res as resources
    participant DB as PostgreSQL
    participant API as Fiber

    M->>M: signal.NotifyContext (SIGINT/SIGTERM)
    M->>Res: InitResources
    Res->>Res: LoadEnv + валидация
    Res->>Res: JSON-логгер (slog)
    Res->>DB: pgxpool + Ping
    Res->>Res: Redis client + Ping
    M->>DB: Migrate — embedded SQL под advisory lock
    opt DEMO_MODE=true
        M->>DB: SeedFixtures — города, бейджи, контент
        M->>DB: EnsureDemoAccount — user и demo_admin
    end
    M->>API: api.New(...).Start(ctx)
    API->>API: фоновые задачи discovery (30m / 10m)
    API->>API: Listen(:APP_PORT)
    Note over M,API: SIGTERM → ShutdownWithTimeout(SHUTDOWN_PERIOD)
```

Важное свойство: миграции применяются под `pg_advisory_xact_lock`, поэтому несколько одновременно стартующих реплик API не сломают схему — лишние просто подождут и увидят, что версия уже применена.

### 2.4. Путь обычного запроса

```mermaid
sequenceDiagram
    participant B as Браузер
    participant N as nginx
    participant MW as Middlewares
    participant H as Handler
    participant S as Service
    participant DB as PostgreSQL

    B->>N: GET /api/v1/world/bootstrap (cookie ow_session)
    N->>MW: proxy_pass api:8080
    MW->>MW: request-id → recover → security headers
    MW->>MW: логирование → CORS → rate limit по IP
    MW->>MW: Authenticate: хеш токена → сессия из БД
    MW->>MW: VerifyCSRF (для GET пропускается)
    MW->>H: ctx с пользователем
    H->>S: вызов бизнес-логики
    S->>DB: SQL
    DB-->>S: строки
    S-->>H: доменные типы
    H-->>B: JSON
```

---

## 3. Ключевой сценарий: AI-подбор поездки

Самая сложная часть системы. Запрос пользователя не выполняется синхронно: клиент создаёт заявку, получает её `id` и подписывается на поток прогресса.

### 3.1. Семь стадий воркфлоу

Список стадий и их названия хранятся в БД (`app_settings`, ключ `recommendation_stages`), поэтому текст прогресса можно менять без релиза. Сервис требует минимум 7 стадий, иначе отказывается стартовать.

| № | Стадия | Что происходит |
|---|---|---|
| 0 | Приём заявки | Guardrails, проверка квот, запись `recommendation_requests` |
| 1 | AI-классификация | DeepSeek разбирает свободный текст в структуру: темы, ограничения, настроение |
| 2 | План поиска | DeepSeek предлагает города-кандидаты с учётом бейджей и предпочтений |
| 3 | Поиск транспорта | Tutu MCP: реальные предложения по авиа, ЖД, автобусам, электричкам |
| 4 | Скоринг на бэкенде | Детерминированная формула, без AI (см. §3.2) |
| 5 | Объяснения | DeepSeek пишет «почему этот город» и «почему сейчас» для топ-вариантов |
| 6 | Финализация | Сохранение `recommendation_options`, статус `completed` или `partial` |

Ключевое решение: **ранжирует не AI, а бэкенд.** DeepSeek генерирует кандидатов и текст, но итоговый порядок считает прозрачная формула. Это делает результат воспроизводимым, отлаживаемым и защищённым от того, что модель «придумает» цену или рейс.

### 3.2. Формула скоринга

`backend/services/scoring/service.go`. Веса лежат в `app_settings.scoring_weights` и меняются без деплоя.

```
score = ( match_предпочтений × w_preference
        + выгода_по_цене     × w_price
        + прирост_карты      × w_map_gain
        +                      w_novelty
        + сезонность         × w_seasonal
        + привязка_к_событию × w_event
        + коммерч_приоритет  × w_commercial
        - утомительность     × w_friction ) / Σ_положительных_весов × 100
```

Предложение вообще не попадает в скоринг, если оно дороже бюджета, в другой валюте, с нулевой длительностью, без снапшота оффера или с транспортом, которого пользователь не выбрал. Все компоненты нормированы в `[0, 1]`, итог — целое число 0–100. При равном score выше идёт более дешёвый вариант.

### 3.3. Поток прогресса (SSE)

```mermaid
sequenceDiagram
    participant F as Frontend
    participant A as API
    participant AI as DeepSeek
    participant T as Tutu MCP
    participant DB as PostgreSQL

    F->>A: POST /api/v1/recommendations
    A->>A: guardrails + квоты (Redis)
    A->>DB: INSERT recommendation_requests (pending)
    A-->>F: 202 { id }
    A->>A: запуск воркфлоу в горутине
    F->>A: GET /api/v1/recommendations/:id/events (SSE)
    loop стадии 1..6
        A->>AI: анализ / план / объяснение
        A->>T: поиск транспорта
        A->>DB: SetStage(code)
        A->>DB: опрос статуса и стадии
        A-->>F: event recommendation
    end
    A->>DB: INSERT recommendation_options, status completed
    A-->>F: event recommendation (completed)
    F->>A: GET /api/v1/recommendations/:id
    A-->>F: варианты с ценами и объяснениями
```

Воркфлоу продолжает работу в `context.WithoutCancel`, то есть закрытие вкладки его не убивает — результат всё равно окажется в базе.

Сам SSE устроен просто: хендлер в цикле опрашивает заявку в БД и отправляет событие `recommendation` с полным состоянием только когда меняется пара `status|stage`. Ошибка приходит событием `error`. Это не полноценный pub/sub, зато не требует шины сообщений и корректно работает при нескольких репликах API — любая реплика может отдавать поток по любой заявке, потому что источник истины один, база.

Чтобы поток не залипал в прокси, хендлер выставляет `X-Accel-Buffering: no`, а в nginx для `/api/` отключена буферизация. На клиенте `useRecommendation.ts` подстраховывается: если `EventSource` не работает, он переходит на обычный polling.

### 3.4. Границы для AI (guardrails)

`backend/internal/security/guardrails.go` — единственный вход для пользовательского текста в промпт. Проверки идут до любого вызова модели.

Сначала нормализация: переводы строк в пробелы, удаление невидимых управляющих символов, форматирующих символов и комбинирующих диакритиков, сжатие пробелов. Это снимает классические обходы фильтров через zero-width и «залговский» текст.

Затем отказ по регулярным выражениям:

| Код ошибки | Что отсекает |
|---|---|
| `INVALID_INPUT` | Длина больше 1500 символов, 40+ одинаковых символов подряд, пустой город или даты, состав группы больше 8, бюджет вне 1 000–2 000 000, пустой или слишком длинный список транспорта |
| `PROMPT_CONTAINS_PII` | Email, российские телефоны, номера карт, паспорт/СНИЛС/ИНН, пары вида `api_key: ...` |
| `PROMPT_INJECTION_DETECTED` | «игнорируй инструкции», «покажи системный промпт», «act as», «jailbreak», псевдотеги `<system>`, `<|...|>`, base64 |
| `POLITICAL_REQUEST_BLOCKED` | Просьбы оценить политику, партии, выборы |
| `PROMPT_NOT_TRAVEL_RELATED` | «напиши код», «реши уравнение», «какой курс валют» |

---

## 4. AI-афиша (event discovery)

События не заводятся вручную и не берутся из демо-каталога — миграция `007_ai_only_events.sql` его удалила. Афиша целиком собирается веб-поиском DeepSeek.

Как это работает:

- Клиент `infra/clients/deepseek/websearch.go` вызывает `POST /responses` с инструментом `web_search` и моделью `DEEPSEEK_SEARCH_MODEL`. Ответ ожидается строгим JSON, дальше нормализуется и валидируется.
- Каждое найденное событие сохраняется с `source_url`, `trust_status = 'ai_web_search'` и JSONB-полем `provenance`. То есть источник любого факта в афише всегда прослеживается.
- Результаты запросов трекаются в таблице `event_discovery_runs` (ключ — `scope` + `scope_key`, статусы `running` / `ready` / `failed`). Это одновременно кеш и защита от параллельного дублирования работы.
- Свежесть управляется `EVENT_DISCOVERY_TTL`. После неудачи по конкретному городу повторная попытка не раньше `EVENT_DISCOVERY_RETRY_BACKOFF`, чтобы не жечь квоту на проблемных запросах.
- Две фоновые горутины держат данные тёплыми: федеральная подборка обновляется каждые 30 минут, прогрев городов идёт каждые 10 минут с параллелизмом `EVENT_DISCOVERY_CONCURRENCY`. Обе не запускаются при `EVENT_DISCOVERY_ENABLED=false`.

---

## 5. Интеграция с Tutu MCP

`infra/tutumcp` — клиент Model Context Protocol поверх Streamable HTTP и JSON-RPC 2.0, версия протокола `2025-11-25`, сессия держится в заголовке `Mcp-Session-Id`.

Доступные инструменты: `search_avia`, `search_rail`, `search_bus`, `search_etrain`, `search_multitransport`, `search_hotels`, `get_offer_details`, `get_rail_seatmap`, `create_checkout_link`, `fetch_resource`.

Устойчивость к сбоям: 3 попытки, экспоненциальная задержка от 300 мс до 5 с, джиттер 20%, учёт `Retry-After`. Общий таймаут вызова 45 с, ответ ограничен 2 МБ. Повторы применяются к идемпотентным вызовам; изменяющие операции (создание заказа, оформление) не переигрываются вслепую.

Оформление — самое чувствительное место, оно в `infra/checkout/tutumcp/creator.go`. Ссылка принимается только если схема ровно `https` и хост совпадает с `CHECKOUT_ALLOWED_HOSTS` (точное совпадение либо суффикс для `*.domain`). Смысл в том, что ссылку возвращает внешняя система, и без проверки она стала бы вектором для редиректа пользователя на чужой домен на шаге оплаты.

---

## 6. Данные

### 6.1. Домены и таблицы

Схема — 37 таблиц (36 из миграций плюс `schema_migrations`, которую создаёт раннер). Расширения: `citext` (регистронезависимый email), `pgcrypto`, `postgis` (геометрия и центроиды территорий).

| Домен | Таблицы |
|---|---|
| Пользователи и доступ | `users`, `sessions`, `user_preferences` |
| География и контент | `territories`, `city_content`, `badge_catalog`, `travel_collections`, `travel_collection_territories` |
| События | `events`, `event_sources`, `event_subscriptions`, `event_discovery_runs` |
| Рекомендации | `recommendation_requests`, `recommendation_options` |
| Поездки и посещения | `trips`, `user_visits`, `demo_travel_history` |
| Награды | `reward_ledger`, `achievements`, `user_achievements`, `user_promo_codes` |
| Сезоны и сообщество | `seasons`, `season_score_ledger`, `leaderboard_snapshots`, `guilds`, `guild_memberships`, `guild_challenges`, `guild_feed`, `travel_cohorts`, `travel_cohort_memberships` |
| Настройки и AI | `app_settings`, `ai_system_prompts` |
| Админ и инфраструктура | `demo_scenarios`, `admin_simulation_actions`, `admin_audit_log`, `outbox_events`, `schema_migrations` |

Центральная сущность — `territories`. Это самореферентное дерево (`country` → `region` → `city` → `settlement`) с PostGIS-полями `geometry` и `centroid GEOGRAPHY(POINT, 4326)`, массивами `tags` и `badges` (GIN-индекс), а также полями, влияющими на скоринг и награды: `rarity`, `reward`, `seasonal_fit`, `commercial_priority`, `promo_percent`.

### 6.2. Решения в схеме, которые стоит знать

**Идемпотентность начислений.** В `reward_ledger` два уникальных индекса на пользователя: по `idempotency_key` и по паре `(reference_type, reference_id)`. Повторная обработка одного события не начислит награду дважды. Миграция `004` специально заменила глобально уникальные ключи на уникальные в рамках пользователя — с глобальными два разных пользователя не могли получить награду за одно и то же событие.

**Никаких enum'ов PostgreSQL.** Все домены значений — это `TEXT` с `CHECK`. Расширить набор статусов можно обычным `ALTER ... DROP/ADD CONSTRAINT` вместо `ALTER TYPE`, что проще откатывать.

**Никаких триггеров и хранимой логики.** Все инварианты либо в `CHECK`/`UNIQUE`, либо в Go. Поведение системы читается из кода, а не из скрытых побочных эффектов в БД.

**Транзакционный outbox.** `outbox_events` с partial-индексом по необработанным записям — заготовка для надёжной публикации событий во внешние системы без потерь при падении процесса.

**Частичные индексы вместо полных.** Например `sessions_expiry_idx` строится только по `revoked_at IS NULL`, `territories_active_name_idx` — только по активным. Индексы остаются компактными на растущих таблицах.

**Приватность в схеме.** У `users` есть `travel_visibility` (`private` / `aggregate`), у `travel_cohort_memberships` — своя `visibility`. Порог агрегации лежит в `app_settings.privacy_threshold`: пока в когорте меньше нужного числа людей, состав не показывается.

### 6.3. Транзакции

`postgres.TransactionManager` даёт два режима. Обычный `WithinTransaction` для большинства операций и `WithinSerializableTransaction` для тех, где важна защита от гонок — с автоматическим повтором до 3 раз при serialization failure и линейным бэкоффом 20 мс × номер попытки. Текущая транзакция передаётся через контекст (`ExecutorFromContext`), поэтому репозитории работают одинаково и внутри транзакции, и вне неё.

### 6.4. Миграции

Файлы вшиты в бинарь через `//go:embed migrations/*.sql`, применяются при старте API. Версия — полное имя файла, применённые записываются в `schema_migrations`. Порядок — лексикографическая сортировка имён, каждая миграция идёт в своей транзакции под общим advisory lock.

| Файл | Содержание |
|---|---|
| `001_initial.sql` | Расширения, все базовые таблицы, 31 curated-город, достижения, сезон, гильдии, базовые `app_settings` |
| `002_recommendation_scoring.sql` | Поля и настройки скоринга, `request_kind` для заявок |
| `003_ai_system_prompts.sql` | Таблица `ai_system_prompts` и четыре промпта v2.0.0 |
| `004_runtime_settings_and_indexes.sql` | Правки идемпотентности, ~15 индексов под реальные запросы, настройки лиг |
| `005_ai_event_discovery.sql` | `source_url` и `popularity_rank` у событий, статус `ai_web_search`, `event_discovery_runs` |
| `006_city_badges.sql` | `badge_catalog`, `territories.badges` + GIN-индекс, промпт v2.1.0 |
| `006_city_promo_codes.sql` | `territories.promo_percent`, `user_promo_codes`, бэкфилл по прошлым посещениям |
| `007_ai_only_events.sql` | Удаление демо-событий и демо-источников, афиша только из веб-поиска |

> Два файла с префиксом `006` — не ошибка исполнения (они независимы: один про бейджи, другой про промокоды, порядок задаётся алфавитом суффикса), но это ловушка на будущее. Новые миграции нумеруйте уникально, иначе порядок начнёт зависеть от букв в названии.

### 6.5. Фикстуры

Загружаются только при `DEMO_MODE=true`, тоже embedded:

- `badge_vocabulary.tsv` — справочник бейджей, upsert в `badge_catalog`, исчезнувшие метки помечаются `active = FALSE`.
- `ru_cities.tsv` — города из GeoNames с координатами, попадают в `territories` с `source_code = 'geonames'`.
- `content_seed.sql` — карточки контента по городам, 30 достижений, 8 коллекций, гильдии и челленджи для активных городов, снапшоты лидербордов, значения `app_settings`.

---

## 7. HTTP API

Базовый путь `/api/v1`. Полная спецификация — [backend/openapi.json](backend/openapi.json).

### 7.1. Формат ошибок

Один формат на все ошибки, формируется в `internal/errors/http/respond.go`:

```json
{
  "error": {
    "code": "PROMPT_CONTAINS_PII",
    "message": "Уберите из пожелания контакты, реквизиты и другие личные данные",
    "request_id": "b4f1c2...",
    "details": []
  }
}
```

`code` — машиночитаемый, `message` — готовый к показу русский текст, `request_id` совпадает с полем в логах, поэтому по нему можно найти конкретный запрос. Ошибки 5xx логируются с уровнем `error`, внутренние детали клиенту не отдаются.

### 7.2. Маршруты

Публичные — доступны без сессии:

| Метод | Путь |
|---|---|
| GET | `/healthz`, `/readyz` |
| GET | `/api/v1/config` |
| POST | `/api/v1/auth/register`, `/api/v1/auth/login` |

Остальные требуют сессионную cookie; для методов, меняющих состояние, дополнительно нужен CSRF-заголовок:

| Группа | Маршруты |
|---|---|
| Аутентификация | `GET /auth/me`, `POST /auth/logout` |
| Профиль | `GET /profile`, `PUT /profile/preferences`, `POST /profile/onboarding/complete`, `PUT /profile/home-city`, `PUT /profile/travel-visibility` |
| Мир | `GET /world/bootstrap`, `GET /world/progress`, `GET /territories`, `GET /territories/:id`, `POST /integrations/tutu/demo-sync` |
| События | `GET /territories/:id/events`, `POST /territories/:id/events/refresh`, `GET /events/popular`, `GET /events/:id`, `POST /events/:id/plan-trip` |
| Рекомендации | `POST /recommendations`, `GET /recommendations/personal`, `POST /recommendations/personal/refresh`, `GET /recommendations/:id`, `GET /recommendations/:id/events` (SSE), `POST /recommendations/:id/select` |
| Поездки | `GET /trips`, `POST /trips/:id/checkout`, `POST /trips/:id/simulate-arrival` |
| Награды | `GET /rewards/ledger`, `GET /rewards/promo-codes`, `GET /achievements` |
| Сообщество | `GET /season/current`, `GET /leaderboard`, `GET /guild`, `POST /guild/join`, `POST /guild/leave`, `GET /travel-cohorts/:territoryId` |
| Админ-симулятор | `GET /admin/overview`, `GET /admin/users`, `GET /admin/scenarios`, `POST /admin/simulations`, `GET /admin/simulations/:id`, `GET /admin/audit-log` |

Админские маршруты дополнительно проверяются в сервисном слое: нужны одновременно `ADMIN_SIMULATOR_ENABLED=true` и роль `demo_admin`. Проверка сознательно сделана в сервисе, а не только в middleware, — так её нельзя обойти, добавив маршрут в неправильную группу.

---

## 8. Frontend

### 8.1. Структура

```
frontend/src/
├── main.tsx        # StrictMode → QueryClientProvider → BrowserRouter → App
├── App.tsx         # гейты: настройки → сессия → онбординг → приложение
├── api.ts          # единственный HTTP-клиент, APIError
├── state.ts        # единственный zustand-стор
├── types.ts        # доменные типы
├── styles.css      # глобальные стили и CSS-переменные (дизайн-токены)
├── components/     # Globe (Three.js), AppNav, CityPhoto, States, Logo
├── features/       # world, trips, community, profile, recommendations,
│                   # auth, onboarding, admin, shared
└── shared/         # format, icons, useCityPhoto, useMediaQuery,
                    # useRecommendation, useSheet
```

### 8.2. Как разделены состояния

Разделение простое и последовательно соблюдается: **всё, что приходит с сервера, живёт в React Query; всё, что относится к UI, — в zustand.**

React Query (`retry: 1`, `staleTime: 30s`) отвечает за кеш, фоновое обновление и инвалидацию после мутаций. Ключи плоские и предсказуемые: `['world-bootstrap']`, `['events', cityID]`, `['leaderboard', scope, period]`, `['trips']`.

Единственный zustand-стор `useApp` держит только UI: активную вкладку, выбранный на глобусе город, цель планирования, открытость панели поиска, текст тоста. Стор небольшой, поэтому не превращается в теневое хранилище серверных данных.

Роутинг гибридный: react-router обслуживает `/` и `/admin` (там нужен реальный редирект по роли), а переключение вкладок «Мир / Поездки / Сезон / Профиль» идёт через `view` в сторе, без изменения URL.

### 8.3. Клиент API

`src/api.ts` — единственное место, где вызывается `fetch` к бэкенду. Он всегда ставит `credentials: 'include'`, для небезопасных методов добавляет `X-CSRF-Token` из cookie `ow_csrf`, разбирает `{ error: { code, message } }` в `APIError` и возвращает `undefined` на `204`. Такая централизация означает, что CSRF и авторизацию невозможно забыть в отдельном компоненте.

### 8.4. 3D-глобус

`components/Globe.tsx` — примерно 800 строк на чистом Three.js: сфера Земли, маркеры городов, управление камерой, тултипы. Компонент грузится через `React.lazy`, так что Three.js уезжает в отдельный чанк и не попадает в начальную загрузку.

Текстуры лежат в `public/textures/`: сначала подгружается `earth-color-2k.jpg`, затем в фоне подменяется на 4K-версию. Нормали и roughness — отдельными картами. При `prefers-reduced-motion` (через `useMediaQuery`) анимации отключаются.

### 8.5. PWA

`vite-plugin-pwa` с `registerType: 'autoUpdate'`. Service worker делает precache билд-ассетов, `cleanupOutdatedCaches()` и navigation fallback на `index.html`. Manifest — standalone, `lang: ru`, тема `#0d0b68`.

Отдельных runtime-стратегий кеширования API не настроено: офлайн работает как «оболочка приложения открывается», но не как «данные доступны без сети». В `App.tsx` есть баннер офлайн-состояния.

### 8.6. Отдача статики

nginx (`frontend/nginx.conf`) держит собранный `dist`, проксирует `/api/` на `api:8080` с отключённой буферизацией (нужно для SSE) и таймаутом 120 с, отдаёт SPA-фолбэк `try_files $uri /index.html`. Кеширование по типам ресурсов:

| Путь | Заголовки |
|---|---|
| `/assets/` | `expires 1y`, `Cache-Control: public, immutable` |
| `/fonts/` | `expires 1y`, `public, immutable` |
| `/textures/` | `expires 30d`, `public` |
| `/index.html`, `/sw.js` | `no-cache` |

Хешированные имена ассетов позволяют кешировать их навсегда, а `no-cache` на `index.html` и `sw.js` гарантирует, что новая версия подхватится сразу.

---

## 9. Безопасность

### 9.1. Пароли

Argon2id (`golang.org/x/crypto/argon2`) с параметрами `t=3, m=64 MiB, p=2`, соль 16 байт, ключ 32 байта, формат хранения `$argon2id$v=19$...`. Выбран memory-hard алгоритм, а не bcrypt — он существенно дороже для перебора на GPU.

Сравнение — в постоянном времени. Если email не найден, код всё равно считает фиктивный хеш, чтобы по времени ответа нельзя было определить, зарегистрирован ли адрес.

### 9.2. Сессии

Сессии серверные, в таблице `sessions`. Клиенту уходит cookie `ow_session` с 32 случайными байтами в hex; в базе лежит только SHA-256 от токена. Утечка дампа базы не даёт возможности войти под пользователем.

Флаги cookie: `HttpOnly`, `SameSite=Lax`, `Path=/`, `MaxAge` из `SESSION_LIFETIME`, а в production ещё и `Secure`. `HttpOnly` закрывает доступ из JS, поэтому XSS не приводит напрямую к угону сессии. Отзыв — через `revoked_at`, что позволяет разлогинить пользователя мгновенно, не дожидаясь истечения срока.

### 9.3. CSRF

Схема double-submit cookie. Наряду с сессионной ставится cookie `ow_csrf` (специально **не** `HttpOnly`, чтобы клиент мог её прочитать), а клиент дублирует значение в заголовке `X-CSRF-Token`. Middleware сравнивает их в постоянном времени. Проверка обязательна для `POST`, `PUT`, `PATCH`, `DELETE` и пропускается для `GET`, `HEAD`, `OPTIONS`.

Работает потому, что сторонний сайт может заставить браузер отправить cookie, но не может прочитать её значение и подставить в заголовок.

### 9.4. Заголовки и транспорт

Глобальный middleware выставляет CSP, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Cross-Origin-Opener-Policy`, `Cross-Origin-Resource-Policy`, `Permissions-Policy`, а в production — HSTS.

CORS ограничен списком `ALLOWED_ORIGINS` с поддержкой credentials, из заголовков разрешены только `Content-Type`, `X-CSRF-Token`, `Idempotency-Key`.

Доверенные прокси: если `TRUSTED_PROXIES` задан, Fiber включает `EnableTrustedProxyCheck` и читает `X-Forwarded-For`. Это принципиально для честного rate limiting — без проверки любой клиент подделал бы IP заголовком и обошёл лимиты.

### 9.5. Ограничение частоты

Два уровня:

| Уровень | Реализация | Ключ | Лимит |
|---|---|---|---|
| Общий HTTP | Fiber limiter, in-memory | IP | `REQUESTS_PER_MINUTE`, `/healthz` и `/readyz` исключены |
| Доменные квоты | Redis, sliding window | scope + identity | см. ниже |

Доменные лимиты в Redis (ключ вида `ratelimit:{scope}:{identity}:{bucket}`):

| Scope | Идентификатор | Лимит |
|---|---|---|
| `login` | email | `LOGIN_ATTEMPTS_PER_HOUR` / час |
| `login` | IP | `LOGIN_ATTEMPTS_PER_HOUR` / час |
| `register` | IP | `LOGIN_ATTEMPTS_PER_HOUR` / час |
| `recommendations_hour` | user ID | `RECOMMENDATIONS_PER_HOUR` |
| `recommendations_day` | user ID | `RECOMMENDATIONS_PER_DAY` |

Логин лимитируется одновременно по email и по IP: первое защищает конкретный аккаунт от брутфорса, второе — от распыления попыток по множеству аккаунтов из одной точки. Квоты на рекомендации существуют не ради безопасности, а ради стоимости: каждый запрос — это несколько платных вызовов DeepSeek.

Лимитер работает по принципу fail-open: если Redis недоступен, запрос пропускается с warning в логе. Доступность приоритетнее строгости лимита; при этом общий HTTP-лимитер продолжает работать, потому что он in-memory.

### 9.6. Внешние данные

Три границы, где не доверяем внешнему миру:

1. **Пользовательский текст → AI.** Guardrails из §3.4 до вызова модели.
2. **Ответ AI → база.** Строгий JSON-парсинг и нормализация, сохранение `source_url` и `provenance`, `trust_status` вместо признания факта проверенным.
3. **Ссылка Tutu → браузер.** Только `https` и только хосты из `CHECKOUT_ALLOWED_HOSTS`.

### 9.7. Проверки production-режима

При `APP_ENV=production` процесс не стартует, если: `SESSION_SECRET` короче 32 символов, `PUBLIC_BASE_URL` не `https://`, `DEEPSEEK_API_KEY` пуст или равен `replace-me`, `DEMO_MODE=true`, `ADMIN_SIMULATOR_ENABLED=true`.

Это сделано намеренно жёстко: демо-аккаунты с известными паролями и админ-симулятор, умеющий менять состояние чужих аккаунтов, в продакшене недопустимы. Лучше не запуститься, чем запуститься с открытым бэкдором.

### 9.8. Прочее

Образ бэкенда — `alpine:3.21` с непривилегированным пользователем `app` (uid 10001), бинарь собран с `CGO_ENABLED=0` и `-trimpath`, без исходников и тулчейна.

`recover` middleware не даёт панике в одном запросе положить весь процесс. Каждому запросу присваивается `request-id`, который попадает и в лог, и в ответ об ошибке.

---

## 10. Масштабируемость и надёжность

### 10.1. Что позволяет масштабировать API горизонтально

API stateless по состоянию пользователя: сессии в PostgreSQL, счётчики в Redis, в памяти процесса ничего важного не хранится. Реплики можно добавлять за балансировщиком без sticky-сессий.

Что уже не мешает такому масштабированию:

- Миграции идут под advisory lock, конкурентный старт реплик безопасен.
- Начисления идемпотентны по ключам в `reward_ledger`, повторная обработка не удваивает награды.
- `event_discovery_runs` служит распределённой отметкой «этот город уже обрабатывается».

Что потребует внимания при переходе на несколько реплик:

- Общий HTTP-лимитер in-memory, значит фактический лимит умножается на число реплик. Для точности его нужно перевести на Redis.
- Фоновые задачи discovery запускаются в каждом экземпляре. `event_discovery_runs` спасает от дублирования работы, но честнее вынести их в отдельный воркер или добавить лидер-элекшн.
- SSE-соединения долгоживущие, балансировщику нужны увеличенные таймауты и отключённая буферизация ответа.

### 10.2. Данные

PostgreSQL — единственная точка, которую нельзя просто размножить. Путь роста обычный: вертикальное масштабирование, затем read-реплики для тяжёлого чтения (лидерборды, каталог территорий, афиша), затем партиционирование append-only таблиц (`reward_ledger`, `season_score_ledger`, `admin_audit_log`) по времени.

Запросы уже подготовлены к росту: миграция `004` добавила индексы под конкретные паттерны чтения, тяжёлые фильтры используют частичные индексы, поиск по бейджам — GIN. Лидерборды не считаются на лету, а лежат готовыми в `leaderboard_snapshots` — расчёт можно вынести в фон и не давать пользователям генерировать тяжёлые агрегации.

Пул соединений ограничен `DATABASE_MAX_CONNECTIONS` (по умолчанию 20 на процесс). При росте числа реплик суммарное число коннектов надо считать заранее и ставить PgBouncer, а не увеличивать пул.

### 10.3. Внешние зависимости

DeepSeek и Tutu MCP — самое медленное и самое ненадёжное в системе, поэтому вокруг них построена вся защита:

| Механизм | DeepSeek | Tutu MCP |
|---|---|---|
| Таймаут | 25 с (chat), `EVENT_DISCOVERY_TIMEOUT` (поиск) | 45 с |
| Повторы | 3 попытки, бэкофф 500 мс ×2, максимум 5 с | 3 попытки, 300 мс → 5 с, джиттер 20% |
| Условия повтора | 429, 408, 5xx | сетевые и временные ошибки, только идемпотентные вызовы |
| Ограничение размера | — | 2 МБ на ответ |
| Кеш | `event_discovery_runs` + TTL | — |
| Выключатель | `EVENT_DISCOVERY_ENABLED=false` | — |

Ограничение размера ответа существенно: без него внешний сервис мог бы исчерпать память процесса одним большим ответом.

Деградация сделана частичной. Недоступность DeepSeek ломает подбор поездок и афишу, но глобус, прогресс, поездки, награды и сообщество продолжают работать — это разные обработчики с разными зависимостями.

### 10.4. Клиентская нагрузка

Значительная часть работы снята с бэкенда на уровне отдачи статики и кеширования. Ассеты с хешами в имени кешируются на год, service worker снимает повторные загрузки оболочки, React Query с `staleTime: 30s` гасит дубли запросов при навигации, Three.js подгружается лениво и только тем, кто дошёл до глобуса. Фото городов берутся напрямую из Wikipedia, минуя наш бэкенд.

### 10.5. Чего не хватает для продакшена

Честный список пробелов:

- **Метрик нет.** Есть структурированные логи, но нет Prometheus-эндпоинта и трейсинга. Латентность внешних вызовов и длительность стадий воркфлоу — первые кандидаты на инструментирование.
- **Circuit breaker не реализован.** Повторы и таймауты есть, но при длительной деградации DeepSeek запросы продолжат идти и упираться в таймаут.
- **Outbox-воркера нет.** Таблица `outbox_events` создана и проиндексирована, но обработчик не написан.
- **Общий rate limiter не распределённый** (см. §10.1).
- **Runtime-кеша API в service worker нет** — офлайн ограничен оболочкой приложения.

---

## 11. Наблюдаемость и качество кода

### Логи

`slog` в JSON, уровень из `LOG_LEVEL`. Каждый HTTP-запрос логируется с `request_id`, методом, путём, статусом и длительностью. Ошибки 5xx пишутся отдельно с `request_id`, совпадающим с тем, что вернулось клиенту, — это основной инструмент разбора инцидентов.

### Проверки состояния

`GET /healthz` — liveness, процесс жив. `GET /readyz` — readiness, зависимости доступны. Оба исключены из rate limiting, чтобы пробы не выедали лимит. В Compose у `postgres` и `redis` есть healthcheck'и, `api` ждёт их готовности, `web` ждёт `api`.

### Линтеры

`.golangci.yml` включает 47 линтеров при выключенных по умолчанию всех. Помимо стандартных: `bodyclose` и `sqlclosecheck` (утечки ресурсов), `contextcheck` и `noctx` (проброс контекста), `errorlint` и `errname` (работа с ошибками), `gosec` (безопасность), `gochecknoglobals` и `gochecknoinits` (запрет скрытого глобального состояния), `mnd` (магические числа), `cyclop` с порогом 20 и `nestif` (сложность). Форматирование — `gofumpt` и `gci`.

Frontend: TypeScript в strict-режиме, ESLint 9 flat config с `typescript-eslint` и `react-hooks`, неиспользуемые переменные — ошибка.

### Тесты

Покрытие сосредоточено на логике, которую нельзя проверить глазами:

| Тест | Что проверяет |
|---|---|
| `services/scoring/service_test.go` | Формула ранжирования и фильтрация непригодных офферов |
| `services/recommendation/service_test.go` | Воркфлоу на заглушках, без БД и сети |
| `internal/security/guardrails_test.go` | PII, инъекции промпта, нормализация |
| `infra/clients/deepseek/*_test.go` | Разбор ответов модели |
| `infra/transport/tutumcp/provider_test.go` | Маппинг MCP-ответов в доменные офферы |
| `frontend/src/shared/format.test.ts` | Форматирование дат, денег, лейблов |

Интеграционных и e2e-тестов HTTP-слоя нет — это осознанный пробел, а не упущение в документации.

---

## 12. Конфигурация

Все переменные читаются один раз при старте в `backend/resources/env.go`. Логика простая: строки и URL валидируются строго (при ошибке процесс падает), числа и длительности при некорректном значении молча откатываются к дефолту. Шаблон — [.env.example](.env.example).

В колонке «Дефолт» ниже указано значение, зашитое в коде, то есть то, что применится, если переменная не задана вообще. `.env.example` местами задаёт другие числа — например `EVENT_DISCOVERY_CITY_LIMIT=8` против дефолта `10` и `EVENT_DISCOVERY_CONCURRENCY=6` против `3`. Это не рассинхронизация, а настройка под демо: файл затюнен под быстрый прогрев афиши.

### Общие

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `APP_ENV` | `local` | Значение `production` включает дополнительные проверки (см. §9.7) |
| `APP_PORT` | `8080` | Порт HTTP-сервера |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SHUTDOWN_PERIOD` | `15s` | Таймаут graceful shutdown, допустимо до `1m` |
| `PUBLIC_BASE_URL` | `http://localhost:5173` | Публичный адрес фронта; в production обязателен `https://` |

### Хранилища

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `DATABASE_URL` | — | **Обязательна.** DSN PostgreSQL |
| `DATABASE_MAX_CONNECTIONS` | `20` | Размер пула pgxpool |
| `REDIS_ADDR` | `localhost:6379` | |
| `REDIS_PASSWORD` | пусто | |

### Сеть, CORS, прокси

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `ALLOWED_ORIGINS` | значение `PUBLIC_BASE_URL` | Список через запятую |
| `TRUSTED_PROXIES` | пусто | Если задан — Fiber включает проверку доверенных прокси и читает `X-Forwarded-For` |

### Сессии и лимиты

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `SESSION_SECRET` | пусто | В production — минимум 32 символа |
| `SESSION_LIFETIME` | `168h` | Допустимо от `1h` до 90 дней |
| `REQUESTS_PER_MINUTE` | `120` | Общий лимит на IP |
| `LOGIN_ATTEMPTS_PER_HOUR` | `10` | Логин и регистрация |
| `RECOMMENDATIONS_PER_HOUR` | `20` | Не больше дневного лимита |
| `RECOMMENDATIONS_PER_DAY` | `100` | |

### DeepSeek

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `DEEPSEEK_API_KEY` | пусто | В production обязателен и не может быть `replace-me` |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` | |
| `DEEPSEEK_MODEL` | `deepseek-chat` | Для анализа запроса, плана поиска, объяснений |
| `DEEPSEEK_SEARCH_MODEL` | `deepseek-v4-flash` | Для веб-поиска событий |
| `DEEPSEEK_SYSTEM_PROMPT_*` | пусто | Четыре переменные (`REQUEST_ANALYSIS`, `TRAVEL_SEARCH`, `EXPLANATION`, `EVENT_ENRICHMENT`). Если пусто — промпт берётся из таблицы `ai_system_prompts` |

### Tutu MCP

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `TUTU_MCP_URL` | `https://mcp.tutu.ru/mcp` | |
| `CHECKOUT_ALLOWED_HOSTS` | `tutu.ru,*.tutu.ru` | Whitelist хостов для ссылок оформления, не может быть пустым |

### Демо-режим

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `DEMO_MODE` | `true` вне production | В production обязан быть `false` |
| `ADMIN_SIMULATOR_ENABLED` | `true` вне production | В production обязан быть `false` |
| `DEMO_USER_EMAIL` / `_PASSWORD` / `_NAME` | пусто | Обязательны при `DEMO_MODE=true` |
| `DEMO_ADMIN_EMAIL` / `_PASSWORD` / `_NAME` | пусто | Обязательны при `DEMO_MODE=true` |

### AI-афиша (event discovery)

| Переменная | Дефолт | Комментарий |
|---|---|---|
| `EVENT_DISCOVERY_ENABLED` | `true` | Выключатель фичи целиком, в том числе фоновых задач |
| `EVENT_DISCOVERY_TTL` | `24h` | Сколько живёт результат поиска, минимум `1h` |
| `EVENT_DISCOVERY_TIMEOUT` | `5m` | Таймаут одного запроса к веб-поиску, минимум `1m` |
| `EVENT_DISCOVERY_RETRY_BACKOFF` | `1h` | Пауза после неудачи по конкретному городу |
| `EVENT_DISCOVERY_CITY_LIMIT` | `10` | Событий на город (1–40) |
| `EVENT_DISCOVERY_POPULAR_LIMIT` | `12` | Размер федеральной подборки (1–40) |
| `EVENT_DISCOVERY_CITY_POOL` | `120` | Сколько городов участвует в ротации |
| `EVENT_DISCOVERY_PREWARM_CITIES` | `250` | Сколько городов прогревать фоном |
| `EVENT_DISCOVERY_WINDOW_DAYS` | `60` | Горизонт поиска событий |
| `EVENT_DISCOVERY_CONCURRENCY` | `3` | Параллельных запросов к DeepSeek |

> `APP_HOST` присутствует в `.env.example`, но бэкендом не читается — сервер всегда слушает на всех интерфейсах.

---

## 13. Структура репозитория

```
tutu_h/
├── docker-compose.yml         # postgres, redis, api, web
├── Makefile                   # up / down / restart / logs / status / build
├── .env.example               # шаблон конфигурации
├── backend/
│   ├── cmd/api/               # точка входа
│   ├── resources/             # env, логгер, пул БД, Redis
│   ├── internal/
│   │   ├── api/               # сборка приложения и DI
│   │   ├── router/            # маршруты
│   │   ├── middlewares/       # CORS, auth, CSRF, лимиты, заголовки, логи
│   │   ├── handlers/          # HTTP по доменам
│   │   ├── models/            # domain и dto
│   │   ├── errors/            # типизированные ошибки и единый respond
│   │   └── security/          # Argon2id, guardrails
│   ├── services/              # бизнес-логика: auth, world, profile, events,
│   │                          # recommendation, scoring, trips, rewards,
│   │                          # community, adminsim
│   ├── infra/
│   │   ├── storage/           # репозитории, миграции, фикстуры, транзакции
│   │   ├── clients/deepseek/  # chat и web search
│   │   ├── tutumcp/           # MCP-клиент (JSON-RPC, streamable HTTP)
│   │   ├── transport/tutumcp/ # маппинг офферов в домен
│   │   ├── checkout/tutumcp/  # ссылки оформления с валидацией хоста
│   │   └── ratelimit/         # лимитер на Redis
│   ├── openapi.json           # спецификация API
│   └── Dockerfile             # golang:1.25-alpine → alpine:3.21
├── frontend/
│   ├── src/                   # см. §8.1
│   ├── public/                # текстуры, шрифты, иконки, бренд
│   ├── vite.config.ts         # React, PWA, dev-прокси
│   ├── nginx.conf             # прокси /api, кеш, SPA-фолбэк
│   └── Dockerfile             # node:24-alpine → nginx:1.27-alpine
└── documents/                 # ТЗ MVP, концепция, чек-лист приёмки,
                               # гайдлайн API, техдок Tutu MCP, дизайн-материалы
```

Подробные продуктовые документы лежат в `documents/docs/`: `open-world-mvp-spec.md` (полное ТЗ), `project-concept.md` (концепция), `functional-acceptance-checklist.md` (приёмка P0/P1/P2), `api-gateway-architecture.md` (соглашения по API), `tutu-mcp.md` (интеграция).

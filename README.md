# «Открывай»

Mobile-first PWA, где путешествия возвращают цвет персональному миру. Репозиторий содержит Go API, React PWA, demo fixtures, OpenAPI и эксплуатационную документацию.

## Быстрый запуск

```bash
cp .env.example .env
docker compose up --build
```

Откройте `http://localhost:5173` и войдите под demo-аккаунтом из `DEMO_USER_EMAIL` / `DEMO_USER_PASSWORD`, админ-сценарии — под `DEMO_ADMIN_EMAIL` / `DEMO_ADMIN_PASSWORD`.

Для локальной разработки без Docker:

```bash
cd backend && go run ./cmd/api
cd frontend && npm install && npm run dev
```

API по умолчанию работает на `http://localhost:8080`, frontend проксирует `/api` в API. Внешние DeepSeek и Tutu MCP необязательны: при `DEMO_MODE=true` используется явно помеченный offline/demo fallback.

## Проверки

```bash
cd backend && go test ./...
cd frontend && npm run typecheck && npm run test && npm run build
```

Архитектура и ограничения описаны в [docs/architecture.md](docs/architecture.md), threat model — в [docs/threat-model.md](docs/threat-model.md), сценарий показа — в [docs/demo-script.md](docs/demo-script.md).


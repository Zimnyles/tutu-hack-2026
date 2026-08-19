.DEFAULT_GOAL := up

COMPOSE := docker compose

.PHONY: up down restart logs status build

up:
	$(COMPOSE) up --build --detach --remove-orphans
	@echo "Открывай запущен: http://localhost:5173"
	@echo "API: http://localhost:8080"

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart

logs:
	$(COMPOSE) logs --follow --tail=200

status:
	$(COMPOSE) ps

build:
	$(COMPOSE) build

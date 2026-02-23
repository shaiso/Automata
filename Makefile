SHELL := /bin/bash

DB_USER=automata
DB_PASS=automata
DB_NAME=automata
DB_HOST=localhost
DB_PORT=55432
DB_URL=postgresql://$(DB_USER):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: up down ps logs tidy build fmt vet test migrate-up db-shell dev-api dev-scheduler dev-orchestrator dev-worker up-all down-all logs-all
up:
	docker compose -f deploy/docker-compose.yml up -d db rabbitmq
	sleep 2

down:
	docker compose -f deploy/docker-compose.yml down

# Поднять всю систему (инфра + сервисы) в Docker
up-all:
	docker compose -f deploy/docker-compose.yml up -d --build

# Остановить всю систему
down-all:
	docker compose -f deploy/docker-compose.yml down

# Логи всех сервисов
logs-all:
	docker compose -f deploy/docker-compose.yml logs -f --tail=100

ps:
	docker compose -f deploy/docker-compose.yml ps

logs:
	docker compose -f deploy/docker-compose.yml logs -f --tail=200
tidy:
	go mod tidy

build:
	go build ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./... -count=1 -v
migrate-up:
	@echo "Applying migrations..."
	@for f in migrations/*.sql; do \
	  echo "--> $$f"; \
	  cat $$f | docker compose -f deploy/docker-compose.yml exec -T db psql -U $(DB_USER) -d $(DB_NAME) -v ON_ERROR_STOP=1; \
	done

db-shell:
	docker compose -f deploy/docker-compose.yml exec -it db psql -U $(DB_USER) -d $(DB_NAME)
DEV_ENV=RABBITMQ_URL=amqp://automata:automata@localhost:5672/ DB_URL=$(DB_URL)

dev-api:
	$(DEV_ENV) go run ./cmd/automata-api

dev-scheduler:
	$(DEV_ENV) go run ./cmd/automata-scheduler

dev-orchestrator:
	$(DEV_ENV) go run ./cmd/automata-orchestrator

dev-worker:
	$(DEV_ENV) go run ./cmd/automata-worker

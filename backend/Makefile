# CashX backend development helpers.
# Все значения берутся из .env (шаблон: .env.example) — секреты в Makefile
# не захардкожены. Go may live outside PATH on this machine; resolve it explicitly.
GO ?= $(shell command -v go 2>/dev/null || echo /usr/local/go/bin/go)
COMPOSE_FILE ?= ../docker-compose.yml

-include .env
export

.PHONY: db-up db-down db-psql build run-api run-redirect run-worker migrate openapi test sqlc tidy

db-up:
	docker compose -f $(COMPOSE_FILE) up -d

db-down:
	docker compose -f $(COMPOSE_FILE) down

db-psql:
	psql "$(CASHX_ADMIN_DATABASE_URL)"

build:
	$(GO) build ./...

run-api:
	$(GO) run ./cmd/api

run-redirect:
	$(GO) run ./cmd/redirect

run-worker:
	$(GO) run ./cmd/worker

migrate:
	$(GO) run ./cmd/migrate

# Интеграционные тесты используют отдельные test-базы из .env.
test:
	$(GO) test ./... -count=1

openapi:
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config api/openapi/codegen.yaml api/openapi/openapi.yaml

sqlc:
	$(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

tidy:
	$(GO) mod tidy
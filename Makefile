.PHONY: run test lint migrate migrate-up migrate-down migrate-create docker db-up

DB_URL ?= postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable

run:
	go run ./cmd/api

test:
	@echo TODO: run tests

lint:
	@echo "Linting OpenAPI (si redocly está disponible)..."
	@command -v redocly >/dev/null 2>&1 && redocly lint docs/openapi.yaml || echo "redocly no encontrado, se omite OpenAPI lint"
	@echo "Ejecutando golangci-lint (si está disponible)..."
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint no encontrado, se omite Go lint"

migrate-up:
	@echo "Running migrations..."
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	@echo "Rolling back last migration..."
	migrate -path migrations -database "$(DB_URL)" down 1

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate: migrate-up

docker:
	docker compose -f docker-compose.yml up --build

db-up:
	docker compose -f docker-compose.yml up -d db

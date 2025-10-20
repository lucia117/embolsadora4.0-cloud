.PHONY: run test lint migrate docker

run:
	go run ./cmd/api

test:
	@echo TODO: run tests

lint:
	@echo "Linting OpenAPI (si redocly está disponible)..."
	@command -v redocly >/dev/null 2>&1 && redocly lint docs/openapi.yaml || echo "redocly no encontrado, se omite OpenAPI lint"
	@echo "Ejecutando golangci-lint (si está disponible)..."
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint no encontrado, se omite Go lint"

migrate:
	@echo TODO: run DB migrations (placeholder)

docker:
	docker compose -f docker-compose.dev.yml up --build

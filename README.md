# Embolsadora Cloud API

## Qué es y dónde corre

Backend en la nube del sistema de monitoreo de máquinas embolsadoras. Es la API
multi-tenant que usa el frontend (administración de tenants, usuarios, roles y permisos,
edge devices, reglas de alarma, logs, notificaciones y dashboards) y el endpoint de
ingesta al que el Edge Pi Service de cada planta envía sus mediciones.

Monolito modular en Go 1.24 (Gin, pgx/v5, Zap, Prometheus). Persiste en PostgreSQL
(datos de administración), MongoDB (mediciones) y Redis (rate limit y caché); la
autenticación de usuarios es Supabase Auth. En producción corre en Google Cloud Run y se
despliega automáticamente al pushear a `main`.

## Quickstart

Requiere Go 1.24+, Docker, la CLI `migrate` y un proyecto de Supabase.

```bash
git clone https://github.com/lucia117/embolsadora4.0-cloud.git
cd embolsadora4.0-cloud
cp .env.example .env.local        # completar DATABASE_URL, SUPABASE_* y APP_BASE_URL

docker compose up -d db redis mongo
migrate -path migrations/ \
  -database "postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable" up

go run ./cmd/api
curl http://localhost:8080/ping   # → pong
```

Tests, lint, variables y datos de prueba: [`docs/development.md`](docs/development.md).

## Arquitectura

Capas hexagonales `handler → usecase → domain ← repo/platform/security`, cableadas en
`internal/routes/url_mappings.go`. Tres superficies HTTP: ABM con JWT + RBAC por tenant,
edge devices con tenant por path, e ingesta con API key.
Detalle en [`docs/architecture.md`](docs/architecture.md); decisiones en [`docs/adr/`](docs/adr/index.md).

## Contratos

- **Expone:** API HTTP descripta en [`docs/openapi.yaml`](docs/openapi.yaml) (OpenAPI 3.1,
  mantenido a mano). La ingesta `POST /api/v1/consumers/events` tiene un contrato
  congelado con el Edge Pi Service, en
  la spec [`022`](specs/022-cloud-ingest-endpoint/spec.md) y [CLOUD-ADR-018](docs/adr/CLOUD-ADR-018-ingesta-http-batch.md).
- **Consume:** Supabase Auth (JWKS y Admin API) y la API HTTP del Edge Pi Service
  (`/status`, `/health`, `/telemetry`, repo `embolsadora-edge`).

## Operación

Deploy, rollback, troubleshooting y observabilidad: [`docs/operations.md`](docs/operations.md).

## Estado y enlaces

- Rama de integración: `develop`. Rama de release (deploy a producción): `main`.
- Specs de features: [`specs/`](specs/).
- Reglas para agentes de código: [`AGENTS.md`](AGENTS.md).
- Material de proceso (auditorías, análisis y planes cerrados): [`docs/_process/`](docs/_process/).

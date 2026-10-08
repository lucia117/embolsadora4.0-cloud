---
title: Desarrollo local — embolsadora4.0-cloud
status: vigente
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Desarrollo local

## Requisitos

- Go 1.24+ (`go.mod`, igual que CI y el `Dockerfile`).
- Docker con Compose v2, para Postgres 16, Redis 7 y MongoDB 7.
- [`golang-migrate`](https://github.com/golang-migrate/migrate) v4.19+ (CLI `migrate`).
- [`golangci-lint`](https://golangci-lint.run/) v2 (CI usa `v2.13.2`).
- Un proyecto de Supabase (URL, JWKS, service role key): la API valida JWT contra el
  JWKS real y no arranca sin las variables de Supabase.

## Configuración

`config.LoadEnvFile()` lee `APP_ENV` (`local` | `beta` | `production`, default `local`) y
carga `.env.<APP_ENV>` si existe. Las variables del sistema tienen prioridad sobre el
archivo.

```bash
cp .env.example .env.local   # completá los valores; .env.* está en .gitignore
```

Obligatorias (sin ellas `config.Load` falla): `DATABASE_URL`, `SUPABASE_URL`,
`SUPABASE_JWKS_URL`, `SUPABASE_JWT_ISSUER`, `SUPABASE_SERVICE_ROLE_KEY`, `APP_BASE_URL`.
El resto tiene default; la lista completa y comentada está en
[`.env.example`](../.env.example).

## Levantar dependencias y la API

```bash
docker compose up -d db redis mongo          # Postgres :5432, Redis :6379, Mongo :27017

export DATABASE_URL="postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable"
migrate -path migrations/ -database "$DATABASE_URL" up   # o: make migrate-up

go run ./cmd/api                              # o: make run
curl http://localhost:8080/ping               # → pong
curl http://localhost:8080/health             # → {"status":"ok","checks":{...}}
```

Para correr también la API en contenedor: `docker compose up --build` (o `make docker`).
El servicio `api` toma las variables de Supabase del entorno o de un `.env` en la raíz.

Alternativa a correr `migrate` a mano: `RUN_MIGRATIONS_ON_BOOT=true` aplica las pendientes
al arrancar desde `MIGRATIONS_SOURCE_URL` (default `file:///app/migrations`, la ruta
dentro de la imagen; en local usá `file://migrations`).

En VS Code, `.vscode/launch.json` trae la configuración "Run API (Dev)" con
`APP_ENV=local`.

## Datos de prueba

- Catálogo del sistema (permisos, roles, tenant MRG): lo siembra la migración `000002`.
- Tenants de prueba (Córdoba, Mendoza, Rosario): `scripts/seed_test_city_tenants.sql`.
  Uso en [`migrations/README.md`](../migrations/README.md#seeds-opcionales-uat--dev).
  **No ejecutar en producción.**
- [`seeds/README.md`](../seeds/README.md): usuarios y asignaciones de demostración.
- Fixture determinista para tests de ingesta:
  `go run ./scripts/genfixture > internal/consumers/testdata/last-batch.json`.

## Tests

```bash
go test ./...                                               # unitarios
go test ./internal/security/... -run TestJWKSVerifier -v    # uno puntual
```

Los tests de integración se saltean **en silencio** si no están exportadas las tres
variables. Para correrlos:

```bash
docker compose up -d db redis mongo
export DATABASE_URL="postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable"
export MONGO_URI="mongodb://localhost:27017"
export REDIS_URL="redis://:embolsadora_redis_pass@localhost:6379/0"
go test ./...
```

CI (`.github/workflows/ci.yml`) no exporta esas variables: en CI solo corren los
unitarios.

## Build, lint y chequeo previo al push

```bash
go build ./...
go vet ./...
golangci-lint run
scripts/ci-check.sh     # build + vet + lint + test, mismo orden que CI
```

`make lint` corre además `redocly lint docs/openapi.yaml` si `redocly` está instalado.

## Chequeos de documentación

Los corre `.github/workflows/docs.yml` en cada PR. Para correrlos en local:

```bash
scripts/harness-check.sh          # layout, prohibiciones, front-matter, ADRs, índice de specs
npx markdownlint-cli2@0.18.1      # reglas y archivos en .markdownlint-cli2.jsonc
scripts/docs-freshness.sh         # documentos con last_reviewed de más de 120 días
go test ./internal/routes/ -run TestOpenAPIMatchesRouter   # rutas del router vs docs/openapi.yaml
```

El drift de OpenAPI es un test de Go y corre también con `go test ./...`. Una ruta nueva
sin documentar lo hace fallar; las excepciones deliberadas (`/ping`, `/health`,
`/metrics`) están listadas con su motivo en `internal/routes/openapi_drift_test.go`.

## Migraciones

```bash
migrate create -ext sql -dir migrations -seq nombre_de_la_feature   # nueva
migrate -path migrations/ -database "$DATABASE_URL" up              # aplicar
migrate -path migrations/ -database "$DATABASE_URL" down 1          # revertir la última
```

Cada migración lleva `up` y `down`. Antes de mergear una que cambie firmas SQL o ids de
permisos, revisá las secciones "⚠️ Orden de deploy" de
[`migrations/README.md`](../migrations/README.md).

## Artefactos derivados

| Artefacto | Cómo se actualiza |
|---|---|
| `docs/openapi.yaml` | A mano, en el mismo PR que cambia la ruta. `TestOpenAPIMatchesRouter` falla si una ruta registrada no está documentada o si un path documentado no existe. |
| Catálogo de permisos `perm_*` | Migración SQL (se lee de `roles.permissions` en runtime). |
| Plantillas de mail de Supabase | `emails/*.html` → `go run ./cmd/renderemails` (vista previa en `tmp/emails/`); publicación con `scripts/publish-email-templates.sh`. Ver [`emails/README.md`](../emails/README.md). |
| Colección Postman | `postman/Embolsadora-API-Complete.postman_collection.json`; guía en [`postman/POSTMAN-GUIDE.md`](../postman/POSTMAN-GUIDE.md). |

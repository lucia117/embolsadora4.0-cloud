# Embolsadora API (skeleton)

Repositorio Go 1.22 con arquitectura clean/hexagonal. Este repo contiene el esqueleto y wiring mínimo (sin lógica de negocio) para dos superficies:

- `/api` (ABM)
- `/consumers` (ingesta)

## Arquitectura (resumen)

- Separación de superficies: `API (ABM)` y `Consumers (ingesta)` bajo `/api/v1` y `/api/v1/consumers`.
- Capas internas: `domain/`, `app/`, `repo/`, `api/`, `consumers/`, `security/`, `telemetry/`, `platform/`, `config/`.
- Inyección de dependencias vía `Deps` en routers; middlewares como stubs (JWT, RBAC, API Key, RateLimit, Idempotency).
- Observabilidad mínima: logger dev (zap) y métricas Prometheus en `/metrics`.
- Repos PG con firmas que reciben `context.Context` y chequeo de `tenant_id` vía `platform.TenantID`.

## Superficies

- `/api/v1/**`: ABM (JWT + RBAC). Rutas stub 501 para `users`, `machines`, `tenants`.
- `/api/v1/consumers/**`: Ingesta (API Key + rate limit + idempotencia). Rutas stub 501 para `events` y `heartbeat`.

## Requisitos

- Go 1.22+ (se utilizará toolchain reciente automáticamente si tu Go lo sugiere)
- Docker y Docker Compose (opcional, para levantar Postgres/Redis y la API)
- VS Code (opcional) con extensión Go para depurar

## Comandos básicos

- `make docker`: levanta dependencias locales (db, redis, api) con `docker-compose.dev.yml`.
- `make run`: ejecuta la API localmente (`go run ./cmd/api`).

## Estructura

Ver carpetas principales:

- `cmd/api/` Entrypoint de la API (Gin minimal)
- `internal/api/` Rutas ABM (stubs 501)
- `internal/consumers/` Rutas de ingesta (stubs 501)
- `internal/config/` Estructuras tipadas de configuración (TODOs)
- `docs/openapi.yaml` Especificación OpenAPI
- `docs/adr/` ADRs
- `docker-compose.dev.yml` Stack local (db, redis, api)
- `Makefile` (targets utilitarios, opcional si tenés make)

## Inicialización del módulo Go

Si abrís el repo por primera vez:

```powershell
# Dentro de la carpeta del proyecto
# (ya configurado el module a github.com/tu-org/embolsadora-api)

# Normalizar dependencias
go mod tidy
```

## Ejecutar la API en local (sin Docker)

```powershell
# Desde la raíz del repo
go run ./cmd/api
```

Endpoints de salud:

- `GET http://localhost:8080/healthz` → 200
- `GET http://localhost:8080/readyz` → 200

Rutas stub (501 Not Implemented):

- `/api/v1/users` (GET/POST)
- `/api/v1/machines` (GET/POST)
- `/api/v1/tenants` (GET/POST)
- `/api/v1/consumers/events` (POST)
- `/api/v1/consumers/heartbeat` (POST)

## Ejecutar con Docker Compose

Levantá Postgres, Redis y la API:

```powershell
docker compose -f docker-compose.dev.yml up --build
```

Variables de entorno usadas por el servicio `api` en `docker-compose.dev.yml`:

- `DB_URL=postgres://postgres:postgres@db:5432/embolsadora?sslmode=disable`
- `REDIS_ADDR=redis:6379`
- `APP_ENV=dev`
- `AUTH_JWT_ISSUER=embolsadora`
- `AUTH_JWT_PUBLIC=__placeholder__`
- `AUTH_JWT_PRIVATE=__placeholder__`

> Nota: reemplazá los placeholders de JWT cuando tengas las llaves.

## Run and Debug en VS Code

Ya se incluye `/.vscode/launch.json` con la configuración "Run API (Dev)":

- `program`: `${workspaceFolder}/cmd/api`
- `env` (local):
  - `APP_ENV=dev`
  - `DB_URL=postgres://postgres:postgres@localhost:5432/embolsadora?sslmode=disable`
  - `REDIS_ADDR=localhost:6379`
  - `AUTH_JWT_ISSUER=embolsadora`
  - `AUTH_JWT_PUBLIC=__placeholder__`
  - `AUTH_JWT_PRIVATE=__placeholder__`

Pasos:

1. Abrí la vista "Run and Debug" (Ctrl+Shift+D).
2. Elegí "Run API (Dev)".
3. F5 para ejecutar.

## Makefile (opcional)

Si tenés `make` instalado:

```powershell
make run      # go run ./cmd/api
make docker   # docker compose -f docker-compose.dev.yml up --build
make migrate  # placeholder
```

## OpenAPI y ADRs

- La especificación está en `docs/openapi.yaml`.
- Decisiones de arquitectura en `docs/adr/ADR-001..004.md`.

## Notas

- Este repo es un esqueleto: no contiene lógica de negocio. Los handlers devuelven `501 Not Implemented`.
- Los comentarios/TODOs están en inglés por consistencia técnica interna.

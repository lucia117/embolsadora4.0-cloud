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

- Go 1.24+ (requerido por el proyecto)
- Docker y Docker Compose (para levantar Postgres/Redis y la API)
- VS Code (opcional) con extensión Go para depurar
- Make (opcional, para usar los comandos del Makefile)

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

### Prerequisitos

1. Docker y Docker Compose instalados
2. Tener el archivo `docker-compose.yml` o `docker-compose.dev.yml` en la raíz del proyecto
3. Tener el `Dockerfile` configurado correctamente

### Levantar los servicios

Para levantar Postgres, Redis y la API por primera vez:

```powershell
# Construir las imágenes desde cero
docker-compose -f docker-compose.yml build --no-cache

# Levantar todos los servicios
docker-compose -f docker-compose.yml up
```

Para levantar los servicios en segundo plano (modo detached):

```powershell
docker-compose -f docker-compose.yml up -d
```

### Detener los servicios

```powershell
# Detener los contenedores
docker-compose -f docker-compose.yml down

# Detener y eliminar volúmenes (elimina datos de la BD)
docker-compose -f docker-compose.yml down -v
```

### Ver logs

```powershell
# Ver logs de todos los servicios
docker-compose -f docker-compose.yml logs -f

# Ver logs de un servicio específico
docker-compose -f docker-compose.yml logs -f api
```

### Variables de entorno

El servicio `api` en `docker-compose.yml` usa las siguientes variables de entorno:

- `DB_URL=postgres://embolsadora_user:embolsadora_password@db:5432/embolsadora_dev?sslmode=disable`
- `DB_HOST=db`
- `DB_PORT=5432`
- `DB_USER=embolsadora_user`
- `DB_PASSWORD=embolsadora_password`
- `DB_NAME=embolsadora_dev`
- `REDIS_HOST=redis`
- `REDIS_PORT=6379`
- `REDIS_PASSWORD=embolsadora_redis_pass`
- `APP_ENV=development`

### Verificar que la API está funcionando

Una vez levantados los servicios, podés verificar que la API está funcionando:

```powershell
# Health check
curl http://localhost:8080/healthz

# Ready check
curl http://localhost:8080/readyz
```

### Servicios disponibles

- **API**: `http://localhost:8080`
- **PostgreSQL**: `localhost:5432`
  - Usuario: `embolsadora_user`
  - Password: `embolsadora_password`
  - Base de datos: `embolsadora_dev`
- **Redis**: `localhost:6379`
  - Password: `embolsadora_redis_pass`

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

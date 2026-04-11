# Embolsadora API

Repositorio Go 1.24+ con arquitectura clean/hexagonal para el monitoreo de máquinas embolsadoras industriales. Expone dos superficies HTTP:

- `/api/v1` (ABM — JWT + RBAC)
- `/api/v1/consumers` (ingesta IoT — API Key + rate limit + idempotencia)

## Arquitectura (resumen)

- Separación de superficies: `API (ABM)` y `Consumers (ingesta)` bajo `/api/v1` y `/api/v1/consumers`.
- Capas internas: `domain/`, `app/`, `repo/`, `api/`, `consumers/`, `security/`, `telemetry/`, `platform/`, `config/`.
- Inyección de dependencias vía `Deps` en routers; middlewares como stubs (JWT, RBAC, API Key, RateLimit, Idempotency).
- Observabilidad mínima: logger dev (zap) y métricas Prometheus en `/metrics`.
- Repos PG con firmas que reciben `context.Context` y chequeo de `tenant_id` vía `platform.TenantID`.

## Superficies

- `/api/v1/**`: ABM (JWT + RBAC). Gestión de usuarios, tenants, roles y asignaciones.
- `/api/v1/consumers/**`: Ingesta (API Key + rate limit + idempotencia). Recepción de eventos y heartbeats desde dispositivos IoT.

## Requisitos

- **Docker y Docker Compose** — único requisito para levantar el stack completo
- Go **no** se instala en el host; todos los comandos `go` se ejecutan dentro de contenedores Docker
- VS Code (opcional) con extensión Go para navegar el código

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

## Levantar el proyecto con Docker Compose

El stack completo incluye PostgreSQL, Redis, MongoDB y la API Go — todo orquestado con `docker compose`.

### 1. Configurar variables de entorno

Copiar el archivo de ejemplo y completar los valores de Supabase:

```bash
cp .env.example .env
```

Editar `.env` y reemplazar los placeholders de Supabase:

```env
SUPABASE_URL=https://<project-ref>.supabase.co
SUPABASE_JWKS_URL=https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json
SUPABASE_JWT_ISSUER=https://<project-ref>.supabase.co/auth/v1
SUPABASE_SERVICE_ROLE_KEY=<obtener del dashboard: Settings → API → service_role>
SUPABASE_ANON_KEY=<obtener del dashboard: Settings → API → anon>
```

El resto de las variables (Postgres, Redis, MongoDB) ya tienen valores por defecto en el archivo.

### 2. Levantar todos los servicios

```bash
# Primera vez: construir imagen de la API y levantar todo
docker compose up --build -d

# Verificar que los contenedores están corriendo
docker compose ps
```

Servicios que levanta:

| Servicio | Puerto | Descripción |
|---|---|---|
| `api` | `8080` | API Go (Gin) |
| `db` | `5432` | PostgreSQL 16 |
| `redis` | `6379` | Redis 7 |
| `mongo` | `27017` | MongoDB 7 |

### 3. Aplicar migraciones de base de datos

Una vez que los contenedores están corriendo, aplicar las migraciones de Postgres:

```bash
docker run --rm \
  --network embolsadora4.0-cloud_embolsadora_network \
  -v $(pwd)/migrations:/migrations \
  migrate/migrate \
  -path /migrations \
  -database "postgres://embolsadora_user:embolsadora_password@db:5432/embolsadora_dev?sslmode=disable" \
  up
```

### 4. Verificar que todo está funcionando

```bash
curl http://localhost:8080/ping
```

Respuesta esperada con los tres stores healthy:

```json
{
  "postgres": { "status": "ok" },
  "redis":    { "status": "ok" },
  "mongo":    { "status": "ok" }
}
```

### Comandos útiles

```bash
# Ver logs en tiempo real
docker compose logs -f

# Ver logs de un servicio específico
docker compose logs -f api

# Reconstruir solo la API (tras cambios de código)
docker compose up --build -d api

# Detener todos los servicios
docker compose down

# Detener y borrar todos los volúmenes (resetea las bases de datos)
docker compose down -v
```

### Ejecutar comandos Go (sin instalar Go en el host)

```bash
# Compilar
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."

# Correr tests
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go test ./..."

# Agregar una dependencia
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go get github.com/some/package && go mod tidy"
```

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

## Colecciones Postman / Bruno

Las colecciones están en [`specs/postman/`](specs/postman/). Ver [`specs/postman/TESTING_GUIDE.md`](specs/postman/TESTING_GUIDE.md) para la guía completa de uso.

| Archivo | Herramienta | Descripción |
|---|---|---|
| [`embolsadora-api.postman_collection.json`](specs/postman/embolsadora-api.postman_collection.json) | Postman | Colección completa: auth, tenants, usuarios, roles, AAS Shells (MongoDB), edge devices, consumers |
| [`Embolsadora API — Local - brunoo.json`](specs/postman/Embolsadora%20API%20%E2%80%94%20Local%20-%20brunoo.json) | Bruno | Misma cobertura, compatible con Bruno runner |
| [`Embolsadora API — Local.environment.json`](specs/postman/Embolsadora%20API%20%E2%80%94%20Local.environment.json) | Bruno | Variables de entorno para Bruno |

**Variables requeridas antes de ejecutar:**

| Variable | Valor |
|---|---|
| `tenantId` | `550e8400-e29b-41d4-a716-446655440001` (demo seed) |
| `roleId` | `admin` |
| `apiKey` | `dev-api-key` |
| `bearerToken` | se llena automáticamente al ejecutar Login |

## OpenAPI y ADRs

- La especificación está en `docs/openapi.yaml`.
- Decisiones de arquitectura en `docs/adr/ADR-001..004.md`.

## Notas

- Los comentarios/TODOs están en inglés por consistencia técnica interna.
- Ver [`specs/postman/TESTING_GUIDE.md`](specs/postman/TESTING_GUIDE.md) para la guía completa de testing manual.

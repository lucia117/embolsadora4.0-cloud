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

Endpoint de salud:

- `GET http://localhost:8080/ping` → 200 "pong"

Endpoints disponibles:

- `/api/v1/users` (GET, POST) — listado y creación de usuarios
- `/api/v1/users/:id` (GET, PATCH, DELETE) — gestión individual de usuarios
- `/api/v1/users/:id/roles` (GET) — roles de un usuario
- `/api/v1/user-roles` (GET, POST) — asignaciones de rol
- `/api/v1/user-roles/bulk` (POST) — asignación masiva
- `/api/v1/user-roles/:id` (PUT, DELETE) — actualización y revocación
- `/api/v1/tenants` (GET, POST) — listado y creación de tenants
- `/api/v1/tenants/:id` (GET, PATCH, DELETE) — gestión individual de tenants
- `/api/v1/machines` (GET, POST) — listado y creación de máquinas
- `/api/v1/consumers/events` (POST) — ingesta batch de eventos
- `/api/v1/consumers/heartbeat` (POST) — heartbeat de dispositivo

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
# Ping endpoint
curl http://localhost:8080/ping
# Debería responder: pong
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

## Colecciones Postman

Las colecciones Postman están en la carpeta [`postman/`](postman/).

| Archivo | Descripción |
|---|---|
| [`User-Management-API.postman_collection.json`](postman/User-Management-API.postman_collection.json) | CRUD completo de usuarios con ejemplos y casos de error |
| [`user-role-assignments.postman_collection.json`](postman/user-role-assignments.postman_collection.json) | Asignación, actualización y revocación de roles |
| [`tenants.postman_collection.json`](postman/tenants.postman_collection.json) | CRUD completo de tenants (`GET`, `POST`, `PATCH`, `DELETE`) |
| [`User-Management-API.postman_environment.json`](postman/User-Management-API.postman_environment.json) | Variables para user management (`base_url`, `tenant_id`, `jwt_token`, `user_id`) |
| [`env-local.postman_environment.json`](postman/env-local.postman_environment.json) | Variables de entorno para desarrollo local (`http://localhost:8080`) |

**Cómo usar:**
1. Importar el archivo de colección en Postman.
2. Importar el ambiente local y seleccionarlo como activo.
3. Completar la variable `token` con el JWT obtenido desde `POST /auth/login`.
4. Ejecutar `POST Create Tenant`: el `id` del tenant creado se guarda automáticamente en la variable `{{tenantId}}` para los demás requests.

## OpenAPI y ADRs

- La especificación está en `docs/openapi.yaml`.
- Decisiones de arquitectura en `docs/adr/ADR-001..004.md`.

## Notas

- Los comentarios/TODOs están en inglés por consistencia técnica interna.
- Ver `postman/README.md` y `postman/TESTING-GUIDE.md` para guías detalladas de uso y testing.

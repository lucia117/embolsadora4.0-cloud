---
title: Arquitectura interna — embolsadora4.0-cloud
status: vigente
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Arquitectura interna

Arquitectura **del componente**: capas, paquetes, superficies HTTP y patrones que todo
cambio tiene que respetar. La vista de sistema (frontend, edge, historian, cloud) no va
acá: vive en el hub de documentación.

## Estilo

Monolito modular en Go con capas hexagonales
([CLOUD-ADR-001](adr/CLOUD-ADR-001-monolito-modular-superficies.md)):

```text
transport (handler) → app (usecase) → domain ← infra (repo / platform / security)
```

- `domain/` son tipos puros, errores e interfaces de repositorio. No importa infraestructura.
- `app/` y `api/usecases/` orquestan: validan, llaman repos, mapean errores de dominio.
- `api/handler/` y `consumers/` son transporte: parsean, llaman al usecase y serializan.
- `repo/`, `platform/` y `security/` implementan las interfaces del dominio contra Postgres,
  MongoDB, Redis, Supabase y el Edge Pi.

El cableado de todas las dependencias está en un solo lugar:
`internal/routes/url_mappings.go` (`RegisterURLMappings`). `cmd/api/main.go` carga
configuración, aplica migraciones si corresponde, abre Postgres y Redis y levanta el
`http.Server`.

## Paquetes

```text
cmd/api/                     entrypoint: config, migraciones on-boot, Postgres, Redis, http.Server
cmd/renderemails/            render de plantillas de mail (emails/)
internal/
  config/                    Config tipada desde env (Load); LoadEnvFile carga .env.<APP_ENV>
  domain/                    User, UserInvitation, errores, estados
    ingest/                  Measurement, DeviceContext, EventError (códigos congelados), Repository
    apikeys/                 APIKey, Credential, DeviceIdentity; keygen (Generate/Parse/Hash/Matches)
    metrics/                 consultas de métricas de dashboards, límites
    edge_devices/ users/ dashboard_layouts/
  app/                       servicios por feature: ingest, dashboards, edge_devices, alarm_rules,
                             logs, notifications, permissions, roles, users, dashboard_layouts
  api/
    middleware/              JWTAuth, TenantFromHeader, PasswordChangeGuard, RBACCheck,
                             AppBaseURLFromHeader, CORS, Logger, RequestID, rate limit de dashboards
    usecases/                auth (ProvisionUser), me (GetMe + tipos de respuesta),
                             invitations, password, tenants/*, user_roles/*
    handler/                 un paquete por recurso; httperr centraliza el mapeo de errores
    router.go                RegisterAdminRoutes: users, tenants, user-roles, machines
  consumers/                 superficie con API key (sin JWT): ingesta del Edge Pi
    middleware/              APIKeyAuth, RateLimit, NoCORS
  security/                  jwt.go (JWKSVerifier, ErrJWKSUnavailable), rbac.go (RoleContext,
                             Can, EffectiveRole), apikeys_authenticator.go
  platform/                  tenantctx (helpers de contexto), supabase (AdminClient), mongo,
                             edgeclient (HTTP al Edge Pi), dbmigrate, apporigin, logwriter
  repo/
    pg/                      un repo por agregado (pgx/v5)
    mongo/                   measurements (insert idempotente), metrics (agregaciones)
    redis/                   token bucket e idempotency store
  routes/                    RegisterURLMappings: cablea todo
  telemetry/                 métricas Prometheus
migrations/                  SQL numerado up/down (golang-migrate)
```

## Superficies HTTP

| Superficie | Prefijo | Autenticación | Cadena de middlewares |
|---|---|---|---|
| Pública | `/ping`, `/health`, `/metrics`, `POST /api/v1/auth/login`, `GET /api/v1/public/tenants/:idOrSubdomain` | ninguna | `RequestID → Logger → CORS` (globales) |
| ABM | `/api/v1/**` | JWT de Supabase + membresía en tenant | `JWTAuth → TenantFromHeader → PasswordChangeGuard → AppBaseURLFromHeader → [RBACCheck por ruta]` |
| Edge devices | `/api/v1/tenants/:tenantId/edge-devices/**` | JWT; tenant por **path** (subdominio del tenant, no UUID) | `JWTAuth → ResolveTenantAndCheckMembership → [RBAC por ruta]` |
| Ingesta | `/api/v1/consumers/**` | API key (`X-Api-Key`) | `RequestID → NoCORS → APIKeyAuth → RateLimit` |
| Métricas de dashboards | `/api/v1/dashboards/metrics/**` | como ABM | ABM + `RBACCheck("perm_metrics_view") → DashboardRateLimit` |

Excepciones dentro de ABM: `GET /api/v1/me` y `POST /api/v1/auth/change-password` no
exigen `X-Tenant-ID` ni pasan por `PasswordChangeGuard` (`isExemptFromTenant`,
`isExemptFromPasswordGuard` en `middleware.go`).

El contrato de todas las rutas es [`openapi.yaml`](openapi.yaml). La superficie de ingesta
tiene además un **contrato congelado** con el Edge Pi Service: sin envelope
`{"success":...}` y con `errors[].code` fijos, definido en
la spec [`022`](../specs/022-cloud-ingest-endpoint/spec.md) y [CLOUD-ADR-018](adr/CLOUD-ADR-018-ingesta-http-batch.md).

Rutas registradas que todavía responden `501 Not Implemented`: `GET/POST /api/v1/machines`
y `POST /api/v1/consumers/heartbeat`.

## Patrones obligatorios

### Autenticación y auto-provisioning

`JWTAuth` valida el token RS256 contra el JWKS de Supabase (`security.NewJWKSVerifier`).
En cada request autenticado llama a `AuthUsecase.ProvisionUser()`, que hace upsert
idempotente del usuario local (`ON CONFLICT (supabase_user_id)`), y activa invitaciones
pendientes del usuario. Si el JWKS no responde, `Verify` devuelve el sentinel
`ErrJWKSUnavailable` y `JWTAuth` responde **503**, no 401.

### Tenant y RBAC

- `TenantFromHeader` lee `X-Tenant-ID` (UUID, canonicalizado a minúsculas porque los
  `$match` de Mongo son byte-exactos), valida membresía activa en `user_tenant_roles` y
  carga el rol.
- Un `admin` del tenant plataforma asciende a `platform_admin` (`security.EffectiveRole`).
- Operadores de plataforma (roles globales o `admin` del tenant plataforma) pueden actuar
  sobre cualquier tenant existente sin membresía directa
  ([CLOUD-ADR-015](adr/CLOUD-ADR-015-plataforma-cross-tenant.md)). Tenant inexistente → 404; sin
  acceso → 403.
- Los permisos (`perm_*`) se leen de `roles.permissions` en Postgres en cada request y
  viajan en `security.RoleContext`. `RBACCheck(perm)` → `security.Can()`. Agregar o
  cambiar permisos es una migración, no un cambio en Go.
- `GET /api/v1/me` arma `Permissions` leyendo la misma tabla en vivo.

### Ingesta (Edge Pi → cloud)

- `APIKeyAuth` resuelve tenant y device **server-side** desde la API key; el Pi nunca
  manda tenant. Solo se guarda `sha256(secreto)`; la key resuelta se cachea en Redis
  (`APIKEY_CACHE_TTL`).
- Idempotencia por índice único `(tenantId, eventId)` en MongoDB; `insertMany` con
  `ordered:false`. Sin ese índice el proceso no arranca: cada reintento del Pi duplicaría
  mediciones.
- Límites: `INGEST_MAX_EVENTS` (1000) como regla de negocio; `INGEST_MAX_BODY_BYTES`
  (4 MiB) solo contra abuso, porque el Edge corta sus batches en 2 MiB exactos.
- El header `Idempotency-Key` se registra pero no decide nada.

### Degradación controlada

| Dependencia caída | Efecto | Dónde |
|---|---|---|
| Redis (no configurado o inalcanzable) | Rate limit y caché de API keys fallan **abiertos** | `cmd/api/main.go`, `consumers/ratelimit.go` |
| MongoDB al arrancar | Ingesta y métricas de dashboards responden 500; el resto de la API sigue | `connectMeasurementsRepo` (`routes/url_mappings.go`) |
| MongoDB conecta pero falla `EnsureIndexes` | **Fatal**: el proceso termina | ídem |
| JWKS de Supabase | 503 en rutas autenticadas | `security/jwt.go`, `JWTAuth` |
| Postgres | Fatal al arrancar | `cmd/api/main.go` |

### Import cycles

Los tipos de respuesta de `GET /me` viven en `internal/api/usecases` (no en
`handler/me/models`) porque el handler importa el usecase.

## Persistencia

| Store | Qué guarda | Esquema |
|---|---|---|
| PostgreSQL 16 | Tenants, usuarios, roles y permisos, membresías, invitaciones, edge devices y sus API keys, reglas de alarma, logs, notificaciones, layouts de dashboards | [`migrations/`](../migrations/README.md) (golang-migrate) |
| MongoDB 7 | Mediciones de la ingesta (`measurements`) y agregaciones de métricas | índices creados en código al arrancar (`EnsureIndexes`) |
| Redis 7 | Rate limit (token bucket), caché de API keys | efímero |

Decisiones: [CLOUD-ADR-017](adr/CLOUD-ADR-017-mediciones-en-mongodb.md) (mediciones en
MongoDB, reemplaza a CLOUD-ADR-003) y [CLOUD-ADR-018](adr/CLOUD-ADR-018-ingesta-http-batch.md)
(contrato y límites de la ingesta).

## Observabilidad

- `GET /metrics`: Prometheus (`internal/telemetry`), con contadores de auth, invitaciones,
  ingesta (`ingest_*`, incluido `ingest_mongo_up`), logs, notificaciones, permisos y
  dashboards.
- `GET /ping`: liveness (`pong`). `GET /health`: estado de Postgres, Mongo y Redis; 503 si
  Postgres o Mongo fallan (Redis degradado no lo cambia).
- Logs: Zap en modo desarrollo (`zap.NewDevelopment()`) en todos los ambientes.
  `LOG_LEVEL` se lee pero hoy no se usa.

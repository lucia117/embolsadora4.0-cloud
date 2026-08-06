# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Stack

- **Go 1.24+** — Gin (HTTP router), pgx/v5 (PostgreSQL), Zap (structured logging), Prometheus (metrics)
- **Auth**: Supabase Auth (JWT RS256 via JWKS) — `MicahParks/keyfunc/v3`, `golang-jwt/jwt/v5`
- **Invitations/password reset**: Supabase Admin REST API (`internal/platform/supabase/admin_client.go`)
- **Rate limiting**: Redis via `go-redis/redis/v8`
- **Measurements storage**: MongoDB via `go.mongodb.org/mongo-driver/v2` (`internal/platform/mongo/client.go`)
- **Testing**: `testify`, `uber/mock`
- **DB migrations**: `golang-migrate`

## Commands

Go is installed natively on this host — run `go` directly, no Docker needed.

```bash
# Build
go build ./...

# Run tests (integration tests are skipped silently unless DATABASE_URL,
# MONGO_URI and REDIS_URL are exported — see below)
go test ./...

# Run single test
go test ./internal/security/... -run TestJWKSVerifier -v

# Add dependency
go get github.com/some/package && go mod tidy

# Apply migrations
migrate -path migrations/ -database $DATABASE_URL up
```

Integration tests need `DATABASE_URL`, `MONGO_URI` and `REDIS_URL` exported (e.g.
`postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable`,
`mongodb://localhost:27017`, `redis://:embolsadora_redis_pass@localhost:6379/0`). Bring up the
dependencies with `docker compose up -d db redis mongo`.

<details>
<summary>Docker fallback (no local Go toolchain)</summary>

```bash
docker build --target builder -t embolsadora-api:dev .

docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."

docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL=postgres://... -e MONGO_URI=mongodb://... -e REDIS_URL=redis://... \
  golang:1.24-alpine \
  sh -c "go test ./..."
```

</details>

## Architecture

Hexagonal layout: `transport (handler) → app (usecase) → domain ← infra (repo/platform/security)`

```
cmd/api/main.go              — entry point; wires config, DB, Redis, routes
internal/
  config/                    — env-based Config struct (Load())
  domain/                    — pure types: User, UserInvitation, errors, UserStatus, InvitationStatus
    ingest/                  — Measurement, DeviceContext, EventError (frozen contract error codes), Repository interface
    apikeys/                 — APIKey, Credential, DeviceIdentity; keygen.go (Generate/Parse/Hash/Matches)
  security/
    jwt.go                   — JWKSVerifier (Verifier interface + ErrJWKSUnavailable sentinel)
    rbac.go                  — rolePermissions map, Can(), PermissionsForRole(), WithRole()
  platform/
    tenantctx.go             — context helpers: WithTenantID, WithDomainUser, WithSupabaseSub, etc.
    supabase/admin_client.go — AdminClient interface (InviteUserByEmail, SendPasswordResetEmail)
    mongo/client.go          — Mongo client wrapper (Connect, Database, Ping, Close)
  app/
    ingest/                  — validate.go (envelope validation), service.go (batch orchestration)
  api/
    middleware/middleware.go — JWTAuth(), TenantFromHeader(), PasswordChangeGuard(), RBACCheck(), CORS(), Logger(), RequestID()
    usecases/
      auth_usecase.go        — ProvisionUser(); InvitationActivator interface
      me_usecase.go          — GetMe() + MeResponse types (defined here to avoid circular imports)
      invitation_usecase.go  — CreateInvitation, Resend, Revoke, List, ActivatePendingInvitations; Log *zap.Logger
      password_usecase.go    — ForcePasswordChange, ClearPasswordChangeRequired
    handler/
      me/get_me.go
      invitations/{create,list,resend,revoke}_invitation/
      users/force_password_change/
      auth/change_password/
  consumers/                 — API-key authenticated surface (no JWT): Edge Pi Service ingest
    events_handler.go        — IngestEvents: POST /api/v1/consumers/events
    ratelimit.go             — RateLimiter (Redis-backed, per API key)
    router.go                — RegisterConsumerRoutes
    dto/events.go            — request/response DTOs
    middleware/               — APIKeyAuth(), RateLimit(), NoCORS()
  repo/pg/
    users/users_repo.go      — UpsertBySupabaseID (ON CONFLICT), GetBySupabaseID, SetStatus, SetPasswordChangeRequired
    invitations/invitations_repo.go — InvitationRepository
    apikeys/                 — API key repository (lookup/create/revoke)
  repo/mongo/
    measurements/            — measurement insert (insertMany ordered:false, dedup on eventId)
  routes/url_mappings.go     — RegisterURLMappings(r, db, cfg, redisClient); wires everything
  telemetry/auth_metrics.go  — Prometheus counters for auth events
migrations/                  — numbered SQL files (up/down)
scripts/genfixture/          — deterministic fixture generator for ingest tests
docs/openapi.yaml            — API spec (v2.0.0-alpha)
specs/                       — feature specs, plans, tasks
docs/superpowers/plans/      — implementation plans (e.g. cloud-ingest-endpoint's frozen HTTP contract)
```

## Key Patterns

**Middleware chain** for `/api/v1` (except `/me` and `/auth/change-password`):
`JWTAuth → TenantFromHeader → PasswordChangeGuard → [RBACCheck per route]`

**`/api/v1/me`** and **`/api/v1/auth/change-password`**: only `JWTAuth`, no tenant header required.

**`/api/v1/consumers/events`**: `APIKeyAuth → RateLimit`, no JWT, no tenant header, no `{"success":...}` envelope — response shape and `errors[].code` values are a frozen contract with the Edge Pi Service, documented in `docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md`.

**Auto-provisioning**: `JWTAuth` calls `AuthUsecase.ProvisionUser()` on every authenticated request — upsert is idempotent via `ON CONFLICT (supabase_user_id)`.

**Redis nil-safety**: Rate limiting fails open if Redis client is nil or unreachable.

**Circular import prevention**: Response types for `GET /me` live in `usecases` package (not in handler/me/models), because handler imports usecase.

**JWKS unavailable → 503**: `ErrJWKSUnavailable` sentinel in `security/jwt.go`; `JWTAuth` maps it to HTTP 503.

## Pending Manual Steps

- **Provisionar MongoDB y setear `MONGO_URI`**: la ingesta (`POST /api/v1/consumers/events`) persiste en MongoDB. `MONGO_URI` no tiene un default productivo (cae a `mongodb://localhost:27017`) — sin provisionar Mongo y setear la variable en el deploy, la ingesta arranca degradada (responde 500 en cada request; el resto de la API sigue funcionando con normalidad, ver `internal/routes/url_mappings.go`). Ver también `MONGO_DATABASE` y `MONGO_TIMEOUT` en `.env.example`.
- **Deploy a Koyeb**: tras mergear, aplicar migraciones contra Koyeb Managed Postgres. Ver `migrations/README.md` (sección "Deploy a Koyeb") y `specs/014-consolidate-migrations/quickstart.md`.
- **Activar admin MRG**: post-deploy, crear el admin en Supabase Auth y asignar el rol `super_admin` en el tenant MRG (`11b36b85-033d-4bb3-9e31-4c92161887c0`) — instrucciones en `migrations/README.md`.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

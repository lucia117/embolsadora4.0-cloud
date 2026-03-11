# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Stack

- **Go 1.24+** — Gin (HTTP router), pgx/v5 (PostgreSQL), Zap (structured logging), Prometheus (metrics)
- **Auth**: Supabase Auth (JWT RS256 via JWKS) — `MicahParks/keyfunc/v3`, `golang-jwt/jwt/v5`
- **Invitations/password reset**: Supabase Admin REST API (`internal/platform/supabase/admin_client.go`)
- **Rate limiting**: Redis via `go-redis/redis/v8`
- **Testing**: `testify`, `uber/mock`
- **DB migrations**: `golang-migrate`

## Commands

> **Go is NOT installed on macOS host.** All `go` commands must run via Docker:

```bash
# Build
docker build --target builder -t embolsadora-api:dev .

# Run go commands (module cache persisted)
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."

# Run tests (integration tests require DATABASE_URL)
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL=postgres://... \
  golang:1.24-alpine \
  sh -c "go test ./..."

# Run single test
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/security/... -run TestJWKSVerifier -v"

# Add dependency
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go get github.com/some/package && go mod tidy"

# Apply migrations
migrate -path migrations/ -database $DATABASE_URL up
```

## Architecture

Hexagonal layout: `transport (handler) → app (usecase) → domain ← infra (repo/platform/security)`

```
cmd/api/main.go              — entry point; wires config, DB, Redis, routes
internal/
  config/                    — env-based Config struct (Load())
  domain/                    — pure types: User, UserInvitation, errors, UserStatus, InvitationStatus
  security/
    jwt.go                   — JWKSVerifier (Verifier interface + ErrJWKSUnavailable sentinel)
    rbac.go                  — rolePermissions map, Can(), PermissionsForRole(), WithRole()
  platform/
    tenantctx.go             — context helpers: WithTenantID, WithDomainUser, WithSupabaseSub, etc.
    supabase/admin_client.go — AdminClient interface (InviteUserByEmail, SendPasswordResetEmail)
  api/
    middleware/middleware.go — JWTAuth(), TenantFromHeader(), PasswordChangeGuard(), RBACCheck(), CORS(), Logger(), RequestID()
    usecases/
      auth_usecase.go        — ProvisionUser(); InvitationActivator interface
      me_usecase.go          — GetMe() + MeResponse types (defined here to avoid circular imports)
      invitation_usecase.go  — CreateInvitation, Resend, Revoke, List, ActivateInvitation; Log *zap.Logger
      password_usecase.go    — ForcePasswordChange, ClearPasswordChangeRequired
    handler/
      me/get_me.go
      invitations/{create,list,resend,revoke}_invitation/
      users/force_password_change/
      auth/change_password/
  repo/pg/
    users/users_repo.go      — UpsertBySupabaseID (ON CONFLICT), GetBySupabaseID, SetStatus, SetPasswordChangeRequired
    invitations/invitations_repo.go — InvitationRepository
  routes/url_mappings.go     — RegisterURLMappings(r, db, cfg, redisClient); wires everything
  telemetry/auth_metrics.go  — Prometheus counters for auth events
migrations/                  — numbered SQL files (up/down)
docs/openapi.yaml            — API spec (v2.0.0-alpha)
specs/                       — feature specs, plans, tasks
```

## Key Patterns

**Middleware chain** for `/api/v1` (except `/me` and `/auth/change-password`):
`JWTAuth → TenantFromHeader → PasswordChangeGuard → [RBACCheck per route]`

**`/api/v1/me`** and **`/api/v1/auth/change-password`**: only `JWTAuth`, no tenant header required.

**Auto-provisioning**: `JWTAuth` calls `AuthUsecase.ProvisionUser()` on every authenticated request — upsert is idempotent via `ON CONFLICT (supabase_user_id)`.

**Redis nil-safety**: Rate limiting fails open if Redis client is nil or unreachable.

**Circular import prevention**: Response types for `GET /me` live in `usecases` package (not in handler/me/models), because handler imports usecase.

**JWKS unavailable → 503**: `ErrJWKSUnavailable` sentinel in `security/jwt.go`; `JWTAuth` maps it to HTTP 503.

## Pending Manual Steps

- T008: `migrate -path migrations/ -database $DATABASE_URL up 1` (migration 004)
- T034: `migrate -path migrations/ -database $DATABASE_URL up 1` (migration 005)
- T048: Run `specs/002-supabase-auth-backend/quickstart.md` curl commands against local server
- T051: Benchmark `GET /api/v1/me` P95 < 300ms

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

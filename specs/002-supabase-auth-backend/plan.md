# Implementation Plan: Supabase Auth — Backend

**Branch**: `002-supabase-auth-backend` | **Date**: 2026-03-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-supabase-auth-backend/spec.md`

---

## Summary

Reemplazar el sistema de auth propio del backend Go (`internal/auth/`, tablas `sessions` y `password_reset_tokens`, campo `password_hash`) por validación real de JWT de Supabase mediante JWKS, RBAC real basado en `user_tenant_roles`, auto-provisioning idempotente de usuarios, y endpoints de gestión de invitaciones que integran la API Admin de Supabase. El tenant scope se resuelve desde el header `X-Tenant-ID`, no desde el JWT.

---

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), `MicahParks/keyfunc/v3` (JWKS cache), `golang-jwt/jwt/v5` (ya importado), Zap (logging), Prometheus client_golang (métricas), Redis (rate limiting via Upstash)
**Storage**: PostgreSQL (Neon) — migraciones en `migrations/`; Redis (Upstash) — rate limiting y JWKS cache auxiliar
**Testing**: Go `testing` + testify + uber/mock; contenedores Docker para tests de integración de migración
**Target Platform**: Linux (Koyeb, Alpine Docker image)
**Project Type**: web-service (monolito modular, arquitectura hexagonal)
**Performance Goals**: `GET /api/v1/me` < 300ms P95 (SC-003); JWKS validation con cache local, sin llamada a Supabase en cada request
**Constraints**: Sin cambio de código para migrar Supabase Cloud → self-hosted (solo env vars). Breaking change en `/api/auth` → ADR requerido (ver ADR pendiente).
**Scale/Scope**: Carga inicial baja (< 1k usuarios), diseño preparado para multi-tenant horizontal

---

## Constitution Check

| Principio | Gate | Estado |
|-----------|------|--------|
| I — Arquitectura hexagonal | Transport → App → Domain ← Repo/Security/Platform; sin lógica de negocio entre superficies | ✅ PASS — nuevos handlers siguen patrón `internal/api/handler/[domain]/[action]/` |
| II — Aislamiento de tenant (NO NEGOCIABLE) | Todas las queries incluyen `tenant_id` via `platform.TenantID`; X-Tenant-ID validado antes de cualquier operación | ✅ PASS — middleware `TenantFromHeader` valida contra `user_tenant_roles` antes de pasar al handler |
| III — Observabilidad (NO NEGOCIABLE) | Prometheus counters + Zap logs para cada evento de auth | ✅ PASS — NFR-001 y NFR-002 documentados; implementados en middleware |
| IV — Tests de integración | Migraciones testeadas contra Postgres; Supabase Admin API mockeada en tests | ✅ PASS — plan incluye tests de integración para migrations y contratos HTTP |
| V — Versionado semántico | Eliminar `/api/auth` es breaking → MAJOR version bump + ADR | ⚠️ REQUIERE ADR — documentar en `docs/adr/002-replace-auth-system.md` antes de merge |

**ADR requerido**: La eliminación de `internal/auth/` y el endpoint `/api/auth` constituye un cambio MAJOR de contrato. Se debe crear `docs/adr/002-replace-auth-system.md` documentando la decisión, alternativas evaluadas, y el plan de migración (no hay clientes que consuman `/api/auth` en producción — solo el frontend con mock).

---

## Project Structure

### Documentation (this feature)

```text
specs/002-supabase-auth-backend/
├── plan.md              # Este archivo
├── research.md          # Decisiones de librerías y patrones
├── data-model.md        # Entidades, migraciones, state transitions
├── quickstart.md        # Setup local y variables de entorno
├── contracts/
│   └── api.yaml         # OpenAPI para endpoints nuevos/modificados
└── tasks.md             # Generado por /speckit.tasks
```

### Source Code (repository root)

```text
internal/
  security/
    jwt.go              # REEMPLAZAR: StubVerifier → JWKSVerifier (MicahParks/keyfunc)
    rbac.go             # REEMPLAZAR: Can() stub → validación real contra user_tenant_roles
    apikeys.go          # SIN CAMBIOS
  api/
    middleware/
      middleware.go     # REEMPLAZAR: JWTAuth(), TenantFromJWT() → TenantFromHeader()
                        #            AGREGAR: RBACCheck(), PasswordChangeGuard()
    usecases/
      auth_usecase.go                        # NUEVO: ProvisionUser() — capa intermedia entre JWTAuth y users_repo (Constitution I)
      me_usecase.go                          # NUEVO: GetMe() — ver también Fase 5
    handler/
      me/
        get_me.go                          # NUEVO: GET /api/v1/me
        models/response.go                 # NUEVO
      invitations/
        list_invitations/                  # NUEVO: GET /api/v1/invitations
        create_invitation/                 # NUEVO: POST /api/v1/invitations
        resend_invitation/                 # NUEVO: POST /api/v1/invitations/:id/resend
        revoke_invitation/                 # NUEVO: DELETE /api/v1/invitations/:id
      users/
        force_password_change/             # NUEVO: POST /api/v1/users/:id/force-password-change
    usecases/
      me_usecase.go                        # NUEVO: lógica de GET /api/v1/me + auto-provisioning
      invitation_usecase.go               # NUEVO: lógica de invitaciones + Supabase Admin API
      password_usecase.go                 # NUEVO: lógica de force-password-change
  repo/
    pg/
      users/
        users_repo.go                      # EXTENDER: UpsertBySupabaseID(), GetBySupabaseID()
      invitations/
        invitations_repo.go                # NUEVO: CRUD invitaciones
  domain/
    user.go                                # EXTENDER: SupabaseUserID, PasswordChangeRequired
    invitation.go                          # NUEVO: UserInvitation entity + states
  platform/
    supabase/
      admin_client.go                      # NUEVO: cliente REST para Supabase Admin API
    tenantctx.go                           # SIN CAMBIOS (ya existe)
  auth/                                    # ELIMINAR: todo el paquete
migrations/
  000004_supabase_auth_migration.up.sql    # NUEVO: alter users, drop auth tables
  000004_supabase_auth_migration.down.sql  # NUEVO: rollback
  000005_user_invitations.up.sql           # NUEVO: tabla user_invitations
  000005_user_invitations.down.sql         # NUEVO: rollback
routes/
  url_mappings.go        # MODIFICAR: eliminar auth routes, agregar me/invitations/force-pw-change
docs/
  adr/
    002-replace-auth-system.md             # NUEVO: ADR obligatorio (Constitution gate V)
```

**Structure Decision**: Monolito modular con handler-per-action siguiendo el patrón existente de `internal/api/handler/[domain]/[action]/`. El cliente de Supabase Admin API va en `internal/platform/supabase/` (capa de plataforma, no dominio). Los usecases orquestan la lógica entre repos y el cliente de plataforma.

---

## Complexity Tracking

| Decisión | Por qué necesaria | Alternativa rechazada |
|----------|-------------------|----------------------|
| `MicahParks/keyfunc` para JWKS | Maneja rotación de claves y cache automáticamente, diseñado para JWT + JWKS | Fetch manual: más complejidad, más superficie de bugs en concurrencia |
| `platform/supabase/admin_client.go` separado | El cliente Admin API es una integración externa con su propio ciclo de vida (service role key, retry, timeout) | Inline en usecase: viola el principio de inversión de dependencias, imposible de mockear en tests |
| Redis para rate limiting de invitaciones | Atomicidad cross-request, bajo overhead, ya disponible (Upstash) | In-memory: no funciona en múltiples instancias del servicio |

---

## Implementation Phases

### Fase 1 — Fundamentos de seguridad (P1, prerequisito de todo)

**Objetivo**: El backend rechaza requests sin token válido. Sin esto, la API sigue siendo pública.

1. **Migración 004** — `migrations/000004_supabase_auth_migration.up.sql`:
   - `ALTER TABLE users ADD COLUMN supabase_user_id TEXT UNIQUE, ADD COLUMN auth_provider TEXT, ADD COLUMN email_verified_at TIMESTAMPTZ, ADD COLUMN last_login_at TIMESTAMPTZ, ADD COLUMN password_change_required BOOLEAN NOT NULL DEFAULT FALSE`
   - `ALTER TABLE users DROP COLUMN password_hash`
   - `DROP TABLE sessions CASCADE`
   - `DROP TABLE password_reset_tokens CASCADE`

2. **JWKSVerifier** — `internal/security/jwt.go`:
   - Implementar `JWKSVerifier` usando `MicahParks/keyfunc/v3`
   - Config: `SUPABASE_JWKS_URL`, `SUPABASE_JWT_ISSUER`, `SUPABASE_JWT_AUDIENCE`
   - Cache invalidation automático por `kid` rotation
   - Exponer `NewJWKSVerifier(cfg Config) (Verifier, error)`

3. **Middleware JWTAuth()** — `internal/api/middleware/middleware.go`:
   - Extraer `Authorization: Bearer <token>`, llamar `Verifier.Verify()`
   - En error: 401 con JSON `{"error": "..."}` y log Zap `warn` + counter Prometheus `auth_requests_total{status="401"}`
   - En éxito: inyectar `supabase_user_id` (claim `sub`) en context via `platform.WithUserID()`
   - Triggear auto-provisioning del usuario (ver Fase 2)

4. **Middleware TenantFromHeader()** — rename de `TenantFromJWT()`:
   - Leer header `X-Tenant-ID`
   - Si falta: 400 `{"error": "missing X-Tenant-ID header"}` (excepto `GET /api/v1/me`)
   - Validar que `user_tenant_roles` tiene registro activo para ese user+tenant
   - Si no existe: 403 `{"error": "not a member of this tenant"}`
   - Si existe: inyectar en context via `platform.WithTenantID()`
   - Log Zap + counter `auth_tenant_violations_total` en violaciones

5. **Eliminar `internal/auth/`** y su registro en `url_mappings.go`

### Fase 2 — Identidad y perfil (P1)

**Objetivo**: Auto-provisioning + `GET /api/v1/me`.

6. **Repo de usuarios** — `internal/repo/pg/users/users_repo.go`:
   - `UpsertBySupabaseID(ctx, supabaseUserID, email string) (*domain.User, error)` — `INSERT ... ON CONFLICT (supabase_user_id) DO UPDATE SET email = EXCLUDED.email, last_login_at = NOW()`
   - `GetBySupabaseID(ctx, supabaseUserID string) (*domain.User, error)`
   - `SetPasswordChangeRequired(ctx, userID string, value bool) error`
   - `SetStatus(ctx, userID string, status domain.UserStatus) error`

7. **`auth_usecase.go`** — `internal/api/usecases/auth_usecase.go`:
   - `ProvisionUser(ctx, sub, email string) (*domain.User, error)` — llama `UserRepo.UpsertBySupabaseID()`
   - Respeta arquitectura hexagonal: `JWTAuth()` (transport) → `AuthUsecase` (app) → `UserRepo` (repo)

8. **Auto-provisioning** en `JWTAuth()` middleware (hook post-verificación):
   - Llamar `AuthUsecase.ProvisionUser(ctx, sub, email)` después de validar el token (NO el repo directamente)
   - Idempotente: `ON CONFLICT DO UPDATE` evita duplicados en requests paralelos
   - Si `user.Status == revoked || disabled`: 403 `{"error": "account suspended"}`
   - Si `user.PasswordChangeRequired && endpoint != GET /api/v1/me && endpoint != POST /api/v1/auth/change-password`: 403 `{"error": "password_change_required"}`

8. **GET /api/v1/me** — `internal/api/handler/me/get_me.go`:
   - Leer usuario del context (ya provisioned en middleware)
   - Query `user_tenant_roles` JOIN `roles` JOIN `tenants` para el user
   - Respuesta: `{ user, tenant | null, role | null, permissions: [...], password_change_required }`

### Fase 3 — RBAC real (P1)

**Objetivo**: El stub `Can()` rechaza o permite basado en permisos reales.

9. **RBAC** — `internal/security/rbac.go`:
   - `Can(ctx context.Context, perm string) error` — leer `tenant_id` y `role` del context, consultar mapa de permisos por rol
   - Mapa de permisos derivado de la tabla `roles` (o hardcodeado como constantes — decisión en research)
   - Middleware `RBACCheck(perm string) gin.HandlerFunc` que envuelve `Can()`

10. **`PasswordChangeGuard()`** middleware — verifica el flag del usuario en context y bloquea si corresponde

### Fase 4 — Invitaciones (P2)

**Objetivo**: Admin puede invitar usuarios; el sistema activa invitaciones automáticamente.

11. **Migración 005** — `migrations/000005_user_invitations.up.sql`:
    - Tabla `user_invitations (id UUID PK, tenant_id TEXT FK, email TEXT, role_id UUID FK, status TEXT CHECK ('pending','accepted','revoked','expired'), invited_by UUID FK users, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, expires_at TIMESTAMPTZ)`
    - Índice: `UNIQUE (tenant_id, email)` WHERE `status = 'pending'`

12. **Supabase Admin Client** — `internal/platform/supabase/admin_client.go`:
    - `InviteUserByEmail(ctx, email, redirectTo string) error` → `POST /auth/v1/admin/invite`
    - `SendPasswordResetEmail(ctx, userID string) error` → `POST /auth/v1/admin/generate-link` con `type: recovery`
    - Config: `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `APP_BASE_URL`
    - Timeout: 10s; retry: 1 reintento en 5xx

13. **Repo de invitaciones** — `internal/repo/pg/invitations/invitations_repo.go`:
    - `Create()`, `GetByID()`, `ListByTenant()`, `UpdateStatus()`, `GetPendingByEmailAndTenant()`
    - `CountByTenantInLastHour(tenantID string) (int, error)` — para rate limiting

14. **Invitation usecase** — `internal/api/usecases/invitation_usecase.go`:
    - `CreateInvitation()` — check rate limit → transacción: `invitations.Create()` + `supabase.InviteUserByEmail()` → rollback si Supabase falla
    - `ActivateInvitation()` — llamado desde el hook de auto-provisioning de nuevo usuario
    - `ResendInvitation()` → `supabase.InviteUserByEmail()` + `invitations.UpdateStatus(updated_at)`
    - `RevokeInvitation()` → `invitations.UpdateStatus(revoked)`
    - `ExpireOldInvitations()` — job/query que marca expiradas las invitaciones con `expires_at < NOW()`

15. **Handlers de invitaciones** — seguir patrón `internal/api/handler/invitations/[action]/`:
    - `POST /api/v1/invitations`
    - `GET /api/v1/invitations`
    - `POST /api/v1/invitations/:id/resend`
    - `DELETE /api/v1/invitations/:id`

16. **Rate limiting Redis** — en `CreateInvitation` usecase:
    - Key: `invitations:ratelimit:{tenantID}:{YYYY-MM-DD-HH}`, INCR + EXPIRE 3600s
    - Si count > `INVITATION_RATE_LIMIT_PER_HOUR` (default 20): 429

### Fase 5 — Force password change (P2)

17. **POST /api/v1/users/:id/force-password-change**:
    - RBAC: solo `admin`
    - Validar que el usuario objetivo pertenece al tenant del admin
    - `users.SetPasswordChangeRequired(true)` + `supabase.SendPasswordResetEmail(supabaseUserID)`

18. **POST /api/v1/auth/change-password**:
    - Sin X-Tenant-ID requerido (ruta de auth)
    - Verificar token válido (JWTAuth middleware)
    - `users.SetPasswordChangeRequired(false)` para el `sub` del token
    - Responde 200 `{"message": "password_change_required cleared"}`

### Fase 6 — Observabilidad y ADR

19. **Prometheus metrics** — definir en `internal/telemetry/` o `internal/security/metrics.go`:
    - `auth_requests_total{status}` — contador por código de respuesta de auth
    - `auth_tenant_violations_total` — X-Tenant-ID inválido
    - `invitations_sent_total{tenant_id}` — invitaciones creadas
    - `invitations_expired_total` — invitaciones expiradas
    - `password_change_forced_total` — force-password-change disparados

20. **ADR** — `docs/adr/002-replace-auth-system.md`:
    - Decisión: eliminar `internal/auth/` + `/api/auth` endpoint
    - Contexto: sistema propio con bcrypt y sesiones en DB vs Supabase JWT
    - Alternativas evaluadas: mantener dual-auth, migración gradual
    - Consecuencias: MAJOR version bump, no hay clientes en producción aún

---

## Testing Strategy

| Tipo | Qué cubre | Dónde |
|------|-----------|-------|
| Unit | `JWKSVerifier` (mock HTTP server), `Can()` con permisos, rate limiting lógica | `internal/security/*_test.go` |
| Integration | Migraciones 004 y 005 contra Postgres real (Docker), `UpsertBySupabaseID` idempotencia | `internal/repo/pg/users/*_test.go` |
| Contract | Endpoints `/api/v1/me`, `/api/v1/invitations` contra spec OpenAPI | `specs/002-supabase-auth-backend/contracts/api.yaml` |
| E2E | Token real de Supabase Cloud → `GET /api/v1/me` retorna 200 con perfil | Manual / CI con Supabase test project |

---

## Environment Variables

| Variable | Descripción | Requerida |
|----------|-------------|-----------|
| `SUPABASE_JWKS_URL` | URL del endpoint JWKS del proveedor | Sí |
| `SUPABASE_JWT_ISSUER` | Issuer esperado en el claim `iss` del JWT | Sí |
| `SUPABASE_JWT_AUDIENCE` | Audience esperado (`aud` claim) | Sí |
| `SUPABASE_URL` | URL base de la instancia Supabase (ej: `https://xyz.supabase.co`) | Sí |
| `SUPABASE_SERVICE_ROLE_KEY` | Service role key para la API Admin | Sí |
| `APP_BASE_URL` | URL pública del frontend (ej: `https://app.tudominio.com`) | Sí |
| `INVITATION_RATE_LIMIT_PER_HOUR` | Máx invitaciones por tenant por hora (default: 20) | No |
| `DATABASE_URL` | PostgreSQL connection string (Neon) | Sí (existente) |
| `REDIS_URL` | Redis connection string (Upstash) | Sí (existente) |

**Self-hosted path**: Cambiar `SUPABASE_JWKS_URL` a `https://auth.tudominio.com/auth/v1/.well-known/jwks.json` y `SUPABASE_URL` al endpoint self-hosted. Sin cambios de código.

---

## Rollout Order

```
Fase 1 (migraciones + JWT) → Fase 2 (me + auto-provision) → Fase 3 (RBAC) → Fase 4 (invitaciones) → Fase 5 (force-pw) → Fase 6 (observabilidad + ADR)
```

Cada fase es deployable independientemente. La Fase 1 puede deployarse con feature flag: si `SUPABASE_JWKS_URL` no está definida, el backend usa `StubVerifier` (comportamiento actual). Esto permite deploy gradual en Koyeb sin downtime.

---
id: 002b
title: "Supabase Auth — Backend"
tier: feature
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-03-06
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: [94]
prs: [16, 43]
adrs: [CLOUD-ADR-005, CLOUD-ADR-015, CLOUD-ADR-016]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 002b — Supabase Auth — Backend

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-03-06,
> conservada en [`design.md`](design.md), con su número `FR-NNN`/`NFR-NNN` original.
> `origen: derivado` se reconstruyó del código, incluidos los fixes de los PRs #43
> (endpoints reales de GoTrue), #53 (mails, spec [021](../021-auth-emails/spec.md)) y #65
> (activación de invitaciones). **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

El backend tenía auth propio en `internal/auth/`: sesiones en base, bcrypt, JWT HS256 con
secreto compartido y tokens de reset propios. Era mantenimiento continuo, con riesgo de
seguridad, y duplicaba lo que ya ofrece Supabase. La decisión de reemplazarlo es
[CLOUD-ADR-005](../../docs/adr/CLOUD-ADR-005-supabase-auth.md). Esta spec define qué tiene
que hacer el backend para que Supabase sea la única fuente de identidad: validar tokens,
mapear la identidad a un usuario local, resolver el tenant, gestionar invitaciones y
forzar cambios de contraseña.

## Alcance

**Entra:**

- Validación de JWT por JWKS y auto-provisioning.
- `GET /api/v1/me`.
- Resolución de tenant por `X-Tenant-ID`.
- Invitaciones: alta, listado, reenvío y revocación.
- Cambio de contraseña forzado.
- Eliminación del auth propio.

**No entra:**

- El login de usuarios: lo hace el frontend contra Supabase. El backend solo expone un
  proxy público, ver RF-017.
- El contenido de los mails, que cubre [021](../021-auth-emails/spec.md).
- El acceso cross-tenant de operadores de plataforma, que cubre CLOUD-ADR-015.

## Requisitos

### Validación de tokens

- **RF-001** `origen: original` (FR-001, FR-002) — El backend DEBE validar la firma de
  cada JWT con las claves del JWKS configurado en `SUPABASE_JWKS_URL`, sin URL fija en el
  código, y cachear las claves, refrescándolas solo ante un `kid` desconocido.
  → `security.NewJWKSVerifier` (`keyfunc.NewDefault`), `internal/security/jwt.go`.
- **RF-003** `origen: original` (FR-003) — El backend DEBE rechazar con 401 los tokens
  con firma inválida, vencidos o con `iss`/`aud` distintos de `SUPABASE_JWT_ISSUER` y
  `SUPABASE_JWT_AUDIENCE`. La expiración es obligatoria.
  → `jwt.WithIssuer`, `jwt.WithAudience`, `jwt.WithExpirationRequired`; `JWTAuth`.
- **RF-018** `origen: derivado` — Si el JWKS no responde, el backend DEBE devolver
  **503**, no 401: un proveedor caído no es un token inválido.
  → sentinel `ErrJWKSUnavailable`, `JWTAuth`.

### Identidad y tenant

- **RF-004** `origen: original` (FR-004) — El backend DEBE auto-provisionar el usuario
  local en el primer request de un `supabase_user_id` desconocido, de forma idempotente,
  incluso con requests concurrentes.
  → `AuthUsecase.ProvisionUser`, `UpsertBySupabaseID` (`ON CONFLICT (supabase_user_id)`).
- **RF-005** `origen: original` (FR-005) — `GET /api/v1/me` DEBE devolver identidad,
  tenant, rol y permisos del usuario autenticado. **Derivado:** los permisos se leen en
  vivo de `roles.permissions`.
  → `internal/api/usecases/me_usecase.go`.
- **RF-006** `origen: original` (FR-006, FR-013) — El tenant activo DEBE leerse del header
  `X-Tenant-ID`, nunca de la URL, el query string o el body, y validarse contra una
  membresía activa en `user_tenant_roles`.
  → `TenantFromHeader`. Las excepciones de path de edge devices e ingesta están en
  [CLOUD-ADR-016](../../docs/adr/CLOUD-ADR-016-resolucion-de-tenant.md).
- **RF-009** `origen: original` (FR-009) — Un usuario sin membresía activa en el tenant
  DEBE recibir 403, salvo en `GET /me` y `POST /auth/change-password`. **Derivado:** los
  operadores de plataforma tienen fallback cross-tenant (CLOUD-ADR-015).
  → `TenantFromHeader`, `isExemptFromTenant`.
- **RF-007** `origen: original` (FR-007) — Un usuario con estado `revoked` o `disabled`
  DEBE recibir 403 (`account suspended`).
  → `JWTAuth`, `domain.UserStatusRevoked` y `domain.UserStatusDisabled`.
- **RF-014** `origen: original` (FR-014) — Un usuario con `password_change_required`
  DEBE recibir 403 `password_change_required` en todas las rutas, salvo `GET /me` y
  `POST /auth/change-password`.
  → `PasswordChangeGuard`.

### Invitaciones

- **RF-008** `origen: original` (FR-008) — El backend DEBE exponer
  `POST /api/v1/invitations`, `GET /api/v1/invitations`,
  `POST /api/v1/invitations/:id/resend` y `DELETE /api/v1/invitations/:id`. **Derivado:**
  las tres escrituras exigen `perm_users_manage`.
  → `internal/routes/url_mappings.go`.
- **RF-015** `origen: original` (FR-015), **con cambios derivados** — Crear una
  invitación DEBE pedir a Supabase que envíe el mail con `redirect_to` apuntando a
  `{base}/s/{tenantId}/auth/callback`, con vencimiento a los 7 días.
  - **Derivado (PR #43):** se usa el endpoint real de GoTrue `/auth/v1/invite`, no una ruta
    `admin`.
  - **Derivado (021):** `{base}` es el origin del frontend que disparó la invitación
    (header `X-App-Base-URL` validado contra `APP_ALLOWED_ORIGINS`), con `APP_BASE_URL`
    como fallback.
  - **Derivado:** si Supabase falla, la invitación ya creada se marca `revoked` como
    compensación. No es un rollback transaccional.
  → `InvitationUsecase`, `callbackURL` (`invite_metadata.go`),
  `InvitationRepository` (`NOW() + INTERVAL '7 days'`).
- **RF-010** `origen: original` (FR-010) — En el primer acceso de un usuario con
  invitaciones pendientes, el backend DEBE aceptarlas y activar la membresía
  correspondiente.
  → `JWTAuth` → `InvitationUsecase.ActivatePendingInvitations` (si el usuario está
  `invited`). Si la activación falla responde 500 `activation failed`; el fallo silencioso
  anterior se corrigió en el PR #65.
- **RNF-003** `origen: original` (NFR-003) — Crear invitaciones DEBE tener un límite por
  tenant y por hora (`INVITATION_RATE_LIMIT_PER_HOUR`, 20 por defecto), con 429
  `invitation rate limit exceeded`. **Derivado:** sin Redis, el límite falla abierto.
  → `InvitationUsecase.checkRateLimit`, `create_invitation.go`.

### Contraseñas

- **RF-016** `origen: original` (FR-016) — `POST /api/v1/users/:id/force-password-change`
  DEBE marcar `password_change_required = true` y pedir a Supabase el mail de reset.
  **Derivado:** usa `/auth/v1/recover` (PR #43/#53) y exige `perm_users_manage`.
  → `PasswordUsecase.ForcePasswordChange`, `supabase.AdminClient.SendPasswordResetEmail`.
- **RF-019** `origen: derivado` — `POST /api/v1/auth/change-password` DEBE limpiar
  `password_change_required` del usuario autenticado.
  → `PasswordUsecase.ClearPasswordChangeRequired`,
  `internal/api/handler/auth/change_password`.

### Eliminación del auth propio

- **RF-011** `origen: original` (FR-011) — No DEBE quedar código ni rutas del auth propio
  (`internal/auth/`, `/api/auth/*`).
- **RF-012** `origen: original` (FR-012) — El esquema NO DEBE tener `password_hash`,
  `sessions` ni `password_reset_tokens`, y `users` DEBE tener `supabase_user_id`,
  `auth_provider`, `email_verified_at`, `last_login_at` y `password_change_required`.
  → `migrations/000001_initial_schema.up.sql`, consolidada (CLOUD-ADR-014).
- **RF-017** `origen: derivado` — `POST /api/v1/auth/login` es un proxy público y sin
  sesión hacia Supabase, que usa `SUPABASE_ANON_KEY`. No es auth propio: no guarda
  credenciales ni emite tokens.
  → `internal/api/handler/auth/login`.

### Observabilidad

- **RNF-001** `origen: original` (NFR-001) — El backend DEBE emitir `auth_requests_total`,
  `auth_tenant_violations_total`, `invitations_sent_total`, `invitations_expired_total` y
  `password_change_forced_total`.
  → `internal/telemetry/metrics.go`.
- **RNF-002** `origen: original` (NFR-002) — Los eventos de seguridad DEBEN loguearse con
  Zap en `warn` o `error`, con usuario, tenant, endpoint y motivo.
  → `JWTAuth` y `TenantFromHeader` (por ejemplo, `tenant access denied`).

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-001** | Todo request a `/api/v1/*` sin token válido responde 401 antes de la lógica de negocio | original (SC-001) | `JWTAuth` como primer middleware del grupo; `internal/api/middleware/*_test.go` |
| **CE-002** | Un usuario revocado o sin membresía responde 403 | original (SC-002) | `JWTAuth`, `tenant_from_header_test.go` |
| **CE-004** | El auth propio no existe en el repo | original (SC-004) | no hay `internal/auth/`; el esquema no tiene esas tablas |
| **CE-005** | Cambiar `SUPABASE_JWKS_URL` alcanza para pasar a un Supabase self-hosted, sin cambios de código | original (SC-005) | URL y emisor configurables (`internal/config/config.go`); no probado contra self-hosted |
| **CE-006** | Dos requests simultáneos de un `supabase_user_id` nuevo crean exactamente un usuario | original (SC-006) | `internal/repo/pg/users/users_repo_test.go` (`TestUpsertBySupabaseID_Idempotency`) |

SC-003 (p95 de `/me` menor a 300 ms) **no tiene medición**: no se traslada como cumplido.

## Decisiones

| Decisión | Por qué | Fuente |
|---|---|---|
| Supabase Auth en vez de auth propio | Menos mantenimiento y superficie de seguridad; OAuth y rotación de claves gratis | CLOUD-ADR-005 |
| RS256 por JWKS, sin secreto compartido | Rotación automática; migrar a self-hosted solo cambia configuración | design (FR-001) |
| Tenant por header y no por claim | Un usuario puede ser miembro de varios tenants | CLOUD-ADR-016 |
| 503 si el JWKS no responde | Distinguir proveedor caído de token inválido | derivado |

## Riesgos y preguntas abiertas

- **Algoritmo de firma:** el verificador no fija `WithValidMethods`, así que confía en
  que `keyfunc` rechace un `alg` que no coincide con la clave del JWKS. Conviene fijar
  RS256 explícitamente.
- **Invitaciones con roles de plataforma en tenants cliente:** ver la nota de la
  migración `000010` en `migrations/README.md`.
- **Latencia de `/me`** (SC-003 original): sin verificar.

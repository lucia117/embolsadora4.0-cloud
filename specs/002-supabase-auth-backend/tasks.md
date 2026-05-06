# Tasks: Supabase Auth — Backend

**Input**: Design documents from `/specs/002-supabase-auth-backend/`
**Prerequisites**: plan.md ✅ | spec.md ✅ | research.md ✅ | data-model.md ✅ | contracts/api.yaml ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Puede correr en paralelo (archivos distintos, sin dependencias incompletas)
- **[Story]**: User story a la que pertenece la tarea (US1–US8 del spec.md)

---

## Phase 1: Setup

**Purpose**: ADR obligatorio (Constitution gate V) y dependencias nuevas

- [x] T001 Crear `docs/adr/002-replace-auth-system.md` documentando la decisión de reemplazar `internal/auth/` (breaking change MAJOR — gate de la Constitución)
- [x] T002 Agregar `github.com/MicahParks/keyfunc/v3` a `go.mod` ejecutando `go get github.com/MicahParks/keyfunc/v3 && go mod tidy`
- [x] T003 [P] Agregar variables de entorno Supabase al struct de config en `internal/config/` (`SUPABASE_JWKS_URL`, `SUPABASE_JWT_ISSUER`, `SUPABASE_JWT_AUDIENCE`, `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `APP_BASE_URL`, `INVITATION_RATE_LIMIT_PER_HOUR`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Migraciones de schema — deben completarse antes de cualquier implementación

**⚠️ CRITICAL**: No puede comenzar ninguna user story hasta que las migraciones estén escritas y aplicadas

- [x] T004 Escribir `migrations/000004_supabase_auth_migration.up.sql` (per data-model.md: ALTER TABLE users ADD supabase_user_id/auth_provider/email_verified_at/last_login_at/password_change_required; DROP COLUMN password_hash; DROP TABLE sessions; DROP TABLE password_reset_tokens)
- [x] T005 [P] Escribir `migrations/000004_supabase_auth_migration.down.sql`
- [x] T006 [P] Escribir `migrations/000005_user_invitations.up.sql` (per data-model.md: CREATE TABLE user_invitations con índice único parcial WHERE status='pending')
- [x] T007 [P] Escribir `migrations/000005_user_invitations.down.sql`
- [ ] T008 Aplicar migración 004 en la DB de desarrollo: `migrate -path migrations/ -database $DATABASE_URL up 1` ← **PENDIENTE: ejecutar manualmente**

**Checkpoint**: Schema actualizado — implementación de user stories puede comenzar

---

## Phase 3: US1 + US7 — JWT validation y middleware chain (Priority: P1) 🎯 MVP

**Goal**: El backend rechaza requests sin token válido y resuelve el tenant desde `X-Tenant-ID`. Eliminar `internal/auth/`.

**Independent Test**:
- `curl http://localhost:8080/api/v1/me` → 401 `{"error":"missing token"}`
- `curl -H "Authorization: Bearer <token_válido>" -H "X-Tenant-ID: demo" http://localhost:8080/api/v1/me` → pasa al handler
- `curl -H "Authorization: Bearer <token_válido>" -H "X-Tenant-ID: otro_tenant" http://localhost:8080/api/v1/me` → 403

- [x] T009 [US1] Implementar `JWKSVerifier` en `internal/security/jwt.go` reemplazando `StubVerifier`: usar `keyfunc.NewDefault([]string{cfg.JWKSUrl})` + `jwt.Parse` con validación de `iss`, `aud`, `exp`
- [x] T010 [US1] Implementar `JWTAuth()` en `internal/api/middleware/middleware.go`: extraer Bearer token, llamar `Verifier.Verify()`, retornar 401 en error (retornar 503 `{"error":"auth service unavailable"}` si el error es de tipo network/JWKS fetch failure — usar error sentinel en el verifier), inyectar `sub` en context via `platform.WithUserID()`
- [x] T011 [P] [US1] Implementar `TenantFromHeader()` en `internal/api/middleware/middleware.go` (reemplaza `TenantFromJWT()`): leer header `X-Tenant-ID`, retornar 400 si falta (excepto path `/api/v1/me`), validar membresía en `user_tenant_roles`, retornar 403 si no existe, inyectar via `platform.WithTenantID()`
- [x] T012 [P] [US1] Implementar `RequestID()`, `Logger()`, `CORS()` reales en `internal/api/middleware/middleware.go` (reemplazar stubs vacíos)
- [x] T013 [US1] Eliminar paquete `internal/auth/` completo: borrar `service.go`, `handlers.go`, `routes.go`, `repository.go`, `models.go`
- [x] T014 [US1] Actualizar `internal/routes/url_mappings.go`: eliminar `auth.RegisterRoutes(authGroup, db)` y el grupo `/api/auth`; renombrar `TenantFromJWT()` → `TenantFromHeader()` en el middleware chain de `/api/v1`

**Checkpoint**: `go build ./...` pasa sin errores; requests sin token reciben 401

---

## Phase 4: US2 — Primer acceso: auto-provisioning (Priority: P1)

**Goal**: El primer request de un usuario nuevo crea su registro en `users` de forma idempotente.

**Independent Test**: Crear usuario en Supabase Cloud. Obtener token. `GET /api/v1/me` → 200 (aunque devuelva perfil vacío). Repetir el request: exactamente un registro en `users`.

- [x] T015 [P] [US2] Extender `internal/domain/user.go`: agregar campos `SupabaseUserID`, `AuthProvider`, `EmailVerifiedAt`, `LastLoginAt`, `PasswordChangeRequired`; agregar tipo `UserStatus` con constantes `invited/active/revoked/disabled`
- [x] T016 [P] [US2] Implementar en `internal/repo/pg/users/users_repo.go`: `UpsertBySupabaseID(ctx, supabaseUserID, email string) (*domain.User, error)` usando `INSERT ... ON CONFLICT (supabase_user_id) DO UPDATE SET email=EXCLUDED.email, last_login_at=NOW() RETURNING *`; `GetBySupabaseID(ctx, id string) (*domain.User, error)`; `SetStatus(ctx, userID string, status domain.UserStatus) error`
- [x] T017 [US2] Crear `internal/api/usecases/auth_usecase.go` con `ProvisionUser(ctx context.Context, sub, email string) (*domain.User, error)` que llama `UserRepo.UpsertBySupabaseID()`; agregar hook en `JWTAuth()` que llama `AuthUsecase.ProvisionUser()` (no el repo directamente — respeta arquitectura hexagonal) e inyecta el `domain.User` en el context. Diseñar `JWTAuth()` para aceptar un `InvitationActivator` opcional (interfaz, nil hasta Phase 7) que será inyectado en T031.
- [x] T018 [US2] Agregar checks de status en `JWTAuth()` después del provisioning: si `user.Status == revoked || disabled` → 403 `{"error":"account suspended"}`

**Checkpoint**: Usuario nuevo en Supabase + token → `users` tiene el registro; usuario revocado → 403

---

## Phase 5: US3 — Perfil propio: GET /api/v1/me (Priority: P1)

**Goal**: El frontend puede obtener identidad, tenant, rol y permisos post-login.

**Independent Test**: Con token de usuario con `user_tenant_role` activo en tenant `demo` + `X-Tenant-ID: demo` → `GET /api/v1/me` devuelve `{user, tenant:{id:"demo",...}, role:{name:"admin"}, permissions:[...]}`. Sin rol asignado → `{user, tenant:null, role:null, permissions:[]}`.

- [x] T019 [P] [US3] Crear `internal/api/handler/me/models/response.go` con structs `MeResponse`, `UserProfileResponse`, `TenantInfoResponse`, `RoleInfoResponse`
- [x] T020 [P] [US3] Crear `internal/api/usecases/me_usecase.go`: query `user_tenant_roles JOIN roles JOIN tenants` para el usuario del context; retornar `MeResponse`
- [x] T021 [US3] Crear `internal/api/handler/me/get_me.go`: handler que llama `me_usecase.GetMe(ctx)` y serializa la respuesta
- [x] T022 [US3] Registrar `GET /api/v1/me` en `internal/routes/url_mappings.go` dentro del grupo `/api/v1` (con `JWTAuth()` pero sin `TenantFromHeader()` y sin `PasswordChangeGuard()`)

**Checkpoint**: `GET /api/v1/me` funciona end-to-end con token real de Supabase

---

## Phase 6: US4 — RBAC real: tenant scope (Priority: P1)

**Goal**: El stub `Can()` se reemplaza con validación real; un `operario` no puede ejecutar acciones de `admin`.

**Independent Test**: Autenticar como `operario`. `DELETE /api/v1/invitations/:id` → 403 `{"error":"forbidden"}`. Autenticar como `admin` → 200.

- [x] T023 [US4] Implementar mapa `rolePermissions` y función `Can(ctx context.Context, perm string) error` en `internal/security/rbac.go` (per research.md): mapa hardcodeado `admin/operario/cliente_admin/cliente_operario` → permisos `"resource:action"`
- [x] T024 [US4] Implementar `RBACCheck(perm string) gin.HandlerFunc` en `internal/api/middleware/middleware.go`: wrapper que llama `security.Can(ctx, perm)` y retorna 403 en error
- [x] T025 [US4] Implementar `PasswordChangeGuard()` gin.HandlerFunc en `internal/api/middleware/middleware.go`: leer `domain.User` del context, si `PasswordChangeRequired == true` retornar 403 `{"error":"password_change_required"}` (primera y única implementación de esta función — no fue creada antes)
- [x] T026 [US4] Aplicar `RBACCheck("invitations:write")` a `POST /api/v1/invitations` y `RBACCheck("users:write")` a `POST /api/v1/users/:id/force-password-change` en `internal/routes/url_mappings.go`; agregar `PasswordChangeGuard()` al chain de `/api/v1` (excepto `/me` y `/auth/change-password`)

**Checkpoint**: RBAC funciona — permisos por rol correctamente aplicados

---

## Phase 7: US5 — Admin invita usuarios al tenant (Priority: P2)

**Goal**: Admin crea invitación → DB registra + Supabase envía email con link de callback correcto.

**Independent Test**: `POST /api/v1/invitations` con admin token + X-Tenant-ID → 201; registro en `user_invitations`; el email invitado recibe email de Supabase con link apuntando a `/s/{tenantId}/auth/callback`.

- [x] T027 [P] [US5] Crear `internal/domain/invitation.go` con tipo `InvitationStatus` (pending/accepted/revoked/expired), struct `UserInvitation`, método `IsExpired() bool`
- [x] T028 [P] [US5] Crear `internal/platform/supabase/admin_client.go` con interface `AdminClient` y métodos `InviteUserByEmail(ctx, email, redirectTo string) error` (POST `/auth/v1/admin/invite`) y `SendPasswordResetEmail(ctx, userEmail string) error` (POST `/auth/v1/admin/generate-link` con `type:"recovery"`); timeout 10s; 1 retry en 5xx
- [x] T029 [P] [US5] Crear `internal/repo/pg/invitations/invitations_repo.go` con `Create()` y `GetPendingByEmailAndTenant()`. El rate limiting NO es responsabilidad de este repo — usa Redis directamente en el usecase (T030). No agregar métodos de conteo en Postgres para este fin.
- [x] T030 [US5] Crear `internal/api/usecases/invitation_usecase.go` con `CreateInvitation()`: check rate limit Redis (`invitations:ratelimit:{tenantID}:{YYYY-MM-DD-HH}` INCR+EXPIRE 3600s) → transacción: `invitations.Create()` + `supabase.InviteUserByEmail(ctx, email, APP_BASE_URL+"/s/"+tenantID+"/auth/callback")` → rollback si Supabase falla
- [x] T031 [US5] Agregar `ActivateInvitation()` en `invitation_usecase.go`: al auto-provisionar en `JWTAuth()`, si existe invitación `pending` para el email+tenantID → UPDATE status='accepted' + UPDATE users SET status='active' + CREATE user_tenant_role
- [x] T032 [US5] Crear `internal/api/handler/invitations/create_invitation/` con `create_invitation.go` y `models/request.go`, `models/response.go`
- [x] T033 [US5] Registrar `POST /api/v1/invitations` en `internal/routes/url_mappings.go` con `RBACCheck("invitations:write")`
- [ ] T034 [US5] Aplicar migración 005: `migrate -path migrations/ -database $DATABASE_URL up 1` ← **PENDIENTE: ejecutar manualmente**

**Checkpoint**: `POST /api/v1/invitations` → 201; email llega; segundo POST con mismo email → 409; 21+ invitaciones/hora → 429

---

## Phase 8: US6 — Gestión del ciclo de vida de invitaciones (Priority: P2)

**Goal**: Admin puede listar, reenviar y revocar invitaciones.

**Independent Test**: `GET /api/v1/invitations` devuelve lista del tenant; `POST /resend` en invitación pending → 200; `DELETE /:id` → status 'revoked'.

- [x] T035 [P] [US6] Crear `internal/api/handler/invitations/list_invitations/` con handler + `models/response.go`
- [x] T036 [P] [US6] Crear `internal/api/handler/invitations/resend_invitation/` con handler + models
- [x] T037 [P] [US6] Crear `internal/api/handler/invitations/revoke_invitation/` con handler + models
- [x] T038 [US6] Agregar `ListByTenant(ctx, tenantID string, status *string) ([]domain.UserInvitation, error)`, `UpdateStatus(ctx, id string, status domain.InvitationStatus) error`, `GetByID(ctx, id, tenantID string) (*domain.UserInvitation, error)` a `internal/repo/pg/invitations/invitations_repo.go`
- [x] T039 [US6] Registrar `GET /api/v1/invitations`, `POST /api/v1/invitations/:id/resend`, `DELETE /api/v1/invitations/:id` en `internal/routes/url_mappings.go`

**Checkpoint**: Ciclo completo de invitaciones funcional desde la API

---

## Phase 9: US8 — Admin fuerza cambio de contraseña (Priority: P2)

**Goal**: Admin fuerza reset; usuario queda bloqueado con 403 hasta completar el cambio; frontend llama `POST /api/v1/auth/change-password` para limpiar el flag.

**Independent Test**: `POST /api/v1/users/:id/force-password-change` → 200 + email enviado; siguiente request del usuario afectado (excepto `/me`) → 403 `{"error":"password_change_required"}`; `POST /api/v1/auth/change-password` con nuevo token → 200; acceso restaurado.

- [x] T040 [P] [US8] Crear `internal/api/handler/users/force_password_change/` con handler + `models/request.go`, `models/response.go`
- [x] T041 [P] [US8] Crear `internal/api/handler/auth/change_password/` con handler + models
- [x] T042 [US8] Crear `internal/api/usecases/password_usecase.go` con `ForcePasswordChange(ctx, targetUserID string) error` (validar scope de tenant → `users.SetPasswordChangeRequired(true)` + `supabase.SendPasswordResetEmail(userEmail)`) y `ClearPasswordChangeRequired(ctx, supabaseUserID string) error`
- [x] T043 [US8] Agregar `SetPasswordChangeRequired(ctx, userID string, value bool) error` a `internal/repo/pg/users/users_repo.go`
- [x] T044 [US8] Registrar `POST /api/v1/users/:id/force-password-change` (con `RBACCheck("users:write")`) y `POST /api/v1/auth/change-password` (sin `TenantFromHeader`, sin `PasswordChangeGuard`) en `internal/routes/url_mappings.go`

**Checkpoint**: Force-password-change funciona end-to-end; flag limpiado correctamente post-reset

---

## Phase 10: Polish & Observabilidad

**Purpose**: NFR-001 (Prometheus), NFR-002 (Zap logs), validación final

- [x] T045 [P] Definir contadores Prometheus en `internal/telemetry/auth_metrics.go` (o `internal/security/metrics.go`): `auth_requests_total{status}`, `auth_tenant_violations_total`, `invitations_sent_total`, `invitations_expired_total`, `password_change_forced_total`; registrarlos en el router
- [x] T046 [P] Agregar eventos Zap estructurados en `JWTAuth()` y `TenantFromHeader()`: loguear `supabase_user_id`, `tenant_id`, `endpoint`, `status_code`, `reason` en nivel `warn`/`error` para cada violación de auth
- [x] T047 [P] Agregar logs Zap en `invitation_usecase.go` y `password_usecase.go`: invitación creada, reenvío, revocación, force-password-change
- [x] T048 Ejecutar `quickstart.md` completo: todos los curl commands contra el servidor local; confirmar respuestas esperadas — validado 2026-03-09
- [x] T049 [P] Escribir test de integración para idempotencia de `UpsertBySupabaseID` en `internal/repo/pg/users/users_repo_test.go`: 2 goroutines simultáneas con el mismo `supabase_user_id` → exactamente 1 registro
- [x] T050 [P] Escribir unit test para `JWKSVerifier` en `internal/security/jwt_test.go`: mock HTTP server sirviendo JWKS válido; verificar token válido → ok; token con `kid` desconocido → refresh y re-verify; token expirado → error; JWKS endpoint caído → error tipo `ErrJWKSUnavailable` (usado en T010 para retornar 503)
- [x] T051 [P] Benchmark de `GET /api/v1/me`: ab 10c=P95 374ms / 50c=P95 2536ms (cuello botella DB pool). SC-003 pendiente de validar en Koyeb con DB_MAX_CONNS=25. Documentado en quickstart.md — 2026-03-09
- [x] T052 [P] Test de contrato para `AdminClient` en `internal/platform/supabase/admin_client_test.go`: mock HTTP server que valida request body y Authorization header para `InviteUserByEmail` y `SendPasswordResetEmail`; verificar comportamiento en 5xx (retry) y 4xx (no retry)
- [x] T053 Bump versión MAJOR: actualizar constante de versión del servidor; tagear `v2.0.0-alpha`; actualizar `docs/openapi.yaml` con los nuevos endpoints y la eliminación de `/api/auth`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sin dependencias — puede empezar inmediatamente
- **Foundational (Phase 2)**: Depende de Phase 1 — bloquea todas las user stories
- **Phase 3 (US1+US7)**: Depende de Phase 2
- **Phase 4 (US2)**: Depende de Phase 3 (necesita `JWTAuth()` implementado)
- **Phase 5 (US3)**: Depende de Phase 4 (necesita auto-provisioning para tener `domain.User` en context)
- **Phase 6 (US4)**: Depende de Phase 5 (necesita roles del usuario para evaluar permisos)
- **Phase 7 (US5)**: Depende de Phase 6 (necesita RBAC para proteger el endpoint) + migration 005 (T034)
- **Phase 8 (US6)**: Depende de Phase 7 (mismos repos y usecases de invitaciones)
- **Phase 9 (US8)**: Depende de Phase 7 (`admin_client.go` ya existe)
- **Phase 10**: Depende de todas las fases anteriores

### Parallel Opportunities

```bash
# Phase 1 — en paralelo:
T002 (go get keyfunc) || T003 (config struct)

# Phase 2 — en paralelo:
T004 (migration 004 up) || T005 (migration 004 down) || T006 (migration 005 up) || T007 (migration 005 down)

# Phase 3 — en paralelo después de T009 y T010:
T011 (TenantFromHeader) || T012 (RequestID/Logger/CORS)

# Phase 4 — en paralelo:
T015 (domain.User) || T016 (users_repo)

# Phase 5 — en paralelo:
T019 (me response models) || T020 (me_usecase)

# Phase 7 — en paralelo:
T027 (domain.invitation) || T028 (supabase admin_client) || T029 (invitations_repo)

# Phase 8 — en paralelo:
T035 (list handler) || T036 (resend handler) || T037 (revoke handler)

# Phase 9 — en paralelo:
T040 (force-pw handler) || T041 (change-password handler)

# Phase 10 — en paralelo:
T045 (metrics) || T046 (auth logs) || T047 (usecase logs) || T049 (idempotency test) || T050 (jwt test)
```

---

## Implementation Strategy

### MVP (Fases 1–5, US1+US2+US3)

1. Phase 1: Setup (ADR + dependencias)
2. Phase 2: Migraciones 004
3. Phase 3: JWT + middleware chain + eliminar `internal/auth/`
4. Phase 4: Auto-provisioning
5. Phase 5: `GET /api/v1/me`
6. **STOP y VALIDAR**: Token de Supabase real → `/api/v1/me` devuelve perfil → deploy en Koyeb

El MVP ya tiene el backend seguro con JWT real y auto-provisioning. El frontend puede integrarse aquí.

### Incremental Delivery

- **MVP** (Fases 1–5): JWT seguro + `/api/v1/me` → deploy inicial en Koyeb
- **+RBAC** (Fase 6): Permisos por rol → seguridad completa
- **+Invitaciones** (Fases 7–8): Onboarding de usuarios → feature completa
- **+Force-pw-change** (Fase 9): Gestión de credenciales → seguridad avanzada
- **+Observabilidad** (Fase 10): Producción-ready

---

## Notes

- Total tasks: **53**
- Tasks por fase: Setup(3), Foundational(5), US1+US7(6), US2(4), US3(4), US4(4), US5(8), US6(5), US8(5), Polish(9)
- `[P]` tasks en parallel: ~28 de 53
- Cada fase tiene un checkpoint de validación independiente
- Siempre ejecutar `go build ./...` después de cada phase antes de avanzar
- La migración 005 se aplica en T034 (dentro de US5) para no bloquear las fases anteriores

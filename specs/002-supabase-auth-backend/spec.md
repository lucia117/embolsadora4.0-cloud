# Feature Specification: Supabase Auth — Backend

**Feature Branch**: `002-supabase-auth-backend`
**Created**: 2026-03-06
**Status**: Draft
**Decisión**: Opción A — Supabase reemplaza el sistema de auth propio del backend.
**Deployment inicial**: Koyeb (Go API) + Neon (PostgreSQL) + Upstash (Redis) + Supabase Cloud (Auth)
**Self-hosted path**: El switch de Supabase Cloud a self-hosted requiere solo cambio de variables de entorno, sin modificar código.

---

## Contexto y qué se elimina

El backend tiene hoy un sistema de auth propio completamente implementado en `internal/auth/`
(login con bcrypt, sesiones en DB, password reset con tokens). Ese sistema se elimina en su totalidad.
El JWT middleware (`internal/security/jwt.go`) y el RBAC (`internal/security/rbac.go`) existen como stubs
que siempre aprueban todo. Esos stubs se reemplazan con implementaciones reales.

**Se elimina:**
- Paquete completo `internal/auth/` (service, handlers, repository, models, routes)
- Tablas `sessions` y `password_reset_tokens` (migration de limpieza)
- Campo `password_hash` de la tabla `users`
- El paquete `internal/auth` deja de registrar rutas

**Se construye:**
- Validador JWT real usando JWKS del proveedor de identidad (URL configurable por env var)
- RBAC real basado en `user_tenant_roles` (ya existe la tabla)
- Endpoint `GET /api/v1/me`
- Migración que altera `users` y crea `user_invitations`
- Endpoints de gestión de invitaciones

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — El backend rechaza requests sin token válido (Priority: P1)

Como operador del sistema, necesito que el backend rechace cualquier request que no traiga un token JWT
válido emitido por Supabase, para que ningún dato de negocio sea accesible sin autenticación real.

**Why this priority**: Sin esto, toda la API es pública. Es el prerequisito de todo lo demás.

**Independent Test**: Enviar un request a `GET /api/v1/me` sin header, con token expirado, y con token válido.
Las tres respuestas deben ser 401, 401, y 200 respectivamente. Sin UI. Sin frontend.

**Acceptance Scenarios**:

1. **Given** un request llega a cualquier endpoint bajo `/api/v1/` sin header `Authorization`, **When** el middleware evalúa el request, **Then** responde 401 `{"error": "missing token"}` y no ejecuta el handler.
2. **Given** un request llega con `Authorization: Bearer <token_expirado>`, **When** el middleware valida la firma contra el JWKS del proveedor, **Then** responde 401 `{"error": "token expired"}`.
3. **Given** un request llega con `Authorization: Bearer <token_válido_de_supabase>`, **When** el middleware valida la firma, **Then** pasa el request al handler con el `sub` (Supabase user ID) disponible en el contexto.
4. **Given** el endpoint JWKS no está disponible momentáneamente, **When** llega un request con token cuya clave no está en caché, **Then** el backend responde 503 `{"error": "auth service unavailable"}` y no permite el acceso.

---

### User Story 2 — Primer acceso: mapeo automático de identidad (Priority: P1)

Como usuario que se acaba de registrar en Supabase, necesito que el backend me reconozca
automáticamente en mi primer request para no necesitar un paso de "registro" separado en el sistema.

**Why this priority**: Sin auto-provisioning, el primer request de cualquier usuario nuevo falla con 404.
Todos los demás flujos dependen de que el usuario interno exista.

**Independent Test**: Crear un usuario en Supabase Cloud. Usar su token en `GET /api/v1/me`.
El backend debe crear el registro interno y devolver el perfil. Repetir el request: el backend
no crea un duplicado.

**Acceptance Scenarios**:

1. **Given** un token JWT válido cuyo `sub` no existe en la tabla `users` del backend, **When** llega cualquier request autenticado, **Then** el backend crea el registro en `users` con `supabase_user_id = sub`, `email` del claim `email`, `status = 'invited'`, y continúa procesando el request.
2. **Given** el mismo token llega por segunda vez, **When** el backend busca el usuario por `supabase_user_id`, **Then** no crea un duplicado y reutiliza el registro existente.
3. **Given** un token contiene `email` distinto al registrado en `users` (cambio de email en Supabase), **When** el backend carga el usuario, **Then** actualiza el campo `email` en `users` sin crear un nuevo registro.

---

### User Story 3 — Perfil propio: identidad + tenant + rol (Priority: P1)

Como frontend Next.js, necesito llamar a un endpoint tras el login para obtener el perfil completo
del usuario (tenant asignado, rol, permisos) para construir la sesión de negocio y decidir
qué rutas son accesibles.

**Why this priority**: Es el primer endpoint real que consume el frontend post-login.
Sin él el frontend no puede construir la sesión.

**Independent Test**: Con un token válido de un usuario con `user_tenant_role` activo,
`GET /api/v1/me` devuelve tenant, rol y permisos. Con un usuario sin rol, devuelve el perfil
pero `tenant` y `role` son `null`.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado tiene un `user_tenant_role` con `status = 'active'`, **When** llama a `GET /api/v1/me`, **Then** recibe `{ user: { id, email, name }, tenant: { id, name, subdomain }, role: { id, name }, permissions: [...] }`.
2. **Given** un usuario autenticado no tiene ningún `user_tenant_role`, **When** llama a `GET /api/v1/me`, **Then** recibe `{ user: { id, email, name }, tenant: null, role: null, permissions: [] }` con HTTP 200.
3. **Given** un usuario tiene `status = 'revoked'` o `status = 'disabled'` en `users`, **When** llama a `GET /api/v1/me`, **Then** recibe HTTP 403 `{"error": "account suspended"}`.

---

### User Story 4 — RBAC real: el tenant scope protege todos los endpoints (Priority: P1)

Como sistema multi-tenant, necesito que cada request autenticado solo pueda acceder a datos
del tenant al que pertenece el usuario, sin importar qué tenant_id se pase en la URL o el body.

**Why this priority**: Sin scope de tenant real, un usuario de "demo" puede leer datos de "utn".
Es una vulnerabilidad crítica. El RBAC stub actual siempre retorna nil (permite todo).

**Independent Test**: Autenticar como usuario del tenant `demo`. Intentar acceder a
`GET /api/v1/users` pasando `tenant_id` de `utn` en el query string. El backend debe responder
403 usando el tenant del token, ignorando el parámetro de la URL.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado tiene `user_tenant_role` activo en el tenant `demo`, **When** realiza cualquier request a `/api/v1/*`, **Then** el backend inyecta `tenant_id = 'demo'` en el contexto y filtra todos los datos por ese tenant, ignorando cualquier `tenant_id` que venga en query string o body.
2. **Given** un usuario autenticado no tiene permisos para la acción que intenta realizar (ej: un `operario` intentando `DELETE /api/v1/users/:id`), **When** el middleware RBAC evalúa la acción, **Then** responde 403 `{"error": "forbidden"}` sin ejecutar el handler.
3. **Given** un `admin` realiza una acción permitida para su rol, **When** el middleware RBAC evalúa, **Then** permite el acceso y el handler se ejecuta normalmente.

---

### User Story 5 — Admin invita usuarios al tenant (Priority: P2)

Como administrador de un tenant, necesito poder invitar nuevos usuarios por email para que
puedan acceder a la plataforma, sin que yo gestione sus credenciales.

**Why this priority**: Sin invitaciones, el onboarding requiere intervención manual en la DB.
Depende de P1 (auth y RBAC reales) para poder validar que quien invita es realmente admin.

**Independent Test**: Autenticar como admin. `POST /api/v1/invitations` con un email.
El sistema registra la invitación y el usuario recibe un email de Supabase con el link de acceso.
Autenticar con ese email (usuario nuevo en Supabase). `GET /api/v1/me` debe retornar `status: active`
y el rol asignado.

**Acceptance Scenarios**:

1. **Given** un admin autenticado envía `POST /api/v1/invitations` con `{ email, role_id }`, **When** el email no tiene invitación vigente en ese tenant, **Then** el backend (a) crea un registro en `user_invitations` con `status = 'pending'`, (b) llama a la API Admin de Supabase (`inviteUserByEmail`) con el email y `redirect_to: {APP_BASE_URL}/s/{tenantId}/auth/callback`, y (c) responde 201 con el ID de la invitación. Si la llamada a Supabase falla, la transacción completa se revierte.
2. **Given** un usuario con token válido entra por primera vez y el backend lo crea con `status = 'invited'`, **When** el sistema detecta que hay una invitación `pending` para su email en ese tenant, **Then** activa la invitación (`status = 'accepted'`), actualiza el usuario a `status = 'active'` y le asigna el `user_tenant_role` correspondiente.
3. **Given** un usuario con token válido no tiene ninguna invitación en ese tenant, **When** intenta acceder a cualquier recurso protegido más allá de `/api/v1/me`, **Then** responde 403 `{"error": "no active invitation for this tenant"}`.
4. **Given** un admin intenta invitar un email que ya tiene invitación `pending` en ese tenant, **When** envía la solicitud, **Then** responde 409 `{"error": "invitation already pending for this email"}`.

---

### User Story 6 — Gestión del ciclo de vida de invitaciones (Priority: P2)

Como administrador, necesito poder listar, reenviar y revocar invitaciones para mantener
control sobre el acceso pendiente.

**Why this priority**: Sin gestión de invitaciones, el admin no puede corregir errores
(email incorrecto, persona que ya no debe ingresar).

**Acceptance Scenarios**:

1. **Given** un admin autenticado llama a `GET /api/v1/invitations`, **When** existen invitaciones en su tenant, **Then** recibe la lista con `{ id, email, role_id, status, invited_by, created_at, expires_at }` filtrada solo a su tenant.
2. **Given** una invitación está en `status = 'pending'`, **When** el admin llama a `POST /api/v1/invitations/:id/resend`, **Then** el backend actualiza `updated_at` (el reenvío del email lo gestiona Supabase Cloud vía su API admin) y responde 200.
3. **Given** una invitación está en `status = 'pending'`, **When** el admin llama a `DELETE /api/v1/invitations/:id`, **Then** el backend cambia el status a `'revoked'` y el usuario ya no puede activarse aunque tenga token válido de Supabase.
4. **Given** una invitación llegó a su fecha de expiración sin ser aceptada, **When** el usuario intenta activarse, **Then** el backend detecta el vencimiento, marca la invitación como `'expired'` y responde 403.

---

### User Story 7 — El backend resuelve el tenant desde el header X-Tenant-ID (Priority: P1)

Como sistema multi-tenant, necesito que el backend determine el tenant activo del request
a partir de un header explícito enviado por el frontend (no desde el JWT de Supabase, que no
tiene claims de tenant).

**Why this priority**: El JWT de Supabase no contiene el tenant. Sin este mecanismo,
el backend no puede saber a qué tenant scope aplicar el RBAC.

**Independent Test**: Autenticar como usuario del tenant `demo`. Enviar un request a
`GET /api/v1/me` con `X-Tenant-ID: demo`. El backend responde 200 con el perfil del tenant `demo`.
Repetir con `X-Tenant-ID: utn` (tenant al que el usuario no pertenece): debe responder 403.

**Acceptance Scenarios**:

1. **Given** un request autenticado llega con header `X-Tenant-ID: demo`, **When** el backend busca la relación `user_tenant_roles` para ese usuario y ese tenant, **Then** si existe con `status = 'active'`, inyecta `tenant_id = 'demo'` en el contexto y continúa.
2. **Given** un request autenticado llega con `X-Tenant-ID: utn` pero el usuario solo tiene rol en `demo`, **When** el backend valida la relación, **Then** responde 403 `{"error": "not a member of this tenant"}`.
3. **Given** un request autenticado llega sin header `X-Tenant-ID`, **When** el backend lo evalúa, **Then** responde 400 `{"error": "missing X-Tenant-ID header"}` (excepto `GET /api/v1/me` que devuelve el único tenant al que pertenece el usuario, o `null` si no tiene ninguno asignado aún). Los usuarios regulares pertenecen exactamente a un tenant; el `super_admin` global con acceso a múltiples tenants queda fuera del scope de esta feature.
4. **Given** el header `X-Tenant-ID` contiene un tenant que no existe en la base de datos, **When** el backend lo valida, **Then** responde 404 `{"error": "tenant not found"}`.

---

### User Story 8 — Admin fuerza cambio de contraseña de un usuario (Priority: P2)

Como administrador, necesito poder forzar que un usuario específico cambie su contraseña
en el próximo login para cumplir políticas de seguridad.

**Why this priority**: Necesario para auditoría y gestión de credenciales comprometidas.
Depende de P1 (auth real) y de que Supabase Admin API esté integrada (US5).

**Independent Test**: Autenticar como admin. Llamar a `POST /api/v1/users/:id/force-password-change`.
El usuario afectado recibe un email de reset. En su próximo request a cualquier endpoint
(excepto `/api/v1/me`), el backend responde 403 `{"error": "password_change_required"}`.
Tras cambiar la contraseña (vía link de email), el acceso se restaura.

**Acceptance Scenarios**:

1. **Given** un admin llama a `POST /api/v1/users/:id/force-password-change`, **When** el usuario objetivo existe en el tenant del admin, **Then** el backend (a) setea `password_change_required = true` en `users`, (b) llama a la API Admin de Supabase para enviar el email de reset de contraseña al usuario, y (c) responde 200.
2. **Given** un usuario tiene `password_change_required = true`, **When** realiza cualquier request a endpoints que no sean `GET /api/v1/me` o `POST /api/v1/auth/change-password`, **Then** el backend responde 403 `{"error": "password_change_required"}`.
3. **Given** el usuario completa el cambio de contraseña a través del link de Supabase, **When** su próximo request autenticado llega con el nuevo token, **Then** el backend limpia el flag `password_change_required = false` y permite el acceso normal.
4. **Given** un admin intenta forzar cambio de contraseña de un usuario de otro tenant, **When** el backend valida el scope, **Then** responde 403 `{"error": "forbidden"}`.

---

### Edge Cases

- ¿Qué pasa si dos requests simultáneos del mismo usuario nuevo llegan al mismo tiempo? La creación en `users` debe ser idempotente (`INSERT ... ON CONFLICT DO NOTHING` + re-fetch).
- ¿Qué pasa si el JWKS endpoint de Supabase rota claves? El caché local debe invalidarse solo cuando una clave referenciada en el header del JWT no se encuentra, sin reiniciar el servidor.
- ¿Qué pasa si el `email` claim viene vacío en el JWT (algunos proveedores OAuth no lo incluyen)? El backend debe aceptar el registro con `email = null` y requerir que el frontend lo complete.
- ¿Qué pasa si un usuario tiene múltiples invitaciones en distintos tenants? El sistema activa solo la del tenant que coincide con el `X-Tenant-ID` del request en curso.
- ¿Qué pasa si el admin crea la invitación pero Supabase Admin API falla al enviar el email? El backend debe revertir la creación del registro en `user_invitations` (transacción) o marcarlo como `draft` hasta confirmar el envío.
- ¿Qué pasa si un usuario con `password_change_required = true` intenta acceder a endpoints que no sean `/api/v1/me` y `/api/v1/auth/change-password`? El backend responde 403 con `{"error": "password_change_required"}` hasta que complete el cambio.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El backend DEBE validar la firma de cada JWT usando las claves públicas obtenidas del endpoint JWKS cuya URL se configura por variable de entorno (`SUPABASE_JWKS_URL`). Sin hardcodear la URL de Supabase Cloud.
- **FR-002**: El backend DEBE mantener caché local de las claves JWKS, refrescándola solo cuando el `kid` del token entrante no esté en caché, para evitar llamadas al proveedor en cada request.
- **FR-003**: El backend DEBE rechazar con 401 tokens con firma inválida, expirados, o con `iss` o `aud` que no coincidan con la instancia de Supabase configurada.
- **FR-004**: El backend DEBE auto-provisionar el registro `users` en el primer request de un `supabase_user_id` desconocido, de forma idempotente.
- **FR-005**: El backend DEBE exponer `GET /api/v1/me` que retorne identidad, tenant, rol y permisos del usuario autenticado.
- **FR-006**: El backend DEBE inyectar `tenant_id` en el contexto de cada request desde `user_tenant_roles`, no desde parámetros de la URL ni del body.
- **FR-007**: El backend DEBE rechazar con 403 cualquier request autenticado cuyo usuario tenga `status = 'revoked'` o `status = 'disabled'`.
- **FR-008**: El backend DEBE exponer `POST /api/v1/invitations`, `GET /api/v1/invitations`, `POST /api/v1/invitations/:id/resend`, y `DELETE /api/v1/invitations/:id`.
- **FR-009**: El backend DEBE rechazar con 403 cualquier request autenticado de un usuario sin invitación aceptada en el tenant al que intenta acceder (excepto `GET /api/v1/me`).
- **FR-010**: El backend DEBE marcar una invitación como `'accepted'` y activar el `user_tenant_role` automáticamente cuando detecta el primer acceso de un usuario con invitación `'pending'`.
- **FR-011**: El backend DEBE eliminar el paquete `internal/auth/` y sus rutas; ningún endpoint de auth propio debe quedar activo.
- **FR-012**: El backend DEBE ejecutar una migración que elimine `password_hash`, `sessions` y `password_reset_tokens`, y agregue `supabase_user_id`, `auth_provider`, `email_verified_at`, `last_login_at`, `password_change_required` a `users`.
- **FR-013**: El backend DEBE leer el tenant activo exclusivamente del header `X-Tenant-ID` enviado por el frontend, validando que el usuario autenticado tiene un `user_tenant_role` activo en ese tenant. No se acepta `tenant_id` desde query string ni body.
- **FR-014**: El backend DEBE rechazar con 403 `{"error": "password_change_required"}` cualquier request de un usuario con `password_change_required = true`, excepto `GET /api/v1/me` y `POST /api/v1/auth/change-password`.
- **FR-015**: El endpoint `POST /api/v1/invitations` DEBE llamar a la API Admin de Supabase (`inviteUserByEmail`) con `redirect_to: {APP_BASE_URL}/s/{tenantId}/auth/callback`, usando `APP_BASE_URL` y `SUPABASE_SERVICE_ROLE_KEY` como variables de entorno. Si Supabase falla, la transacción completa se revierte. El campo `expires_at` se calcula como `created_at + 7 días`.
- **FR-016**: El endpoint `POST /api/v1/users/:id/force-password-change` DEBE setear `password_change_required = true` y llamar a la API Admin de Supabase para enviar el email de reset al usuario.

### Key Entities

- **JWTClaims**: Payload del token JWT de Supabase. Campos relevantes: `sub` (Supabase user ID), `email`, `aud`, `iss`, `exp`, `role` (Supabase role, distinto al rol de negocio).
- **UserAccount**: Usuario de negocio. Vinculado a un `supabase_user_id`. Tiene `status` (`invited` → `active` → `revoked` / `disabled`). `revoked` es una acción manual del admin (decisión de negocio). `disabled` es una acción automática del sistema (actividad sospechosa, violaciones de política). Ambos resultan en 403, pero solo `revoked` es reversible por el admin desde la UI.
- **UserInvitation**: Invitación de un email a un tenant con un rol. Estados: `pending` → `accepted` | `revoked` | `expired`. Tiene `expires_at`.
- **UserTenantRole**: Asignación activa de rol a usuario en tenant. Ya existe en el esquema. Se crea al aceptar una invitación.

### Non-Functional Requirements

- **NFR-003**: El backend DEBE aplicar rate limiting en `POST /api/v1/invitations` por tenant: el número máximo de invitaciones por hora es configurable via `INVITATION_RATE_LIMIT_PER_HOUR` (default: 20). Superado el límite, responde 429 `{"error": "invitation rate limit exceeded"}`.

- **NFR-001**: El backend DEBE emitir contadores Prometheus para los siguientes eventos de auth: `auth_requests_total` (por status: ok/401/403), `auth_tenant_violations_total` (X-Tenant-ID inválido), `invitations_sent_total`, `invitations_expired_total`, `password_change_forced_total`.
- **NFR-002**: El backend DEBE emitir logs Zap estructurados (nivel `warn` o `error`) para cada evento de seguridad relevante, incluyendo: `supabase_user_id`, `tenant_id` (si disponible), `endpoint`, `status_code`, y `reason`.

### Deployment

- **Runtime**: Koyeb — deploy desde Dockerfile existente (Go binary Alpine, sin cambios).
- **PostgreSQL**: Neon (serverless Postgres, free tier). `DATABASE_URL` como env var en Koyeb.
- **Redis**: Upstash (ya en dependencias del frontend). `REDIS_URL` como env var en Koyeb.
- **Auth**: Supabase Cloud (free tier, hasta 50k MAU). Env vars: `SUPABASE_JWKS_URL`, `SUPABASE_JWT_ISSUER`, `SUPABASE_JWT_AUDIENCE`, `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`. (Nota: `SUPABASE_JWT_SECRET` NO es necesario — la validación usa RS256 con claves públicas via JWKS, no el secret simétrico.)
- **Self-hosted path**: Cambiar `SUPABASE_JWKS_URL` a `https://auth.tudominio.com/auth/v1/.well-known/jwks.json` y `SUPABASE_URL` al endpoint self-hosted. Sin cambios de código.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% de los requests a `/api/v1/*` sin token válido retornan 401 antes de ejecutar cualquier lógica de negocio.
- **SC-002**: 100% de los requests de usuarios con `status = 'revoked'` o sin invitación activa retornan 403.
- **SC-003**: El endpoint `GET /api/v1/me` responde en menos de 300ms en P95 bajo carga normal (excluye el primer request que hace fetch de JWKS).
- **SC-004**: El sistema de auth propio (`internal/auth/`, tablas `sessions`, `password_reset_tokens`, campo `password_hash`) no existe en el repositorio al completar la feature.
- **SC-005**: Cambiar `SUPABASE_JWKS_URL` a una instancia self-hosted y reiniciar el proceso es suficiente para migrar de Supabase Cloud a self-hosted, sin ningún cambio de código.
- **SC-006**: Dos requests simultáneos del mismo `supabase_user_id` desconocido producen exactamente un registro en `users`, verificable con un test de concurrencia.

## Clarifications

### Session 2026-03-06

- Q: ¿Cuánto tiempo deben durar las invitaciones antes de expirar? → A: 7 días (Opción B — estándar SaaS)
- Q: ¿Cuál es la diferencia entre `status = 'revoked'` y `status = 'disabled'` en `users`? → A: `revoked` = acción manual del admin; `disabled` = acción automática del sistema (ej: actividad sospechosa, intentos fallidos)
- Q: ¿Los eventos de auth deben emitirse como métricas Prometheus, logs Zap, o ambos? → A: Ambos — métricas Prometheus para contadores y alertas, logs Zap estructurados para contexto de debugging
- Q: ¿El backend debe aplicar rate limiting en la creación de invitaciones, o se delega a Supabase? → A: Rate limit en el backend por tenant, máximo configurable por env var (`INVITATION_RATE_LIMIT_PER_HOUR`)
- Q: ¿Qué mecanismo usa el backend para limpiar el flag `password_change_required` tras cambio de contraseña? → A: El frontend llama a `POST /api/v1/auth/change-password` desde el callback de Supabase con el nuevo access token; el backend verifica el token y limpia el flag

---

## Assumptions

- Supabase Cloud está configurado antes de comenzar el desarrollo: proyecto creado, JWKS URL disponible, SMTP activo.
- El reenvío de email de invitación se delega a la API Admin de Supabase (`POST /auth/v1/admin/users` + `inviteUserByEmail`), no se implementa en el backend Go.
- Los roles disponibles para asignar en invitaciones son los ya existentes en la tabla `roles` (`admin`, `operario`, `cliente_admin`, `cliente_operario`).
- Neon y Upstash tienen cuentas creadas y las connection strings disponibles antes de comenzar el deploy en Koyeb.
- El `super_admin` como rol global (con acceso a todos los tenants) se implementa en una feature posterior. En esta feature cada usuario regular pertenece a exactamente un tenant. `admin` es el rol de mayor privilegio dentro de un tenant.
- `APP_BASE_URL` es una variable de entorno que contiene la URL pública del frontend (ej: `https://app.tudominio.com`). Se usa exclusivamente para construir el `redirect_to` de las invitaciones y del force-password-change.
- El endpoint `POST /api/v1/auth/change-password` no gestiona el cambio de contraseña directamente — el cambio lo realiza Supabase vía el token del link de email. El frontend llama a este endpoint desde el callback de Supabase (después de que Supabase procesa el reset token) incluyendo el nuevo access token. El backend verifica el token y limpia `password_change_required = false`.

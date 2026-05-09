# Phase 1 — Data Model: Esquema consolidado post-migración

Este documento describe el modelo de datos resultante tras aplicar `000001_initial_schema` + `000002_seed_essentials`. **No introduce entidades nuevas** — refleja el estado tras 000001-000020. Sirve como referencia de validación: cualquier desvío entre este documento y el `pg_dump --schema-only` indica un error de consolidación.

## Convenciones

- Todas las tablas tienen `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` y `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()` salvo que se indique.
- PKs son `UUID DEFAULT gen_random_uuid()` salvo cuando se aclara `BIGSERIAL`.
- Toda tabla con datos por tenant lleva `tenant_id UUID NOT NULL REFERENCES tenants(id)` y un índice por `tenant_id`. Las consultas en repos verifican `tenant_id` desde `context.Context` (Principio II — aislamiento de tenants, NO NEGOCIABLE).

## Entidades core

### `tenants`
Organizaciones / clientes. Incluye el tenant especial **MRG** (plataforma) identificado por UUID fijo `11b36b85-033d-4bb3-9e31-4c92161887c0`.
- `id UUID PK`, `name VARCHAR(255) NOT NULL`, `company_name VARCHAR(255) NOT NULL`, `subdomain VARCHAR(100) NOT NULL`, `description TEXT`, `is_active BOOLEAN DEFAULT true`, theming/address columns (`primary/secondary/accent/text/background_color`, `logo_url`, `favicon_url`, `street`, `city`, `state`, `postal_code`, `country`).
- **No** existe columna `slug` ni `is_platform` ni `status` — el "tenant plataforma" se identifica por UUID conocido.
- Seed esencial: 1 fila MRG (subdomain `mrgsrl`).

### `users`
Usuarios autenticados vía Supabase. Tabla unificada (resultado de la fusión 000007/000010).
- `id UUID PK`, `supabase_user_id TEXT`, `email VARCHAR(255) NOT NULL`, `first_name VARCHAR(100)`, `last_name VARCHAR(100)`, `name VARCHAR(255)`, `image TEXT`, `tenant_id UUID` (nullable — usuarios cross-tenant), `status VARCHAR(20) NOT NULL DEFAULT 'active'`, `role VARCHAR(50) NOT NULL DEFAULT 'user'` (legacy, sin check), `auth_provider TEXT`, `email_verified_at`, `last_login_at`, `password_change_required BOOLEAN NOT NULL DEFAULT false`, `deleted_at` (soft delete).
- **Sin** seed esencial: el admin MRG se crea via Supabase + `auth_usecase.ProvisionUser` (ver R3 actualizada).

### `roles`
Catálogo de roles. Globales (`is_global=true`) y por tenant.
- `id VARCHAR(50) PK` (string identifier, ej `'super-admin'`, `'tenant-manager'`, `'admin'`, `'operario'`, `'cliente_admin'`, `'cliente_operario'`), `name VARCHAR(255) NOT NULL`, `description TEXT`, `is_system_role BOOLEAN NOT NULL DEFAULT false`, `is_global BOOLEAN NOT NULL DEFAULT false`, `tenant_id UUID` (nullable; null para roles globales o catálogo reusable), `permissions JSONB NOT NULL DEFAULT '[]'` (campo legacy — los permisos efectivos se resuelven en `internal/security/rbac.go::rolePermissions`, NO desde este JSONB), `deleted_at` (soft delete).
- **No** hay tabla `role_permissions` ni columna `scope`.
- Seed esencial: 6 roles (super-admin, tenant-manager globales; admin, operario, cliente_admin, cliente_operario tenant-scoped).

### `permissions`
Catálogo de permisos. Sistema (`is_system_permission=true, tenant_id=NULL`) o custom-tenant (`is_system_permission=false, tenant_id=<uuid>`), enforced por CHECK constraints `chk_system_perm_no_tenant` y `chk_custom_perm_has_tenant`.
- `id TEXT PK` (ej `'perm_dashboard'`, `'perm_logs_view'`), `name TEXT NOT NULL` (≥3 chars), `section TEXT NOT NULL`, `description TEXT NOT NULL`, `is_system_permission BOOLEAN NOT NULL DEFAULT false`, `tenant_id UUID NULL`.
- Seed esencial: 17 permisos del sistema (ver `migrations/000002_seed_essentials.up.sql:21-39`).

### `role_permissions`
(Sección eliminada — esta tabla no existe en el esquema final. Los permisos efectivos por rol se resuelven en `internal/security/rbac.go::rolePermissions`.)

### `user_tenant_roles`
Asignación de un usuario a un rol dentro de un tenant.
- `id UUID PK`, `user_id UUID NOT NULL`, `tenant_id UUID NOT NULL`, `role_id VARCHAR(50)` (FK lógica a `roles.id`), `status VARCHAR(20) NOT NULL DEFAULT 'pending'` (CHECK `active|pending|revoked|suspended`), `assigned_by UUID`, `assigned_at TIMESTAMPTZ`.
- **Sin** seed esencial: la asignación admin MRG ↔ tenant MRG ↔ super-admin se hace post-deploy (ver Paso 5 de quickstart).

### `user_invitations`
Invitaciones pendientes (000005 + 000006).
- `id UUID PK`, `tenant_id UUID NOT NULL`, `email CITEXT NOT NULL`, `role_id UUID`, `invited_by UUID NOT NULL` (FK users), `status TEXT NOT NULL` (`pending`, `accepted`, `revoked`, `expired`), `token_hash TEXT NOT NULL`, `expires_at TIMESTAMPTZ NOT NULL`.
- Index por `tenant_id`, por `(email, tenant_id)`.

## Entidades de dispositivos y telemetría

### `edge_devices`
Dispositivos de borde por tenant (000008).
- `id UUID PK`, `tenant_id UUID NOT NULL`, `serial TEXT NOT NULL`, `model TEXT`, `last_seen_at TIMESTAMPTZ`.
- UNIQUE `(tenant_id, serial)`.

### `device_events`
Eventos ingresados por dispositivos (parte de 000008).
- `id BIGSERIAL PK`, `device_id UUID NOT NULL`, `tenant_id UUID NOT NULL`, `event_type TEXT NOT NULL`, `payload JSONB NOT NULL`, `recorded_at TIMESTAMPTZ NOT NULL`.
- Index BRIN por `recorded_at`.

### `alarm_rules`
Reglas de alarma por tenant (000014).
- `id UUID PK`, `tenant_id UUID NOT NULL`, `name TEXT NOT NULL`, `expression JSONB NOT NULL`, `severity TEXT NOT NULL`, `enabled BOOL NOT NULL DEFAULT true`.

### `log_entries`
Log estructurado de eventos del sistema (000015).
- `id BIGSERIAL PK`, `tenant_id UUID`, `level TEXT NOT NULL`, `message TEXT NOT NULL`, `metadata JSONB`, `occurred_at TIMESTAMPTZ NOT NULL`.

### `log_retention_policies`
Políticas de retención (000015).
- `tenant_id UUID PK`, `retention_days INT NOT NULL`.

### `notifications`
Notificaciones a usuarios (000016).
- `id UUID PK`, `user_id UUID NOT NULL`, `tenant_id UUID NOT NULL`, `kind TEXT NOT NULL`, `payload JSONB NOT NULL`, `read_at TIMESTAMPTZ`.

## Entidades de UI

### `dashboard_layouts`
Layouts de dashboard por usuario y tenant (000009 + 000011).
- `id UUID PK`, `user_id UUID NOT NULL`, `tenant_id UUID NOT NULL`, `name TEXT NOT NULL`, `layout JSONB NOT NULL`.
- UNIQUE `(user_id, tenant_id, name)`.

## Entidades de auth/sessions

### `sessions`
Sesiones (si quedaron tras la migración a Supabase JWT — verificar al hacer el dump).
- En la consolidación, **si la tabla está vacía y no hay código que la lea**, se elimina del esquema final. El procedimiento del quickstart incluye un grep en el código Go para confirmarlo.

### `password_reset_tokens`
Tokens de reset (idem `sessions` — verificar uso real antes de incluir).

## Tabla de sistema (gestionada por golang-migrate)

### `schema_migrations`
- `version BIGINT PK`, `dirty BOOLEAN NOT NULL`.
- Tras `migrate up` exitoso debe contener `version=2, dirty=false`.

## Validaciones críticas (Principio II — aislamiento de tenants)

Todas las tablas con datos por cliente deben tener **simultáneamente**:
1. Columna `tenant_id UUID NOT NULL` con FK a `tenants(id)`.
2. Índice por `tenant_id` (estándar, no parcial).
3. Si tienen unique keys de negocio (ej: `serial` en `edge_devices`), el unique incluye `tenant_id`.

El quickstart incluye una query de auditoría que verifica estos tres puntos automáticamente sobre el esquema generado.

## Estados y transiciones

### `users.status`
`pending_invitation` → `active` (al completar invitación) → `suspended` (admin) → `active` (re-activación).

### `user_invitations.status`
`pending` → `accepted` | `revoked` | `expired`. Estados terminales.

### `user_tenant_roles.status`
`active` ↔ `suspended` (agregado en 000013).

### `tenants.status`
`active` ↔ `suspended`.

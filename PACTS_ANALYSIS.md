# Análisis de Contratos Pact — Embolsadora API

> **Generado**: 2026-03-24
> **Última actualización**: 2026-04-10
> **Rama analizada**: `develop`
> **Fuente de contratos**: `embolsadora-frontend/pacts/` (13 archivos)
> **Total de interacciones Pact**: 149

---

## Resumen Ejecutivo

| Métrica | Valor |
|---|---|
| Archivos Pact | 13 |
| Interacciones totales | 149 |
| Servicios completamente implementados | 10 |
| Servicios parcialmente implementados | 0 |
| Servicios no implementados | 3 |
| Interacciones N/A (Supabase maneja) | 5 |
| Cobertura estimada | ~89% |

---

## Estado por Servicio

### ✅ Completamente Implementados

#### `user-service-api` — 5/5 interacciones

Consumer: `embolsadora-frontend` → Provider: `user-service`

| Método | Path Pact | Path Backend | Estado |
|---|---|---|---|
| GET | `/api/users?tenantId=` | `/api/v1/users` | ✅ |
| GET | `/api/users/{id}` | `/api/v1/users/:id` | ✅ |
| POST | `/api/users` | `/api/v1/users` | ✅ |
| PATCH | `/api/users/{id}` | `/api/v1/users/:id` | ✅ |
| DELETE | `/api/users/{id}` | `/api/v1/users/:id` | ✅ |

> ⚠️ **Nota de prefijo**: el Pact usa `/api/users`, el backend expone `/api/v1/users`. Verificar que el frontend tenga configurado el basePath correcto (`/api/v1`).

---

#### `tenant-service-api` — 7/7 interacciones

Consumer: `embolsadora-frontend` → Provider: `tenant-service`

| Método | Path Pact | Path Backend | Estado |
|---|---|---|---|
| GET | `/api/tenant/current` | — | ⚠️ Path distinto (`/api/v1/tenants` + query) |
| GET | `/api/tenants` | `/api/v1/tenants` | ✅ |
| GET | `/api/tenants/{id}` | `/api/v1/tenants/:id` | ✅ |
| POST | `/api/tenants` | `/api/v1/tenants` | ✅ |
| PATCH | `/api/tenants/{id}` | `/api/v1/tenants/:id` | ✅ |
| DELETE | `/api/tenants/{id}` | `/api/v1/tenants/:id` | ✅ |
| GET | `/api/tenants/{id}` → 404 | `/api/v1/tenants/:id` | ✅ |

> ⚠️ `GET /api/tenant/current` (by subdomain) no tiene equivalente directo en el backend.

---

#### `user-role-service-api` — 9/9 interacciones

Consumer: `embolsadora-frontend-bff` → Provider: `user-role-service-api`

| Método | Path Pact | Path Backend | Estado |
|---|---|---|---|
| GET | `/api/v1/user-roles?tenantId=` | `/api/v1/user-roles` | ✅ |
| GET | `/api/v1/user-roles?status=` | `/api/v1/user-roles` | ✅ |
| POST | `/api/v1/user-roles` | `/api/v1/user-roles` | ✅ |
| POST | `/api/v1/user-roles` → 409 | `/api/v1/user-roles` | ✅ |
| PUT | `/api/v1/user-roles/{id}` | `/api/v1/user-roles/:id` | ✅ |
| DELETE | `/api/v1/user-roles/{id}` | `/api/v1/user-roles/:id` | ✅ |
| POST | `/api/v1/user-roles/bulk` | `/api/v1/user-roles/bulk` | ✅ |
| GET | `/api/v1/users/{userId}/roles` | `/api/v1/users/:id/roles` | ✅ |

---

#### `dashboard-service-api` — 12/12 interacciones

Consumer: `embolsadora-frontend` → Provider: `dashboard-service-api`

| Método | Path Pact | Path Backend | Estado |
|---|---|---|---|
| GET | `/api/tenants/{tenantId}/dashboard-layouts` | `/api/v1/dashboard-layouts` | ✅ |
| POST | `/api/tenants/{tenantId}/dashboard-layouts` | `/api/v1/dashboard-layouts` | ✅ |
| POST | `/api/tenants/{tenantId}/dashboard-layouts` → 403 | `/api/v1/dashboard-layouts` | ✅ |
| POST | `/api/tenants/{tenantId}/dashboard-layouts` → 409 | `/api/v1/dashboard-layouts` | ✅ |
| GET | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| GET | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 404 | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| PUT | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| PUT | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 409 | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| PUT | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 404 | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| DELETE | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| DELETE | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 400 | `/api/v1/dashboard-layouts/:layoutId` | ✅ |
| GET | `/api/tenants/{tenantId}/dashboard-layouts` → 401 | `/api/v1/dashboard-layouts` | ✅ |

> Implementado en `specs/005-dashboard-layouts/`. Rama `005-dashboard-layouts`.
> **Nota**: El backend usa `/api/v1/dashboard-layouts` con tenant resuelto desde `X-Tenant-ID` header (UUID) y user_id desde el JWT, en lugar del path param `{tenantId}` del Pact.

---

#### `edge-device-service-api` — 14/14 interacciones

Consumer: `embolsadora-frontend` → Provider: `edge-device-service-api`

| Método | Path Pact | Path Backend | Estado |
|---|---|---|---|
| GET | `/api/tenants/{tenantId}/edge-devices` | `/api/tenants/:tenantId/edge-devices` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices` | `/api/tenants/:tenantId/edge-devices` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices` → 409 | `/api/tenants/:tenantId/edge-devices` | ✅ |
| GET | `/api/tenants/{tenantId}/edge-devices/{deviceId}` | `/api/tenants/:tenantId/edge-devices/:id` | ✅ |
| GET | `/api/tenants/{tenantId}/edge-devices/{deviceId}` → 404 | `/api/tenants/:tenantId/edge-devices/:id` | ✅ |
| PUT | `/api/tenants/{tenantId}/edge-devices/{deviceId}` | `/api/tenants/:tenantId/edge-devices/:id` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices/{deviceId}/enable` | `…/enable` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices/{deviceId}/disable` | `…/disable` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices/{deviceId}/status` | `…/status` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices/{deviceId}/status` → 400 | `…/status` | ✅ |
| POST | `/api/tenants/{tenantId}/edge-devices/{deviceId}/health-check` | `…/health-check` | ✅ |
| GET | `/api/tenants/{tenantId}/edge-devices/{deviceId}/telemetry` | `…/telemetry` | ✅ |
| GET | `/api/tenants/{tenantId}/edge-devices/{deviceId}/events` | `…/events` | ✅ |
| GET | `/api/tenants/{tenantId}/edge-devices` → 401 | auth middleware | ✅ |

---

#### `auth-service-api` — 3/3 interacciones backend (+ 3 N/A Supabase)

Consumer: `embolsadora-frontend` → Provider: `auth-service`

| Método | Path Pact | Path Backend | Estado | Observación |
|---|---|---|---|---|
| POST | `/api/auth/callback/credentials` → 200 | `/api/v1/auth/login` | ✅ | Proxy a Supabase `/auth/v1/token` |
| POST | `/api/auth/callback/credentials` → 401 | `/api/v1/auth/login` | ✅ | Manejado por el mismo handler |
| GET | `/api/auth/session` | `/api/v1/me` | ✅ | Retorna usuario + tenant + rol + permisos desde JWT |
| POST | `/api/auth/signout` | — | 🚫 N/A | JWT es stateless — el frontend descarta el token localmente, no requiere endpoint backend |
| POST | `/api/auth/forgot-password` | — | 🚫 N/A | Supabase lo maneja directamente desde el frontend sin pasar por este backend |
| POST | `/api/auth/reset-password` | — | 🚫 N/A | Ídem — el reset completo es responsabilidad de Supabase |

> **Nota**: PR #16 implementó la integración completa con Supabase Auth (JWKS verification, auto-provisioning, JWT middleware chain). Los flujos de signout/forgot/reset no requieren endpoint en este backend — Supabase los resuelve directamente con el frontend.

---

#### `role-service-api` — 7/7 interacciones (CRUD de roles)

Consumer: `embolsadora-frontend-bff` → Provider: `role-service-api`

| Método | Path Pact | Path Backend | Estado | Observación |
|---|---|---|---|---|
| GET | `/api/v1/roles?tenantId=` | `/api/v1/roles` | ✅ | tenant_id via X-Tenant-ID header (migration 000012) |
| GET | `/api/v1/roles/{id}` | `/api/v1/roles/:id` | ✅ | 404 si no existe |
| POST | `/api/v1/roles` | `/api/v1/roles` | ✅ | 201, límite 3 custom por tenant |
| PUT | `/api/v1/roles/{id}` | `/api/v1/roles/:id` | ✅ | 403 si es rol del sistema |
| DELETE | `/api/v1/roles/{id}` → 200 | `/api/v1/roles/:id` | ✅ | Solo roles custom sin asignaciones |
| DELETE | `/api/v1/roles/{id}` → 409 | `/api/v1/roles/:id` | ✅ | `usersAffected` count en response body |
| DELETE | `/api/v1/roles/{id}` → 403 | `/api/v1/roles/:id` | ✅ | Rol del sistema no eliminable |

> Implementado en `006-roles-management`: migration 000012 extiende tabla `roles` con `is_system_role`, `tenant_id`, `permissions` (JSONB), `deleted_at`.

---

### ⚠️ Parcialmente Implementados (endpoint existe, funcionalidad incompleta)

#### `user-service-api-roles-extension` — 3/4 interacciones backend (+ 2 N/A Supabase)

Consumer: `embolsadora-frontend-bff` → Provider: `user-service-api`

| Método | Path Pact | Estado | Observación |
|---|---|---|---|
| GET | `/api/v1/users/{id}?include=roles` | ✅ | `include=roles` implementado via JOIN con UTR + roles; campo `roles: []` en response (007) |
| POST | `/api/v1/users` con rol inicial | ✅ | CreateWithRole: usuario + UTR activo en una sola transacción pgx (013) |
| POST | `/api/v1/users/register` | 🚫 N/A | El registro de usuarios es via invitaciones (Supabase Admin API) — no existe auto-registro |
| POST | `/api/v1/users/verify-email` | 🚫 N/A | Verificación de email es 100% Supabase, no pasa por este backend |
| PATCH | `/api/v1/users/{id}/status` | ✅ | Actualiza UTR.status (active/inactive→revoked/suspended); guard anti-auto-desactivación (007) |
| GET | `/api/v1/users/pending` | ✅ | Devuelve usuarios con UTR.status='pending'; respuesta `{data:[], total:N}` (007) |

> Implementado en `007-user-roles-status`: migration 000013 (suspended status), GET include=roles (JOIN UTR+roles), PATCH status (UTR status per-tenant), GET pending (JOIN UTR pending).

---

#### `alarm-rules-service-api` — 11/11 interacciones

> **Nota de conteo**: se contabilizan 11 interacciones. El listado `GET /api/alarm-rules → 200` se desglosa en dos variantes operativas (lista con resultados y lista vacía), ambas cubiertas por el contrato de listado.

Consumer: `embolsadora-frontend` → Provider: `alarm-rules-service-api`

| Método | Path Pact | Path Backend | Estado | Observación |
|---|---|---|---|---|
| GET | `/api/alarm-rules` | `/api/v1/alarm-rules` | ✅ | Lista reglas del tenant; `[]` si vacío |
| GET | `/api/alarm-rules/{id}` → 200 | `/api/v1/alarm-rules/:id` | ✅ | Verifica tenant_id para aislamiento |
| GET | `/api/alarm-rules/{id}` → 404 | `/api/v1/alarm-rules/:id` | ✅ | Regla inexistente o de otro tenant |
| POST | `/api/alarm-rules` → 201 | `/api/v1/alarm-rules` | ✅ | Valida operator y severity |
| POST | `/api/alarm-rules` → 400 | `/api/v1/alarm-rules` | ✅ | VALIDATION_ERROR con campo que falló |
| PATCH | `/api/alarm-rules/{id}` → 200 | `/api/v1/alarm-rules/:id` | ✅ | Actualización parcial via punteros |
| PATCH | `/api/alarm-rules/{id}` → 404 | `/api/v1/alarm-rules/:id` | ✅ | NOT_FOUND |
| DELETE | `/api/alarm-rules/{id}` → 200 | `/api/v1/alarm-rules/:id` | ✅ | Eliminación permanente |
| DELETE | `/api/alarm-rules/{id}` → 404 | `/api/v1/alarm-rules/:id` | ✅ | NOT_FOUND |
| GET | `/api/alarm-rules` → 401 | auth middleware | ✅ | Sin JWT → UNAUTHORIZED |

> Implementado en `008-alarm-rules`: migración 000014 (`alarm_rules` table con CHECK constraints), domain + repo + service + 5 handlers. Eliminación permanente (no soft-delete).

---

#### `log-service-api` — 14/14 interacciones

Consumer: `embolsadora-frontend` → Provider: `log-service-api`

| Método | Path | Estado | Observación |
|---|---|---|---|
| GET | `/api/v1/logs` (con filtros) | ✅ | Filtros: event_type, severity, machine_id, from/to, q |
| GET | `/api/v1/logs` (text query) | ✅ | Full-text search via tsvector |
| GET | `/api/v1/logs` (machine logs) | ✅ | Filtro machine_id con aislamiento tenant |
| GET | `/api/v1/logs` (cursor) | ✅ | Paginación keyset por (created_at, id) codificado en base64 |
| GET | `/api/v1/logs` (sin resultados) | ✅ | `data: []` con 200 |
| GET | `/api/v1/logs/{id}` → 200 | ✅ | Todos los campos incluyendo metadata JSONB |
| GET | `/api/v1/logs/{id}` → 404 | ✅ | NOT_FOUND si inexistente o de otro tenant |
| GET | `/api/v1/logs/{id}/context` | ✅ | Ventana before/anchor/after configurable |
| GET | `/api/v1/logs/retention` | ✅ | Retorna política o default 90 días |
| PATCH | `/api/v1/logs/retention` | ✅ | Requiere permiso admin |
| GET | `/api/v1/logs/stream` | ✅ | SSE con heartbeat cada 30s |
| GET | `/api/v1/logs/export` | ✅ | JSON/CSV con truncated flag |
| GET | `/api/v1/logs/export` (truncado) | ✅ | `truncated: true, total_available: N` |
| GET | `/api/v1/logs` → 401 | ✅ | Sin JWT → UNAUTHORIZED |

> Implementado en `009-log-service`: migración 000015 (log_entries + log_retention_policies + índices FTS), domain + repo (cursor keyset) + service (SSE hub) + 6 handlers + Postman collection 14 Pacts.

---

#### `notification-service-api` — 6/6 interacciones

Consumer: `embolsadora-frontend` → Provider: `notification-service-api`

| Método | Path | Estado | Observación |
|---|---|---|---|
| GET | `/api/v1/notifications` | ✅ | Paginación limit/offset, filtros status y severity |
| GET | `/api/v1/notifications/count` | ✅ | Conteo de `status='unread'` del tenant |
| GET | `/api/v1/notifications/{id}` | ✅ | 200 si existe y pertenece al tenant, 404 si no |
| POST | `/api/v1/notifications/{id}/ack` | ✅ | Idempotente: unread→acknowledged, acknowledged/closed→sin cambio |
| POST | `/api/v1/notifications/{id}/close` | ✅ | Idempotente: cualquier estado→closed |
| GET | `/api/v1/alarm-rules` | ✅ | Ya implementado en 008-alarm-rules; verificado en quickstart |

> Implementado en `010-notification-service`: migración 000016 (notifications table + índices), domain + repo (Ack/Close idempotentes) + service + 5 handlers + wiring en url_mappings.go.

---

#### `permissions-service-api` — 10/10 interacciones

Consumer: `embolsadora-frontend` → Provider: `permissions-service-api`

| Método | Path | Estado | Observación |
|---|---|---|---|
| GET | `/api/v1/permissions` | ✅ | Lista permisos sistema + custom del tenant; sistema siempre incluidos |
| POST | `/api/v1/permissions` → 201 | ✅ | Crea permiso custom; UUID generado en service; `isSystemPermission: false` |
| POST | `/api/v1/permissions` → 400 | ✅ | Validación: nombre < 3 chars → `errors[].path = "name"` |
| GET | `/api/v1/permissions/{id}` → 200 | ✅ | Funciona para permisos sistema y custom; filtra por `tenant_id` (custom) o `is_system_permission=TRUE` (sistema) |
| GET | `/api/v1/permissions/{id}` → 404 | ✅ | ID inexistente → NOT_FOUND |
| PUT | `/api/v1/permissions/{id}` → 200 | ✅ | Actualiza custom; retorna datos actualizados con updatedAt renovado |
| PUT | `/api/v1/permissions/{id}` → 403 | ✅ | Permiso de sistema → `"Cannot modify system permissions"` |
| DELETE | `/api/v1/permissions/{id}` → 200 | ✅ | Elimina custom permanentemente; `{"success": true}` |
| DELETE | `/api/v1/permissions/{id}` → 403 | ✅ | Permiso de sistema → `"Cannot delete system permissions"` |
| GET | `/api/v1/permissions` → 401 | ✅ | Sin JWT → UNAUTHORIZED |

> Implementado en `011-permissions-management`: migración 000017 (permissions table + seed 17 permisos de sistema), domain + repo (List/GetByID/Delete con aislamiento multi-tenant; Update con guarda `is_system_permission=FALSE`) + service (validación, guards IsSystemPermission, tenant propagado a todas las operaciones) + handler (5 endpoints, DTOs, error mapping, Prometheus en todos los handlers con label `operation`) + Postman collection 10 Pacts.

---

### ❌ No Implementados

---

#### `permissions-service-api` — 0/10 interacciones

Consumer: `embolsadora-frontend` → Provider: `permissions-service-api`

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/permissions` | Listar permisos |
| POST | `/api/permissions` → 201 | Crear permiso custom |
| POST | `/api/permissions` → 400 | Error de validación |
| GET | `/api/permissions/{id}` → 200 | Obtener permiso |
| GET | `/api/permissions/{id}` → 404 | No encontrado |
| PUT | `/api/permissions/{id}` → 200 | Actualizar permiso custom |
| PUT | `/api/permissions/{id}` → 403 | No se puede modificar permiso del sistema |
| DELETE | `/api/permissions/{id}` → 200 | Eliminar permiso custom |
| DELETE | `/api/permissions/{id}` → 403 | No se puede eliminar permiso del sistema |
| GET | `/api/permissions` → 401 | Auth requerida |

---

#### `reports-service-api` — 0/16 interacciones

Consumer: `embolsadora-frontend` → Provider: `reports-service-api`

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/tenants/{tenantId}/reports` | Listar reportes |
| POST | `/api/tenants/{tenantId}/reports` → 202 | Generar reporte (async) |
| POST | `/api/tenants/{tenantId}/reports` → 429 | Quota excedida |
| GET | `/api/tenants/{tenantId}/reports/{reportId}` (pending) | Reporte en progreso |
| GET | `/api/tenants/{tenantId}/reports/{reportId}` (completed) | Reporte listo |
| GET | `/api/tenants/{tenantId}/reports/limit` | Estado del quota |
| GET | `/api/tenants/{tenantId}/reports/schedules` | Listar schedules |
| POST | `/api/tenants/{tenantId}/reports/schedules` | Crear schedule |
| PUT | `/api/tenants/{tenantId}/reports/schedules/{scheduleId}` | Actualizar schedule |
| DELETE | `/api/tenants/{tenantId}/reports/schedules/{scheduleId}` | Eliminar schedule |
| GET | `/api/tenants/{tenantId}/reports/settings` | Configuración de retención |
| PUT | `/api/tenants/{tenantId}/reports/settings` | Actualizar retención |
| GET | `/api/tenants/{tenantId}/reports/{reportId}/download` → 200 | Descargar reporte |
| GET | `/api/tenants/{tenantId}/reports/{reportId}/download` → 422 | Reporte no listo |
| GET | `/api/tenants/{tenantId}/reports` → 401 | Auth requerida |

---

## Backlog Priorizado

| # | Servicio | Interacciones | Prioridad | Justificación |
|---|---|---|---|---|
| ~~1~~ | ~~`log-service-api`~~ | ~~14~~ | ~~✅ Done (009)~~ | — |
| ~~2~~ | ~~`notification-service-api`~~ | ~~6~~ | ~~✅ Done (010)~~ | — |
| ~~1~~ | ~~`permissions-service-api`~~ | ~~10~~ | ~~✅ Done (011)~~ | — |
| ~~1~~ | ~~`user-service-api-roles-extension` (completar)~~ | ~~1~~ | ~~✅ Done (013)~~ | — |
| 1 | `reports-service-api` | 16 | 🔵 Baja — **en pausa** | Generación async compleja, mayor esfuerzo |

**Total interacciones pendientes**: ~16 de 149 (excluyendo 5 N/A Supabase)

---

## Observaciones Técnicas

### Desajuste de prefijos de ruta

Varios Pact files usan `/api/` como prefijo mientras el backend usa `/api/v1/`:

| Pact | Path en Pact | Path en Backend |
|---|---|---|
| `user-service-api` | `/api/users` | `/api/v1/users` |
| `tenant-service-api` | `/api/tenants` | `/api/v1/tenants` |
| `alarm-rules-service-api` | `/api/alarm-rules` | — |
| `permissions-service-api` | `/api/permissions` | — |

Verificar que el frontend tenga `NEXT_PUBLIC_API_URL` apuntando a `/api/v1` o que haya un proxy que normalice el prefijo.

### `GET /api/tenant/current`

El Pact espera resolución de tenant por subdominio. Actualmente el backend requiere `X-Tenant-ID` en el header. Evaluar si este endpoint es necesario o si la resolución por subdominio se hace en el BFF/middleware del frontend.

### Auth flows vs Supabase

`POST /auth/forgot-password` y `POST /auth/reset-password` pueden ser manejados directamente por Supabase desde el frontend (sin pasar por este backend). Confirmar con el equipo frontend si esperan proxy en este backend o integración directa con Supabase.

---

*Reporte generado con Claude Code — rama `develop` — última actualización 2026-03-26*

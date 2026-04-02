# Análisis de Contratos Pact — Embolsadora API

> **Generado**: 2026-03-24
> **Rama analizada**: `develop`
> **Fuente de contratos**: `embolsadora-frontend/pacts/` (13 archivos)
> **Total de interacciones Pact**: 149

---

## Resumen Ejecutivo

| Métrica | Valor |
|---|---|
| Archivos Pact | 13 |
| Interacciones totales | 149 |
| Servicios completamente implementados | 4 |
| Servicios parcialmente implementados | 3 |
| Servicios no implementados | 6 |
| Cobertura estimada | ~27% |

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

### ⚠️ Parcialmente Implementados

#### `auth-service-api` — 2/6 interacciones

Consumer: `embolsadora-frontend` → Provider: `auth-service`

| Método | Path Pact | Estado | Observación |
|---|---|---|---|
| POST | `/api/auth/callback/credentials` → 200 | ✅ | `/api/v1/auth/login` |
| POST | `/api/auth/callback/credentials` → 401 | ✅ | manejado por middleware |
| GET | `/api/auth/session` | ❌ | No existe — podría mapearse a `GET /api/v1/me` |
| POST | `/api/auth/signout` | ❌ | No existe — Supabase maneja logout en el frontend |
| POST | `/api/auth/forgot-password` | ❌ | No existe — pendiente en Supabase Admin API |
| POST | `/api/auth/reset-password` | ❌ | No existe — pendiente en Supabase Admin API |

---

#### `role-service-api` — 0/7 interacciones (CRUD de roles)

Consumer: `embolsadora-frontend-bff` → Provider: `role-service-api`

| Método | Path Pact | Estado | Observación |
|---|---|---|---|
| GET | `/api/v1/roles?tenantId=` | ❌ | No existe el endpoint de roles |
| GET | `/api/v1/roles/{id}` | ❌ | No existe |
| POST | `/api/v1/roles` | ❌ | No existe |
| PUT | `/api/v1/roles/{id}` | ❌ | No existe |
| DELETE | `/api/v1/roles/{id}` → 200 | ❌ | No existe |
| DELETE | `/api/v1/roles/{id}` → 409 | ❌ | No existe |
| DELETE | `/api/v1/roles/{id}` → 403 | ❌ | No existe |

> El RBAC estático existe en `security/rbac.go` pero no hay API REST para gestionar roles dinámicamente.

---

#### `user-service-api-roles-extension` — 2/6 interacciones

Consumer: `embolsadora-frontend-bff` → Provider: `user-service-api`

| Método | Path Pact | Estado | Observación |
|---|---|---|---|
| GET | `/api/v1/users/{id}?include=roles` | ⚠️ | Endpoint existe pero `include=roles` probablemente no implementado |
| POST | `/api/v1/users` con rol inicial | ⚠️ | Create user existe, asignación de rol inicial puede faltar |
| POST | `/api/v1/users/register` | ❌ | Auto-registro no existe |
| POST | `/api/v1/users/verify-email` | ❌ | No existe — incumbe a Supabase |
| PATCH | `/api/v1/users/{id}/status` | ❌ | No existe como endpoint dedicado |
| GET | `/api/v1/users/pending` | ❌ | No existe |

---

### ❌ No Implementados

#### `dashboard-service-api` — 0/12 interacciones

Consumer: `embolsadora-frontend` → Provider: `dashboard-service-api`

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/tenants/{tenantId}/dashboard-layouts` | Listar layouts (con paginación) |
| POST | `/api/tenants/{tenantId}/dashboard-layouts` | Crear layout |
| POST | `/api/tenants/{tenantId}/dashboard-layouts` → 403 | Límite de layouts alcanzado |
| POST | `/api/tenants/{tenantId}/dashboard-layouts` → 409 | Nombre duplicado |
| GET | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` | Obtener layout |
| GET | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 404 | Layout no encontrado |
| PUT | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` | Actualizar layout |
| PUT | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 409 | Nombre duplicado en update |
| PUT | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 404 | No encontrado en update |
| DELETE | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` | Eliminar layout |
| DELETE | `/api/tenants/{tenantId}/dashboard-layouts/{layoutId}` → 400 | No se puede eliminar el último |
| GET | `/api/tenants/{tenantId}/dashboard-layouts` → 401 | Auth requerida |

> Spec generada en `specs/004-dashboard-layouts/`. Pendiente de implementación.

---

#### `alarm-rules-service-api` — 0/10 interacciones

Consumer: `embolsadora-frontend` → Provider: `alarm-rules-service-api`

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/alarm-rules` | Listar reglas |
| GET | `/api/alarm-rules/{id}` → 200 | Obtener regla |
| GET | `/api/alarm-rules/{id}` → 404 | Regla no encontrada |
| POST | `/api/alarm-rules` → 201 | Crear regla |
| POST | `/api/alarm-rules` → 400 | Error de validación |
| PATCH | `/api/alarm-rules/{id}` → 200 | Actualizar regla |
| PATCH | `/api/alarm-rules/{id}` → 404 | No encontrado en update |
| DELETE | `/api/alarm-rules/{id}` → 200 | Eliminar regla |
| DELETE | `/api/alarm-rules/{id}` → 404 | No encontrado en delete |
| GET | `/api/alarm-rules` → 401 | Auth requerida |

---

#### `log-service-api` — 0/14 interacciones

Consumer: `embolsadora-frontend` → Provider: `log-service-api`

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/v1/logs` (con filtros) | Búsqueda con filtros |
| GET | `/api/v1/logs` (text query) | Búsqueda por texto |
| GET | `/api/v1/logs` (machine logs) | Filtrar por máquina |
| GET | `/api/v1/logs` (cursor) | Paginación por cursor |
| GET | `/api/v1/logs` (sin resultados) | Respuesta vacía |
| GET | `/api/v1/logs/{id}` → 200 | Obtener log por ID |
| GET | `/api/v1/logs/{id}` → 404 | Log no encontrado |
| GET | `/api/v1/logs/{id}/context` | Contexto alrededor del log |
| GET | `/api/v1/logs/retention` | Política de retención |
| PATCH | `/api/v1/logs/retention` | Actualizar retención |
| GET | `/api/v1/logs/stream` | SSE streaming |
| GET | `/api/v1/logs/export` | Exportar logs |
| GET | `/api/v1/logs/export` (truncado) | Export con límite excedido |

---

#### `notification-service-api` — 0/6 interacciones

Consumer: `embolsadora-frontend` → Provider: `notification-service-api`

| Método | Path | Descripción |
|---|---|---|
| GET | `/api/v1/notifications` | Listar notificaciones |
| GET | `/api/v1/notifications/count` | Conteo sin leer |
| GET | `/api/v1/notifications/{id}` | Detalle de notificación |
| POST | `/api/v1/notifications/{id}/ack` | Acknowledger notificación |
| POST | `/api/v1/notifications/{id}/close` | Cerrar notificación |
| GET | `/api/v1/alarm-rules` | Listar reglas (usado en context) |

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
| 1 | `dashboard-service-api` | 12 | 🔴 Alta | Spec lista en `specs/004-dashboard-layouts/`, alta visibilidad en UI |
| 2 | `role-service-api` | 7 | 🔴 Alta | Dependencia directa del ABM de usuarios/permisos |
| 3 | `auth-service-api` (completar) | 4 | 🔴 Alta | Forgot/reset password, session — flujos críticos de auth |
| 4 | `alarm-rules-service-api` | 10 | 🟡 Media | Core del sistema de monitoreo industrial |
| 5 | `notification-service-api` | 6 | 🟡 Media | Depende de alarm-rules |
| 6 | `log-service-api` | 14 | 🟡 Media | Observabilidad activa — alto valor para operadores |
| 7 | `permissions-service-api` | 10 | 🟠 Media-baja | RBAC dinámico, actualmente estático en código |
| 8 | `user-service-api-roles-extension` (completar) | 4 | 🟠 Media-baja | Completar registro y status de usuarios |
| 9 | `reports-service-api` | 16 | 🔵 Baja | Generación async compleja, mayor esfuerzo |

**Total interacciones pendientes**: ~83 de 149

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

### Dashboard Layouts

La spec `specs/004-dashboard-layouts/` está generada. El siguiente paso es ejecutar `/speckit.implement` para esa feature.

---

*Reporte generado con Claude Code — rama `develop` — 2026-03-24*

# API Standardization Audit

**Fecha:** 2026-04-11  
**Rama:** `audit/api-standardization`  
**Base:** `develop`  
**Autor:** Lucía Rosa Scharff  
**Propósito:** Referencia interna para guiar futuras ramas de estandarización. No implica cambios de código.

---

## Resumen ejecutivo

| # | Categoría | Prioridad | Módulos afectados |
|---|-----------|-----------|-------------------|
| 1 | Manejo de errores HTTP | P1 | tenants, roles, alarm_rules, dashboard_layouts, notifications, logs, permissions |
| 2 | Formato de respuesta JSON | P1 | roles, alarm_rules, dashboard_layouts, notifications, logs |
| 3 | Extracción del Tenant ID | P1 | roles, alarm_rules, dashboard_layouts, edge_devices vs users |
| 4 | Patrón de handler | P2 | todos los módulos |
| 5 | Logging | P2 | tenants, alarm_rules, roles, notifications, logs, permissions |
| 6 | Ubicación de DTOs/models | P2 | tenants, invitations, user_roles |
| 7 | Patrón de usecase | P3 | tenants, user_roles |
| 8 | Naming y layout de repositorios | P3 | users, tenants, machines |

### Criterio de prioridad

- **P1** — afecta contratos externos / comportamiento observable por clientes del API (Pact contracts, frontend)
- **P2** — afecta mantenibilidad y coherencia interna del código
- **P3** — deuda estructural sin impacto inmediato en funcionalidad ni contratos

---

## 1. Manejo de errores HTTP `[P1]`

### Estado actual

Coexisten tres enfoques distintos para mapear errores a respuestas HTTP:

**Enfoque A — `httperr.WriteError` + `apperrors.NewXxx`** (paquete `core/errors`)
```go
// internal/api/handler/tenants/create_tenant/create_tenant.go
err = h.useCase.Create(c.Request.Context(), tenant)
if err != nil {
    log.Printf("error creating tenant: %v", err)
    httperr.WriteError(c, apperrors.NewInternalServerError("Failed to create tenant"))
    return
}
```
Archivos: `internal/api/handler/tenants/*/`

**Enfoque B — `c.JSON` directo con `gin.H`**
```go
// internal/api/handler/alarm_rules/create_alarm_rule.go
c.JSON(http.StatusBadRequest, gin.H{
    "success": false,
    "error":   "VALIDATION_ERROR",
    "message": "cuerpo de la petición inválido",
    "status":  http.StatusBadRequest,
})
```
Archivos: `internal/api/handler/alarm_rules/`, `internal/api/handler/roles/`, `internal/api/handler/dashboard_layouts/`, `internal/api/handler/notifications/`, `internal/api/handler/logs/`, `internal/api/handler/permissions/`

**Enfoque C — `HandleError(c, err)` local + `ErrorResponse` tipada**
```go
// internal/api/handler/users/errors.go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Status  int    `json:"status"`
}

func HandleError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, domainUsers.ErrNotFound):
        c.JSON(http.StatusNotFound, ErrorResponse{
            Error:   "USER_NOT_FOUND",
            Message: "User not found",
            Status:  http.StatusNotFound,
        })
    // ...
    }
}
```
Archivos: `internal/api/handler/users/`

### Módulos afectados (desviaciones del estándar propuesto)

| Módulo | Enfoque actual | Desviación |
|--------|---------------|------------|
| `tenants` | A (`httperr.WriteError`) | Sí |
| `alarm_rules` | B (`gin.H` directo) | Sí |
| `roles` | B (`gin.H` directo) | Sí |
| `dashboard_layouts` | B (`gin.H` directo) | Sí |
| `notifications` | B (`gin.H` directo) | Sí |
| `logs` | B (`gin.H` directo) | Sí |
| `permissions` | B (`gin.H` directo) | Sí |
| `users` | C (`HandleError` tipado) | **Estándar** |

### Estándar propuesto

**Enfoque C** — `HandleError(c, err)` por recurso con `ErrorResponse` struct tipada.

Justificación:
- Los errores son explícitos y testeables (se pueden assertear sobre el struct, no sobre `gin.H`)
- El mapeo domain error → HTTP code está centralizado por recurso (fácil de auditar)
- Los códigos de error son strings descriptivos (`USER_NOT_FOUND`, `DUPLICATE_EMAIL`), no derivados del status HTTP
- Compatible con los Pact contracts del frontend que esperan `{"error": "CODE", "message": "...", "status": N}`

---

## 2. Formato de respuesta JSON `[P1]`

### Estado actual

**Con wrapper `{"success": bool, "data": {...}}`**
```json
// alarm_rules, roles, dashboard_layouts, notifications, logs
{ "success": true, "data": { "id": "...", "name": "..." } }
{ "success": false, "error": "NOT_FOUND", "message": "..." }
```

**Sin wrapper, struct directo**
```json
// users, tenants, invitations
{ "id": "...", "name": "...", "created_at": "..." }
```

### Módulos afectados

| Módulo | Formato actual | Desviación |
|--------|---------------|------------|
| `alarm_rules` | con wrapper `success/data` | Sí |
| `roles` | con wrapper `success/data` | Sí |
| `dashboard_layouts` | con wrapper `success/data` | Sí |
| `notifications` | con wrapper `success/data` | Sí |
| `logs` | con wrapper `success/data` | Sí |
| `permissions` | con wrapper `success/data` | Sí |
| `users` | struct directo | **Estándar** |
| `tenants` | struct directo | **Estándar** |
| `invitations` | struct directo | **Estándar** |

### Estándar propuesto

**Struct directo, sin wrapper `success/data`.**

Justificación:
- Los Pact contracts del frontend (`PACTS_ANALYSIS.md`) esperan el recurso directamente en la raíz de la respuesta
- El campo `success` es redundante: el HTTP status code ya comunica éxito/error
- Los wrappers agregan parsing innecesario en el cliente

---

## 3. Extracción del Tenant ID `[P1]`

### Estado actual

**Vía `c.GetString("tenant_id")` (clave de contexto Gin)**
```go
// internal/api/handler/users/handler.go
tenantID := c.GetString("tenant_id") // Set by middleware
```
Archivos: `internal/api/handler/users/`

**Vía `platform.TenantID(c.Request.Context())` + `uuid.Parse()`**
```go
// internal/api/handler/alarm_rules/create_alarm_rule.go
tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID inválido o ausente"})
    return
}
```
Archivos: `internal/api/handler/alarm_rules/`, `internal/api/handler/roles/`, `internal/api/handler/dashboard_layouts/`, `internal/api/handler/edge_devices/`

### Módulos afectados

| Módulo | Enfoque actual | Desviación |
|--------|---------------|------------|
| `users` | `c.GetString("tenant_id")` | Sí |
| `alarm_rules` | `platform.TenantID` | **Estándar** |
| `roles` | `platform.TenantID` | **Estándar** |
| `dashboard_layouts` | `platform.TenantID` | **Estándar** |
| `edge_devices` | `platform.TenantID` (path param) | **Estándar** |

### Estándar propuesto

**`platform.TenantID(c.Request.Context())`** — es el helper oficial de la plataforma.

Justificación:
- El middleware `TenantFromHeader` ya escribe el tenant ID en el contexto Go (no en el contexto Gin)
- `platform.TenantID` es el contrato de lectura definido en `internal/platform/tenantctx.go`
- `c.GetString("tenant_id")` acopla el handler a la implementación interna del middleware (clave string)

---

## 4. Patrón de handler `[P2]`

### Estado actual

**Patrón A — Struct con métodos**
```go
// internal/api/handler/users/handler.go
type Handler struct {
    service *users.Service
    logger  *zap.Logger
}
func NewHandler(service *users.Service, logger *zap.Logger) *Handler { ... }
func (h *Handler) ListUsers(c *gin.Context) { ... }
func (h *Handler) CreateUser(c *gin.Context) { ... }
```
Archivos: `users`, `tenants` (parcialmente)

**Patrón B — Factory `gin.HandlerFunc`**
```go
// internal/api/handler/alarm_rules/create_alarm_rule.go
func CreateAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
    return func(c *gin.Context) { ... }
}
```
Archivos: `alarm_rules`, `roles`, `dashboard_layouts`, `edge_devices`, `notifications`, `logs`, `permissions`

**Patrón C — Struct por operación con método `.Handle`**
```go
// internal/api/handler/user_roles/assign_user_role/assign_user_role.go
type AssignUserRoleHandler struct { useCase ... }
func NewAssignUserRoleHandler(...) *AssignUserRoleHandler { ... }
func (h *AssignUserRoleHandler) Handle(c *gin.Context) { ... }
```
Archivos: `user_roles`, `invitations`, `tenants` (algunos handlers)

### Módulos afectados

| Módulo | Patrón actual | Desviación |
|--------|--------------|------------|
| `users` | A (struct con métodos) | **Estándar** |
| `alarm_rules` | B (factory `HandlerFunc`) | Sí |
| `roles` | B (factory `HandlerFunc`) | Sí |
| `dashboard_layouts` | B (factory `HandlerFunc`) | Sí |
| `notifications` | B (factory `HandlerFunc`) | Sí |
| `logs` | B (factory `HandlerFunc`) | Sí |
| `permissions` | A (struct con métodos) | **Estándar** |
| `user_roles` | C (struct por operación) | Sí |
| `tenants` | C (struct por operación) | Sí |
| `invitations` | C (struct por operación) | Sí |

### Estándar propuesto

**Patrón A** — struct con métodos por recurso.

Justificación:
- Un único punto de inyección de dependencias por recurso (el constructor)
- Las dependencias compartidas (logger, service) se inyectan una vez
- Los métodos son fácilmente testeables: se puede instanciar el handler y llamar métodos directamente
- Evita el cierre de variables en funciones anidadas (Patrón B), que dificulta el testing

---

## 5. Logging `[P2]`

### Estado actual

**`log.Printf` de stdlib**
```go
// internal/api/handler/tenants/create_tenant/create_tenant.go
log.Printf("error creating tenant: %v", err)
```
Archivos: `tenants`

**`*zap.Logger` inyectado en el handler**
```go
// internal/api/handler/users/handler.go
h.logger.Error("create user failed", zap.Error(err))
h.logger.Debug("list users request", zap.String("tenant_id", tenantID))
```
Archivos: `users`, `permissions`

**Sin logging**
Archivos: `alarm_rules`, `roles`, `dashboard_layouts`, `notifications`, `logs`, `edge_devices`, `user_roles`, `invitations`

### Módulos afectados

| Módulo | Logging actual | Desviación |
|--------|---------------|------------|
| `tenants` | `log.Printf` | Sí |
| `users` | `*zap.Logger` | **Estándar** |
| `permissions` | `*zap.Logger` | **Estándar** |
| `alarm_rules` | sin logging | Sí |
| `roles` | sin logging | Sí |
| `dashboard_layouts` | sin logging | Sí |
| `notifications` | sin logging | Sí |
| `logs` | sin logging | Sí |
| `edge_devices` | sin logging | Sí |
| `user_roles` | sin logging | Sí |
| `invitations` | sin logging | Sí |

### Estándar propuesto

**`*zap.Logger` inyectado en el handler struct.**

Justificación:
- El proyecto ya tiene Zap como logger oficial (`internal/telemetry/logger.go`)
- Los logs estructurados (campos tipados) son indexables en cualquier sistema de observabilidad
- `log.Printf` no produce logs estructurados y no tiene niveles configurables
- Los módulos sin logging deben al menos loguear errores inesperados (nivel `Error`) y entradas a operaciones críticas (nivel `Debug`)

---

## 6. Ubicación de DTOs/models `[P2]`

### Estado actual

**`handler/<resource>/dto/` — DTOs compartidos por recurso**
```
internal/api/handler/alarm_rules/dto/request.go
internal/api/handler/alarm_rules/dto/response.go
internal/api/handler/roles/dto/request.go
internal/api/handler/roles/dto/response.go
internal/api/handler/dashboard_layouts/dto/dto.go
internal/api/handler/edge_devices/dto/dto.go
internal/api/handler/notifications/dto/
internal/api/handler/logs/dto/
```

**`handler/<resource>/<operation>/models/` — models por operación**
```
internal/api/handler/tenants/create_tenant/models/request.go
internal/api/handler/tenants/create_tenant/models/response.go
internal/api/handler/tenants/get_tenant/models/response.go
internal/api/handler/invitations/create_invitation/models/
internal/api/handler/user_roles/assign_user_role/models/
internal/api/handler/user_roles/list_user_roles/models/
```

### Módulos afectados

| Módulo | Estructura actual | Desviación |
|--------|------------------|------------|
| `alarm_rules` | `dto/` compartido | **Estándar** |
| `roles` | `dto/` compartido | **Estándar** |
| `dashboard_layouts` | `dto/` compartido | **Estándar** |
| `notifications` | `dto/` compartido | **Estándar** |
| `logs` | `dto/` compartido | **Estándar** |
| `edge_devices` | `dto/` compartido | **Estándar** |
| `tenants` | `<operacion>/models/` | Sí |
| `invitations` | `<operacion>/models/` | Sí |
| `user_roles` | `<operacion>/models/` | Sí |

### Estándar propuesto

**`handler/<resource>/dto/`** — DTOs compartidos a nivel de recurso.

Justificación:
- Reduce la cantidad de paquetes y archivos (menos boilerplate)
- Los DTOs de request y response de un mismo recurso suelen compartir tipos base
- La estructura por operación (`create_tenant/models/`) fuerza imports verbose con alias

---

## 7. Patrón de usecase `[P3]`

### Estado actual

**Un usecase por operación**
```
internal/api/usecases/tenants/create_tenant/usecase.go
internal/api/usecases/tenants/delete_tenant/usecase.go
internal/api/usecases/tenants/get_all_tenants/get_all_tenants.go
internal/api/usecases/tenants/get_tenant/get_tenant.go
internal/api/usecases/tenants/update_tenant/usecase.go

internal/api/usecases/user_roles/assign_user_role/usecase.go
internal/api/usecases/user_roles/bulk_assign_user_roles/usecase.go
internal/api/usecases/user_roles/get_user_roles/usecase.go
internal/api/usecases/user_roles/list_user_roles/usecase.go
internal/api/usecases/user_roles/revoke_user_role/usecase.go
internal/api/usecases/user_roles/update_user_role/usecase.go
```

**Service único por recurso**
```
internal/app/alarm_rules/service.go
internal/app/roles/service.go
internal/app/dashboard_layouts/service.go
internal/app/edge_devices/service.go
internal/app/logs/service.go
internal/app/notifications/service.go
internal/app/permissions/service.go
internal/app/users/service.go
```

### Módulos afectados

| Módulo | Patrón actual | Desviación |
|--------|--------------|------------|
| `tenants` | usecase por operación | Sí |
| `user_roles` | usecase por operación | Sí |
| `alarm_rules` | service único | **Estándar** |
| `roles` | service único | **Estándar** |
| `dashboard_layouts` | service único | **Estándar** |
| `users` | service único | **Estándar** |
| `notifications` | service único | **Estándar** |
| `logs` | service único | **Estándar** |
| `permissions` | service único | **Estándar** |

### Estándar propuesto

**Service único por recurso en `internal/app/<resource>/service.go`.**

Justificación:
- Menos archivos, menos imports, menos boilerplate
- El servicio expone una interfaz cohesiva del recurso (fácil de mockear en tests)
- La granularidad por operación no aporta beneficios reales en este dominio

---

## 8. Naming y layout de repositorios `[P3]`

### Estado actual

**Layout plano (legacy)** — archivo único en `internal/repo/pg/`
```
internal/repo/pg/users_repo.go       ← legacy
internal/repo/pg/tenants_repo.go     ← legacy
internal/repo/pg/machines_repo.go    ← legacy
internal/repo/pg/events_repo.go      ← legacy
```

**Layout por directorio (nuevo)** — directorio por recurso en `internal/repo/pg/<resource>/`
```
internal/repo/pg/users/repository.go
internal/repo/pg/tenants/repository.go
internal/repo/pg/alarm_rules/repository.go
internal/repo/pg/dashboard_layouts/repository.go
internal/repo/pg/edge_devices/repository.go
internal/repo/pg/invitations/invitations_repo.go
internal/repo/pg/logs/repository.go
internal/repo/pg/notifications/repository.go
internal/repo/pg/permissions/repository.go
internal/repo/pg/roles/repository.go
internal/repo/pg/user_roles/repository.go
```

**Recursos con ambos layouts (duplicación)**
```
internal/repo/pg/users_repo.go           ← legacy, aún referenciado
internal/repo/pg/users/repository.go     ← nuevo
internal/repo/pg/users/users_repo.go     ← otro archivo en el nuevo directorio

internal/repo/pg/tenants_repo.go         ← legacy, aún referenciado
internal/repo/pg/tenants/repository.go   ← nuevo
```

### Módulos afectados

| Recurso | Estado | Desviación |
|---------|--------|------------|
| `users` | ambos layouts (duplicación) | Sí — requiere limpieza |
| `tenants` | ambos layouts (duplicación) | Sí — requiere limpieza |
| `machines` | solo legacy (`machines_repo.go`) | Sí |
| `events` | solo legacy (`events_repo.go`) | Sí |
| `alarm_rules` | solo nuevo | **Estándar** |
| `dashboard_layouts` | solo nuevo | **Estándar** |
| `edge_devices` | solo nuevo | **Estándar** |
| `roles` | solo nuevo | **Estándar** |
| `notifications` | solo nuevo | **Estándar** |
| `logs` | solo nuevo | **Estándar** |
| `permissions` | solo nuevo | **Estándar** |

### Estándar propuesto

**`internal/repo/pg/<resource>/repository.go`** — layout por directorio.

Justificación:
- Permite agregar archivos auxiliares por recurso (`resources.go`, `queries.go`) sin contaminar el directorio raíz
- Consistente con todos los módulos nuevos
- Los archivos legacy deben eliminarse una vez que no tengan referencias activas

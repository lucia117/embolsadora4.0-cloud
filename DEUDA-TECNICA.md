# 📋 Deuda Técnica - Embolsadora API

Registro de problemas técnicos identificados que requieren corrección pero no son bloqueantes inmediatos.

---

## 🔴 Críticos (Fix Soon)

### 1. JWT Middleware es un Stub sin Validación
**Severity**: ALTA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
El middleware `JWTAuth()` en `internal/api/middleware/middleware.go:12` es un no-op que solo llama `c.Next()` sin validar tokens:

```go
func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        /* TODO */ c.Next()  // Sin validación
    }
}
```

#### Impacto
- ✗ Rutas `/api/tenants/:tenantId/edge-devices` son **públicas sin autenticación**
- ✗ Cualquiera puede leer/modificar dispositivos de otros tenants
- ✗ Sin contexto de usuario autenticado en handlers
- ✗ Incumple especificación que requiere Bearer JWT

#### Rutas Afectadas
```
GET    /api/tenants/:tenantId/edge-devices
POST   /api/tenants/:tenantId/edge-devices
GET    /api/tenants/:tenantId/edge-devices/:deviceId
PUT    /api/tenants/:tenantId/edge-devices/:deviceId
POST   /api/tenants/:tenantId/edge-devices/:deviceId/enable
POST   /api/tenants/:tenantId/edge-devices/:deviceId/disable
POST   /api/tenants/:tenantId/edge-devices/:deviceId/status
POST   /api/tenants/:tenantId/edge-devices/:deviceId/health-check
GET    /api/tenants/:tenantId/edge-devices/:deviceId/telemetry
GET    /api/tenants/:tenantId/edge-devices/:deviceId/events
```

#### Ubicación
- **Archivo**: `internal/api/middleware/middleware.go:12`
- **Aplicación**: `internal/routes/url_mappings.go:92`
- **Contrato**: `specs/003-edge-device-management/contracts/edge-device-service-api.openapi.yaml`

#### Solución Requerida
Implementar validación real de JWT que:
1. Extraiga token del header `Authorization: Bearer <token>`
2. Valide la firma con la clave secreta
3. Extraiga `user_id` y `email` de los claims
4. Popule el contexto con `platform.WithUserID()` y `platform.WithUserEmail()`
5. Retorne 401 si el token es inválido o está expirado

---

### 2. Auditoría Rota: Usuarios Hardcodeados/Aleatorios
**Severity**: ALTA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
Los handlers de status check y health check generan IDs de usuario aleatorios y emails fake:

**Archivo**: `internal/api/handler/edge_devices/status_check.go:33-35`
```go
userID := uuid.New()                    // ← Random UUID cada vez
userEmail := "operator@example.com"     // ← Fake email
result, err := service.StatusCheck(c.Request.Context(), *tenantID, deviceID, userID, userEmail)
```

**Archivo**: `internal/api/handler/edge_devices/health_check.go:25-27`
```go
userID := uuid.New()
userEmail := "operator@example.com"
result, err := service.HealthCheck(c.Request.Context(), *tenantID, deviceID, userID, userEmail)
```

#### Impacto
- ✗ Tabla `device_events` registra usuario fake en cada operación
- ✗ Auditoría completamente rota (no se sabe quién hizo qué)
- ✗ Compliance: imposible cumplir requisitos de auditoría
- ✗ Debugging: no se puede rastrear operaciones a usuarios reales
- ✗ Security: sin responsabilidad de acciones

#### Datos en BD
Cada status/health check crea un evento con usuario diferente:
```sql
INSERT INTO device_events (event_type, user_id, user_email, ...)
VALUES ('STATUS_CHECK', '550e8400-...', 'operator@example.com')  -- random ID
VALUES ('HEALTH_CHECK', '550e8401-...', 'operator@example.com')  -- different ID
```

#### Ubicación
- **Archivo 1**: `internal/api/handler/edge_devices/status_check.go:33-35`
- **Archivo 2**: `internal/api/handler/edge_devices/health_check.go:25-27`
- **Tabla BD**: `device_events(user_id, user_email)`

#### Solución Requerida
Extraer user ID y email del contexto (poblado por JWT middleware):

```go
userID := platform.UserID(c.Request.Context())
if userID == nil {
    c.JSON(http.StatusUnauthorized, gin.H{
        "success": false,
        "error": "user ID not found in context"
    })
    return
}

userEmail := platform.UserEmail(c.Request.Context())
if userEmail == "" {
    c.JSON(http.StatusUnauthorized, gin.H{
        "success": false,
        "error": "user email not found in context"
    })
    return
}

result, err := service.StatusCheck(c.Request.Context(), *tenantID, deviceID, *userID, userEmail)
```

---

## 🟡 Medianos (Fix Before Release)

### 3. Error Handling Demasiado Broad
**Severity**: MEDIA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
Múltiples métodos en service convierten todos los errores a `ErrDeviceNotFound` (404):

**Archivo**: `internal/app/edge_devices/service.go`
- `GetDevice()` línea ~XX
- `StatusCheck()` línea ~XX

#### Impacto
- ✗ Errores de conectividad BD se reportan como 404
- ✗ Timeouts de DB se disfrazan como "device not found"
- ✗ Debugging más difícil (no se ve el error real)
- ✗ Observabilidad: métricas de 404 infladas

#### Solución Requerida
Solo mapear `pgx.ErrNoRows` → `ErrDeviceNotFound` (404)
Otros errores deben propagarse para retornar 500

---

### 4. Postman Collection con URLs Inconsistentes
**Severity**: MEDIA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
Master collection usa `baseUrl = /api/v1` pero Edge Devices está en `/api/tenants/...` (sin `/v1`)

**Archivo**: `postman/Embolsadora-API-Complete.postman_collection.json`

URLs generadas:
```
❌ /api/v1/tenants/:tenantId/edge-devices  (no existe)
✅ /api/tenants/:tenantId/edge-devices     (correcto)
```

#### Impacto
- ✗ Edge Devices requests en Postman retornan 404
- ✗ Usuarios no pueden testear endpoints con la colección
- ✗ Confusión sobre URL correcta

#### Solución Requerida
Separar `baseUrl` y ajustar rutas de Edge Devices:
- Opción A: Crear variable `edgeDevicesBaseUrl = /api`
- Opción B: Documentar que usar `/api/tenants/` en lugar de `{{baseUrl}}/tenants/`

---

### 5. Down Migration con Orden Incorrecto
**Severity**: MEDIA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
Down migration intenta dropar trigger después de la tabla:

**Archivo**: `migrations/0005_create_edge_devices_tables.down.sql`
```sql
DROP TABLE edge_devices;                              -- Borra tabla primero
DROP TRIGGER trg_edge_devices_updated_at ON edge_devices;  -- Trigger falla
```

#### Impacto
- ✗ `DOWN` migration falla en rollback
- ✗ No se puede deshacer la migración

#### Solución Requerida
```sql
DROP TRIGGER IF EXISTS trg_edge_devices_updated_at ON edge_devices;
DROP FUNCTION IF EXISTS update_edge_devices_updated_at();
DROP TABLE IF EXISTS device_events;
DROP TABLE IF EXISTS edge_devices;
```

---

### 6. Timestamps Cero en Repository
**Severity**: MEDIA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
`Create()` inserta `created_at` / `updated_at` de valores del struct (no inicializados):

**Archivo**: `internal/repo/pg/edge_devices/repository.go` - método `Create()`

#### Impacto
- ✗ Timestamps guardados como zero value (1970-01-01)
- ✗ Consultas por fecha no funcionan
- ✗ Auditoría temporal incorrecta

#### Solución Requerida
Omitir columnas de timestamp en INSERT y dejar BD asigne defaults:
```sql
INSERT INTO edge_devices (id, tenant_id, name, ...)
VALUES ($1, $2, $3, ...)
RETURNING created_at, updated_at
```

Luego asignar los valores devueltos al struct.

---

### 7. Update Migration Usa Timestamp del Struct
**Severity**: MEDIA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
`Update()` asigna `updated_at = $5` con valor posiblemente cero:

**Archivo**: `internal/repo/pg/edge_devices/repository.go` - método `Update()`

#### Impacto
- ✗ Campo `updated_at` queda con valor stale
- ✗ Trigger de actualización automática se ignora

#### Solución Requerida
Usar `CURRENT_TIMESTAMP` y leer el valor actual:
```sql
UPDATE edge_devices
SET updated_at = CURRENT_TIMESTAMP
RETURNING updated_at
```

---

## 🟢 Menores (Nice to Have)

### 8. Documentación con Comandos Incorrectos
**Severity**: BAJA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Archivos Afectados
- `postman/POSTMAN-GUIDE.md` - refiere `go run cmd/main.go` (no existe)
- `postman/EDGE-DEVICE-README.md` - idem
- `specs/003-edge-device-management/plan/quickstart.md` - refiere `cmd/migrate/main.go` (no existe)

#### Solución
Actualizar a:
```bash
go run cmd/api/main.go      # Para iniciar servidor
make migrate-up             # Para migraciones
```

---

### 9. Spec vs Implementación Mismatch en Telemetry
**Severity**: BAJA
**Status**: NO INICIADO
**Detectado por**: Copilot Code Review (PR #17)
**Fecha**: 2026-03-12

#### Problema
DTO y domain models no coinciden con OpenAPI contract:

**Actual**:
```go
type TelemetryResponse struct {
    CPU float64   // Simple número
    RAM float64   // Simple número
    Disk float64  // Simple número
}
```

**Contrato OpenAPI**:
```yaml
cpu:
  type: object
  properties:
    usagePercent: number
ram:
  type: object
  properties:
    usedPercent: number
    usedMb: number
    totalMb: number
```

#### Impacto
- ✗ Pact validation falla
- ✗ Respuestas no conforman al contrato
- ✗ Documentación vs código desalineados

---

## 📊 Resumen por Severidad

| Severidad | Cantidad | Fix Before | Archivos |
|-----------|----------|------------|----------|
| 🔴 ALTA | 2 | MVP Completamente | 4 |
| 🟡 MEDIA | 5 | Pre-Release | 10 |
| 🟢 BAJA | 2 | Nice-to-Have | 4 |
| **TOTAL** | **9** | | **18** |

---

## 🔄 Tracking de Resolución

| ID | Problema | Status | PR/Commit | Resuelto Por |
|----|----------|--------|-----------|-------------|
| 1 | JWT Stub | ⏳ NO INICIADO | - | - |
| 2 | Auditoría Rota | ⏳ NO INICIADO | - | - |
| 3 | Error Handling Broad | ⏳ NO INICIADO | - | - |
| 4 | Postman URLs | ⏳ NO INICIADO | - | - |
| 5 | Down Migration | ⏳ NO INICIADO | - | - |
| 6 | Timestamps Create | ⏳ NO INICIADO | - | - |
| 7 | Timestamps Update | ⏳ NO INICIADO | - | - |
| 8 | Docs Comandos | ⏳ NO INICIADO | - | - |
| 9 | Telemetry Mismatch | ⏳ NO INICIADO | - | - |

---

**Última actualización**: 2026-03-12
**Detectado en**: PR #17 (Edge Device Management API MVP)
**Fuente**: Copilot Code Review (17 comentarios primera review, 7 segunda review)

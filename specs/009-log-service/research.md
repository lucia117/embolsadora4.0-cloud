# Research: Log Service API (009)

**Feature**: `009-log-service`  
**Date**: 2026-04-07

---

## 1. Paginación por cursor en PostgreSQL + Go

**Decision**: Cursor keyset encoding con `(created_at DESC, id DESC)` como clave compuesta, codificado en base64 JSON.

**Rationale**: La paginación por offset (`LIMIT/OFFSET`) es inconsistente con inserciones concurrentes y degrada en O(n) para páginas tardías. El cursor keyset con `WHERE (created_at, id) < ($cursor_ts, $cursor_id)` es O(log n) con el índice compuesto correcto.

**Implementación**:
```sql
-- Índice necesario:
CREATE INDEX idx_log_entries_tenant_cursor ON log_entries(tenant_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL;

-- Query de paginación:
SELECT * FROM log_entries
WHERE tenant_id = $1
  AND (created_at, id) < ($cursor_ts, $cursor_id)  -- si hay cursor
ORDER BY created_at DESC, id DESC
LIMIT $limit;
```

**Alternatives considered**: UUID v7 como cursor (más simple pero acopla el ID a timestamp), Relay cursor spec (más estándar pero overhead de serialización).

---

## 2. Server-Sent Events (SSE) en Gin

**Decision**: SSE nativo con Gin `c.Stream()` + `c.SSEvent()`, sin librerías adicionales.

**Rationale**: Gin tiene soporte nativo de SSE. No se justifica agregar dependencias para funcionalidad incluida en el framework. El patrón de streaming con canal Go es idiomático.

**Implementación**:
```go
func (h *Handler) Stream(c *gin.Context) {
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    tenantID := platform.TenantID(c.Request.Context())
    ch := h.svc.Subscribe(tenantID)
    defer h.svc.Unsubscribe(tenantID, ch)

    c.Stream(func(w io.Writer) bool {
        select {
        case event, ok := <-ch:
            if !ok { return false }
            c.SSEvent("log", event)
            return true
        case <-time.After(30 * time.Second):
            c.SSEvent("heartbeat", "")
            return true
        case <-c.Request.Context().Done():
            return false
        }
    })
}
```

**Alternatives considered**: WebSockets (overhead innecesario para streaming unidireccional), polling (peor UX y más carga en servidor).

---

## 3. Ingesta de logs (write path)

**Decision**: El servicio de logs expone una interfaz `LogWriter` interna usada por otros servicios del mismo proceso. No hay endpoint HTTP de ingesta en esta feature.

**Rationale**: La spec define este servicio como de lectura desde el frontend. La escritura de logs ocurre internamente cuando eventos de dominio suceden (alarma disparada, dispositivo conectado/desconectado, acción de usuario). El `LogWriter` es una interfaz inyectable, lo que permite testing sin BD.

**Arquitectura del write path**:
```
Evento de dominio → app/alarm_rules/service.go
                  → platform/logwriter/writer.go (LogWriter interface)
                  → repo/pg/logs/repository.go (implementación)
                  → tabla log_entries
```

**Alternatives considered**: Endpoint HTTP interno (requiere red + auth), tabla de eventos separada (duplicación), trigger de BD (poco flexible para metadata).

**Nota**: En esta feature solo se implementa el read path + `LogWriter` interface. La integración de escritura en otros servicios es trabajo futuro (deuda técnica).

---

## 4. Formato de exportación

**Decision**: JSON por defecto, con soporte opcional de CSV via query param `format=csv`.

**Rationale**: El Pact del frontend usa JSON. CSV es útil para análisis en planillas pero no está especificado como obligatorio. Se implementa JSON primero y CSV como extensión simple.

**Response para export**:
```json
{
  "data": [...],
  "truncated": false,
  "total_available": 1523,
  "exported_count": 1000
}
```

**Alternatives considered**: NDJSON (mejor para streaming pero peor soporte en browsers), Parquet (overkill para este volumen).

---

## 5. Número de migración

**Decision**: Usar `000004` en esta rama (branched desde main donde la última es `000003`).

**Rationale**: La rama `008-alarm-rules` (PR #25, pendiente de merge) usa 000004–000014. Habrá conflicto de números cuando ambas PRs se mergeen a main. La resolución se hace al momento del merge: esta migración se renumerará al siguiente disponible.

**Plan de resolución**: Al hacer merge de esta rama a main, si 008 ya fue mergeado, renumerar `000004` → `000015`.

---

## 6. Ruta de registro: conflicto Gin `/logs/retention` vs `/logs/:id`

**Decision**: Registrar rutas estáticas antes que rutas con parámetros en el grupo de Gin.

**Rationale**: Gin resuelve rutas en orden de registro. `GET /logs/retention`, `GET /logs/stream` y `GET /logs/export` deben registrarse **antes** de `GET /logs/:id` para que Gin no trate "retention", "stream" y "export" como valores del parámetro `:id`.

**Implementación**:
```go
// routes.go — orden crítico:
readGroup.GET("/logs/retention", h.GetRetention)  // ← primero estáticas
readGroup.GET("/logs/stream", h.Stream)
readGroup.GET("/logs/export", h.Export)
readGroup.GET("/logs", h.List)
readGroup.GET("/logs/:id/context", h.GetContext)
readGroup.GET("/logs/:id", h.Get)                 // ← último el wildcard
```

---

## 7. Contexto alrededor de un log (`/context`)

**Decision**: `window_size` configurable via query param (default 10), retorna N eventos antes y N después del log solicitado, ordenados cronológicamente.

**Query**:
```sql
(SELECT * FROM log_entries WHERE tenant_id=$1 AND created_at <= $ts AND id != $id ORDER BY created_at DESC, id DESC LIMIT $n)
UNION ALL
(SELECT * FROM log_entries WHERE tenant_id=$1 AND created_at > $ts ORDER BY created_at ASC LIMIT $n)
ORDER BY created_at ASC, id ASC
```

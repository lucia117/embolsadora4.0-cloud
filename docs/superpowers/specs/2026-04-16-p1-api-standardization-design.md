# P1 API Standardization — Design Spec

**Fecha:** 2026-04-16
**Rama:** `fix/copilot-review-pr31`
**Referencia:** `docs/audit/api-standardization-audit.md`, `docs/superpowers/plans/2026-04-11-p1-error-response-standardization.md`

---

## Objetivo

Estandarizar el manejo de errores HTTP y el formato de respuesta JSON en todos los módulos de la API para que sean compatibles con los Pact contracts del frontend.

---

## Patrón estándar

### Error responses

Cada módulo define su propio `errors.go` con:

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Status  int    `json:"status"`
}

func HandleError(c *gin.Context, err error) { /* domain error → HTTP */ }
func invalidTenantResponse(c *gin.Context)  { /* 400 INVALID_TENANT */ }
func invalidIDResponse(c *gin.Context)      { /* 400 INVALID_ID */ }
```

Referencia implementada: `internal/api/handler/users/errors.go`
Piloto disponible: `internal/api/handler/alarm_rules/errors.go` (rama `audit/api-standardization`)

### Success responses

Struct directo, sin wrapper `{"success": true, "data": ...}`:

```go
c.JSON(http.StatusOK, dto.FromDomain(item))   // single resource
c.JSON(http.StatusOK, items)                   // list (plain array)
```

**Excepción — logs:** El Pact espera `{"data": [...], ...}`. Se mantiene el campo `data` pero se elimina `success`.

---

## Cambios por módulo

| Módulo | Errors | Success responses | Observación |
|--------|--------|-------------------|-------------|
| `alarm_rules` | cherry-pick piloto | array directo | Piloto ya listo en rama audit |
| `roles` | crear `errors.go` | array directo | |
| `dashboard_layouts` | crear `errors.go` | struct directo sin wrapper | |
| `notifications` | crear `errors.go`, fix `code`→`error` | sin cambios | Success ya es correcto |
| `logs` | crear `errors.go` | mantener `data:[]`, quitar `success` | Pact requiere campo `data` |
| `permissions` | alinear `handlePermissionError` al struct tipado | sin cambios | Success ya es correcto |
| `tenants` | `httperr.WriteError` → `ErrorResponse` SCREAMING_SNAKE_CASE | sin cambios | |
| `users` | sin cambios | sin cambios | Solo fix tenant ID: `c.GetString` → `platform.TenantID` |

---

## Orden de ejecución

1. `alarm_rules` — cherry-pick + compilar
2. `roles` — errors.go + reescribir handlers + compilar
3. `dashboard_layouts` — errors.go + reescribir handlers + compilar
4. `notifications` — errors.go + fix error fields + compilar
5. `logs` — errors.go + quitar `success` de list responses + compilar
6. `permissions` — alinear error handler + compilar
7. `tenants` — reemplazar httperr + compilar
8. `users` — fix tenant ID extraction + compilar
9. Build completo final

---

## Constraints

- Compilar después de cada módulo
- No tocar lógica de negocio ni signatures de servicio
- No cambiar campos de success responses que el Pact ya valida como correctos
- Mantener `data` field en logs (Pact lo requiere)

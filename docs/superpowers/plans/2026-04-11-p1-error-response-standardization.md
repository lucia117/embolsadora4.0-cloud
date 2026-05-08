# P1 Error & Response Standardization Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Estandarizar el manejo de errores HTTP y el formato de respuesta JSON en todos los módulos de la API, eliminando el wrapper `success/data` y unificando el tipo `ErrorResponse`.

**Architecture:** Cada módulo handler define su propio `errors.go` con la función `HandleError` y el struct `ErrorResponse` (patrón del módulo `users`). Las respuestas exitosas retornan el recurso directamente, sin wrapper. Se elimina `gin.H{...}` para errores y se usan structs tipados.

**Tech Stack:** Go 1.24, Gin, Zap, `internal/platform` (TenantID helper), `internal/domain` (errores de dominio)

---

## Estándar de referencia

El patrón correcto está implementado en `internal/api/handler/users/`:

```go
// ErrorResponse — struct tipado, sin campo "success"
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Status  int    `json:"status"`
}

// HandleError — mapeo explícito domain error → HTTP
func HandleError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, domain.ErrFoo):
        c.JSON(http.StatusNotFound, ErrorResponse{
            Error:   "FOO_NOT_FOUND",
            Message: "foo no encontrado",
            Status:  http.StatusNotFound,
        })
    default:
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Error:   "INTERNAL_ERROR",
            Message: "An internal error occurred",
            Status:  http.StatusInternalServerError,
        })
    }
}
```

Respuestas exitosas: struct directo, sin wrapper:
```go
// ✅ correcto
c.JSON(http.StatusOK, dto.FromDomain(rule))

// ❌ incorrecto
c.JSON(http.StatusOK, gin.H{"success": true, "data": dto.FromDomain(rule)})
```

---

## Verificación de compilación

Todos los pasos de verificación usan:
```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."
```
Salida esperada: sin output (exit 0).

---

## Task 1: alarm_rules — errors.go + reescritura de handlers

**Archivos:**
- Crear: `internal/api/handler/alarm_rules/errors.go`
- Modificar: `internal/api/handler/alarm_rules/list_alarm_rules.go`
- Modificar: `internal/api/handler/alarm_rules/get_alarm_rule.go`
- Modificar: `internal/api/handler/alarm_rules/create_alarm_rule.go`
- Modificar: `internal/api/handler/alarm_rules/update_alarm_rule.go`
- Modificar: `internal/api/handler/alarm_rules/delete_alarm_rule.go`

- [ ] **Paso 1.1: Crear `internal/api/handler/alarm_rules/errors.go`**

```go
package alarm_rules

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// ErrorResponse es el formato estándar de error HTTP para alarm rules.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// HandleError mapea errores de dominio/aplicación a respuestas HTTP.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrAlarmRuleNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "ALARM_RULE_NOT_FOUND",
			Message: "Regla de alarma no encontrada",
			Status:  http.StatusNotFound,
		})
	case errors.Is(err, appAlarmRules.ErrNameRequired),
		errors.Is(err, appAlarmRules.ErrMetricRequired),
		errors.Is(err, appAlarmRules.ErrInvalidOperator),
		errors.Is(err, appAlarmRules.ErrInvalidSeverity):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
			Status:  http.StatusBadRequest,
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An internal error occurred",
			Status:  http.StatusInternalServerError,
		})
	}
}

// invalidTenantResponse responde con error de tenant inválido.
func invalidTenantResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_TENANT",
		Message: "X-Tenant-ID inválido o ausente",
		Status:  http.StatusBadRequest,
	})
}

// invalidIDResponse responde con error de UUID inválido.
func invalidIDResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_ID",
		Message: "El ID proporcionado no es un UUID válido",
		Status:  http.StatusBadRequest,
	})
}
```

- [ ] **Paso 1.2: Reescribir `list_alarm_rules.go`**

```go
package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListAlarmRules godoc
// GET /api/v1/alarm-rules
func ListAlarmRules(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		rules, err := service.ListAlarmRules(c.Request.Context(), tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		items := make([]dto.AlarmRuleResponse, len(rules))
		for i, r := range rules {
			items[i] = dto.FromDomain(r)
		}

		c.JSON(http.StatusOK, items)
	}
}
```

- [ ] **Paso 1.3: Reescribir `get_alarm_rule.go`**

```go
package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// GetAlarmRule godoc
// GET /api/v1/alarm-rules/:id
func GetAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			invalidIDResponse(c)
			return
		}

		rule, err := service.GetAlarmRule(c.Request.Context(), id, tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(rule))
	}
}
```

- [ ] **Paso 1.4: Reescribir `create_alarm_rule.go`**

```go
package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateAlarmRule godoc
// POST /api/v1/alarm-rules
func CreateAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		var req dto.CreateAlarmRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: "Cuerpo de la petición inválido",
				Status:  http.StatusBadRequest,
			})
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		input := appAlarmRules.CreateAlarmRuleInput{
			Name:        req.Name,
			Description: req.Description,
			Metric:      req.Metric,
			Operator:    req.Operator,
			Threshold:   req.Threshold,
			Severity:    req.Severity,
			Enabled:     enabled,
		}

		rule, err := service.CreateAlarmRule(c.Request.Context(), tenantID, input)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusCreated, dto.FromDomain(rule))
	}
}
```

- [ ] **Paso 1.5: Reescribir `update_alarm_rule.go`**

```go
package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// UpdateAlarmRule godoc
// PATCH /api/v1/alarm-rules/:id
func UpdateAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			invalidIDResponse(c)
			return
		}

		var req dto.UpdateAlarmRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: "Cuerpo de la petición inválido",
				Status:  http.StatusBadRequest,
			})
			return
		}

		input := appAlarmRules.UpdateAlarmRuleInput{
			Name:        req.Name,
			Description: req.Description,
			Metric:      req.Metric,
			Operator:    req.Operator,
			Threshold:   req.Threshold,
			Severity:    req.Severity,
			Enabled:     req.Enabled,
		}

		rule, err := service.UpdateAlarmRule(c.Request.Context(), id, tenantID, input)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(rule))
	}
}
```

- [ ] **Paso 1.6: Reescribir `delete_alarm_rule.go`**

```go
package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// DeleteAlarmRule godoc
// DELETE /api/v1/alarm-rules/:id
func DeleteAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			invalidIDResponse(c)
			return
		}

		if err := service.DeleteAlarmRule(c.Request.Context(), id, tenantID); err != nil {
			HandleError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
```

> **Nota:** DELETE cambia de `200 {"success": true}` a `204 No Content`. Si el Pact espera 200, ajustar a `c.JSON(http.StatusOK, gin.H{})`.

- [ ] **Paso 1.7: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/alarm_rules/..."
```
Salida esperada: sin output (exit 0).

- [ ] **Paso 1.8: Pedir aprobación y commitear**

```bash
git add internal/api/handler/alarm_rules/
git commit -m "refactor(alarm_rules): estandarizar manejo de errores y formato de respuesta"
```

---

## Task 2: roles — errors.go + reescritura de handlers

**Archivos:**
- Crear: `internal/api/handler/roles/errors.go`
- Modificar: `internal/api/handler/roles/list_roles.go`
- Modificar: `internal/api/handler/roles/get_role.go`
- Modificar: `internal/api/handler/roles/create_role.go`
- Modificar: `internal/api/handler/roles/update_role.go`
- Modificar: `internal/api/handler/roles/delete_role.go`

- [ ] **Paso 2.1: Crear `internal/api/handler/roles/errors.go`**

```go
package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// ErrorResponse es el formato estándar de error HTTP para roles.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// HandleError mapea errores de dominio a respuestas HTTP.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrRoleNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "ROLE_NOT_FOUND",
			Message: "Rol no encontrado",
			Status:  http.StatusNotFound,
		})
	case errors.Is(err, domain.ErrRoleIsSystemRole):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "SYSTEM_ROLE",
			Message: err.Error(),
			Status:  http.StatusForbidden,
		})
	case errors.Is(err, domain.ErrRoleDuplicateName):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "DUPLICATE_NAME",
			Message: err.Error(),
			Status:  http.StatusConflict,
		})
	case errors.Is(err, domain.ErrRoleLimitReached):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "LIMIT_REACHED",
			Message: err.Error(),
			Status:  http.StatusForbidden,
		})
	case errors.Is(err, domain.ErrRoleHasAssignments):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "ROLE_HAS_ASSIGNMENTS",
			Message: err.Error(),
			Status:  http.StatusConflict,
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An internal error occurred",
			Status:  http.StatusInternalServerError,
		})
	}
}

// invalidTenantResponse responde con error de tenant inválido.
func invalidTenantResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_TENANT",
		Message: "X-Tenant-ID inválido o ausente",
		Status:  http.StatusBadRequest,
	})
}
```

- [ ] **Paso 2.2: Reescribir `list_roles.go`**

```go
package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListRoles godoc
// GET /api/v1/roles
func ListRoles(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		roles, err := service.ListRoles(c.Request.Context(), tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		items := make([]dto.RoleResponse, len(roles))
		for i, r := range roles {
			items[i] = dto.FromDomain(r)
		}

		c.JSON(http.StatusOK, items)
	}
}
```

- [ ] **Paso 2.3: Reescribir `get_role.go`**

```go
package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
)

// GetRole godoc
// GET /api/v1/roles/:id
func GetRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		role, err := service.GetRole(c.Request.Context(), id)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(role))
	}
}
```

- [ ] **Paso 2.4: Reescribir `create_role.go`**

```go
package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateRole godoc
// POST /api/v1/roles
func CreateRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		var req dto.CreateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_REQUEST",
				Message: err.Error(),
				Status:  http.StatusBadRequest,
			})
			return
		}

		role, err := service.CreateRole(c.Request.Context(), tenantID, req.Name, req.Description, req.Permissions)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusCreated, dto.FromDomain(role))
	}
}
```

- [ ] **Paso 2.5: Reescribir `update_role.go`**

```go
package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
)

// UpdateRole godoc
// PUT /api/v1/roles/:id
func UpdateRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req dto.UpdateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_REQUEST",
				Message: err.Error(),
				Status:  http.StatusBadRequest,
			})
			return
		}

		role, err := service.UpdateRole(c.Request.Context(), id, req.Name, req.Description, req.Permissions)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(role))
	}
}
```

- [ ] **Paso 2.6: Reescribir `delete_role.go`**

El caso `ErrRoleHasAssignments` incluía un campo extra `usersAffected` que requería una query extra. Se elimina ese campo: el conteo ya no forma parte de la respuesta de error (solo el código).

```go
package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
)

// DeleteRole godoc
// DELETE /api/v1/roles/:id
func DeleteRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		if err := service.DeleteRole(c.Request.Context(), id); err != nil {
			HandleError(c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
```

> **Nota:** Se elimina el campo `usersAffected` de la respuesta de error `ROLE_HAS_ASSIGNMENTS`. Si el Pact frontend lo espera, mantener en `HandleError` con una query adicional al service.

- [ ] **Paso 2.7: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/roles/..."
```
Salida esperada: sin output (exit 0).

- [ ] **Paso 2.8: Pedir aprobación y commitear**

```bash
git add internal/api/handler/roles/
git commit -m "refactor(roles): estandarizar manejo de errores y formato de respuesta"
```

---

## Task 3: dashboard_layouts — limpiar DTOs + reescribir handlers

**Archivos:**
- Modificar: `internal/api/handler/dashboard_layouts/dto/dto.go`
- Modificar: `internal/api/handler/dashboard_layouts/create_layout.go`
- Modificar: `internal/api/handler/dashboard_layouts/list_layouts.go`
- Modificar: `internal/api/handler/dashboard_layouts/get_layout.go`
- Modificar: `internal/api/handler/dashboard_layouts/update_layout.go`
- Modificar: `internal/api/handler/dashboard_layouts/delete_layout.go`

- [ ] **Paso 3.1: Actualizar `dto/dto.go` — eliminar campo `Success` de todos los DTOs**

Reemplazar los structs de respuesta existentes con versiones sin `Success`:

```go
// ListLayoutsResponse — sin campo Success
type ListLayoutsResponse struct {
	Data []LayoutDTO `json:"data"`
	Meta MetaDTO     `json:"meta"`
}

// DeleteLayoutResponse — sin campo Success ni Message
// (se elimina este struct; DELETE devuelve 204 No Content)

// ErrorResponse — sin campo Success, con Message y Status
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}
```

Eliminar `SingleLayoutResponse` y `DeleteLayoutResponse`. Las respuestas de operaciones singulares retornan `LayoutDTO` directamente.

El archivo `dto/dto.go` completo queda:

```go
package dto

import (
	"time"

	"github.com/google/uuid"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

// PositionDTO represents the grid position of a widget.
type PositionDTO struct {
	X int    `json:"x"`
	Y int    `json:"y"`
	W int    `json:"w"`
	H int    `json:"h"`
	I string `json:"i"`
}

// WidgetDTO represents a widget in JSON responses and requests.
type WidgetDTO struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Icon        string      `json:"icon"`
	Position    PositionDTO `json:"position"`
}

// LayoutDTO represents a dashboard layout in JSON responses.
type LayoutDTO struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Widgets   []WidgetDTO `json:"widgets"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// MetaDTO carries pagination/limit metadata for list responses.
type MetaDTO struct {
	Total int `json:"total"`
	Limit int `json:"limit"`
}

// ListLayoutsResponse is the response for GET /dashboard-layouts.
type ListLayoutsResponse struct {
	Data []LayoutDTO `json:"data"`
	Meta MetaDTO     `json:"meta"`
}

// ErrorResponse is the standard error response for dashboard layouts.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// CreateLayoutRequest is the request body for POST /dashboard-layouts.
type CreateLayoutRequest struct {
	Name    string      `json:"name" binding:"required"`
	Widgets []WidgetDTO `json:"widgets"`
}

// UpdateLayoutRequest is the request body for PUT /dashboard-layouts/:id.
type UpdateLayoutRequest struct {
	Name    string      `json:"name" binding:"required"`
	Widgets []WidgetDTO `json:"widgets"`
}

// ToLayoutDTO converts a domain DashboardLayout to a LayoutDTO.
func ToLayoutDTO(layout *domain.DashboardLayout) LayoutDTO {
	widgets := make([]WidgetDTO, len(layout.Widgets))
	for i, w := range layout.Widgets {
		widgets[i] = WidgetDTO{
			ID:          w.ID,
			Type:        w.Type,
			Name:        w.Name,
			Title:       w.Title,
			Description: w.Description,
			Category:    w.Category,
			Icon:        w.Icon,
			Position: PositionDTO{
				X: w.Position.X,
				Y: w.Position.Y,
				W: w.Position.W,
				H: w.Position.H,
				I: w.Position.I,
			},
		}
	}
	return LayoutDTO{
		ID:        layout.ID,
		Name:      layout.Name,
		Widgets:   widgets,
		CreatedAt: layout.CreatedAt,
		UpdatedAt: layout.UpdatedAt,
	}
}

// ToWidgetsDomain converts a slice of WidgetDTO to domain Widgets.
func ToWidgetsDomain(dtos []WidgetDTO) []domain.Widget {
	if dtos == nil {
		return []domain.Widget{}
	}
	widgets := make([]domain.Widget, len(dtos))
	for i, d := range dtos {
		widgets[i] = domain.Widget{
			ID:          d.ID,
			Type:        d.Type,
			Name:        d.Name,
			Title:       d.Title,
			Description: d.Description,
			Category:    d.Category,
			Icon:        d.Icon,
			Position: domain.Position{
				X: d.Position.X,
				Y: d.Position.Y,
				W: d.Position.W,
				H: d.Position.H,
				I: d.Position.I,
			},
		}
	}
	return widgets
}
```

- [ ] **Paso 3.2: Reescribir `create_layout.go`**

```go
package dashboard_layouts

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts/dto"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateLayout creates a new dashboard layout for the (tenant, user).
func CreateLayout(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "INVALID_TENANT",
				Message: "X-Tenant-ID inválido o ausente",
				Status:  http.StatusBadRequest,
			})
			return
		}

		userID := platform.UserID(c.Request.Context())
		if userID == nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   "UNAUTHORIZED",
				Message: "User not authenticated",
				Status:  http.StatusUnauthorized,
			})
			return
		}

		var req dto.CreateLayoutRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: err.Error(),
				Status:  http.StatusBadRequest,
			})
			return
		}

		cmd := domain.CreateLayoutCommand{
			Name:    req.Name,
			Widgets: dto.ToWidgetsDomain(req.Widgets),
		}

		layout, err := service.CreateLayout(c.Request.Context(), tenantID, *userID, cmd)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrLimitReached):
				c.JSON(http.StatusForbidden, dto.ErrorResponse{
					Error:   "LIMIT_REACHED",
					Message: err.Error(),
					Status:  http.StatusForbidden,
				})
			case errors.Is(err, domain.ErrDuplicateName):
				c.JSON(http.StatusConflict, dto.ErrorResponse{
					Error:   "DUPLICATE_NAME",
					Message: err.Error(),
					Status:  http.StatusConflict,
				})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
					Error:   "INTERNAL_ERROR",
					Message: "An internal error occurred",
					Status:  http.StatusInternalServerError,
				})
			}
			return
		}

		c.JSON(http.StatusOK, dto.ToLayoutDTO(layout))
	}
}
```

- [ ] **Paso 3.3: Actualizar `list_layouts.go`, `get_layout.go`, `update_layout.go`, `delete_layout.go`**

Leer cada archivo y aplicar los mismos cambios:
1. Reemplazar `dto.ErrorResponse{Success: false, Error: "..."}` por `dto.ErrorResponse{Error: "...", Message: "...", Status: N}`
2. Reemplazar `dto.SingleLayoutResponse{Success: true, Data: ...}` por `dto.ToLayoutDTO(layout)` directamente
3. Reemplazar `dto.ListLayoutsResponse{Success: true, Data: ..., Meta: ...}` por `dto.ListLayoutsResponse{Data: ..., Meta: ...}`
4. `delete_layout.go`: retornar `c.Status(http.StatusNoContent)` en lugar de `dto.DeleteLayoutResponse{Success: true, ...}`

- [ ] **Paso 3.4: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/dashboard_layouts/..."
```

- [ ] **Paso 3.5: Pedir aprobación y commitear**

```bash
git add internal/api/handler/dashboard_layouts/
git commit -m "refactor(dashboard_layouts): eliminar wrapper success/data de respuestas"
```

---

## Task 4: notifications — errors.go + estandarizar error responses

**Contexto:** Los handlers de notifications usan `gin.H{"error": "...", "code": "..."}` — los campos tienen nombres invertidos respecto al estándar (`code` en lugar de `error`, `error` en lugar de `message`). Las respuestas de éxito ya usan structs tipados (sin wrapper `success`). Solo se corrigen los responses de error.

**Archivos:**
- Crear: `internal/api/handler/notifications/errors.go`
- Modificar: `internal/api/handler/notifications/list_notifications.go`
- Modificar: `internal/api/handler/notifications/get_notification.go`
- Modificar: `internal/api/handler/notifications/ack_notification.go`
- Modificar: `internal/api/handler/notifications/close_notification.go`
- Modificar: `internal/api/handler/notifications/count_notifications.go`

- [ ] **Paso 4.1: Crear `internal/api/handler/notifications/errors.go`**

```go
package notifications

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// ErrorResponse es el formato estándar de error HTTP para notifications.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// HandleError mapea errores de dominio a respuestas HTTP.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrNotificationNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOTIFICATION_NOT_FOUND",
			Message: "Notificación no encontrada",
			Status:  http.StatusNotFound,
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An internal error occurred",
			Status:  http.StatusInternalServerError,
		})
	}
}

// invalidTenantResponse responde con error de tenant inválido.
func invalidTenantResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_TENANT",
		Message: "X-Tenant-ID inválido o ausente",
		Status:  http.StatusBadRequest,
	})
}

// invalidIDResponse responde con error de UUID inválido.
func invalidIDResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_ID",
		Message: "El ID proporcionado no es un UUID válido",
		Status:  http.StatusBadRequest,
	})
}
```

- [ ] **Paso 4.2: Actualizar `list_notifications.go`**

Reemplazar todas las ocurrencias de `gin.H{"error": "...", "code": "..."}` por `ErrorResponse{...}`:

```go
// Antes:
c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID inválido o ausente", "code": "BAD_REQUEST"})
// Después:
invalidTenantResponse(c)

// Antes:
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "BAD_REQUEST"})
// Después:
c.JSON(http.StatusBadRequest, ErrorResponse{Error: "VALIDATION_ERROR", Message: err.Error(), Status: http.StatusBadRequest})

// Antes:
c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor", "code": "INTERNAL_ERROR"})
// Después:
HandleError(c, err)
```

- [ ] **Paso 4.3: Actualizar `get_notification.go`, `ack_notification.go`, `close_notification.go`, `count_notifications.go`**

Aplicar los mismos reemplazos que en el paso 4.2. En cada archivo:
- `gin.H{"error": "...", "code": "BAD_REQUEST"}` → `invalidTenantResponse(c)` o `invalidIDResponse(c)`
- `gin.H{"error": "...", "code": "NOT_FOUND"}` → `HandleError(c, err)`
- `gin.H{"error": "...", "code": "INTERNAL_ERROR"}` → `HandleError(c, err)`

- [ ] **Paso 4.4: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/notifications/..."
```

- [ ] **Paso 4.5: Pedir aprobación y commitear**

```bash
git add internal/api/handler/notifications/
git commit -m "refactor(notifications): estandarizar formato de error responses"
```

---

## Task 5: tenants — reemplazar httperr.WriteError con ErrorResponse tipado

**Contexto:** Los handlers de tenants usan `httperr.WriteError(c, apperrors.NewXxx(...))`. El formato resultante es `{"status": N, "error": "not_found", "message": "..."}` con código en minúsculas. El estándar usa códigos en SCREAMING_SNAKE_CASE (`TENANT_NOT_FOUND`). Además, algunos handlers usan `log.Printf`.

**Archivos:**
- Crear: `internal/api/handler/tenants/errors.go`
- Modificar: `internal/api/handler/tenants/create_tenant/create_tenant.go`
- Modificar: `internal/api/handler/tenants/get_tenant/get_tenant.go`
- Modificar: `internal/api/handler/tenants/get_all_tenants/get_all_tenants.go`
- Modificar: `internal/api/handler/tenants/update_tenant/update_tenant.go`
- Modificar: `internal/api/handler/tenants/delete_tenant/delete_tenant.go`

- [ ] **Paso 5.1: Crear `internal/api/handler/tenants/errors.go`**

Este archivo está en el paquete padre `tenants` pero los handlers están en sub-paquetes. Cada sub-paquete tiene su propio package name, por lo que el `ErrorResponse` debe definirse en cada uno, o bien en un shared file dentro del paquete de cada handler.

La opción más limpia dado el layout actual (cada operación es un paquete separado) es definir `ErrorResponse` localmente en cada handler. Para evitar repetición, crear un paquete compartido `internal/api/handler/tenants/dto/`:

```go
// internal/api/handler/tenants/dto/errors.go
package dto

import "net/http"

// ErrorResponse es el formato estándar de error HTTP para tenants.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// InvalidID retorna un ErrorResponse para UUID inválido.
func InvalidID() ErrorResponse {
	return ErrorResponse{
		Error:   "INVALID_ID",
		Message: "El ID proporcionado no es un UUID válido",
		Status:  http.StatusBadRequest,
	}
}

// NotFound retorna un ErrorResponse para tenant no encontrado.
func NotFound() ErrorResponse {
	return ErrorResponse{
		Error:   "TENANT_NOT_FOUND",
		Message: "Tenant no encontrado",
		Status:  http.StatusNotFound,
	}
}

// InternalError retorna un ErrorResponse genérico de servidor.
func InternalError() ErrorResponse {
	return ErrorResponse{
		Error:   "INTERNAL_ERROR",
		Message: "An internal error occurred",
		Status:  http.StatusInternalServerError,
	}
}
```

- [ ] **Paso 5.2: Actualizar `create_tenant/create_tenant.go`**

```go
package create_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/dto"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
)

type CreateTenantHandler struct {
	useCase ucCreateTenant.UseCase
}

func NewCreateTenantHandler(useCase ucCreateTenant.UseCase) *CreateTenantHandler {
	return &CreateTenantHandler{useCase: useCase}
}

func (h *CreateTenantHandler) CreateTenant(c *gin.Context) {
	tenant, err := models.Parse(c)
	if err != nil {
		return
	}

	if err := h.useCase.Create(c.Request.Context(), tenant); err != nil {
		c.JSON(http.StatusInternalServerError, dto.InternalError())
		return
	}

	response := models.FromDomain(tenant)
	c.JSON(http.StatusCreated, models.TenantResponseSingle{Tenant: *response})
}
```

- [ ] **Paso 5.3: Actualizar `get_tenant/get_tenant.go`**

```go
package get_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/dto"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
)

type GetTenantHandler struct {
	uc *get_tenant.UseCase
}

func NewGetTenantHandler(uc *get_tenant.UseCase) *GetTenantHandler {
	return &GetTenantHandler{uc: uc}
}

func (h *GetTenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.InvalidID())
		return
	}

	tenant, err := h.uc.Execute(c.Request.Context(), id)
	if err != nil {
		if err == get_tenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, dto.NotFound())
			return
		}
		c.JSON(http.StatusInternalServerError, dto.InternalError())
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenant))
}
```

- [ ] **Paso 5.4: Actualizar `delete_tenant/delete_tenant.go`**

```go
package delete_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/dto"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
)

type DeleteTenantHandler struct {
	useCase ucDeleteTenant.UseCase
}

func NewDeleteTenantHandler(useCase ucDeleteTenant.UseCase) *DeleteTenantHandler {
	return &DeleteTenantHandler{useCase: useCase}
}

func (h *DeleteTenantHandler) DeleteTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.InvalidID())
		return
	}

	if err := h.useCase.Delete(c.Request.Context(), id); err != nil {
		if err == ucDeleteTenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, dto.NotFound())
			return
		}
		c.JSON(http.StatusInternalServerError, dto.InternalError())
		return
	}

	c.Status(http.StatusNoContent)
}
```

- [ ] **Paso 5.5: Actualizar `get_all_tenants` y `update_tenant` aplicando el mismo patrón**

Para `get_all_tenants/get_all_tenants.go`: leer el archivo, reemplazar `httperr.WriteError` por `c.JSON(http.StatusInternalServerError, dto.InternalError())`.

Para `update_tenant/update_tenant.go`: leer el archivo, reemplazar `httperr.WriteError` por los ErrorResponse correspondientes usando el paquete `dto`.

- [ ] **Paso 5.6: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/tenants/..."
```

- [ ] **Paso 5.7: Pedir aprobación y commitear**

```bash
git add internal/api/handler/tenants/
git commit -m "refactor(tenants): reemplazar httperr.WriteError con ErrorResponse tipado"
```

---

## Task 6: users — corregir extracción de Tenant ID

**Contexto:** El handler de users usa `c.GetString("tenant_id")` en lugar de `platform.TenantID(c.Request.Context())`. El middleware `TenantFromHeader` escribe el tenant_id en el contexto Go (vía `platform.WithTenantID`), no en el contexto Gin. El uso de `c.GetString` acopla el handler a un detalle de implementación del middleware.

**Archivo:** `internal/api/handler/users/handler.go`

- [ ] **Paso 6.1: Actualizar las extracciones de tenant_id en `handler.go`**

Hay 7 métodos que hacen `tenantID := c.GetString("tenant_id")`. Reemplazar todos por:

```go
tenantID := platform.TenantID(c.Request.Context())
```

Agregar el import de `"github.com/tu-org/embolsadora-api/internal/platform"` si no está presente.

La verificación de presencia (actualmente implícita — el middleware garantiza que esté) sigue siendo implícita. El middleware `ExtractTenantID` ya maneja el caso de ausencia antes de llegar al handler.

- [ ] **Paso 6.2: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/users/..."
```

- [ ] **Paso 6.3: Pedir aprobación y commitear**

```bash
git add internal/api/handler/users/handler.go
git commit -m "fix(users): usar platform.TenantID en lugar de c.GetString para tenant_id"
```

---

## Task 7: permissions — corregir DeletePermission success response

**Contexto:** `DeletePermission` retorna `gin.H{"success": true}` en el happy path. El resto del handler ya es correcto (struct directo, sin wrapper). Un solo ajuste.

**Archivo:** `internal/api/handler/permissions/handler.go`

- [ ] **Paso 7.1: Cambiar la respuesta de éxito en `DeletePermission`**

```go
// Antes (línea ~283):
c.JSON(http.StatusOK, gin.H{"success": true})

// Después:
c.Status(http.StatusNoContent)
```

- [ ] **Paso 7.2: Verificar compilación**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./internal/api/handler/permissions/..."
```

- [ ] **Paso 7.3: Pedir aprobación y commitear**

```bash
git add internal/api/handler/permissions/handler.go
git commit -m "fix(permissions): retornar 204 No Content en DeletePermission"
```

---

## Task 8: Verificación final — compilación completa

- [ ] **Paso 8.1: Build completo del proyecto**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."
```
Salida esperada: sin output (exit 0).

- [ ] **Paso 8.2: Ejecutar tests existentes**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go test ./internal/api/... -v 2>&1 | tail -30"
```

- [ ] **Paso 8.3: Actualizar el audit doc**

En `docs/audit/api-standardization-audit.md`, marcar las categorías P1 como resueltas:
- ✅ 1. Manejo de errores HTTP
- ✅ 2. Formato de respuesta JSON
- ✅ 3. Extracción del Tenant ID

---

## Notas importantes antes de ejecutar

1. **DELETE responses:** Varios handlers cambian de `200 {"success": true}` a `204 No Content`. Verificar si los Pact contracts del frontend esperan 200 o 204 antes de cambiar. Si esperan 200, usar `c.JSON(http.StatusOK, struct{}{})` en lugar de `c.Status(http.StatusNoContent)`.

2. **`usersAffected` en roles:** El campo extra en `DELETE /roles/:id` cuando hay asignaciones activas se elimina. Si el frontend lo consume, mantenerlo en `HandleError` pasando el count.

3. **dashboard_layouts Pact:** El contrato Pact de dashboard_layouts espera `{"error": "LIMIT_REACHED"}` sin `message` ni `status`. Verificar `PACTS_ANALYSIS.md` antes de agregar esos campos al ErrorResponse de este módulo.

package permissions

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/app/permissions"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// Handler gestiona los endpoints de permisos.
type Handler struct {
	service *permissions.Service
	logger  *zap.Logger
}

// NewHandler crea un nuevo handler de permisos.
func NewHandler(service *permissions.Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// --- DTOs ---

// PermissionResponse es la representación JSON de un permiso.
type PermissionResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Section            string  `json:"section"`
	Description        string  `json:"description"`
	IsSystemPermission bool    `json:"isSystemPermission"`
	CreatedAt          *string `json:"createdAt,omitempty"`
	UpdatedAt          *string `json:"updatedAt,omitempty"`
}

// CreatePermissionRequest es el body para crear un permiso custom.
type CreatePermissionRequest struct {
	Name        string `json:"name"`
	Section     string `json:"section"`
	Description string `json:"description"`
}

// UpdatePermissionRequest es el body para actualizar un permiso custom.
type UpdatePermissionRequest struct {
	Name        string `json:"name"`
	Section     string `json:"section"`
	Description string `json:"description"`
}

// fieldError representa un error de validación de un campo específico.
type fieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// validationErrorResponse es la respuesta de error de validación (HTTP 400).
type validationErrorResponse struct {
	Error  string       `json:"error"`
	Errors []fieldError `json:"errors"`
}

// errorResponse es la respuesta de error genérica.
type errorResponse struct {
	Error string `json:"error"`
}

// deleteSuccessResponse is the Pact-required shape for DELETE 200.
type deleteSuccessResponse struct {
	Success bool `json:"success"`
}

// --- Conversión ---

func toPermissionResponse(p *domain.Permission) PermissionResponse {
	r := PermissionResponse{
		ID:                 p.ID,
		Name:               p.Name,
		Section:            p.Section,
		Description:        p.Description,
		IsSystemPermission: p.IsSystemPermission,
	}
	if !p.IsSystemPermission {
		createdAt := p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
		updatedAt := p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.CreatedAt = &createdAt
		r.UpdatedAt = &updatedAt
	}
	return r
}

// --- Error mapping ---

func (h *Handler) handlePermissionError(c *gin.Context, err error) {
	var ve *permissions.ValidationError
	if errors.As(err, &ve) {
		c.JSON(http.StatusBadRequest, validationErrorResponse{
			Error: "Validation failed",
			Errors: []fieldError{
				{Path: ve.Field, Message: ve.Message},
			},
		})
		return
	}
	if errors.Is(err, domain.ErrPermissionIsSystem) {
		msg := err.Error()
		// Mensajes específicos para el contrato Pact
		if msg == domain.ErrPermissionIsSystem.Error() {
			c.JSON(http.StatusForbidden, errorResponse{Error: "Cannot modify system permissions"})
		} else {
			c.JSON(http.StatusForbidden, errorResponse{Error: err.Error()})
		}
		return
	}
	if errors.Is(err, domain.ErrPermissionNotFound) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "Permission not found"})
		return
	}
	h.logger.Error("error inesperado en handler de permisos", zap.Error(err))
	c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
}

// --- Handlers ---

// ListPermissions godoc
// GET /api/v1/permissions
// Retorna todos los permisos disponibles para el tenant (sistema + custom).
func (h *Handler) ListPermissions(c *gin.Context) {
	start := time.Now()
	tenantID, ok := getTenantID(c)
	if !ok {
		telemetry.PermissionsRequestsTotal.WithLabelValues("list", "400").Inc()
		return
	}

	perms, err := h.service.ListPermissions(c.Request.Context(), tenantID)
	if err != nil {
		telemetry.PermissionsRequestsTotal.WithLabelValues("list", "500").Inc()
		h.handlePermissionError(c, err)
		return
	}

	telemetry.PermissionsListDuration.Observe(time.Since(start).Seconds())
	telemetry.PermissionsRequestsTotal.WithLabelValues("list", "200").Inc()

	response := make([]PermissionResponse, len(perms))
	for i, p := range perms {
		response[i] = toPermissionResponse(p)
	}
	c.JSON(http.StatusOK, response)
}

// GetPermission godoc
// GET /api/v1/permissions/:id
// Retorna un permiso específico por ID (de sistema o custom del tenant).
func (h *Handler) GetPermission(c *gin.Context) {
	id := c.Param("id")
	tenantID, ok := getTenantID(c)
	if !ok {
		telemetry.PermissionsRequestsTotal.WithLabelValues("get", "400").Inc()
		return
	}

	p, err := h.service.GetPermission(c.Request.Context(), id, tenantID)
	if err != nil {
		status := "500"
		if errors.Is(err, domain.ErrPermissionNotFound) {
			status = "404"
		}
		telemetry.PermissionsRequestsTotal.WithLabelValues("get", status).Inc()
		h.handlePermissionError(c, err)
		return
	}

	telemetry.PermissionsRequestsTotal.WithLabelValues("get", "200").Inc()
	c.JSON(http.StatusOK, toPermissionResponse(p))
}

// CreatePermission godoc
// POST /api/v1/permissions
// Crea un nuevo permiso custom para el tenant. Solo admin.
func (h *Handler) CreatePermission(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		telemetry.PermissionsRequestsTotal.WithLabelValues("create", "400").Inc()
		return
	}

	var req CreatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		telemetry.PermissionsRequestsTotal.WithLabelValues("create", "400").Inc()
		c.JSON(http.StatusBadRequest, validationErrorResponse{
			Error:  "Validation failed",
			Errors: []fieldError{{Path: "body", Message: "invalid JSON"}},
		})
		return
	}

	p, err := h.service.CreatePermission(c.Request.Context(), tenantID, req.Name, req.Section, req.Description)
	if err != nil {
		status := "500"
		var ve *permissions.ValidationError
		if errors.As(err, &ve) {
			status = "400"
		}
		telemetry.PermissionsRequestsTotal.WithLabelValues("create", status).Inc()
		h.handlePermissionError(c, err)
		return
	}

	telemetry.PermissionsRequestsTotal.WithLabelValues("create", "201").Inc()
	c.JSON(http.StatusCreated, toPermissionResponse(p))
}

// UpdatePermission godoc
// PUT /api/v1/permissions/:id
// Actualiza un permiso custom. Solo admin. Los permisos de sistema retornan 403.
func (h *Handler) UpdatePermission(c *gin.Context) {
	id := c.Param("id")
	tenantID, ok := getTenantID(c)
	if !ok {
		telemetry.PermissionsRequestsTotal.WithLabelValues("update", "400").Inc()
		return
	}

	var req UpdatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		telemetry.PermissionsRequestsTotal.WithLabelValues("update", "400").Inc()
		c.JSON(http.StatusBadRequest, validationErrorResponse{
			Error:  "Validation failed",
			Errors: []fieldError{{Path: "body", Message: "invalid JSON"}},
		})
		return
	}

	p, err := h.service.UpdatePermission(c.Request.Context(), id, tenantID, req.Name, req.Section, req.Description)
	if err != nil {
		status := "500"
		switch {
		case errors.Is(err, domain.ErrPermissionNotFound):
			status = "404"
		case errors.Is(err, domain.ErrPermissionIsSystem):
			status = "403"
		}
		telemetry.PermissionsRequestsTotal.WithLabelValues("update", status).Inc()
		h.handlePermissionError(c, err)
		return
	}

	telemetry.PermissionsRequestsTotal.WithLabelValues("update", "200").Inc()
	c.JSON(http.StatusOK, toPermissionResponse(p))
}

// DeletePermission godoc
// DELETE /api/v1/permissions/:id
// Elimina permanentemente un permiso custom. Solo admin. Los permisos de sistema retornan 403.
func (h *Handler) DeletePermission(c *gin.Context) {
	id := c.Param("id")
	tenantID, ok := getTenantID(c)
	if !ok {
		telemetry.PermissionsRequestsTotal.WithLabelValues("delete", "400").Inc()
		return
	}

	if err := h.service.DeletePermission(c.Request.Context(), id, tenantID); err != nil {
		status := "500"
		switch {
		case errors.Is(err, domain.ErrPermissionNotFound):
			status = "404"
		case errors.Is(err, domain.ErrPermissionIsSystem):
			status = "403"
		}
		telemetry.PermissionsRequestsTotal.WithLabelValues("delete", status).Inc()
		// Para delete de permiso de sistema, el mensaje Pact es específico
		if errors.Is(err, domain.ErrPermissionIsSystem) {
			c.JSON(http.StatusForbidden, errorResponse{Error: "Cannot delete system permissions"})
			return
		}
		h.handlePermissionError(c, err)
		return
	}

	telemetry.PermissionsRequestsTotal.WithLabelValues("delete", "200").Inc()
	c.JSON(http.StatusOK, deleteSuccessResponse{Success: true})
}

// --- Helpers ---

func getTenantID(c *gin.Context) (uuid.UUID, bool) {
	tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "tenant ID required"})
		return uuid.Nil, false
	}
	return tenantID, true
}

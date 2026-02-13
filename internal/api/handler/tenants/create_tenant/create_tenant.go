package create_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant/models"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

type CreateTenantHandler struct{}

func NewCreateTenantHandler() *CreateTenantHandler {
	return &CreateTenantHandler{}
}

func (h *CreateTenantHandler) CreateTenant(c *gin.Context) {
	var req models.TenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	log.Printf("not implemented: CreateTenant with data: %+v", req)
	// TODO: Implementar lógica de negocio para crear tenant
	response := models.TenantResponse{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Domain:      req.Domain,
		Active:      req.Active,
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}
	c.JSON(http.StatusCreated, models.TenantResponseSingle{Tenant: response})
}

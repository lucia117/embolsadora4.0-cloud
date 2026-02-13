package update_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant/models"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

type UpdateTenantHandler struct{}

func NewUpdateTenantHandler() *UpdateTenantHandler {
	return &UpdateTenantHandler{}
}

func (h *UpdateTenantHandler) UpdateTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tenant inválido"))
		return
	}

	var req models.TenantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	log.Printf("not implemented: UpdateTenant with ID: %s, data: %+v", id.String(), req)
	// TODO: Implementar lógica de negocio para actualizar tenant
	response := models.TenantResponse{
		ID:          id.String(),
		Name:        "Tenant Demo Updated",
		Description: "Tenant de ejemplo actualizado",
		Domain:      "demo.example.com",
		Active:      true,
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}
	c.JSON(http.StatusOK, models.TenantResponseSingle{Tenant: response})
}

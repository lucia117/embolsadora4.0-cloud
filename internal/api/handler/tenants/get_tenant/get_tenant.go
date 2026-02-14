package get_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

type GetTenantHandler struct {
	service *get_tenant.Service
}

func NewGetTenantHandler(repo tenants.TenantRepository) *GetTenantHandler {
	return &GetTenantHandler{
		service: get_tenant.NewService(repo),
	}
}

func (h *GetTenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tenant inválido"))
		return
	}

	tenant, err := h.service.Execute(c.Request.Context(), id)
	if err != nil {
		if err == get_tenant.ErrTenantNotFound {
			httperr.WriteError(c, apperrors.NewNotFound("Tenant no encontrado"))
			return
		}
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al obtener tenant"))
		return
	}

	response := models.TenantResponse{
		ID:          tenant.ID.String(),
		Name:        tenant.Name,
		Description: tenant.Description,
		Domain:      tenant.Domain,
		Active:      tenant.Active,
		CreatedAt:   tenant.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   tenant.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	c.JSON(http.StatusOK, models.TenantResponseSingle{Tenant: response})
}

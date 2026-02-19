package create_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant/models"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

type CreateTenantHandler struct {
	useCase ucCreateTenant.UseCase
}

func NewCreateTenantHandler(useCase ucCreateTenant.UseCase) *CreateTenantHandler {
	return &CreateTenantHandler{
		useCase: useCase,
	}
}

func (h *CreateTenantHandler) CreateTenant(c *gin.Context) {
	tenant, err := models.Parse(c)
	if err != nil {
		return
	}

	err = h.useCase.Create(c.Request.Context(), tenant)
	if err != nil {
		httperr.WriteError(c, apperrors.NewInternalServerError("Failed to create tenant. "+err.Error()))
		return
	}

	response := models.FromDomain(tenant)
	c.JSON(http.StatusCreated, models.TenantResponseSingle{Tenant: *response})
}

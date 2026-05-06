package create_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant/models"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
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
		log.Printf("error creating tenant: %v", err)
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Failed to create tenant", Status: http.StatusInternalServerError})
		return
	}

	response := models.FromDomain(tenant)
	c.JSON(http.StatusCreated, models.TenantResponseSingle{Tenant: *response})
}

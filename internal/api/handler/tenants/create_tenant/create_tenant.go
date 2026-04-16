package create_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/create_tenant/models"
	ucCreateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/create_tenant"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

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
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "INTERNAL_ERROR", Message: "Failed to create tenant", Status: http.StatusInternalServerError})
		return
	}

	response := models.FromDomain(tenant)
	c.JSON(http.StatusCreated, models.TenantResponseSingle{Tenant: *response})
}

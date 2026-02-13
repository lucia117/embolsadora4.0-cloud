package get_tenants

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenants/models"
)

type GetTenantsHandler struct{}

func NewGetTenantsHandler() *GetTenantsHandler {
	return &GetTenantsHandler{}
}

func (h *GetTenantsHandler) GetTenants(c *gin.Context) {
	log.Println("not implemented: GetTenants")
	// TODO: Implementar lógica de negocio para obtener tenants
	response := []models.TenantResponse{
		{
			ID:          uuid.New().String(),
			Name:        "Tenant Demo",
			Description: "Tenant de ejemplo",
			Domain:      "demo.example.com",
			Active:      true,
			CreatedAt:   "2024-01-01T00:00:00Z",
			UpdatedAt:   "2024-01-01T00:00:00Z",
		},
	}
	c.JSON(http.StatusOK, models.TenantsResponse{Tenants: response})
}

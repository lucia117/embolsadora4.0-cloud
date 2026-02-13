package get_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant/models"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

type GetTenantHandler struct{}

func NewGetTenantHandler() *GetTenantHandler {
	return &GetTenantHandler{}
}

func (h *GetTenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tenant inválido"))
		return
	}

	log.Printf("not implemented: GetTenant with ID: %s", id.String())
	// TODO: Implementar lógica de negocio para obtener tenant por ID
	response := models.TenantResponse{
		ID:          id.String(),
		Name:        "Tenant Demo",
		Description: "Tenant de ejemplo",
		Domain:      "demo.example.com",
		Active:      true,
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}
	c.JSON(http.StatusOK, models.TenantResponseSingle{Tenant: response})
}

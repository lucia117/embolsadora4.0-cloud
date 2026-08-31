package get_public_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_public_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_public_tenant"
)

type GetPublicTenantHandler struct {
	uc *get_public_tenant.UseCase
}

func NewGetPublicTenantHandler(uc *get_public_tenant.UseCase) *GetPublicTenantHandler {
	return &GetPublicTenantHandler{uc: uc}
}

// GetPublicTenant serves GET /api/v1/public/tenants/:idOrSubdomain — no
// session, no X-Tenant-ID. Backs the invitation/password-reset callback link
// (which runs before any session exists) and the public tenant landing page.
func (h *GetPublicTenantHandler) GetPublicTenant(c *gin.Context) {
	idOrSubdomain := c.Param("idOrSubdomain")

	tenant, err := h.uc.Execute(c.Request.Context(), idOrSubdomain)
	if err != nil {
		if err == get_public_tenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, tenantserrors.ErrorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Error al obtener tenant", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenant))
}

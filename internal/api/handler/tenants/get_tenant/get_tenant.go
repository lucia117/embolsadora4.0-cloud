package get_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type GetTenantHandler struct {
	uc *get_tenant.UseCase
}

func NewGetTenantHandler(uc *get_tenant.UseCase) *GetTenantHandler {
	return &GetTenantHandler{
		uc: uc,
	}
}

func (h *GetTenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	tenant, err := h.uc.Execute(c.Request.Context(), id)
	if err != nil {
		if err == get_tenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, errorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "INTERNAL_ERROR", Message: "Error al obtener tenant", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenant))
}

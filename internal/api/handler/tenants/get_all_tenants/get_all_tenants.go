package get_all_tenants

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// GetAllTenantsHandler maneja las solicitudes para obtener todos los tenants
type GetAllTenantsHandler struct {
	uc *get_all_tenants.UseCase
}

// NewGetAllTenantsHandler crea una nueva instancia del handler
func NewGetAllTenantsHandler(uc *get_all_tenants.UseCase) *GetAllTenantsHandler {
	return &GetAllTenantsHandler{
		uc: uc,
	}
}

// GetAllTenants obtiene todos los tenants
func (h *GetAllTenantsHandler) GetAllTenants(c *gin.Context) {
	tenants, err := h.uc.Execute(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "INTERNAL_ERROR", Message: "Error al obtener tenants", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenants))
}

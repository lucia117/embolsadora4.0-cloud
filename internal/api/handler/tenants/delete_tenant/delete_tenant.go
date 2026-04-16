package delete_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type DeleteTenantHandler struct {
	useCase ucDeleteTenant.UseCase
}

func NewDeleteTenantHandler(useCase ucDeleteTenant.UseCase) *DeleteTenantHandler {
	return &DeleteTenantHandler{
		useCase: useCase,
	}
}

func (h *DeleteTenantHandler) DeleteTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	err = h.useCase.Delete(c.Request.Context(), id)
	if err != nil {
		if err == ucDeleteTenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, errorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		log.Printf("error deleting tenant: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "INTERNAL_ERROR", Message: "Failed to delete tenant", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tenant deleted successfully",
	})
}

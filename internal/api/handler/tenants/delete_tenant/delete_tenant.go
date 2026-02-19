package delete_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

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
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tenant inválido"))
		return
	}

	err = h.useCase.Delete(c.Request.Context(), id)
	if err != nil {
		log.Printf("error deleting tenant: %v", err)
		httperr.WriteError(c, apperrors.NewInternalServerError("Failed to delete tenant"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tenant deleted successfully",
	})
}

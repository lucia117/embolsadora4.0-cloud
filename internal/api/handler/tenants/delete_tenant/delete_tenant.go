package delete_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

type DeleteTenantHandler struct{}

func NewDeleteTenantHandler() *DeleteTenantHandler {
	return &DeleteTenantHandler{}
}

func (h *DeleteTenantHandler) DeleteTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tenant inválido"))
		return
	}

	log.Printf("not implemented: DeleteTenant with ID: %s", id.String())
	// TODO: Implementar lógica de negocio para eliminar tenant
	c.Status(http.StatusNoContent)
}

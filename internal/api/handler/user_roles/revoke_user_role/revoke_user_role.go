package revoke_user_role

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/revoke_user_role/models"
	ucRevokeUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/revoke_user_role"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Handler handles DELETE /api/v1/user-roles/:id requests.
type Handler struct {
	useCase ucRevokeUserRole.UseCase
}

// NewRevokeUserRoleHandler creates a new Handler.
func NewRevokeUserRoleHandler(useCase ucRevokeUserRole.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle processes the revoke-role request.
func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id: must be a UUID"})
		return
	}

	result, err := h.useCase.Execute(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAssignmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": models.FromDomain(result)})
}

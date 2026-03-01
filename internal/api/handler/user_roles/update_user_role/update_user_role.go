package update_user_role

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/update_user_role/models"
	ucUpdateUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/update_user_role"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Handler handles PUT /api/v1/user-roles/:id requests.
type Handler struct {
	useCase ucUpdateUserRole.UseCase
}

// NewUpdateUserRoleHandler creates a new Handler.
func NewUpdateUserRoleHandler(useCase ucUpdateUserRole.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle processes the update-role request.
func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id: must be a UUID"})
		return
	}

	req, err := models.Parse(c)
	if err != nil {
		return
	}

	result, err := h.useCase.Execute(c.Request.Context(), id, req.RoleID)
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

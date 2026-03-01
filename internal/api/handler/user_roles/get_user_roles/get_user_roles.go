package get_user_roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/get_user_roles/models"
	ucGetUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/get_user_roles"
)

// Handler handles GET /api/v1/users/:id/roles requests.
type Handler struct {
	useCase ucGetUserRoles.UseCase
}

// NewGetUserRolesHandler creates a new Handler.
func NewGetUserRolesHandler(useCase ucGetUserRoles.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle retrieves all role assignments for a user across all tenants.
func (h *Handler) Handle(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id: must be a UUID"})
		return
	}

	results, err := h.useCase.Execute(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": models.FromDomain(results)})
}

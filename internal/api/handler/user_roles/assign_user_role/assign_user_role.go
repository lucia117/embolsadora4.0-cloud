package assign_user_role

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ucAssignUserRole "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/assign_user_role"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/assign_user_role/models"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Handler handles POST /api/v1/user-roles requests.
type Handler struct {
	useCase ucAssignUserRole.UseCase
}

// NewAssignUserRoleHandler creates a new Handler.
func NewAssignUserRoleHandler(useCase ucAssignUserRole.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle processes the assign-role request.
func (h *Handler) Handle(c *gin.Context) {
	req, err := models.Parse(c)
	if err != nil {
		return
	}

	var assignedBy *uuid.UUID
	if v, exists := c.Get("userID"); exists {
		if id, ok := v.(uuid.UUID); ok {
			assignedBy = &id
		}
	}

	result, err := h.useCase.Execute(c.Request.Context(), ucAssignUserRole.AssignRequest{
		UserID:     req.UserID,
		TenantID:   req.TenantID,
		RoleID:     req.RoleID,
		AssignedBy: assignedBy,
	})
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyHasActiveRole) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": models.FromDomain(result)})
}

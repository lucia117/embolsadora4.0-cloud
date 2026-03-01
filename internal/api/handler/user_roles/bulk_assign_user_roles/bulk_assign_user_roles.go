package bulk_assign_user_roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/bulk_assign_user_roles/models"
	ucBulkAssignUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/bulk_assign_user_roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Handler handles POST /api/v1/user-roles/bulk requests.
type Handler struct {
	useCase ucBulkAssignUserRoles.UseCase
}

// NewBulkAssignUserRolesHandler creates a new Handler.
func NewBulkAssignUserRolesHandler(useCase ucBulkAssignUserRoles.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle processes the bulk-assign request.
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

	result, err := h.useCase.Execute(c.Request.Context(), ucBulkAssignUserRoles.BulkAssignRequest{
		UserIDs:    req.UserIDs,
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

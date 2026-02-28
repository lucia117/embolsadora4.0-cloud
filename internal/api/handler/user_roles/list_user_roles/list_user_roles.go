package list_user_roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/list_user_roles/models"
	ucListUserRoles "github.com/tu-org/embolsadora-api/internal/api/usecases/user_roles/list_user_roles"
)

// Handler handles GET /api/v1/user-roles requests.
type Handler struct {
	useCase ucListUserRoles.UseCase
}

// NewListUserRolesHandler creates a new Handler.
func NewListUserRolesHandler(useCase ucListUserRoles.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Handle lists UTR assignments for a tenant, with optional status filter.
func (h *Handler) Handle(c *gin.Context) {
	tenantIDStr := c.Query("tenantId")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenantId query parameter is required"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid tenantId: must be a UUID"})
		return
	}

	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}

	results, err := h.useCase.Execute(c.Request.Context(), tenantID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": models.FromDomain(results)})
}

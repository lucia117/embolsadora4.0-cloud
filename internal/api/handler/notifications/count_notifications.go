package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appNotifications "github.com/tu-org/embolsadora-api/internal/app/notifications"
	"github.com/tu-org/embolsadora-api/internal/api/handler/notifications/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CountNotifications godoc
// GET /api/v1/notifications/count
func CountNotifications(service *appNotifications.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID inválido o ausente", "code": "BAD_REQUEST"})
			return
		}

		count, err := service.CountUnread(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor", "code": "INTERNAL_ERROR"})
			return
		}

		c.JSON(http.StatusOK, dto.NotificationCountResponse{Unread: count})
	}
}

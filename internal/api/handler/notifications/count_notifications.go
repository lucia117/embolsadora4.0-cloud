package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appNotifications "github.com/tu-org/embolsadora-api/internal/app/notifications"
	"github.com/tu-org/embolsadora-api/internal/api/handler/notifications/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func CountNotifications(service *appNotifications.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		count, err := service.CountUnread(c.Request.Context(), tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.NotificationCountResponse{Unread: count})
	}
}

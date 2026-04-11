package notifications

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appNotifications "github.com/tu-org/embolsadora-api/internal/app/notifications"
	"github.com/tu-org/embolsadora-api/internal/api/handler/notifications/dto"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CloseNotification godoc
// POST /api/v1/notifications/:id/close
func CloseNotification(service *appNotifications.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID inválido o ausente", "code": "BAD_REQUEST"})
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID de notificación inválido", "code": "BAD_REQUEST"})
			return
		}

		n, err := service.Close(c.Request.Context(), id, tenantID)
		if err != nil {
			if errors.Is(err, domain.ErrNotificationNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "notificación no encontrada", "code": "NOT_FOUND"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor", "code": "INTERNAL_ERROR"})
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(n))
	}
}

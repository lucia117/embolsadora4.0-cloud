package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appNotifications "github.com/tu-org/embolsadora-api/internal/app/notifications"
	"github.com/tu-org/embolsadora-api/internal/api/handler/notifications/dto"
	notifRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListNotifications godoc
// GET /api/v1/notifications
func ListNotifications(service *appNotifications.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID inválido o ausente", "code": "BAD_REQUEST"})
			return
		}

		var params dto.ListNotificationsParams
		if err := c.ShouldBindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "BAD_REQUEST"})
			return
		}

		// Defaults
		if params.Limit <= 0 {
			params.Limit = 20
		}
		if params.Limit > 100 {
			params.Limit = 100
		}

		repoParams := notifRepo.ListParams{
			Limit:  params.Limit,
			Offset: params.Offset,
		}
		if params.Status != "" {
			repoParams.Status = &params.Status
		}
		if params.Severity != "" {
			repoParams.Severity = &params.Severity
		}

		items, total, err := service.List(c.Request.Context(), tenantID, repoParams)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno del servidor", "code": "INTERNAL_ERROR"})
			return
		}

		responses := make([]dto.NotificationResponse, len(items))
		for i, n := range items {
			responses[i] = dto.FromDomain(n)
		}

		c.JSON(http.StatusOK, dto.NotificationListResponse{
			Data:   responses,
			Total:  total,
			Limit:  params.Limit,
			Offset: params.Offset,
		})
	}
}

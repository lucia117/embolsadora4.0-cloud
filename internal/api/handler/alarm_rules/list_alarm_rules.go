package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListAlarmRules godoc
// GET /api/v1/alarm-rules
func ListAlarmRules(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID inválido o ausente"})
			return
		}

		rules, err := service.ListAlarmRules(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno del servidor"})
			return
		}

		items := make([]dto.AlarmRuleResponse, len(rules))
		for i, r := range rules {
			items[i] = dto.FromDomain(r)
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
	}
}

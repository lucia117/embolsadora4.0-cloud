package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListAlarmRules godoc
// GET /api/v1/alarm-rules
func ListAlarmRules(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		rules, err := service.ListAlarmRules(c.Request.Context(), tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		items := make([]dto.AlarmRuleResponse, len(rules))
		for i, r := range rules {
			items[i] = dto.FromDomain(r)
		}

		c.JSON(http.StatusOK, items)
	}
}

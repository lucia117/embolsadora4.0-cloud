package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// GetAlarmRule godoc
// GET /api/v1/alarm-rules/:id
func GetAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			invalidIDResponse(c)
			return
		}

		rule, err := service.GetAlarmRule(c.Request.Context(), id, tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(rule))
	}
}

package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// DeleteAlarmRule godoc
// DELETE /api/v1/alarm-rules/:id
func DeleteAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
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

		if err := service.DeleteAlarmRule(c.Request.Context(), id, tenantID); err != nil {
			HandleError(c, err)
			return
		}

		// Pact espera 200 en DELETE exitoso
		c.Status(http.StatusOK)
	}
}

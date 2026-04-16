package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// UpdateAlarmRule godoc
// PATCH /api/v1/alarm-rules/:id
func UpdateAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
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

		var req dto.UpdateAlarmRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: "Cuerpo de la petición inválido",
				Status:  http.StatusBadRequest,
			})
			return
		}

		input := appAlarmRules.UpdateAlarmRuleInput{
			Name:        req.Name,
			Description: req.Description,
			Metric:      req.Metric,
			Operator:    req.Operator,
			Threshold:   req.Threshold,
			Severity:    req.Severity,
			Enabled:     req.Enabled,
		}

		rule, err := service.UpdateAlarmRule(c.Request.Context(), id, tenantID, input)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(rule))
	}
}

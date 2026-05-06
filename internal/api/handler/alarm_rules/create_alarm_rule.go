package alarm_rules

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateAlarmRule godoc
// POST /api/v1/alarm-rules
func CreateAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		var req dto.CreateAlarmRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: "Cuerpo de la petición inválido",
				Status:  http.StatusBadRequest,
			})
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		input := appAlarmRules.CreateAlarmRuleInput{
			Name:        req.Name,
			Description: req.Description,
			Metric:      req.Metric,
			Operator:    req.Operator,
			Threshold:   req.Threshold,
			Severity:    req.Severity,
			Enabled:     enabled,
		}

		rule, err := service.CreateAlarmRule(c.Request.Context(), tenantID, input)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusCreated, dto.FromDomain(rule))
	}
}

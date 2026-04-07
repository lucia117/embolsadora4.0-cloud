package alarm_rules

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateAlarmRule godoc
// POST /api/v1/alarm-rules
func CreateAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID inválido o ausente"})
			return
		}

		var req dto.CreateAlarmRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "VALIDATION_ERROR",
				"message": "cuerpo de la petición inválido",
				"status":  http.StatusBadRequest,
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
			if isValidationError(err) {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "VALIDATION_ERROR",
					"message": err.Error(),
					"status":  http.StatusBadRequest,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno del servidor"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": dto.FromDomain(rule)})
	}
}

// isValidationError determina si el error es de validación de dominio.
func isValidationError(err error) bool {
	return errors.Is(err, appAlarmRules.ErrInvalidOperator) ||
		errors.Is(err, appAlarmRules.ErrInvalidSeverity) ||
		errors.Is(err, appAlarmRules.ErrNameRequired) ||
		errors.Is(err, appAlarmRules.ErrMetricRequired)
}

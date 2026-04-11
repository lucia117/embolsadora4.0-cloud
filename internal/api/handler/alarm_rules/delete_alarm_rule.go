package alarm_rules

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// DeleteAlarmRule godoc
// DELETE /api/v1/alarm-rules/:id
func DeleteAlarmRule(service *appAlarmRules.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID inválido o ausente"})
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "INVALID_ID",
				"message": "el ID proporcionado no es un UUID válido",
				"status":  http.StatusBadRequest,
			})
			return
		}

		if err := service.DeleteAlarmRule(c.Request.Context(), id, tenantID); err != nil {
			if errors.Is(err, domain.ErrAlarmRuleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "NOT_FOUND",
					"message": "regla de alarma no encontrada",
					"status":  http.StatusNotFound,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno del servidor"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

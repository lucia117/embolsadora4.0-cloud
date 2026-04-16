package alarm_rules

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appAlarmRules "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// ErrorResponse es el formato estándar de error HTTP para alarm rules.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// HandleError mapea errores de dominio/aplicación a respuestas HTTP.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrAlarmRuleNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "ALARM_RULE_NOT_FOUND",
			Message: "Regla de alarma no encontrada",
			Status:  http.StatusNotFound,
		})
	case errors.Is(err, appAlarmRules.ErrNameRequired),
		errors.Is(err, appAlarmRules.ErrMetricRequired),
		errors.Is(err, appAlarmRules.ErrInvalidOperator),
		errors.Is(err, appAlarmRules.ErrInvalidSeverity):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
			Status:  http.StatusBadRequest,
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An internal error occurred",
			Status:  http.StatusInternalServerError,
		})
	}
}

// invalidTenantResponse responde con error de tenant inválido.
func invalidTenantResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_TENANT",
		Message: "X-Tenant-ID inválido o ausente",
		Status:  http.StatusBadRequest,
	})
}

// invalidIDResponse responde con error de UUID inválido.
func invalidIDResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_ID",
		Message: "El ID proporcionado no es un UUID válido",
		Status:  http.StatusBadRequest,
	})
}

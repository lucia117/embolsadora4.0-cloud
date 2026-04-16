package notifications

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrNotificationNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOTIFICATION_NOT_FOUND",
			Message: "Notificación no encontrada",
			Status:  http.StatusNotFound,
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An internal error occurred",
			Status:  http.StatusInternalServerError,
		})
	}
}

func invalidTenantResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_TENANT",
		Message: "X-Tenant-ID inválido o ausente",
		Status:  http.StatusBadRequest,
	})
}

func invalidIDResponse(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "INVALID_ID",
		Message: "El ID proporcionado no es un UUID válido",
		Status:  http.StatusBadRequest,
	})
}

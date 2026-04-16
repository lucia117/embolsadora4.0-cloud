package logs

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
	switch {
	case errors.Is(err, domain.ErrLogNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "NOT_FOUND", Message: "log entry not found", Status: http.StatusNotFound})
	case errors.Is(err, domain.ErrInvalidCursor):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: "invalid cursor", Status: http.StatusBadRequest})
	case errors.Is(err, domain.ErrInvalidRetentionDays):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "INTERNAL_ERROR", Message: "internal server error", Status: http.StatusInternalServerError})
	}
}

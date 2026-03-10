package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// HandleError maps domain/application errors to HTTP responses
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	// Check for domain errors
	switch {
	case errors.Is(err, domainUsers.ErrNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "USER_NOT_FOUND",
			Message: "User not found",
			Status:  http.StatusNotFound,
		})

	case errors.Is(err, domainUsers.ErrEmailTaken):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "DUPLICATE_EMAIL",
			Message: "Email already exists in this tenant",
			Status:  http.StatusConflict,
		})

	case errors.Is(err, domainUsers.ErrImmutableField):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "IMMUTABLE_FIELD",
			Message: "Field cannot be modified",
			Status:  http.StatusBadRequest,
		})

	case errors.Is(err, domainUsers.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
			Status:  http.StatusBadRequest,
		})

	case errors.Is(err, domainUsers.ErrTenantMismatch):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "ACCESS_DENIED",
			Message: "User does not belong to this tenant",
			Status:  http.StatusForbidden,
		})

	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An internal error occurred",
			Status:  http.StatusInternalServerError,
		})
	}
}

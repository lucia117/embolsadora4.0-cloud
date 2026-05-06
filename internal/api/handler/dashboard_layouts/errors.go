package dashboard_layouts

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
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
	case errors.Is(err, domain.ErrLayoutNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "LAYOUT_NOT_FOUND",
			Message: "Layout no encontrado",
			Status:  http.StatusNotFound,
		})
	case errors.Is(err, domain.ErrLimitReached):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "LIMIT_REACHED",
			Message: err.Error(),
			Status:  http.StatusForbidden,
		})
	case errors.Is(err, domain.ErrDuplicateName):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "DUPLICATE_NAME",
			Message: err.Error(),
			Status:  http.StatusConflict,
		})
	case errors.Is(err, domain.ErrCannotDeleteLastLayout):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "CANNOT_DELETE_LAST_LAYOUT",
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

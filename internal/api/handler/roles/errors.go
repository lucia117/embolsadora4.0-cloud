package roles

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

type roleHasAssignmentsResponse struct {
	Error         string `json:"error"`
	Message       string `json:"message"`
	Status        int    `json:"status"`
	UsersAffected int    `json:"usersAffected"`
}

func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrRoleNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "ROLE_NOT_FOUND",
			Message: "Rol no encontrado",
			Status:  http.StatusNotFound,
		})
	case errors.Is(err, domain.ErrRoleLimitReached):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "LIMIT_REACHED",
			Message: err.Error(),
			Status:  http.StatusForbidden,
		})
	case errors.Is(err, domain.ErrRoleDuplicateName):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "DUPLICATE_NAME",
			Message: err.Error(),
			Status:  http.StatusConflict,
		})
	case errors.Is(err, domain.ErrRoleIsSystemRole):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "SYSTEM_ROLE",
			Message: err.Error(),
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

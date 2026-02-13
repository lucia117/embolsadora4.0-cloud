package delete_task

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

// DeleteTaskHandler maneja las solicitudes HTTP para eliminar tareas
type DeleteTaskHandler struct {
	service tasks.Service
}

// NewDeleteTaskHandler crea una nueva instancia de DeleteTaskHandler
func NewDeleteTaskHandler(service tasks.Service) *DeleteTaskHandler {
	return &DeleteTaskHandler{service: service}
}

// DeleteTask maneja la eliminación de una tarea
func (h *DeleteTaskHandler) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tarea inválido"))
		return
	}

	if err := h.service.DeleteTask(c.Request.Context(), id); err != nil {
		if err == tasks.ErrTaskNotFound {
			httperr.WriteError(c, apperrors.NewNotFound("Tarea no encontrada"))
			return
		}
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al eliminar la tarea"))
		return
	}

	c.Status(http.StatusNoContent)
}

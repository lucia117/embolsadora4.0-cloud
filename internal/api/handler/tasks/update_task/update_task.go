package update_task

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tasks/update_task/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

// UpdateTaskHandler maneja las solicitudes HTTP para actualizar tareas
type UpdateTaskHandler struct {
	service tasks.Service
}

// NewUpdateTaskHandler crea una nueva instancia de UpdateTaskHandler
func NewUpdateTaskHandler(service tasks.Service) *UpdateTaskHandler {
	return &UpdateTaskHandler{service: service}
}

// UpdateTask maneja la actualización de una tarea existente
func (h *UpdateTaskHandler) UpdateTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tarea inválido"))
		return
	}

	var req models.TaskUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	task, err := h.service.UpdateTask(
		c.Request.Context(),
		id,
		req.Title,
		req.Description,
		req.Completed,
	)

	if err != nil {
		if err == tasks.ErrTaskNotFound {
			httperr.WriteError(c, apperrors.NewNotFound("Tarea no encontrada"))
			return
		}
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al actualizar la tarea"))
		return
	}

	response := models.TaskResponse{
		ID:          task.ID.String(),
		Title:       task.Title,
		Description: task.Description,
		Completed:   task.Completed,
	}

	c.JSON(http.StatusOK, models.TaskResponseSingle{Task: response})
}

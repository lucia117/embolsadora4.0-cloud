package get_task

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tasks/get_task/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

// GetTaskHandler maneja las solicitudes HTTP para obtener una tarea
type GetTaskHandler struct {
	service tasks.Service
}

// NewGetTaskHandler crea una nueva instancia de GetTaskHandler
func NewGetTaskHandler(service tasks.Service) *GetTaskHandler {
	return &GetTaskHandler{service: service}
}

// GetTask maneja la obtención de una tarea por su ID
func (h *GetTaskHandler) GetTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tarea inválido"))
		return
	}

	task, err := h.service.GetTaskByID(c.Request.Context(), id)
	if err != nil {
		if err == tasks.ErrTaskNotFound {
			httperr.WriteError(c, apperrors.NewNotFound("Tarea no encontrada"))
			return
		}
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al obtener la tarea"))
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

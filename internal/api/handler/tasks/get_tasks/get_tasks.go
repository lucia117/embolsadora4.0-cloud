package get_tasks

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tasks/get_tasks/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

// GetTasksHandler maneja las solicitudes HTTP para obtener tareas
type GetTasksHandler struct {
	service tasks.Service
}

// NewGetTasksHandler crea una nueva instancia de GetTasksHandler
func NewGetTasksHandler(service tasks.Service) *GetTasksHandler {
	return &GetTasksHandler{service: service}
}

// GetTasks maneja la solicitud para listar todas las tareas
func (h *GetTasksHandler) GetTasks(c *gin.Context) {
	tasks, err := h.service.GetTasks(c.Request.Context())
	if err != nil {
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al obtener las tareas"))
		return
	}

	// Convertir tareas a TaskResponse
	response := make([]models.TaskResponse, len(tasks))
	for i, task := range tasks {
		response[i] = models.TaskResponse{
			ID:          task.ID.String(),
			Title:       task.Title,
			Description: task.Description,
			Completed:   task.Completed,
		}
	}

	c.JSON(http.StatusOK, models.Response{Tasks: response})
}

package crate_task

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tasks/crate_task/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
)

// CreateTaskHandler maneja las solicitudes HTTP para crear tareas
type CreateTaskHandler struct {
	service tasks.Service
}

// NewCreateTaskHandler crea una nueva instancia de CreateTaskHandler
func NewCreateTaskHandler(service tasks.Service) *CreateTaskHandler {
	return &CreateTaskHandler{service: service}
}

// CreateTask maneja la creación de una nueva tarea
func (h *CreateTaskHandler) CreateTask(c *gin.Context) {
	var req models.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	task, err := h.service.CreateTask(c.Request.Context(), req.Title, req.Description)
	if err != nil {
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al crear la tarea"))
		return
	}

	response := models.TaskResponse{
		ID:          task.ID.String(),
		Title:       task.Title,
		Description: task.Description,
		Completed:   task.Completed,
	}

	c.JSON(http.StatusCreated, models.TaskResponseSingle{Task: response})
}

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tasks/internal/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
)

// TaskHandler maneja las solicitudes HTTP para las tareas
type TaskHandler struct {
	service tasks.Service
}

// NewTaskHandler crea una nueva instancia de TaskHandler
func NewTaskHandler(service tasks.Service) *TaskHandler {
	return &TaskHandler{service: service}
}

// ListTasks maneja la solicitud para listar todas las tareas
func (h *TaskHandler) ListTasks(c *gin.Context) {
	tasks, err := h.service.GetTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las tareas"})
		return
	}

	response := make([]models.TaskResponse, len(tasks))
	for i, task := range tasks {
		response[i] = models.TaskResponse{
			ID:          task.ID.String(),
			Title:       task.Title,
			Description: task.Description,
			Completed:   task.Completed,
		}
	}

	c.JSON(http.StatusOK, models.TasksResponse{Tasks: response})
}

// CreateTask maneja la creación de una nueva tarea
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req models.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.CreateTask(c.Request.Context(), req.Title, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear la tarea"})
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

// GetTask maneja la obtención de una tarea por su ID
func (h *TaskHandler) GetTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de tarea inválido"})
		return
	}

	task, err := h.service.GetTaskByID(c.Request.Context(), id)
	if err != nil {
		if err == tasks.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tarea no encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la tarea"})
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

// UpdateTask maneja la actualización de una tarea existente
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de tarea inválido"})
		return
	}

	var req models.TaskUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Tarea no encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar la tarea"})
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

// DeleteTask maneja la eliminación de una tarea
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de tarea inválido"})
		return
	}

	if err := h.service.DeleteTask(c.Request.Context(), id); err != nil {
		if err == tasks.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tarea no encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar la tarea"})
		return
	}

	c.Status(http.StatusNoContent)
}

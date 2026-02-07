package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type stubTaskService struct {
	getTasksErr    error
	getTaskByIDErr error
}

func (s stubTaskService) CreateTask(ctx context.Context, title, description string) (*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (s stubTaskService) GetTasks(ctx context.Context) ([]domain.Task, error) {
	return nil, s.getTasksErr
}

func (s stubTaskService) GetTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	return nil, s.getTaskByIDErr
}

func (s stubTaskService) UpdateTask(ctx context.Context, id uuid.UUID, title, description *string, completed *bool) (*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (s stubTaskService) DeleteTask(ctx context.Context, id uuid.UUID) error {
	return errors.New("not implemented")
}

type errorPayload struct {
	Status  int    `json:"status"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

func TestTaskHandlers_ErrorPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("list tasks internal error", func(t *testing.T) {
		router := gin.New()
		handler := NewTaskHandler(stubTaskService{getTasksErr: errors.New("boom")})
		router.GET("/tasks", handler.ListTasks)

		resp := performRequest(router, http.MethodGet, "/tasks")

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("status mismatch: %d", resp.Code)
		}

		var payload errorPayload
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if payload.Status != http.StatusInternalServerError {
			t.Fatalf("payload status mismatch: %d", payload.Status)
		}
		if payload.Error != "internal_server_error" {
			t.Fatalf("payload error mismatch: %s", payload.Error)
		}
		if payload.Message != "Error al obtener las tareas" {
			t.Fatalf("payload message mismatch: %s", payload.Message)
		}
	})

	t.Run("get task not found", func(t *testing.T) {
		router := gin.New()
		handler := NewTaskHandler(stubTaskService{getTaskByIDErr: tasks.ErrTaskNotFound})
		router.GET("/tasks/:id", handler.GetTask)

		taskID := uuid.New().String()
		resp := performRequest(router, http.MethodGet, "/tasks/"+taskID)

		if resp.Code != http.StatusNotFound {
			t.Fatalf("status mismatch: %d", resp.Code)
		}

		var payload errorPayload
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if payload.Status != http.StatusNotFound {
			t.Fatalf("payload status mismatch: %d", payload.Status)
		}
		if payload.Error != "not_found" {
			t.Fatalf("payload error mismatch: %s", payload.Error)
		}
		if payload.Message != "Tarea no encontrada" {
			t.Fatalf("payload message mismatch: %s", payload.Message)
		}
	})
}

func performRequest(router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

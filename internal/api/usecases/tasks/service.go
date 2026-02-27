package tasks

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

// Service define la interfaz del servicio de tareas
type Service interface {
	CreateTask(ctx context.Context, title, description string) (*domain.Task, error)
	GetTasks(ctx context.Context) ([]domain.Task, error)
	GetTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error)
	UpdateTask(ctx context.Context, id uuid.UUID, title, description *string, completed *bool) (*domain.Task, error)
	DeleteTask(ctx context.Context, id uuid.UUID) error
}

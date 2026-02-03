package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Task representa una tarea en el dominio
type Task struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	Completed   bool      `json:"completed" db:"completed"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TaskRepository define la interfaz para el repositorio de tareas
type TaskRepository interface {
	Create(ctx context.Context, task *Task) error
	FindAll(ctx context.Context) ([]Task, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Task, error)
	Update(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id uuid.UUID) error
}

package tasks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// MockService es una implementación simple para pruebas
type MockService struct {
	db *pgxpool.Pool
}

// NewMockService crea una nueva instancia del servicio mock
func NewMockService(db *pgxpool.Pool) Service {
	return &MockService{db: db}
}

// CreateTask crea una nueva tarea
func (s *MockService) CreateTask(ctx context.Context, title, description string) (*domain.Task, error) {
	id := uuid.New()
	task := &domain.Task{
		ID:          id,
		Title:       title,
		Description: description,
		Completed:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Insertar en la base de datos
	query := `INSERT INTO tasks (id, title, description, completed, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.Exec(ctx, query, id, title, description, false, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// GetTasks obtiene todas las tareas
func (s *MockService) GetTasks(ctx context.Context) ([]domain.Task, error) {
	query := `SELECT id, title, description, completed, created_at, updated_at FROM tasks ORDER BY created_at DESC`
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		var task domain.Task

		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&task.Completed,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// GetTaskByID obtiene una tarea por ID
func (s *MockService) GetTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	query := `SELECT id, title, description, completed, created_at, updated_at FROM tasks WHERE id = $1`
	var task domain.Task

	err := s.db.QueryRow(ctx, query, id).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Completed,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	return &task, nil
}

// UpdateTask actualiza una tarea
func (s *MockService) UpdateTask(ctx context.Context, id uuid.UUID, title, description *string, completed *bool) (*domain.Task, error) {
	// Primero obtener la tarea actual
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Actualizar campos si se proporcionan
	if title != nil {
		task.Title = *title
	}
	if description != nil {
		task.Description = *description
	}
	if completed != nil {
		task.Completed = *completed
	}
	task.UpdatedAt = time.Now()

	// Actualizar en la base de datos
	query := `UPDATE tasks SET title = $1, description = $2, completed = $3, updated_at = $4 WHERE id = $5`
	_, err = s.db.Exec(ctx, query, task.Title, task.Description, task.Completed, task.UpdatedAt, id)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// DeleteTask elimina una tarea
func (s *MockService) DeleteTask(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tasks WHERE id = $1`
	result, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

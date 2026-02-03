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

type service struct {
	repo domain.TaskRepository
}

// NewService crea una nueva instancia del servicio de tareas
func NewService(repo domain.TaskRepository) Service {
	return &service{repo: repo}
}

func (s *service) CreateTask(ctx context.Context, title, description string) (*domain.Task, error) {
	task := &domain.Task{
		Title:       title,
		Description: description,
		Completed:   false,
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *service) GetTasks(ctx context.Context) ([]domain.Task, error) {
	tasks, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func (s *service) GetTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if task == nil {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

func (s *service) UpdateTask(ctx context.Context, id uuid.UUID, title, description *string, completed *bool) (*domain.Task, error) {
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if title != nil {
		task.Title = *title
	}

	if description != nil {
		task.Description = *description
	}

	if completed != nil {
		task.Completed = *completed
	}

	if err := s.repo.Update(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *service) DeleteTask(ctx context.Context, id uuid.UUID) error {
	if _, err := s.GetTaskByID(ctx, id); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id)
}

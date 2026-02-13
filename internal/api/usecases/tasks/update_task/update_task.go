package update_task

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks/get_task_by_id"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tasks"
)

type Service struct {
	repo       tasks.TaskRepository
	getService *get_task_by_id.Service
}

func NewService(repo tasks.TaskRepository) *Service {
	return &Service{
		repo:       repo,
		getService: get_task_by_id.NewService(repo),
	}
}

func (s *Service) Execute(ctx context.Context, id uuid.UUID, title, description *string, completed *bool) (*domain.Task, error) {
	task, err := s.getService.Execute(ctx, id)
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

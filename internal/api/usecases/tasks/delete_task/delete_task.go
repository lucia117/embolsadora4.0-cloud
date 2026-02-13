package delete_task

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tasks/get_task_by_id"
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

func (s *Service) Execute(ctx context.Context, id uuid.UUID) error {
	if _, err := s.getService.Execute(ctx, id); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id)
}

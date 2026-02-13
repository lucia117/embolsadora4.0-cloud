package get_task

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tasks"
)

type Service struct {
	repo tasks.TaskRepository
}

func NewService(repo tasks.TaskRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Execute(ctx context.Context) ([]domain.Task, error) {
	tasks, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

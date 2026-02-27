package create_task

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

func (s *Service) Execute(ctx context.Context, title, description string) (*domain.Task, error) {
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

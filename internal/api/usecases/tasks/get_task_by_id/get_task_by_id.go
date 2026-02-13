package get_task_by_id

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tasks"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

type Service struct {
	repo tasks.TaskRepository
}

func NewService(repo tasks.TaskRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Execute(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if task == nil {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

package get_tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var (
	ErrTenantNotFound = errors.New("tenant not found")
)

type Service struct {
	repo tenants.TenantRepository
}

func NewService(repo tenants.TenantRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Execute(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	tenant, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}

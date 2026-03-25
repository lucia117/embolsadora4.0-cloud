package dashboard_layouts

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

const maxLayoutsPerTenant = 3

// Service implements application business logic for dashboard layouts.
type Service struct {
	repo   domain.Repository
	logger *zap.Logger
}

// NewService creates a new dashboard layouts service.
func NewService(repo domain.Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// ListLayouts returns all active layouts for the tenant.
func (s *Service) ListLayouts(ctx context.Context, tenantID uuid.UUID) ([]*domain.DashboardLayout, error) {
	layouts, err := s.repo.List(ctx, tenantID)
	if err != nil {
		s.logger.Error("failed to list layouts",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("operation", "list_layouts"),
		)
		return nil, err
	}
	s.logger.Info("layouts listed",
		zap.String("tenant_id", tenantID.String()),
		zap.Int("count", len(layouts)),
		zap.String("operation", "list_layouts"),
	)
	return layouts, nil
}

// GetLayout returns a single layout by ID for the tenant.
func (s *Service) GetLayout(ctx context.Context, tenantID, layoutID uuid.UUID) (*domain.DashboardLayout, error) {
	layout, err := s.repo.GetByID(ctx, tenantID, layoutID)
	if err != nil {
		if errors.Is(err, domain.ErrLayoutNotFound) {
			s.logger.Warn("layout not found",
				zap.String("tenant_id", tenantID.String()),
				zap.String("layout_id", layoutID.String()),
				zap.String("operation", "get_layout"),
			)
			return nil, domain.ErrLayoutNotFound
		}
		s.logger.Error("failed to get layout",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "get_layout"),
		)
		return nil, err
	}
	s.logger.Info("layout retrieved",
		zap.String("tenant_id", tenantID.String()),
		zap.String("layout_id", layoutID.String()),
		zap.String("operation", "get_layout"),
	)
	return layout, nil
}

// CreateLayout creates a new dashboard layout for the tenant.
// Enforces: max 3 layouts per tenant, unique name per tenant.
func (s *Service) CreateLayout(ctx context.Context, tenantID uuid.UUID, cmd domain.CreateLayoutCommand) (*domain.DashboardLayout, error) {
	count, err := s.repo.CountByTenant(ctx, tenantID)
	if err != nil {
		s.logger.Error("failed to count layouts",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("operation", "create_layout"),
		)
		return nil, err
	}
	if count >= maxLayoutsPerTenant {
		s.logger.Warn("layout limit reached",
			zap.String("tenant_id", tenantID.String()),
			zap.Int("count", count),
			zap.String("operation", "create_layout"),
		)
		return nil, domain.ErrLimitReached
	}

	exists, err := s.repo.ExistsByName(ctx, tenantID, cmd.Name, nil)
	if err != nil {
		s.logger.Error("failed to check name uniqueness",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("name", cmd.Name),
			zap.String("operation", "create_layout"),
		)
		return nil, err
	}
	if exists {
		s.logger.Warn("duplicate layout name",
			zap.String("tenant_id", tenantID.String()),
			zap.String("name", cmd.Name),
			zap.String("operation", "create_layout"),
		)
		return nil, domain.ErrDuplicateName
	}

	widgets := cmd.Widgets
	if widgets == nil {
		widgets = []domain.Widget{}
	}

	layout := &domain.DashboardLayout{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     cmd.Name,
		Widgets:  widgets,
	}

	if err := s.repo.Create(ctx, layout); err != nil {
		if errors.Is(err, domain.ErrDuplicateName) {
			return nil, domain.ErrDuplicateName
		}
		s.logger.Error("failed to create layout",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("name", cmd.Name),
			zap.String("operation", "create_layout"),
		)
		return nil, err
	}

	s.logger.Info("layout created",
		zap.String("tenant_id", tenantID.String()),
		zap.String("layout_id", layout.ID.String()),
		zap.String("name", cmd.Name),
		zap.String("operation", "create_layout"),
	)
	return layout, nil
}

// UpdateLayout replaces the name and widgets of an existing layout.
func (s *Service) UpdateLayout(ctx context.Context, tenantID, layoutID uuid.UUID, cmd domain.UpdateLayoutCommand) (*domain.DashboardLayout, error) {
	layout, err := s.repo.GetByID(ctx, tenantID, layoutID)
	if err != nil {
		if errors.Is(err, domain.ErrLayoutNotFound) {
			return nil, domain.ErrLayoutNotFound
		}
		s.logger.Error("failed to get layout for update",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "update_layout"),
		)
		return nil, err
	}

	exists, err := s.repo.ExistsByName(ctx, tenantID, cmd.Name, &layoutID)
	if err != nil {
		s.logger.Error("failed to check name uniqueness for update",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "update_layout"),
		)
		return nil, err
	}
	if exists {
		return nil, domain.ErrDuplicateName
	}

	widgets := cmd.Widgets
	if widgets == nil {
		widgets = []domain.Widget{}
	}
	layout.Name = cmd.Name
	layout.Widgets = widgets

	if err := s.repo.Update(ctx, layout); err != nil {
		if errors.Is(err, domain.ErrLayoutNotFound) {
			return nil, domain.ErrLayoutNotFound
		}
		if errors.Is(err, domain.ErrDuplicateName) {
			return nil, domain.ErrDuplicateName
		}
		s.logger.Error("failed to update layout",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "update_layout"),
		)
		return nil, err
	}

	s.logger.Info("layout updated",
		zap.String("tenant_id", tenantID.String()),
		zap.String("layout_id", layoutID.String()),
		zap.String("name", cmd.Name),
		zap.String("operation", "update_layout"),
	)
	return layout, nil
}

// DeleteLayout soft-deletes a layout. Rejects deletion of the last remaining layout.
func (s *Service) DeleteLayout(ctx context.Context, tenantID, layoutID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, tenantID, layoutID); err != nil {
		if errors.Is(err, domain.ErrLayoutNotFound) {
			return domain.ErrLayoutNotFound
		}
		s.logger.Error("failed to get layout for delete",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "delete_layout"),
		)
		return err
	}

	count, err := s.repo.CountByTenant(ctx, tenantID)
	if err != nil {
		s.logger.Error("failed to count layouts for delete",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("operation", "delete_layout"),
		)
		return err
	}
	if count <= 1 {
		s.logger.Warn("cannot delete last layout",
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "delete_layout"),
		)
		return domain.ErrCannotDeleteLastLayout
	}

	if err := s.repo.SoftDelete(ctx, tenantID, layoutID); err != nil {
		s.logger.Error("failed to soft-delete layout",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "delete_layout"),
		)
		return err
	}

	s.logger.Info("layout deleted",
		zap.String("tenant_id", tenantID.String()),
		zap.String("layout_id", layoutID.String()),
		zap.String("operation", "delete_layout"),
	)
	return nil
}

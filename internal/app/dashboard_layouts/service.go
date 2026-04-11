package dashboard_layouts

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

// Service implements application business logic for dashboard layouts.
type Service struct {
	repo   domain.Repository
	logger *zap.Logger
}

// NewService creates a new dashboard layouts service.
func NewService(repo domain.Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// ListLayouts returns all active layouts for the (tenant, user).
func (s *Service) ListLayouts(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.DashboardLayout, error) {
	layouts, err := s.repo.List(ctx, tenantID, userID)
	if err != nil {
		s.logger.Error("failed to list layouts",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("user_id", userID.String()),
			zap.String("operation", "list_layouts"),
		)
		return nil, err
	}
	s.logger.Info("layouts listed",
		zap.String("tenant_id", tenantID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("count", len(layouts)),
		zap.String("operation", "list_layouts"),
	)
	return layouts, nil
}

// GetLayout returns a single layout by ID for the (tenant, user).
func (s *Service) GetLayout(ctx context.Context, tenantID, userID, layoutID uuid.UUID) (*domain.DashboardLayout, error) {
	layout, err := s.repo.GetByID(ctx, tenantID, userID, layoutID)
	if err != nil {
		if errors.Is(err, domain.ErrLayoutNotFound) {
			s.logger.Warn("layout not found",
				zap.String("tenant_id", tenantID.String()),
				zap.String("user_id", userID.String()),
				zap.String("layout_id", layoutID.String()),
				zap.String("operation", "get_layout"),
			)
			return nil, domain.ErrLayoutNotFound
		}
		s.logger.Error("failed to get layout",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("user_id", userID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "get_layout"),
		)
		return nil, err
	}
	s.logger.Info("layout retrieved",
		zap.String("tenant_id", tenantID.String()),
		zap.String("user_id", userID.String()),
		zap.String("layout_id", layoutID.String()),
		zap.String("operation", "get_layout"),
	)
	return layout, nil
}

// CreateLayout creates a new dashboard layout for the (tenant, user).
// Limit enforcement and uniqueness are handled atomically in the repository.
func (s *Service) CreateLayout(ctx context.Context, tenantID, userID uuid.UUID, cmd domain.CreateLayoutCommand) (*domain.DashboardLayout, error) {
	widgets := cmd.Widgets
	if widgets == nil {
		widgets = []domain.Widget{}
	}

	layout := &domain.DashboardLayout{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   userID,
		Name:     cmd.Name,
		Widgets:  widgets,
	}

	if err := s.repo.Create(ctx, layout); err != nil {
		if !errors.Is(err, domain.ErrLimitReached) && !errors.Is(err, domain.ErrDuplicateName) {
			s.logger.Error("failed to create layout",
				zap.Error(err),
				zap.String("tenant_id", tenantID.String()),
				zap.String("user_id", userID.String()),
				zap.String("name", cmd.Name),
				zap.String("operation", "create_layout"),
			)
		}
		return nil, err
	}

	s.logger.Info("layout created",
		zap.String("tenant_id", tenantID.String()),
		zap.String("user_id", userID.String()),
		zap.String("layout_id", layout.ID.String()),
		zap.String("name", cmd.Name),
		zap.String("operation", "create_layout"),
	)
	return layout, nil
}

// UpdateLayout replaces the name and widgets of an existing layout.
func (s *Service) UpdateLayout(ctx context.Context, tenantID, userID, layoutID uuid.UUID, cmd domain.UpdateLayoutCommand) (*domain.DashboardLayout, error) {
	layout, err := s.repo.GetByID(ctx, tenantID, userID, layoutID)
	if err != nil {
		if errors.Is(err, domain.ErrLayoutNotFound) {
			return nil, domain.ErrLayoutNotFound
		}
		s.logger.Error("failed to get layout for update",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("user_id", userID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "update_layout"),
		)
		return nil, err
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
			zap.String("user_id", userID.String()),
			zap.String("layout_id", layoutID.String()),
			zap.String("operation", "update_layout"),
		)
		return nil, err
	}

	s.logger.Info("layout updated",
		zap.String("tenant_id", tenantID.String()),
		zap.String("user_id", userID.String()),
		zap.String("layout_id", layoutID.String()),
		zap.String("name", cmd.Name),
		zap.String("operation", "update_layout"),
	)
	return layout, nil
}

// DeleteLayout soft-deletes a layout. Rejects deletion of the last remaining layout.
// Existence and count checks are handled atomically in the repository.
func (s *Service) DeleteLayout(ctx context.Context, tenantID, userID, layoutID uuid.UUID) error {
	if err := s.repo.SoftDelete(ctx, tenantID, userID, layoutID); err != nil {
		if !errors.Is(err, domain.ErrLayoutNotFound) && !errors.Is(err, domain.ErrCannotDeleteLastLayout) {
			s.logger.Error("failed to soft-delete layout",
				zap.Error(err),
				zap.String("tenant_id", tenantID.String()),
				zap.String("user_id", userID.String()),
				zap.String("layout_id", layoutID.String()),
				zap.String("operation", "delete_layout"),
			)
		}
		return err
	}

	s.logger.Info("layout deleted",
		zap.String("tenant_id", tenantID.String()),
		zap.String("user_id", userID.String()),
		zap.String("layout_id", layoutID.String()),
		zap.String("operation", "delete_layout"),
	)
	return nil
}

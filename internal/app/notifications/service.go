package notifications

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	notifRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
	"go.uber.org/zap"
)

// Service contiene la lógica de negocio para gestión de notificaciones.
type Service struct {
	repo   notifRepo.Repository
	logger *zap.Logger
}

// New crea un nuevo servicio de notificaciones.
func New(repo notifRepo.Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// List retorna las notificaciones paginadas del tenant con filtros opcionales.
// Retorna la lista, el total de resultados (sin paginación) y un error.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, params notifRepo.ListParams) ([]*domain.Notification, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	items, total, err := s.repo.List(ctx, tenantID, params)
	if err != nil {
		s.logger.Error("error listando notificaciones",
			zap.String("tenant_id", tenantID.String()),
			zap.Error(err),
		)
		return nil, 0, err
	}
	return items, total, nil
}

// CountUnread retorna el número de notificaciones no leídas del tenant.
func (s *Service) CountUnread(ctx context.Context, tenantID uuid.UUID) (int, error) {
	count, err := s.repo.CountUnread(ctx, tenantID)
	if err != nil {
		s.logger.Error("error contando notificaciones no leídas",
			zap.String("tenant_id", tenantID.String()),
			zap.Error(err),
		)
		return 0, err
	}
	return count, nil
}

// Get retorna una notificación por ID verificando el tenant.
func (s *Service) Get(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) {
	n, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotificationNotFound) {
			s.logger.Error("error obteniendo notificación",
				zap.String("notification_id", id.String()),
				zap.String("tenant_id", tenantID.String()),
				zap.Error(err),
			)
		}
		return nil, err
	}
	return n, nil
}

// Ack marca una notificación como acknowledged (idempotente).
func (s *Service) Ack(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) {
	n, err := s.repo.Ack(ctx, id, tenantID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotificationNotFound) {
			s.logger.Error("error haciendo ack de notificación",
				zap.String("notification_id", id.String()),
				zap.String("tenant_id", tenantID.String()),
				zap.Error(err),
			)
		}
		return nil, err
	}
	s.logger.Info("notificación acknowledged",
		zap.String("notification_id", id.String()),
		zap.String("tenant_id", tenantID.String()),
		zap.String("status", string(n.Status)),
	)
	return n, nil
}

// Close marca una notificación como closed (idempotente).
func (s *Service) Close(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) {
	n, err := s.repo.Close(ctx, id, tenantID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotificationNotFound) {
			s.logger.Error("error cerrando notificación",
				zap.String("notification_id", id.String()),
				zap.String("tenant_id", tenantID.String()),
				zap.Error(err),
			)
		}
		return nil, err
	}
	s.logger.Info("notificación closed",
		zap.String("notification_id", id.String()),
		zap.String("tenant_id", tenantID.String()),
	)
	return n, nil
}

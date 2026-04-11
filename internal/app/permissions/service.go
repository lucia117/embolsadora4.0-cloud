package permissions

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/domain"
	permissionsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/permissions"
)

// Service contiene la lógica de negocio para gestión de permisos.
type Service struct {
	repo   permissionsRepo.Repository
	logger *zap.Logger
}

// NewService crea un nuevo servicio de permisos.
func NewService(repo permissionsRepo.Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// ListPermissions devuelve todos los permisos disponibles para el tenant:
// los permisos de sistema (globales) más los permisos custom del tenant.
func (s *Service) ListPermissions(ctx context.Context, tenantID uuid.UUID) ([]*domain.Permission, error) {
	perms, err := s.repo.List(ctx, tenantID)
	if err != nil {
		s.logger.Error("error listando permisos", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		return nil, err
	}
	return perms, nil
}

// GetPermission devuelve un permiso por su ID (de sistema o custom).
func (s *Service) GetPermission(ctx context.Context, id string) (*domain.Permission, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err != domain.ErrPermissionNotFound {
			s.logger.Error("error obteniendo permiso", zap.String("permission_id", id), zap.Error(err))
		}
		return nil, err
	}
	return p, nil
}

// CreatePermission crea un nuevo permiso custom para el tenant.
// Valida que name >= 3 caracteres, section y description no estén vacíos.
func (s *Service) CreatePermission(ctx context.Context, tenantID uuid.UUID, name, section, description string) (*domain.Permission, error) {
	if err := validatePermissionFields(name, section, description); err != nil {
		return nil, err
	}

	id := uuid.New().String()
	p := &domain.Permission{
		ID:                 id,
		Name:               name,
		Section:            section,
		Description:        description,
		IsSystemPermission: false,
		TenantID:           &tenantID,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		s.logger.Error("error creando permiso", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		return nil, err
	}

	// Leer el permiso recién creado para obtener los timestamps generados por la BD
	created, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("error leyendo permiso recién creado", zap.String("permission_id", id), zap.Error(err))
		return nil, err
	}

	s.logger.Info("permiso creado", zap.String("permission_id", id), zap.String("tenant_id", tenantID.String()))
	return created, nil
}

// UpdatePermission actualiza nombre, sección y descripción de un permiso custom.
// Retorna ErrPermissionIsSystem si se intenta modificar un permiso de sistema.
func (s *Service) UpdatePermission(ctx context.Context, id, name, section, description string) (*domain.Permission, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.IsSystemPermission {
		return nil, domain.ErrPermissionIsSystem
	}

	if err := validatePermissionFields(name, section, description); err != nil {
		return nil, err
	}

	p.Name = name
	p.Section = section
	p.Description = description

	if err := s.repo.Update(ctx, p); err != nil {
		s.logger.Error("error actualizando permiso", zap.String("permission_id", id), zap.Error(err))
		return nil, err
	}

	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("error leyendo permiso actualizado", zap.String("permission_id", id), zap.Error(err))
		return nil, err
	}

	s.logger.Info("permiso actualizado", zap.String("permission_id", id))
	return updated, nil
}

// DeletePermission elimina permanentemente un permiso custom.
// Retorna ErrPermissionIsSystem si se intenta eliminar un permiso de sistema.
func (s *Service) DeletePermission(ctx context.Context, id string) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if err != domain.ErrPermissionNotFound && err != domain.ErrPermissionIsSystem {
			s.logger.Error("error eliminando permiso", zap.String("permission_id", id), zap.Error(err))
		}
		return err
	}
	s.logger.Info("permiso eliminado", zap.String("permission_id", id))
	return nil
}

// validatePermissionFields valida los campos editables de un permiso.
func validatePermissionFields(name, section, description string) error {
	if len(strings.TrimSpace(name)) < 3 {
		return &ValidationError{Field: "name", Message: "name must be at least 3 characters"}
	}
	if strings.TrimSpace(section) == "" {
		return &ValidationError{Field: "section", Message: "section is required"}
	}
	if strings.TrimSpace(description) == "" {
		return &ValidationError{Field: "description", Message: "description is required"}
	}
	return nil
}

// ValidationError representa un error de validación de campo específico.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

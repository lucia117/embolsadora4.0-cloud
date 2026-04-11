package roles

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// Service contiene la lógica de negocio para gestión de roles.
type Service struct {
	repo   rolesRepo.Repository
	logger *zap.Logger
}

// NewService crea un nuevo servicio de roles.
func NewService(repo rolesRepo.Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// ListRoles devuelve los roles del sistema + roles custom del tenant.
func (s *Service) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.Role, error) {
	roles, err := s.repo.List(ctx, tenantID)
	if err != nil {
		s.logger.Error("error listando roles", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		return nil, err
	}
	return roles, nil
}

// GetRole devuelve un rol por su ID.
func (s *Service) GetRole(ctx context.Context, id string) (*domain.Role, error) {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err != domain.ErrRoleNotFound {
			s.logger.Error("error obteniendo rol", zap.String("role_id", id), zap.Error(err))
		}
		return nil, err
	}
	return role, nil
}

// CreateRole crea un nuevo rol personalizado para el tenant.
// Verifica el límite de 3 roles custom y unicidad de nombre.
func (s *Service) CreateRole(ctx context.Context, tenantID uuid.UUID, name, description string, permissions []string) (*domain.Role, error) {
	count, err := s.repo.CountCustomByTenant(ctx, tenantID)
	if err != nil {
		s.logger.Error("error contando roles del tenant", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		return nil, err
	}
	if count >= domain.MaxCustomRolesPerTenant {
		return nil, domain.ErrRoleLimitReached
	}

	id, err := generateRoleID()
	if err != nil {
		return nil, err
	}

	role := &domain.Role{
		ID:          id,
		Name:        name,
		Description: description,
		Permissions: deduplicatePermissions(permissions),
		TenantID:    &tenantID,
	}

	if err := s.repo.Create(ctx, role); err != nil {
		if err != domain.ErrRoleDuplicateName {
			s.logger.Error("error creando rol", zap.String("tenant_id", tenantID.String()), zap.Error(err))
		}
		return nil, err
	}

	s.logger.Info("rol creado", zap.String("role_id", role.ID), zap.String("tenant_id", tenantID.String()))
	return role, nil
}

// UpdateRole actualiza nombre, descripción y permisos de un rol personalizado.
// Los roles del sistema no pueden modificarse.
func (s *Service) UpdateRole(ctx context.Context, id, name, description string, permissions []string) (*domain.Role, error) {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.IsSystemRole {
		return nil, domain.ErrRoleIsSystemRole
	}

	role.Name = name
	role.Description = description
	role.Permissions = deduplicatePermissions(permissions)

	if err := s.repo.Update(ctx, role); err != nil {
		if err != domain.ErrRoleDuplicateName && err != domain.ErrRoleNotFound {
			s.logger.Error("error actualizando rol", zap.String("role_id", id), zap.Error(err))
		}
		return nil, err
	}

	s.logger.Info("rol actualizado", zap.String("role_id", id))
	return role, nil
}

// DeleteRole elimina (soft delete) un rol personalizado.
// Los roles del sistema no pueden eliminarse.
// Los roles con asignaciones activas no pueden eliminarse.
func (s *Service) DeleteRole(ctx context.Context, id string) error {
	role, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if role.IsSystemRole {
		return domain.ErrRoleIsSystemRole
	}

	count, err := s.repo.CountActiveAssignments(ctx, id)
	if err != nil {
		s.logger.Error("error contando asignaciones del rol", zap.String("role_id", id), zap.Error(err))
		return err
	}
	if count > 0 {
		return domain.ErrRoleHasAssignments
	}

	if err := s.repo.SoftDelete(ctx, id); err != nil {
		s.logger.Error("error eliminando rol", zap.String("role_id", id), zap.Error(err))
		return err
	}

	s.logger.Info("rol eliminado", zap.String("role_id", id))
	return nil
}

// CountActiveAssignments devuelve la cantidad de usuarios asignados activamente al rol.
func (s *Service) CountActiveAssignments(ctx context.Context, roleID string) (int, error) {
	return s.repo.CountActiveAssignments(ctx, roleID)
}

// generateRoleID genera un ID único para roles personalizados con formato "custom_<6 hex chars>".
func generateRoleID() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "custom_" + hex.EncodeToString(b), nil
}

// deduplicatePermissions elimina duplicados y ordena la lista de permisos.
func deduplicatePermissions(permissions []string) []string {
	if len(permissions) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(permissions))
	result := make([]string, 0, len(permissions))
	for _, p := range permissions {
		normalized := strings.TrimSpace(p)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; !exists {
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	sort.Strings(result)
	return result
}

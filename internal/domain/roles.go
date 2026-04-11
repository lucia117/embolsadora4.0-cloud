package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// MaxCustomRolesPerTenant es el máximo de roles personalizados permitidos por tenant.
const MaxCustomRolesPerTenant = 3

// Errores de dominio para la gestión de roles.
var (
	ErrRoleNotFound       = errors.New("rol no encontrado")
	ErrRoleIsSystemRole   = errors.New("los roles del sistema no pueden modificarse ni eliminarse")
	ErrRoleHasAssignments = errors.New("el rol tiene usuarios asignados activos")
	ErrRoleDuplicateName  = errors.New("ya existe un rol con ese nombre en este tenant")
	ErrRoleLimitReached   = errors.New("se alcanzó el máximo de roles personalizados por tenant")
)

// Role representa un rol de acceso en el sistema.
// Los roles del sistema (is_system_role=true) son globales, inmutables y no eliminables.
// Los roles personalizados (is_system_role=false) son creados por tenant y tienen límite de 3.
type Role struct {
	ID           string
	Name         string
	Description  string
	Permissions  []string
	IsSystemRole bool
	IsGlobal     bool
	TenantID     *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

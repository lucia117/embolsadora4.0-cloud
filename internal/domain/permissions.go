package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errores de dominio para la gestión de permisos.
var (
	ErrPermissionNotFound         = errors.New("permiso no encontrado")
	ErrPermissionIsSystem         = errors.New("los permisos del sistema no pueden modificarse ni eliminarse")
	ErrPermissionValidationFailed = errors.New("datos del permiso inválidos")
)

// Permission representa un permiso del sistema de control de acceso.
// Los permisos de sistema (IsSystemPermission=true) son globales, inmutables y no eliminables.
// Los permisos custom (IsSystemPermission=false) son creados por tenant admin y están aislados por tenant.
type Permission struct {
	ID                 string
	Name               string
	Section            string
	Description        string
	IsSystemPermission bool
	TenantID           *uuid.UUID // nil para permisos de sistema (globales)
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

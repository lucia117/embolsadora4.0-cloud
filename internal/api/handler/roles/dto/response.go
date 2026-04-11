package dto

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// RoleResponse es la representación JSON de un rol.
type RoleResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Permissions  []string  `json:"permissions"`
	IsSystemRole bool      `json:"isSystemRole"`
	IsGlobal     bool      `json:"isGlobal"`
	TenantID     *string   `json:"tenantId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// FromDomain convierte un domain.Role en RoleResponse.
func FromDomain(r *domain.Role) RoleResponse {
	resp := RoleResponse{
		ID:           r.ID,
		Name:         r.Name,
		Description:  r.Description,
		Permissions:  r.Permissions,
		IsSystemRole: r.IsSystemRole,
		IsGlobal:     r.IsGlobal,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if r.TenantID != nil {
		s := r.TenantID.String()
		resp.TenantID = &s
	}
	return resp
}

package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// UserRoleResponse is the JSON shape returned for a single UTR assignment.
type UserRoleResponse struct {
	ID         string  `json:"id"`
	UserID     string  `json:"userId"`
	TenantID   string  `json:"tenantId"`
	RoleID     *string `json:"roleId"`
	Status     string  `json:"status"`
	AssignedBy *string `json:"assignedBy"`
	AssignedAt *string `json:"assignedAt"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

// FromDomain converts a domain.UserTenantRole to a UserRoleResponse.
func FromDomain(utr *domain.UserTenantRole) *UserRoleResponse {
	resp := &UserRoleResponse{
		ID:        utr.ID.String(),
		UserID:    utr.UserID.String(),
		TenantID:  utr.TenantID.String(),
		RoleID:    utr.RoleID,
		Status:    string(utr.Status),
		CreatedAt: utr.CreatedAt.Format(time.RFC3339),
		UpdatedAt: utr.UpdatedAt.Format(time.RFC3339),
	}
	if utr.AssignedBy != nil {
		s := utr.AssignedBy.String()
		resp.AssignedBy = &s
	}
	if utr.AssignedAt != nil {
		s := utr.AssignedAt.Format(time.RFC3339)
		resp.AssignedAt = &s
	}
	return resp
}

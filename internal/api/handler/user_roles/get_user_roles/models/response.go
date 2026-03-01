package models

import "github.com/tu-org/embolsadora-api/internal/domain"

// UserRoleContextResponse represents a role assignment with tenant and role details.
type UserRoleContextResponse struct {
	TenantID   string `json:"tenantId"`
	TenantName string `json:"tenantName"`
	RoleID     string `json:"roleId"`
	RoleName   string `json:"roleName"`
	Status     string `json:"status"`
}

// FromDomain converts a slice of domain.UserRoleWithContext to a slice of UserRoleContextResponse.
func FromDomain(items []domain.UserRoleWithContext) []UserRoleContextResponse {
	result := make([]UserRoleContextResponse, 0, len(items))
	for _, item := range items {
		resp := UserRoleContextResponse{
			TenantID:   item.TenantID.String(),
			TenantName: item.TenantName,
			RoleID:     item.RoleID,
			RoleName:   item.RoleName,
			Status:     string(item.Status),
		}
		result = append(result, resp)
	}
	return result
}

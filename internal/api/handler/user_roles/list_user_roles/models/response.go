package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// UserSummary is the minimal user identity embedded in a UserRoleResponse,
// resolved via the JOIN in FindByTenant so callers can render a name or
// email without a second request.
type UserSummary struct {
	Email     string  `json:"email"`
	Name      *string `json:"name"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}

// UserRoleResponse is the JSON shape for a single UTR assignment in list responses.
type UserRoleResponse struct {
	ID         string       `json:"id"`
	UserID     string       `json:"userId"`
	TenantID   string       `json:"tenantId"`
	RoleID     *string      `json:"roleId"`
	RoleName   string       `json:"roleName"`
	Status     string       `json:"status"`
	AssignedBy *string      `json:"assignedBy"`
	AssignedAt *string      `json:"assignedAt"`
	CreatedAt  string       `json:"createdAt"`
	UpdatedAt  string       `json:"updatedAt"`
	User       *UserSummary `json:"user"`
}

// FromDomain converts a slice of domain.UserTenantRoleDetail to a slice of UserRoleResponse.
func FromDomain(utrs []domain.UserTenantRoleDetail) []UserRoleResponse {
	result := make([]UserRoleResponse, 0, len(utrs))
	for _, utr := range utrs {
		resp := UserRoleResponse{
			ID:        utr.ID.String(),
			UserID:    utr.UserID.String(),
			TenantID:  utr.TenantID.String(),
			RoleID:    utr.RoleID,
			RoleName:  utr.RoleName,
			Status:    string(utr.Status),
			CreatedAt: utr.CreatedAt.Format(time.RFC3339),
			UpdatedAt: utr.UpdatedAt.Format(time.RFC3339),
			User: &UserSummary{
				Email:     utr.UserEmail,
				Name:      utr.UserName,
				FirstName: utr.UserFirstName,
				LastName:  utr.UserLastName,
			},
		}
		if utr.AssignedBy != nil {
			s := utr.AssignedBy.String()
			resp.AssignedBy = &s
		}
		if utr.AssignedAt != nil {
			s := utr.AssignedAt.Format(time.RFC3339)
			resp.AssignedAt = &s
		}
		result = append(result, resp)
	}
	return result
}

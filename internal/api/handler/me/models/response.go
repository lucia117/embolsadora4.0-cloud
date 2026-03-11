package models

// MeResponse is the response body for GET /api/v1/me.
type MeResponse struct {
	User        UserProfileResponse `json:"user"`
	Tenant      *TenantInfoResponse `json:"tenant"`
	Role        *RoleInfoResponse   `json:"role"`
	Permissions []string            `json:"permissions"`
}

// UserProfileResponse contains public user identity fields.
type UserProfileResponse struct {
	ID                     string  `json:"id"`
	Email                  *string `json:"email"`
	Name                   *string `json:"name"`
	PasswordChangeRequired bool    `json:"password_change_required"`
}

// TenantInfoResponse contains tenant display fields.
type TenantInfoResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
}

// RoleInfoResponse contains the user's role in the current tenant.
type RoleInfoResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

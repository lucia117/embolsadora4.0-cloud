package dto

// RoleInfo is a compact representation of a role returned when include=roles is requested.
type RoleInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// UserWithRolesResponse extends UserResponse with the active role assignment.
type UserWithRolesResponse struct {
	UserResponse
	Roles []RoleInfo `json:"roles"`
}

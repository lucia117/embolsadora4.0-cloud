package models

// UserResponse define la estructura de respuesta para los usuarios
type UserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	TenantID  string `json:"tenant_id"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UsersResponse define la estructura de respuesta para listar múltiples usuarios
type UsersResponse struct {
	Users []UserResponse `json:"users"`
}

// UserResponseSingle define la estructura de respuesta para un solo usuario
type UserResponseSingle struct {
	User UserResponse `json:"user"`
}

// UserProfileResponse define la estructura de respuesta para el perfil de usuario (sin datos sensibles)
type UserProfileResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

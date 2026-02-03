package models

// UserRequest define la estructura de la solicitud para crear/actualizar un usuario
type UserRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name" binding:"required"`
	TenantID    string `json:"tenant_id" binding:"required"`
	Role        string `json:"role"`
	Active      bool   `json:"active"`
}

// UserUpdateRequest define la estructura para actualizar un usuario (con campos opcionales)
type UserUpdateRequest struct {
	Username  *string `json:"username"`
	Email     *string `json:"email"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	TenantID  *string `json:"tenant_id"`
	Role      *string `json:"role"`
	Active    *bool   `json:"active"`
}

// UserPasswordUpdateRequest define la estructura para actualizar la contraseña
type UserPasswordUpdateRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

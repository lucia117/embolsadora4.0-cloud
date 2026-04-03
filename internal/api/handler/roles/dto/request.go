package dto

// CreateRoleRequest es el cuerpo de la petición para crear un rol personalizado.
type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleRequest es el cuerpo de la petición para actualizar un rol personalizado.
type UpdateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

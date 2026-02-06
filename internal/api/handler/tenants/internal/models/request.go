package models

// TenantRequest define la estructura de la solicitud para crear/actualizar un tenant
type TenantRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	Active      bool   `json:"active"`
}

// TenantUpdateRequest define la estructura para actualizar un tenant (con campos opcionales)
type TenantUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Domain      *string `json:"domain"`
	Active      *bool   `json:"active"`
}

package models

// TenantUpdateRequest define la estructura para actualizar un tenant (con campos opcionales)
type TenantUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Domain      *string `json:"domain"`
	Active      *bool   `json:"active"`
}

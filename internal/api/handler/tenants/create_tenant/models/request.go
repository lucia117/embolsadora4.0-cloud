package models

// TenantRequest define la estructura de la solicitud para crear un tenant
type TenantRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	Active      bool   `json:"active"`
}

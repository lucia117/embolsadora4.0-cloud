package models

// TenantResponse define la estructura de respuesta para los tenants
type TenantResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TenantsResponse define la estructura de respuesta para listar múltiples tenants
type TenantsResponse struct {
	Tenants []TenantResponse `json:"tenants"`
}

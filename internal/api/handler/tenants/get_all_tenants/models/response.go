package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// GetAllTenantsResponse representa la respuesta del endpoint GET /api/tenants
type GetAllTenantsResponse []TenantResponse

// TenantResponse representa un tenant individual en la respuesta
type TenantResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	CompanyName string  `json:"companyName"`
	Subdomain   string  `json:"subdomain"`
	Description string  `json:"description"`
	IsActive    bool    `json:"isActive"`
	Theme       Theme   `json:"theme"`
	Address     Address `json:"address"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// Theme representa la configuración de tema de un tenant
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Address representa la dirección de un tenant
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

func FromDomain(tenants []domain.Tenant) GetAllTenantsResponse {
	response := make(GetAllTenantsResponse, len(tenants))
	for i, tenant := range tenants {
		response[i] = TenantResponse{
			ID:          tenant.ID.String(),
			Name:        tenant.Name,
			CompanyName: tenant.CompanyName,
			Subdomain:   tenant.Subdomain,
			Description: tenant.Description,
			IsActive:    tenant.IsActive,
			Theme: Theme{
				PrimaryColor:    tenant.Theme.PrimaryColor,
				SecondaryColor:  tenant.Theme.SecondaryColor,
				AccentColor:     tenant.Theme.AccentColor,
				TextColor:       tenant.Theme.TextColor,
				BackgroundColor: tenant.Theme.BackgroundColor,
				LogoUrl:         tenant.Theme.LogoUrl,
				FaviconUrl:      tenant.Theme.FaviconUrl,
			},
			Address: Address{
				Street:     tenant.Address.Street,
				City:       tenant.Address.City,
				State:      tenant.Address.State,
				PostalCode: tenant.Address.PostalCode,
				Country:    tenant.Address.Country,
			},
			CreatedAt: tenant.CreatedAt.Format(time.RFC3339),
			UpdatedAt: tenant.UpdatedAt.Format(time.RFC3339),
		}
	}
	return response
}

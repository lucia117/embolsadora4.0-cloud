package models

import "github.com/tu-org/embolsadora-api/internal/domain"

// Theme is the branding-safe subset of domain.Theme.
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Settings is the localization subset of domain.TenantSettings — no
// contactEmail, no companyWebsite (those are not safe to expose without auth).
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

// PublicTenantResponse is deliberately narrower than get_tenant/models.TenantResponse:
// no address, no contactEmail, no companyWebsite, no description. This is served
// without authentication, so only branding/routing fields belong here.
type PublicTenantResponse struct {
	ID          string   `json:"id"`
	Subdomain   string   `json:"subdomain"`
	Name        string   `json:"name"`
	CompanyName string   `json:"companyName"`
	IsActive    bool     `json:"isActive"`
	Theme       Theme    `json:"theme"`
	Settings    Settings `json:"settings"`
}

func FromDomain(tenant *domain.Tenant) *PublicTenantResponse {
	return &PublicTenantResponse{
		ID:          tenant.ID.String(),
		Subdomain:   tenant.Subdomain,
		Name:        tenant.Name,
		CompanyName: tenant.CompanyName,
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
		Settings: Settings{
			Locale:     tenant.Settings.Locale,
			Timezone:   tenant.Settings.Timezone,
			DateFormat: tenant.Settings.DateFormat,
			TimeFormat: tenant.Settings.TimeFormat,
			Currency:   tenant.Settings.Currency,
		},
	}
}

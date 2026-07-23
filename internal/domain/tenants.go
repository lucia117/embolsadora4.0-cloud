package domain

import (
	"time"

	"github.com/google/uuid"
)

// Theme represents the visual theme configuration for a tenant
type Theme struct {
	PrimaryColor    string `json:"primaryColor" db:"primary_color"`
	SecondaryColor  string `json:"secondaryColor" db:"secondary_color"`
	AccentColor     string `json:"accentColor" db:"accent_color"`
	TextColor       string `json:"textColor" db:"text_color"`
	BackgroundColor string `json:"backgroundColor" db:"background_color"`
	LogoUrl         string `json:"logoUrl" db:"logo_url"`
	FaviconUrl      string `json:"faviconUrl" db:"favicon_url"`
}

// Address represents the address information for a tenant
type Address struct {
	Street     string `json:"street" db:"street"`
	City       string `json:"city" db:"city"`
	State      string `json:"state" db:"state"`
	PostalCode string `json:"postalCode" db:"postal_code"`
	Country    string `json:"country" db:"country"`
}

// TenantSettings represents tenant-level contact and localization preferences
type TenantSettings struct {
	ContactEmail   string `json:"contactEmail" db:"contact_email"`
	CompanyWebsite string `json:"companyWebsite" db:"company_website"`
	Locale         string `json:"locale" db:"locale"`
	Timezone       string `json:"timezone" db:"timezone"`
	DateFormat     string `json:"dateFormat" db:"date_format"`
	TimeFormat     string `json:"timeFormat" db:"time_format"`
	Currency       string `json:"currency" db:"currency"`
}

// Tenant representa una organización/empresa en el sistema
type Tenant struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	CompanyName string         `json:"companyName" db:"company_name"`
	Subdomain   string         `json:"subdomain" db:"subdomain"`
	Description string         `json:"description" db:"description"`
	IsActive    bool           `json:"isActive" db:"is_active"`
	Theme       Theme          `json:"theme" db:"theme"`
	Address     Address        `json:"address" db:"address"`
	Settings    TenantSettings `json:"settings" db:"settings"`
	CreatedAt   time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time      `json:"updatedAt" db:"updated_at"`
}

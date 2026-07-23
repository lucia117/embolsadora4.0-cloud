package update_tenant

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var ErrTenantNotFound = errors.New("tenant not found")

// UseCase defines the interface for tenant update use case
type UseCase interface {
	Update(ctx context.Context, id uuid.UUID, req *UpdateTenantRequest) (*domain.Tenant, error)
}

// UpdateTenantRequest represents the request to update a tenant
type UpdateTenantRequest struct {
	Name           *string
	CompanyName    *string
	Subdomain      *string
	Description    *string
	IsActive       *bool
	ContactEmail   *string
	CompanyWebsite *string
	Theme          *ThemeUpdate
	Address        *AddressUpdate
	Settings       *SettingsUpdate
}

// ThemeUpdate represents the theme configuration for update
type ThemeUpdate struct {
	PrimaryColor    *string
	SecondaryColor  *string
	AccentColor     *string
	TextColor       *string
	BackgroundColor *string
	LogoUrl         *string
	FaviconUrl      *string
}

// AddressUpdate represents the address information for update
type AddressUpdate struct {
	Street     *string
	City       *string
	State      *string
	PostalCode *string
	Country    *string
}

// SettingsUpdate represents the localization/preferences configuration for update
type SettingsUpdate struct {
	Locale     *string
	Timezone   *string
	DateFormat *string
	TimeFormat *string
	Currency   *string
}

type useCase struct {
	repo tenants.TenantRepository
}

// NewUseCase creates a new instance of the tenant update use case
func NewUseCase(repo tenants.TenantRepository) UseCase {
	return &useCase{
		repo: repo,
	}
}

// Update updates an existing tenant
func (uc *useCase) Update(ctx context.Context, id uuid.UUID, req *UpdateTenantRequest) (*domain.Tenant, error) {
	// First, get the existing tenant
	tenant, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	// Update fields if they are provided (not nil)
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.CompanyName != nil {
		tenant.CompanyName = *req.CompanyName
	}
	if req.Subdomain != nil {
		tenant.Subdomain = *req.Subdomain
	}
	if req.Description != nil {
		tenant.Description = *req.Description
	}
	if req.IsActive != nil {
		tenant.IsActive = *req.IsActive
	}

	// Update theme fields if provided
	if req.Theme != nil {
		if req.Theme.PrimaryColor != nil {
			tenant.Theme.PrimaryColor = *req.Theme.PrimaryColor
		}
		if req.Theme.SecondaryColor != nil {
			tenant.Theme.SecondaryColor = *req.Theme.SecondaryColor
		}
		if req.Theme.AccentColor != nil {
			tenant.Theme.AccentColor = *req.Theme.AccentColor
		}
		if req.Theme.TextColor != nil {
			tenant.Theme.TextColor = *req.Theme.TextColor
		}
		if req.Theme.BackgroundColor != nil {
			tenant.Theme.BackgroundColor = *req.Theme.BackgroundColor
		}
		if req.Theme.LogoUrl != nil {
			tenant.Theme.LogoUrl = *req.Theme.LogoUrl
		}
		if req.Theme.FaviconUrl != nil {
			tenant.Theme.FaviconUrl = *req.Theme.FaviconUrl
		}
	}

	// Update address fields if provided
	if req.Address != nil {
		if req.Address.Street != nil {
			tenant.Address.Street = *req.Address.Street
		}
		if req.Address.City != nil {
			tenant.Address.City = *req.Address.City
		}
		if req.Address.State != nil {
			tenant.Address.State = *req.Address.State
		}
		if req.Address.PostalCode != nil {
			tenant.Address.PostalCode = *req.Address.PostalCode
		}
		if req.Address.Country != nil {
			tenant.Address.Country = *req.Address.Country
		}
	}

	// Update contact/website fields if provided
	if req.ContactEmail != nil {
		tenant.Settings.ContactEmail = *req.ContactEmail
	}
	if req.CompanyWebsite != nil {
		tenant.Settings.CompanyWebsite = *req.CompanyWebsite
	}

	// Update settings fields if provided
	if req.Settings != nil {
		if req.Settings.Locale != nil {
			tenant.Settings.Locale = *req.Settings.Locale
		}
		if req.Settings.Timezone != nil {
			tenant.Settings.Timezone = *req.Settings.Timezone
		}
		if req.Settings.DateFormat != nil {
			tenant.Settings.DateFormat = *req.Settings.DateFormat
		}
		if req.Settings.TimeFormat != nil {
			tenant.Settings.TimeFormat = *req.Settings.TimeFormat
		}
		if req.Settings.Currency != nil {
			tenant.Settings.Currency = *req.Settings.Currency
		}
	}

	// Update the timestamp
	tenant.UpdatedAt = time.Now()

	err = uc.repo.Update(ctx, tenant)
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

package models

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// AdminUser represents the admin user information for tenant creation
type AdminUser struct {
	Email     string `json:"email" binding:"required,email"`
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Password  string `json:"password" binding:"required,min=8"`
}

// ThemeRequest represents the theme configuration for tenant creation
type ThemeRequest struct {
	PrimaryColor    string `json:"primaryColor" binding:"required"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
}

// AddressRequest represents the address information for tenant creation
type AddressRequest struct {
	Street     string `json:"street" binding:"required"`
	City       string `json:"city" binding:"required"`
	State      string `json:"state" binding:"required"`
	PostalCode string `json:"postalCode" binding:"required"`
	Country    string `json:"country" binding:"required"`
}

// TenantRequest define la estructura de la solicitud para crear un tenant
type TenantRequest struct {
	Name        string         `json:"name" binding:"required"`
	CompanyName string         `json:"companyName" binding:"required"`
	Subdomain   string         `json:"subdomain" binding:"required"`
	Description string         `json:"description"`
	AdminUser   AdminUser      `json:"adminUser" binding:"required"`
	Theme       ThemeRequest   `json:"theme" binding:"required"`
	Address     AddressRequest `json:"address" binding:"required"`
}

// Parse parses and validates the tenant creation request
func Parse(c *gin.Context) (*domain.Tenant, error) {
	var req TenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest(err.Error()))
		return nil, err
	}

	now := time.Now()

	// Convert request to domain tenant
	tenant := &domain.Tenant{
		ID:          uuid.New(),
		Name:        req.Name,
		CompanyName: req.CompanyName,
		Subdomain:   req.Subdomain,
		Description: req.Description,
		IsActive:    true,
		Theme: domain.Theme{
			PrimaryColor:    req.Theme.PrimaryColor,
			SecondaryColor:  req.Theme.SecondaryColor,
			AccentColor:     req.Theme.AccentColor,
			TextColor:       req.Theme.TextColor,
			BackgroundColor: req.Theme.BackgroundColor,
			LogoUrl:         "",
			FaviconUrl:      "/favicon.ico",
		},
		Address: domain.Address{
			Street:     req.Address.Street,
			City:       req.Address.City,
			State:      req.Address.State,
			PostalCode: req.Address.PostalCode,
			Country:    req.Address.Country,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return tenant, nil
}

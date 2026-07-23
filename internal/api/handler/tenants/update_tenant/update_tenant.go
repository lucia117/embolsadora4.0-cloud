package update_tenant

import (
	"log"
	"net/http"
	"net/mail"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant/models"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

var (
	validLocales     = []string{"es-AR", "es-ES", "en-US", "pt-BR"}
	validTimezones   = []string{"America/Argentina/Buenos_Aires", "America/Sao_Paulo", "America/Santiago", "America/Lima", "America/Bogota", "America/Mexico_City", "UTC"}
	validDateFormats = []string{"dd/MM/yyyy", "MM/dd/yyyy", "yyyy-MM-dd"}
	validTimeFormats = []string{"HH:mm", "hh:mm a"}
	validCurrencies  = []string{"ARS", "USD", "EUR", "BRL", "CLP", "MXN"}
)

func isOneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

type UpdateTenantHandler struct {
	useCase ucUpdateTenant.UseCase
}

func NewUpdateTenantHandler(useCase ucUpdateTenant.UseCase) *UpdateTenantHandler {
	return &UpdateTenantHandler{
		useCase: useCase,
	}
}

func (h *UpdateTenantHandler) UpdateTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("tenantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	role := security.RoleFromContext(c.Request.Context())
	if !security.IsCrossTenantRole(role) && !platform.TenantMatches(c.Request.Context(), id) {
		c.JSON(http.StatusForbidden, tenantserrors.ErrorResponse{Error: "FORBIDDEN", Message: "No tenés acceso a este tenant", Status: http.StatusForbidden})
		return
	}

	var req models.TenantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
		return
	}

	if req.ContactEmail != nil {
		if _, err := mail.ParseAddress(*req.ContactEmail); err != nil {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Email de contacto inválido", Status: http.StatusBadRequest})
			return
		}
	}

	if req.CompanyWebsite != nil && *req.CompanyWebsite != "" {
		if _, err := url.ParseRequestURI(*req.CompanyWebsite); err != nil {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Sitio web inválido", Status: http.StatusBadRequest})
			return
		}
	}

	if req.Settings != nil {
		if req.Settings.Locale != nil && !isOneOf(*req.Settings.Locale, validLocales) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Idioma inválido", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.Timezone != nil && !isOneOf(*req.Settings.Timezone, validTimezones) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Zona horaria inválida", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.DateFormat != nil && !isOneOf(*req.Settings.DateFormat, validDateFormats) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Formato de fecha inválido", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.TimeFormat != nil && !isOneOf(*req.Settings.TimeFormat, validTimeFormats) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Formato de hora inválido", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.Currency != nil && !isOneOf(*req.Settings.Currency, validCurrencies) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Moneda inválida", Status: http.StatusBadRequest})
			return
		}
	}

	// Convert request to usecase request
	useCaseReq := &ucUpdateTenant.UpdateTenantRequest{}

	// Only set fields that are provided in the request
	if req.Name != nil {
		useCaseReq.Name = req.Name
	}
	if req.CompanyName != nil {
		useCaseReq.CompanyName = req.CompanyName
	}
	if req.Subdomain != nil {
		useCaseReq.Subdomain = req.Subdomain
	}
	if req.Description != nil {
		useCaseReq.Description = req.Description
	}
	if req.IsActive != nil {
		useCaseReq.IsActive = req.IsActive
	}
	if req.ContactEmail != nil {
		useCaseReq.ContactEmail = req.ContactEmail
	}
	if req.CompanyWebsite != nil {
		useCaseReq.CompanyWebsite = req.CompanyWebsite
	}

	// Handle theme updates
	if req.Theme != nil {
		themeUpdate := &ucUpdateTenant.ThemeUpdate{}
		if req.Theme.PrimaryColor != nil {
			themeUpdate.PrimaryColor = req.Theme.PrimaryColor
		}
		if req.Theme.SecondaryColor != nil {
			themeUpdate.SecondaryColor = req.Theme.SecondaryColor
		}
		if req.Theme.AccentColor != nil {
			themeUpdate.AccentColor = req.Theme.AccentColor
		}
		if req.Theme.TextColor != nil {
			themeUpdate.TextColor = req.Theme.TextColor
		}
		if req.Theme.BackgroundColor != nil {
			themeUpdate.BackgroundColor = req.Theme.BackgroundColor
		}
		if req.Theme.LogoUrl != nil {
			themeUpdate.LogoUrl = req.Theme.LogoUrl
		}
		if req.Theme.FaviconUrl != nil {
			themeUpdate.FaviconUrl = req.Theme.FaviconUrl
		}
		useCaseReq.Theme = themeUpdate
	}

	// Handle address updates
	if req.Address != nil {
		addressUpdate := &ucUpdateTenant.AddressUpdate{}
		if req.Address.Street != nil {
			addressUpdate.Street = req.Address.Street
		}
		if req.Address.City != nil {
			addressUpdate.City = req.Address.City
		}
		if req.Address.State != nil {
			addressUpdate.State = req.Address.State
		}
		if req.Address.PostalCode != nil {
			addressUpdate.PostalCode = req.Address.PostalCode
		}
		if req.Address.Country != nil {
			addressUpdate.Country = req.Address.Country
		}
		useCaseReq.Address = addressUpdate
	}

	// Handle settings updates
	if req.Settings != nil {
		settingsUpdate := &ucUpdateTenant.SettingsUpdate{}
		if req.Settings.Locale != nil {
			settingsUpdate.Locale = req.Settings.Locale
		}
		if req.Settings.Timezone != nil {
			settingsUpdate.Timezone = req.Settings.Timezone
		}
		if req.Settings.DateFormat != nil {
			settingsUpdate.DateFormat = req.Settings.DateFormat
		}
		if req.Settings.TimeFormat != nil {
			settingsUpdate.TimeFormat = req.Settings.TimeFormat
		}
		if req.Settings.Currency != nil {
			settingsUpdate.Currency = req.Settings.Currency
		}
		useCaseReq.Settings = settingsUpdate
	}

	tenant, err := h.useCase.Update(c.Request.Context(), id, useCaseReq)
	if err != nil {
		if err == ucUpdateTenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, tenantserrors.ErrorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		log.Printf("error updating tenant: %v", err)
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Failed to update tenant", Status: http.StatusInternalServerError})
		return
	}

	response := models.FromDomain(tenant)
	c.JSON(http.StatusOK, response)
}

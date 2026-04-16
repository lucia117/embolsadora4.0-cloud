package update_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant/models"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
)

type UpdateTenantHandler struct {
	useCase ucUpdateTenant.UseCase
}

func NewUpdateTenantHandler(useCase ucUpdateTenant.UseCase) *UpdateTenantHandler {
	return &UpdateTenantHandler{
		useCase: useCase,
	}
}

func (h *UpdateTenantHandler) UpdateTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	var req models.TenantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
		return
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

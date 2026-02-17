package get_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/httperr"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	apperrors "github.com/tu-org/embolsadora-api/internal/core/errors"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

type GetTenantHandler struct {
	service *get_tenant.Service
}

func NewGetTenantHandler(repo tenants.TenantRepository) *GetTenantHandler {
	return &GetTenantHandler{
		service: get_tenant.NewService(repo),
	}
}

func (h *GetTenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.WriteError(c, apperrors.NewBadRequest("ID de tenant inválido"))
		return
	}

	tenant, err := h.service.Execute(c.Request.Context(), id)
	if err != nil {
		if err == get_tenant.ErrTenantNotFound {
			httperr.WriteError(c, apperrors.NewNotFound("Tenant no encontrado"))
			return
		}
		httperr.WriteError(c, apperrors.NewInternalServerError("Error al obtener tenant"))
		return
	}

	response := models.TenantResponse{
		ID:          tenant.ID.String(),
		Name:        tenant.Name,
		CompanyName: tenant.CompanyName,
		Subdomain:   tenant.Subdomain,
		Description: tenant.Description,
		IsActive:    tenant.IsActive,
		Theme: models.Theme{
			PrimaryColor:    tenant.Theme.PrimaryColor,
			SecondaryColor:  tenant.Theme.SecondaryColor,
			AccentColor:     tenant.Theme.AccentColor,
			TextColor:       tenant.Theme.TextColor,
			BackgroundColor: tenant.Theme.BackgroundColor,
			LogoUrl:         tenant.Theme.LogoUrl,
			FaviconUrl:      tenant.Theme.FaviconUrl,
		},
		Address: models.Address{
			Street:     tenant.Address.Street,
			City:       tenant.Address.City,
			State:      tenant.Address.State,
			PostalCode: tenant.Address.PostalCode,
			Country:    tenant.Address.Country,
		},
		CreatedAt: tenant.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt: tenant.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	c.JSON(http.StatusOK, response)
}

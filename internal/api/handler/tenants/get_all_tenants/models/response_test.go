package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_IncludesSettings(t *testing.T) {
	tenants := []domain.Tenant{
		{
			ID:   uuid.New(),
			Name: "Test",
			Settings: domain.TenantSettings{
				ContactEmail:   "contacto@test.com",
				CompanyWebsite: "https://test.com",
				Locale:         "es-ES",
				Timezone:       "America/Santiago",
				DateFormat:     "dd/MM/yyyy",
				TimeFormat:     "HH:mm",
				Currency:       "CLP",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	resp := FromDomain(tenants)

	assert.Len(t, resp, 1)
	assert.Equal(t, "contacto@test.com", resp[0].ContactEmail)
	assert.Equal(t, "https://test.com", resp[0].CompanyWebsite)
	assert.Equal(t, "es-ES", resp[0].Settings.Locale)
	assert.Equal(t, "America/Santiago", resp[0].Settings.Timezone)
	assert.Equal(t, "CLP", resp[0].Settings.Currency)
}

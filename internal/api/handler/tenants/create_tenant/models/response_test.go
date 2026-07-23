package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_IncludesSettings(t *testing.T) {
	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Name: "Test",
		Settings: domain.TenantSettings{
			ContactEmail:   "contacto@test.com",
			CompanyWebsite: "https://test.com",
			Locale:         "pt-BR",
			Timezone:       "America/Sao_Paulo",
			DateFormat:     "MM/dd/yyyy",
			TimeFormat:     "hh:mm a",
			Currency:       "BRL",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := FromDomain(tenant)

	assert.Equal(t, "contacto@test.com", resp.ContactEmail)
	assert.Equal(t, "https://test.com", resp.CompanyWebsite)
	assert.Equal(t, "pt-BR", resp.Settings.Locale)
	assert.Equal(t, "America/Sao_Paulo", resp.Settings.Timezone)
	assert.Equal(t, "MM/dd/yyyy", resp.Settings.DateFormat)
	assert.Equal(t, "hh:mm a", resp.Settings.TimeFormat)
	assert.Equal(t, "BRL", resp.Settings.Currency)
}

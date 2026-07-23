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
			Locale:         "en-US",
			Timezone:       "UTC",
			DateFormat:     "yyyy-MM-dd",
			TimeFormat:     "HH:mm",
			Currency:       "USD",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := FromDomain(tenant)

	assert.Equal(t, "contacto@test.com", resp.ContactEmail)
	assert.Equal(t, "https://test.com", resp.CompanyWebsite)
	assert.Equal(t, "en-US", resp.Settings.Locale)
	assert.Equal(t, "UTC", resp.Settings.Timezone)
	assert.Equal(t, "yyyy-MM-dd", resp.Settings.DateFormat)
	assert.Equal(t, "HH:mm", resp.Settings.TimeFormat)
	assert.Equal(t, "USD", resp.Settings.Currency)
}

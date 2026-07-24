package models

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SetsSettingsDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := TenantRequest{
		Name:        "New Tenant",
		CompanyName: "New Tenant Co",
		Subdomain:   "new-tenant",
		AdminUser: AdminUser{
			Email:     "admin@newtenant.com",
			FirstName: "Admin",
			LastName:  "User",
			Password:  "password123",
		},
		Theme: ThemeRequest{
			PrimaryColor: "#000000",
		},
		Address: AddressRequest{
			Street:     "Main St 1",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1001",
			Country:    "Argentina",
		},
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	httpReq, _ := http.NewRequest("POST", "/api/v1/tenants", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	tenant, err := Parse(c)
	require.NoError(t, err)

	assert.Equal(t, "es-AR", tenant.Settings.Locale)
	assert.Equal(t, "America/Argentina/Buenos_Aires", tenant.Settings.Timezone)
	assert.Equal(t, "dd/MM/yyyy", tenant.Settings.DateFormat)
	assert.Equal(t, "HH:mm", tenant.Settings.TimeFormat)
	assert.Equal(t, "ARS", tenant.Settings.Currency)
	assert.Equal(t, "", tenant.Settings.ContactEmail)
	assert.Equal(t, "", tenant.Settings.CompanyWebsite)
}

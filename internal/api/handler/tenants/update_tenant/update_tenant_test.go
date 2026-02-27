package update_tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant/models"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type mockRepo struct{}

func (m *mockRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	return nil
}

func (m *mockRepo) FindAll(ctx context.Context) ([]domain.Tenant, error) {
	return []domain.Tenant{}, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{
		ID:          id,
		Name:        "Demo Tenant",
		CompanyName: "Demo Company",
		Subdomain:   "demo",
		Description: "Demo tenant for testing purposes",
		IsActive:    true,
		Theme: domain.Theme{
			PrimaryColor:    "#3b82f6",
			SecondaryColor:  "#6366f1",
			AccentColor:     "#8b5cf6",
			TextColor:       "#1f2937",
			BackgroundColor: "#ffffff",
			LogoUrl:         "/logos/demo-logo.png",
			FaviconUrl:      "/favicon.ico",
		},
		Address: domain.Address{
			Street:     "123 Main St",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1001",
			Country:    "Argentina",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	return nil
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func TestUpdateTenantHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := &mockRepo{}
	useCase := ucUpdateTenant.NewUseCase(mockRepo)
	h := NewUpdateTenantHandler(useCase)
	r := gin.Default()
	r.PATCH("/api/tenants/:id", h.UpdateTenant)

	id := uuid.New().String()
	updateReq := models.TenantUpdateRequest{
		Name:        ptrString("Updated Tenant Name"),
		Description: ptrString("Updated description"),
		IsActive:    ptrBool(true),
		Theme: &models.ThemeUpdate{
			PrimaryColor: ptrString("#4f46e5"),
		},
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PATCH", "/api/tenants/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.TenantResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Tenant Name", resp.Name)
	assert.Equal(t, "Updated description", resp.Description)
	assert.Equal(t, true, resp.IsActive)
	assert.Equal(t, "#4f46e5", resp.Theme.PrimaryColor)
}

func TestUpdateTenantHandler_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockRepo := &mockRepo{}
	useCase := ucUpdateTenant.NewUseCase(mockRepo)
	h := NewUpdateTenantHandler(useCase)
	r := gin.Default()
	r.PATCH("/api/tenants/:id", h.UpdateTenant)

	updateReq := models.TenantUpdateRequest{
		Name: ptrString("Updated Tenant Name"),
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PATCH", "/api/tenants/invalid-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func ptrString(s string) *string { return &s }
func ptrBool(b bool) *bool       { return &b }

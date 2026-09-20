package update_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.Tenant
	findByIDErr    error

	updateErr   error
	updateCalls []*domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)   { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(_ context.Context, tenant *domain.Tenant) error {
	f.updateCalls = append(f.updateCalls, tenant)
	return f.updateErr
}
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func baseTenant() *domain.Tenant {
	return &domain.Tenant{
		ID: uuid.New(), Name: "Vieja", CompanyName: "Vieja SA", Subdomain: "vieja", Description: "d", IsActive: true,
		Theme:    domain.Theme{PrimaryColor: "#000000", LogoUrl: "http://old-logo"},
		Address:  domain.Address{Street: "Calle Vieja 123", City: "CABA"},
		Settings: domain.TenantSettings{ContactEmail: "old@x.com", Locale: "es-AR"},
	}
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func TestUpdate_TenantNoEncontrado(t *testing.T) {
	repo := &fakeRepo{findByIDErr: nil, findByIDResult: nil}
	uc := NewUseCase(repo)

	_, err := uc.Update(context.Background(), uuid.New(), &UpdateTenantRequest{})

	assert.ErrorIs(t, err, ErrTenantNotFound)
	assert.Empty(t, repo.updateCalls)
}

func TestUpdate_ErrorDeFindByIDPropaga(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRepo{findByIDErr: dbErr}
	uc := NewUseCase(repo)

	_, err := uc.Update(context.Background(), uuid.New(), &UpdateTenantRequest{})

	assert.ErrorIs(t, err, dbErr)
}

func TestUpdate_TodosLosCamposNilNoCambiaNadaExceptoUpdatedAt(t *testing.T) {
	original := baseTenant()
	before := *original // copia por valor para comparar después
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{})

	require.NoError(t, err)
	assert.Equal(t, before.Name, got.Name)
	assert.Equal(t, before.CompanyName, got.CompanyName)
	assert.Equal(t, before.Theme, got.Theme)
	assert.Equal(t, before.Address, got.Address)
	assert.Equal(t, before.Settings, got.Settings)
	assert.True(t, got.UpdatedAt.After(before.UpdatedAt) || !got.UpdatedAt.IsZero(), "UpdatedAt siempre se refresca")
}

func TestUpdate_ErrorDeRepoUpdatePropaga(t *testing.T) {
	repo := &fakeRepo{findByIDResult: baseTenant(), updateErr: errors.New("constraint violation")}
	uc := NewUseCase(repo)

	_, err := uc.Update(context.Background(), uuid.New(), &UpdateTenantRequest{Name: strp("Nueva")})

	assert.Error(t, err)
}

func TestUpdate_SubconjuntoDeCamposTopLevel(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Name:     strp("Nueva"),
		IsActive: boolp(false),
	})

	require.NoError(t, err)
	assert.Equal(t, "Nueva", got.Name)
	assert.False(t, got.IsActive)
	assert.Equal(t, "Vieja SA", got.CompanyName, "campo no enviado no cambia")
	assert.Equal(t, "vieja", got.Subdomain, "campo no enviado no cambia")
}

func TestUpdate_ThemeParcial(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Theme: &ThemeUpdate{PrimaryColor: strp("#FFFFFF")},
	})

	require.NoError(t, err)
	assert.Equal(t, "#FFFFFF", got.Theme.PrimaryColor)
	assert.Equal(t, "http://old-logo", got.Theme.LogoUrl, "campo de Theme no enviado no cambia")
}

func TestUpdate_AddressParcial(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Address: &AddressUpdate{City: strp("Rosario")},
	})

	require.NoError(t, err)
	assert.Equal(t, "Rosario", got.Address.City)
	assert.Equal(t, "Calle Vieja 123", got.Address.Street, "campo de Address no enviado no cambia")
}

func TestUpdate_ContactEmailYCompanyWebsiteEscribenEnSettings(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		ContactEmail:   strp("new@x.com"),
		CompanyWebsite: strp("https://new.example.com"),
	})

	require.NoError(t, err)
	assert.Equal(t, "new@x.com", got.Settings.ContactEmail)
	assert.Equal(t, "https://new.example.com", got.Settings.CompanyWebsite)
	assert.Equal(t, "es-AR", got.Settings.Locale, "el resto de Settings no cambia")
}

func TestUpdate_SettingsParcial(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Settings: &SettingsUpdate{Locale: strp("en-US"), Currency: strp("USD")},
	})

	require.NoError(t, err)
	assert.Equal(t, "en-US", got.Settings.Locale)
	assert.Equal(t, "USD", got.Settings.Currency)
	assert.Equal(t, "old@x.com", got.Settings.ContactEmail, "campo de Settings no enviado no cambia")
}

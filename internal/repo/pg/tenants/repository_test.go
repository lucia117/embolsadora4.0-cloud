package tenants_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

func newTestTenant() *domain.Tenant {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Tenant{
		ID:          uuid.New(),
		Name:        "Repo Test Tenant",
		CompanyName: "Repo Test Co",
		Subdomain:   "repo-test-" + uuid.NewString()[:8],
		Description: "created by repository_test.go",
		IsActive:    true,
		Theme: domain.Theme{
			PrimaryColor:    "#111111",
			SecondaryColor:  "#222222",
			AccentColor:     "#333333",
			TextColor:       "#444444",
			BackgroundColor: "#555555",
			LogoUrl:         "",
			FaviconUrl:      "",
		},
		Address: domain.Address{
			Street:     "Test St 123",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1001",
			Country:    "Argentina",
		},
		Settings: domain.TenantSettings{
			ContactEmail:   "contacto@repotest.com",
			CompanyWebsite: "https://repotest.com",
			Locale:         "es-AR",
			Timezone:       "America/Argentina/Buenos_Aires",
			DateFormat:     "dd/MM/yyyy",
			TimeFormat:     "HH:mm",
			Currency:       "ARS",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TestSettings_RoundTrip verifies TenantSettings fields survive Create -> FindByID.
func TestSettings_RoundTrip(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	ctx := context.Background()

	tenant := newTestTenant()
	require.NoError(t, repo.Create(ctx, tenant))
	defer func() { _ = repo.Delete(ctx, tenant.ID) }()

	found, err := repo.FindByID(ctx, tenant.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, tenant.Settings, found.Settings)
}

// TestSettings_Update verifies TenantSettings fields survive Update -> FindByID.
func TestSettings_Update(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	ctx := context.Background()

	tenant := newTestTenant()
	require.NoError(t, repo.Create(ctx, tenant))
	defer func() { _ = repo.Delete(ctx, tenant.ID) }()

	tenant.Settings = domain.TenantSettings{
		ContactEmail:   "nuevo@repotest.com",
		CompanyWebsite: "https://nuevo.repotest.com",
		Locale:         "en-US",
		Timezone:       "UTC",
		DateFormat:     "yyyy-MM-dd",
		TimeFormat:     "hh:mm a",
		Currency:       "USD",
	}
	tenant.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.Update(ctx, tenant))

	found, err := repo.FindByID(ctx, tenant.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, tenant.Settings, found.Settings)
}

func TestFindBySubdomain_RoundTrip(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	ctx := context.Background()

	tenant := newTestTenant()
	require.NoError(t, repo.Create(ctx, tenant))
	defer func() { _ = repo.Delete(ctx, tenant.ID) }()

	found, err := repo.FindBySubdomain(ctx, tenant.Subdomain)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tenant.ID, found.ID)
	assert.Equal(t, tenant.Subdomain, found.Subdomain)
}

func TestFindBySubdomain_NotFound(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	found, err := repo.FindBySubdomain(context.Background(), "no-such-subdomain-"+uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, found)
}

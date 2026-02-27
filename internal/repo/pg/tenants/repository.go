package tenants

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// TenantRepository define la interfaz para el repositorio de tenants
type TenantRepository interface {
	Create(ctx context.Context, tenant *domain.Tenant) error
	FindAll(ctx context.Context) ([]domain.Tenant, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	Update(ctx context.Context, tenant *domain.Tenant) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type tenantRepository struct {
	db *pgxpool.Pool
}

// NewTenantRepository crea una nueva instancia del repositorio de tenants
func NewTenantRepository(db *pgxpool.Pool) TenantRepository {
	return &tenantRepository{db: db}
}
func (r *tenantRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var tenant domain.Tenant
	var theme domain.Theme
	var address domain.Address
	var tenantID uuid.UUID

	var description, logoUrl, faviconUrl *string
	var street, city, state, postalCode, country *string

	err := r.db.QueryRow(ctx, FindByIDQuery, id).Scan(
		&tenantID, &tenant.Name, &tenant.CompanyName, &tenant.Subdomain, &description, &tenant.IsActive,
		&theme.PrimaryColor, &theme.SecondaryColor, &theme.AccentColor, &theme.TextColor, &theme.BackgroundColor, &logoUrl, &faviconUrl,
		&street, &city, &state, &postalCode, &country,
		&tenant.CreatedAt, &tenant.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	tenant.ID = tenantID
	tenant.Description = derefString(description)
	theme.LogoUrl = derefString(logoUrl)
	theme.FaviconUrl = derefString(faviconUrl)
	tenant.Theme = theme
	address.Street = derefString(street)
	address.City = derefString(city)
	address.State = derefString(state)
	address.PostalCode = derefString(postalCode)
	address.Country = derefString(country)
	tenant.Address = address
	return &tenant, nil
}

func (r *tenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	_, err := r.db.Exec(ctx, CreateQuery,
		tenant.ID, tenant.Name, tenant.CompanyName, tenant.Subdomain, tenant.Description, tenant.IsActive,
		tenant.Theme.PrimaryColor, tenant.Theme.SecondaryColor, tenant.Theme.AccentColor, tenant.Theme.TextColor, tenant.Theme.BackgroundColor, tenant.Theme.LogoUrl, tenant.Theme.FaviconUrl,
		tenant.Address.Street, tenant.Address.City, tenant.Address.State, tenant.Address.PostalCode, tenant.Address.Country,
		tenant.CreatedAt, tenant.UpdatedAt,
	)
	return err
}

func (r *tenantRepository) FindAll(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := r.db.Query(ctx, FindAllQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		var tenant domain.Tenant
		var theme domain.Theme
		var address domain.Address

		var description, logoUrl, faviconUrl *string
		var street, city, state, postalCode, country *string

		err := rows.Scan(
			&tenant.ID, &tenant.Name, &tenant.CompanyName, &tenant.Subdomain, &description, &tenant.IsActive,
			&theme.PrimaryColor, &theme.SecondaryColor, &theme.AccentColor, &theme.TextColor, &theme.BackgroundColor, &logoUrl, &faviconUrl,
			&street, &city, &state, &postalCode, &country,
			&tenant.CreatedAt, &tenant.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		tenant.Description = derefString(description)
		theme.LogoUrl = derefString(logoUrl)
		theme.FaviconUrl = derefString(faviconUrl)
		tenant.Theme = theme
		address.Street = derefString(street)
		address.City = derefString(city)
		address.State = derefString(state)
		address.PostalCode = derefString(postalCode)
		address.Country = derefString(country)
		tenant.Address = address
		tenants = append(tenants, tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tenants, nil
}

func (r *tenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	_, err := r.db.Exec(ctx, UpdateQuery,
		tenant.Name, tenant.CompanyName, tenant.Subdomain, tenant.Description, tenant.IsActive,
		tenant.Theme.PrimaryColor, tenant.Theme.SecondaryColor, tenant.Theme.AccentColor, tenant.Theme.TextColor, tenant.Theme.BackgroundColor, tenant.Theme.LogoUrl, tenant.Theme.FaviconUrl,
		tenant.Address.Street, tenant.Address.City, tenant.Address.State, tenant.Address.PostalCode, tenant.Address.Country,
		tenant.UpdatedAt,
		tenant.ID,
	)
	return err
}

func (r *tenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, DeleteQuery, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// derefString convierte un *string nullable a string, retornando "" si es nil
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

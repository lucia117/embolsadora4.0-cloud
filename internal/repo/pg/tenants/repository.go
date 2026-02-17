package tenants

import (
	"context"

	"github.com/google/uuid"
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

func (r *tenantRepository) buildNamedQuery(query string, params map[string]interface{}) (string, []interface{}, error) {
	var args []interface{}
	for _, param := range params {
		args = append(args, param)
	}
	return query, args, nil
}

func (r *tenantRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var tenant domain.Tenant
	var theme domain.Theme
	var address domain.Address

	// Create parameter map for named query
	params := map[string]interface{}{
		"id": id,
	}

	// Convert named query to positional query
	query, args, err := r.buildNamedQuery(FindByIDQuery, params)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRow(ctx, query, args...).Scan(
		&tenant.ID, &tenant.Name, &tenant.CompanyName, &tenant.Subdomain, &tenant.Description, &tenant.IsActive,
		&theme.PrimaryColor, &theme.SecondaryColor, &theme.AccentColor, &theme.TextColor, &theme.BackgroundColor, &theme.LogoUrl, &theme.FaviconUrl,
		&address.Street, &address.City, &address.State, &address.PostalCode, &address.Country,
		&tenant.CreatedAt, &tenant.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	tenant.Theme = theme
	tenant.Address = address

	return &tenant, nil
}

func (r *tenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	params := map[string]interface{}{
		"id":               tenant.ID,
		"name":             tenant.Name,
		"company_name":     tenant.CompanyName,
		"subdomain":        tenant.Subdomain,
		"description":      tenant.Description,
		"is_active":        tenant.IsActive,
		"primary_color":    tenant.Theme.PrimaryColor,
		"secondary_color":  tenant.Theme.SecondaryColor,
		"accent_color":     tenant.Theme.AccentColor,
		"text_color":       tenant.Theme.TextColor,
		"background_color": tenant.Theme.BackgroundColor,
		"logo_url":         tenant.Theme.LogoUrl,
		"favicon_url":      tenant.Theme.FaviconUrl,
		"street":           tenant.Address.Street,
		"city":             tenant.Address.City,
		"state":            tenant.Address.State,
		"postal_code":      tenant.Address.PostalCode,
		"country":          tenant.Address.Country,
		"created_at":       tenant.CreatedAt,
		"updated_at":       tenant.UpdatedAt,
	}

	// Convert named query to positional query
	query, args, err := r.buildNamedQuery(CreateQuery, params)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query, args...)
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

		err := rows.Scan(
			&tenant.ID, &tenant.Name, &tenant.CompanyName, &tenant.Subdomain, &tenant.Description, &tenant.IsActive,
			&theme.PrimaryColor, &theme.SecondaryColor, &theme.AccentColor, &theme.TextColor, &theme.BackgroundColor, &theme.LogoUrl, &theme.FaviconUrl,
			&address.Street, &address.City, &address.State, &address.PostalCode, &address.Country,
			&tenant.CreatedAt, &tenant.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		tenant.Theme = theme
		tenant.Address = address
		tenants = append(tenants, tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tenants, nil
}

func (r *tenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	params := map[string]interface{}{
		"id":               tenant.ID,
		"name":             tenant.Name,
		"company_name":     tenant.CompanyName,
		"subdomain":        tenant.Subdomain,
		"description":      tenant.Description,
		"is_active":        tenant.IsActive,
		"primary_color":    tenant.Theme.PrimaryColor,
		"secondary_color":  tenant.Theme.SecondaryColor,
		"accent_color":     tenant.Theme.AccentColor,
		"text_color":       tenant.Theme.TextColor,
		"background_color": tenant.Theme.BackgroundColor,
		"logo_url":         tenant.Theme.LogoUrl,
		"favicon_url":      tenant.Theme.FaviconUrl,
		"street":           tenant.Address.Street,
		"city":             tenant.Address.City,
		"state":            tenant.Address.State,
		"postal_code":      tenant.Address.PostalCode,
		"country":          tenant.Address.Country,
		"updated_at":       tenant.UpdatedAt,
	}

	// Convert named query to positional query
	query, args, err := r.buildNamedQuery(UpdateQuery, params)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query, args...)
	return err
}

func (r *tenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	params := map[string]interface{}{
		"id": id,
	}

	// Convert named query to positional query
	query, args, err := r.buildNamedQuery(DeleteQuery, params)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query, args...)
	return err
}

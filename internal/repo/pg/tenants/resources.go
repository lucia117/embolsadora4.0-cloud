package tenants

const (
	// FindByIDQuery retrieves a tenant by ID with all related data
	FindByIDQuery = `
		SELECT 
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			created_at, updated_at
		FROM tenants 
		WHERE id = $1
	`

	// CreateQuery inserts a new tenant with all fields
	CreateQuery = `
		INSERT INTO tenants (
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20
		)
	`

	// FindAllQuery retrieves all tenants ordered by creation date
	FindAllQuery = `
		SELECT 
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			created_at, updated_at
		FROM tenants 
		ORDER BY created_at DESC
	`

	// UpdateQuery updates an existing tenant by ID
	UpdateQuery = `
		UPDATE tenants SET
			name = $1, company_name = $2, subdomain = $3, description = $4, is_active = $5,
			primary_color = $6, secondary_color = $7, accent_color = $8, text_color = $9, background_color = $10, logo_url = $11, favicon_url = $12,
			street = $13, city = $14, state = $15, postal_code = $16, country = $17,
			updated_at = $18
		WHERE id = $19
	`

	// DeleteQuery removes a tenant by ID
	DeleteQuery = `DELETE FROM tenants WHERE id = $1`
)

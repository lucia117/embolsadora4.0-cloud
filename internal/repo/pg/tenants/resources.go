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
		WHERE id = $id
	`

	// CreateQuery inserts a new tenant with all fields
	CreateQuery = `
		INSERT INTO tenants (
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			created_at, updated_at
		) VALUES (
			$id, $name, $company_name, $subdomain, $description, $is_active,
			$primary_color, $secondary_color, $accent_color, $text_color, $background_color, $logo_url, $favicon_url,
			$street, $city, $state, $postal_code, $country,
			$created_at, $updated_at
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
			name = $name, company_name = $company_name, subdomain = $subdomain, description = $description, is_active = $is_active,
			primary_color = $primary_color, secondary_color = $secondary_color, accent_color = $accent_color, text_color = $text_color, background_color = $background_color, logo_url = $logo_url, favicon_url = $favicon_url,
			street = $street, city = $city, state = $state, postal_code = $postal_code, country = $country,
			updated_at = $updated_at
		WHERE id = $id
	`

	// DeleteQuery removes a tenant by ID
	DeleteQuery = `DELETE FROM tenants WHERE id = $id`
)

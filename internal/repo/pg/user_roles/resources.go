package user_roles

const (
	// FindByTenantQuery retrieves all UTR assignments for a tenant, ordered by creation date.
	FindByTenantQuery = `
		SELECT id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
		FROM user_tenant_roles
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	// FindByTenantWithStatusQuery retrieves UTR assignments for a tenant filtered by status.
	FindByTenantWithStatusQuery = `
		SELECT id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
		FROM user_tenant_roles
		WHERE tenant_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	// CreateQuery inserts a new UTR assignment and returns the full created row.
	CreateQuery = `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_by, assigned_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
	`

	// FindByIDQuery retrieves a single UTR assignment by its UUID.
	FindByIDQuery = `
		SELECT id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
		FROM user_tenant_roles
		WHERE id = $1
	`

	// UpdateQuery updates the role_id of an existing assignment and returns the updated row.
	UpdateQuery = `
		UPDATE user_tenant_roles
		SET role_id = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
	`

	// RevokeQuery soft-deletes an assignment by setting status to 'revoked'.
	RevokeQuery = `
		UPDATE user_tenant_roles
		SET status = 'revoked', updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
	`

	// FindByUserQuery retrieves all UTR assignments for a user across all tenants,
	// joining tenants and roles tables to include display names.
	FindByUserQuery = `
		SELECT utr.tenant_id, t.name, utr.role_id, COALESCE(r.name, utr.role_id), utr.status
		FROM user_tenant_roles utr
		JOIN tenants t ON t.id = utr.tenant_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.user_id = $1
		ORDER BY utr.created_at DESC
	`
)

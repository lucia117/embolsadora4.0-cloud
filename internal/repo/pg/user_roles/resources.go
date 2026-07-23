package user_roles

const (
	// FindByTenantQuery retrieves all UTR assignments for a tenant, ordered by creation date.
	// Joins users on the real membership relation (user_id, not users.tenant_id — see
	// docs/superpowers/specs/2026-07-21-tenant-user-roles-enrichment-design.md) so
	// auto-provisioned users (users.tenant_id IS NULL) resolve correctly. Joins roles
	// (LEFT, since role_id is nullable for pending assignments) for the display name.
	FindByTenantQuery = `
		SELECT utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at,
		       COALESCE(r.name, '') AS role_name,
		       u.email AS user_email,
		       u.name AS user_name,
		       u.first_name AS user_first_name,
		       u.last_name AS user_last_name
		FROM user_tenant_roles utr
		JOIN users u ON u.id = utr.user_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.tenant_id = $1
		ORDER BY utr.created_at DESC
	`

	// FindByTenantWithStatusQuery retrieves UTR assignments for a tenant filtered by status.
	// Same JOINs as FindByTenantQuery — see comment there.
	FindByTenantWithStatusQuery = `
		SELECT utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at,
		       COALESCE(r.name, '') AS role_name,
		       u.email AS user_email,
		       u.name AS user_name,
		       u.first_name AS user_first_name,
		       u.last_name AS user_last_name
		FROM user_tenant_roles utr
		JOIN users u ON u.id = utr.user_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.tenant_id = $1 AND utr.status = $2
		ORDER BY utr.created_at DESC
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
	// Scoped by tenant_id so the repository enforces tenant ownership itself,
	// not only whichever usecase happens to call it.
	RevokeQuery = `
		UPDATE user_tenant_roles
		SET status = 'revoked', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
	`

	// UpdateStatusQuery changes the status of a UTR by (user_id, tenant_id).
	// Does not affect 'pending' assignments (those are managed via invitation flow).
	UpdateStatusQuery = `
		UPDATE user_tenant_roles
		SET status = $1, updated_at = NOW()
		WHERE user_id = $2 AND tenant_id = $3 AND status != 'pending'
		RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
	`

	// FindByUserQuery retrieves all UTR assignments for a user across all tenants,
	// joining tenants and roles tables to include display names.
	FindByUserQuery = `
		SELECT utr.tenant_id, t.name, utr.role_id, COALESCE(r.name, utr.role_id, ''), utr.status
		FROM user_tenant_roles utr
		JOIN tenants t ON t.id = utr.tenant_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.user_id = $1
		ORDER BY utr.created_at DESC
	`
)

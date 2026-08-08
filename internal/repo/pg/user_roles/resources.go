package user_roles

// Cloaking en este dominio (ver docs/superpowers/specs/2026-08-04-platform-operator-rbac-design.md §2):
// una asignación cuyo rol es is_global (super_admin, tenant_manager) tiene que ser
// indistinguible de inexistente para un caller que no sea super_admin. El predicado
// viaja siempre como parámetro booleano explícito (includeGlobal) decidido por el
// handler con security.CanSeePlatformInternals — los repos no leen el contexto.
//
// La forma del predicado es la misma en todas las consultas de acá y en las de
// users/invitations: `(COALESCE(r.is_global, FALSE) = FALSE OR $n)`. El COALESCE
// cubre el LEFT JOIN sin fila (role_id NULL en las asignaciones pending), que no
// debe ocultarse.
const (
	// FindByTenantQuery retrieves all UTR assignments for a tenant, ordered by creation date.
	// Joins users on the real membership relation (user_id, not users.tenant_id — see
	// docs/superpowers/specs/2026-07-21-tenant-user-roles-enrichment-design.md) so
	// auto-provisioned users (users.tenant_id IS NULL) resolve correctly. Joins roles
	// (LEFT, since role_id is nullable for pending assignments) for the display name.
	//
	// $2 = includeGlobal: sin él esta consulta devolvía la fila del super_admin con
	// role_id, role_name, user_email y user_name a cualquier caller con users:read.
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
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)
		ORDER BY utr.created_at DESC
	`

	// FindByTenantWithStatusQuery retrieves UTR assignments for a tenant filtered by status.
	// Same JOINs y mismo predicado de cloaking que FindByTenantQuery — ver comentario allá.
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
		WHERE utr.tenant_id = $1
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)
		  AND utr.status = $3
		ORDER BY utr.created_at DESC
	`

	// CreateQuery inserts a new UTR assignment and returns the full created row.
	//
	// $8 = includeGlobal. Este es el único predicado del dominio que cloakea por
	// IDENTIDAD y no por fila, y la razón es que acá la fila la escribe el caller:
	// el resto de las consultas mira `r.is_global` de una fila que ya existe, pero
	// en un INSERT el rol lo elige el atacante. Un admin de un tenant cliente que
	// consiguiera el uuid del super_admin hacía POST /user-roles con roleId
	// 'operario' contra su propio tenant: el rol pedido no es global (pasa
	// checkRoleAllowedForTenant), no hay membresía activa previa ahí (no choca
	// contra idx_utr_active_unique, así que resolveActiveUniqueViolation ni se
	// dispara) y la UTR quedaba escrita — y visible en GET /user-roles con
	// user_email y user_name reales. Des-anonimización completa y persistente.
	//
	// El NOT EXISTS define "identidad de plataforma": el usuario destino tiene una
	// membresía NO REVOCADA a un rol is_global en ALGÚN tenant. Sin fijar tenant_id
	// a propósito — el atacante escribe desde su tenant, donde el superadmin no
	// tiene ninguna membresía, así que un lookup acotado al tenant del request no
	// vería nada. El corte en `<> 'revoked'` incluye pending y suspended (un
	// super_admin invitado y sin activar ya está cloakeado en ListPendingUsers y en
	// invitations) y excluye revoked, para no bloquear para siempre la degradación
	// legítima de un ex superadmin.
	//
	// El guard va en el INSERT y no en un precheck en Go por dos motivos: no hay
	// ventana TOCTOU entre chequear y escribir, y BulkCreate comparte esta misma
	// consulta dentro de su transacción sin código extra. Cero filas insertadas →
	// pgx.ErrNoRows en el RETURNING → ErrInvalidUserID, byte-idéntico a lo que
	// devuelve la violación de FK de un user_id inexistente (ver Create).
	CreateQuery = `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_by, assigned_at)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE $8 OR NOT EXISTS (
			SELECT 1
			FROM user_tenant_roles otra
			JOIN roles rg ON rg.id = otra.role_id
			WHERE otra.user_id = $2
			  AND otra.status <> 'revoked'
			  AND COALESCE(rg.is_global, FALSE) = TRUE
		)
		RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
	`

	// FindByIDQuery retrieves a single UTR assignment by its UUID.
	// $2 = includeGlobal: una asignación a rol global devuelve cero filas, que los
	// usecases traducen en ErrAssignmentNotFound → 404, el mismo resultado que un id
	// inexistente. Es lo que impide que DELETE/PUT /user-roles/:id operen sobre la
	// membresía del superadmin usando el id que la lista ya no muestra.
	FindByIDQuery = `
		SELECT utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at
		FROM user_tenant_roles utr
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.id = $1
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $2)
	`

	// UpdateQuery updates the role_id of an existing assignment and returns the updated row.
	// $3 = includeGlobal sobre el rol ACTUAL de la fila (el subselect lee el valor viejo,
	// no el nuevo): el repo se niega a mutar una asignación oculta aunque el precheck del
	// usecase se saltee o se reordene. El rol NUEVO lo valida el usecase contra
	// roles.GetByIDForTenant antes de llegar acá.
	//
	// El segundo predicado es el mismo guard de identidad de CreateQuery — ver el
	// comentario largo allá. Cierra el eje que el cloaking por fila no cubre: una
	// UTR con rol NO global (que el predicado de arriba deja pasar) cuyo dueño es
	// una cuenta de plataforma. Es la fila que el superadmin legítimo puede crear,
	// y que un admin de tenant no debería poder re-rolear. user_id sale de la fila
	// (utr.user_id), no de lo que mandó el caller: el guard no depende de que el
	// usecase haya poblado bien la struct. Cero filas → nil, nil →
	// ErrAssignmentNotFound → 404, igual que un id inexistente.
	UpdateQuery = `
		UPDATE user_tenant_roles utr
		SET role_id = $1, updated_at = NOW()
		WHERE utr.id = $2
		  AND (COALESCE((SELECT r.is_global FROM roles r WHERE r.id = utr.role_id), FALSE) = FALSE OR $3)
		  AND ($3 OR NOT EXISTS (
			SELECT 1
			FROM user_tenant_roles otra
			JOIN roles rg ON rg.id = otra.role_id
			WHERE otra.user_id = utr.user_id
			  AND otra.status <> 'revoked'
			  AND COALESCE(rg.is_global, FALSE) = TRUE
		  ))
		RETURNING utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at
	`

	// RevokeQuery soft-deletes an assignment by setting status to 'revoked'.
	// Scoped by tenant_id so the repository enforces tenant ownership itself,
	// not only whichever usecase happens to call it. $3 = includeGlobal, por el
	// mismo motivo: revocar la membresía del superadmin es una mutación destructiva
	// sobre algo que para este caller no existe, y el efecto sería observable.
	RevokeQuery = `
		UPDATE user_tenant_roles utr
		SET status = 'revoked', updated_at = NOW()
		WHERE utr.id = $1 AND utr.tenant_id = $2
		  AND (COALESCE((SELECT r.is_global FROM roles r WHERE r.id = utr.role_id), FALSE) = FALSE OR $3)
		RETURNING utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at
	`

	// UpdateStatusQuery changes the status of a UTR by (user_id, tenant_id).
	// Does not affect 'pending' assignments (those are managed via invitation flow).
	// $4 = includeGlobal: mismo criterio que RevokeQuery — suspender al superadmin
	// desde PATCH /users/:id/status sería un efecto observable sobre un invisible.
	UpdateStatusQuery = `
		UPDATE user_tenant_roles utr
		SET status = $1, updated_at = NOW()
		WHERE utr.user_id = $2 AND utr.tenant_id = $3 AND utr.status != 'pending'
		  AND (COALESCE((SELECT r.is_global FROM roles r WHERE r.id = utr.role_id), FALSE) = FALSE OR $4)
		RETURNING utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at
	`

	// FindByUserQuery retrieves the UTR assignments for a user, joining tenants and
	// roles for display names.
	//
	// $2/$3 = tenantID + crossTenant: la consulta era global (todos los tenants del
	// usuario) sin ningún scope, así que GET /users/:id/roles le contaba a cualquiera
	// a qué tenants pertenece un usuario. Ahora solo los callers con rol cross-tenant
	// (super_admin, tenant_manager, platform_admin) ven más allá del tenant del request.
	// $4 = includeGlobal, mismo cloaking que el resto del dominio.
	FindByUserQuery = `
		SELECT utr.tenant_id, t.name, utr.role_id, COALESCE(r.name, utr.role_id, ''), utr.status
		FROM user_tenant_roles utr
		JOIN tenants t ON t.id = utr.tenant_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.user_id = $1
		  AND ($3 OR utr.tenant_id = $2)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $4)
		ORDER BY utr.created_at DESC
	`

	// HasVisibleActiveAssignmentQuery responde si la asignación activa que ya tiene
	// el usuario en el tenant es VISIBLE para este caller. Se usa solo cuando el
	// INSERT ya chocó contra idx_utr_active_unique, para decidir qué error devolver
	// sin filtrar la existencia de una membresía oculta (ver Create).
	HasVisibleActiveAssignmentQuery = `
		SELECT EXISTS (
			SELECT 1
			FROM user_tenant_roles utr
			LEFT JOIN roles r ON r.id = utr.role_id
			WHERE utr.user_id = $1
			  AND utr.tenant_id = $2
			  AND utr.status = 'active'
			  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
		)
	`
)

package roles_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// seedUser crea un usuario minimo descartable, sin tenant ni rol — alcanza
// como FK target de user_tenant_roles.user_id para los tests de
// CountActiveAssignments.
func seedUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, status) VALUES ($1, $2, 'active')`,
		id, id.String()+"@roles-crud.local")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func newCustomRole(tenantID uuid.UUID, name string) *domain.Role {
	return &domain.Role{
		ID:          "custom_" + uuid.NewString()[:8],
		Name:        name,
		Description: "rol de test",
		Permissions: []string{},
		TenantID:    &tenantID,
	}
}

func TestCreateDuplicateNameEnElMismoTenant(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	first := newCustomRole(tenantID, "Rol duplicado")
	require.NoError(t, repo.Create(ctx, first))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, first.ID) })

	dup := newCustomRole(tenantID, "Rol duplicado")
	err := repo.Create(ctx, dup)
	assert.ErrorIs(t, err, domain.ErrRoleDuplicateName)
}

func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	role := newCustomRole(tenantID, "Original")
	require.NoError(t, repo.Create(ctx, role))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, role.ID) })

	role.Name = "Renombrado"
	role.Description = "nueva descripcion"
	role.Permissions = []string{"perm_users_view"}
	require.NoError(t, repo.Update(ctx, role))

	got, err := repo.GetByIDForTenant(ctx, role.ID, tenantID, false)
	require.NoError(t, err)
	assert.Equal(t, "Renombrado", got.Name)
	assert.Equal(t, "nueva descripcion", got.Description)
	assert.Equal(t, []string{"perm_users_view"}, got.Permissions)

	ghost := newCustomRole(tenantID, "fantasma")
	ghost.ID = "no-existe-" + uuid.NewString()[:8]
	err = repo.Update(ctx, ghost)
	assert.ErrorIs(t, err, domain.ErrRoleNotFound)
}

func TestUpdateNombreDuplicado(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	roleA := newCustomRole(tenantID, "Rol A")
	require.NoError(t, repo.Create(ctx, roleA))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA.ID) })

	roleB := newCustomRole(tenantID, "Rol B")
	require.NoError(t, repo.Create(ctx, roleB))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleB.ID) })

	roleB.Name = "Rol A"
	err := repo.Update(ctx, roleB)
	assert.ErrorIs(t, err, domain.ErrRoleDuplicateName)
}

func TestSoftDeleteHappyPathAndNotFound(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	role := newCustomRole(tenantID, "Para borrar")
	require.NoError(t, repo.Create(ctx, role))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, role.ID) })

	require.NoError(t, repo.SoftDelete(ctx, role.ID))

	_, err := repo.GetByIDForTenant(ctx, role.ID, tenantID, false)
	assert.ErrorIs(t, err, domain.ErrRoleNotFound, "un rol soft-deleted debe dejar de ser visible")

	err = repo.SoftDelete(ctx, role.ID)
	assert.ErrorIs(t, err, domain.ErrRoleNotFound, "borrar de nuevo un rol ya borrado debe fallar, no ser idempotente")
}

func TestCountCustomByTenant(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantA := createTestTenant(t, pool)
	tenantB := createTestTenant(t, pool)

	roleA1 := newCustomRole(tenantA, "A1")
	require.NoError(t, repo.Create(ctx, roleA1))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA1.ID) })
	roleA2 := newCustomRole(tenantA, "A2")
	require.NoError(t, repo.Create(ctx, roleA2))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA2.ID) })
	roleB := newCustomRole(tenantB, "B1")
	require.NoError(t, repo.Create(ctx, roleB))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleB.ID) })

	count, err := repo.CountCustomByTenant(ctx, tenantA)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, repo.SoftDelete(ctx, roleA2.ID))
	count, err = repo.CountCustomByTenant(ctx, tenantA)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "un rol soft-deleted no debe contarse")
}

func TestCountActiveAssignments(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	role := newCustomRole(tenantID, "Con asignaciones")
	require.NoError(t, repo.Create(ctx, role))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, role.ID) })

	count, err := repo.CountActiveAssignments(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	userA := seedUser(t, pool)
	utrAID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at) VALUES ($1, $2, $3, $4, 'active', NOW())`,
		utrAID, userA, tenantID, role.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrAID) })

	userB := seedUser(t, pool)
	utrBID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at) VALUES ($1, $2, $3, $4, 'revoked', NOW())`,
		utrBID, userB, tenantID, role.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrBID) })

	count, err = repo.CountActiveAssignments(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "solo debe contar la asignacion 'active', no la 'revoked'")
}

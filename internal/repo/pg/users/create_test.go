package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	domusers "github.com/tu-org/embolsadora-api/internal/domain/users"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

func newCrudUser(tenantID, email string) *domusers.User {
	return &domusers.User{
		TenantID:  tenantID,
		FirstName: "Test",
		LastName:  "User",
		Email:     email,
		Role:      domusers.RoleUser,
	}
}

func TestCreate_HappyPathAndDuplicateEmail(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)

	email := "crud-" + uuid.NewString()[:8] + "@test.local"
	user := newCrudUser(tenantID, email)
	created, err := repo.Create(ctx, user)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	assert.Equal(t, email, created.Email)
	assert.False(t, created.CreatedAt.IsZero())

	dup := newCrudUser(tenantID, email)
	_, err = repo.Create(ctx, dup)
	assert.ErrorIs(t, err, domusers.ErrEmailTaken)
}

func TestCreate_ValidacionFalla(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	invalid := &domusers.User{TenantID: "algun-tenant", Email: "sin-nombre@test.local", Role: domusers.RoleUser}
	_, err := repo.Create(ctx, invalid)
	assert.ErrorIs(t, err, domusers.ErrValidation, "sin FirstName/LastName debe fallar la validacion antes de tocar la DB")
}

func ptrTo(s string) *string { return &s }

func TestCreateWithRole_HappyPath(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)
	tenantUUID, err := uuid.Parse(tenantID)
	require.NoError(t, err)

	email := "crud-role-" + uuid.NewString()[:8] + "@test.local"
	user := newCrudUser(tenantID, email)
	utr := &domain.UserTenantRole{
		ID:       uuid.New(),
		TenantID: tenantUUID,
		RoleID:   ptrTo("cliente_operario"),
		Status:   domain.UserRoleStatusActive,
	}

	created, err := repo.CreateWithRole(ctx, user, utr)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utr.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID)
	})

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM user_tenant_roles WHERE id = $1`, utr.ID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "active", status)
}

func TestCreateWithRole_RoleIDInexistente(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)
	tenantUUID, err := uuid.Parse(tenantID)
	require.NoError(t, err)

	email := "crud-badrole-" + uuid.NewString()[:8] + "@test.local"
	user := newCrudUser(tenantID, email)
	utr := &domain.UserTenantRole{
		ID:       uuid.New(),
		TenantID: tenantUUID,
		RoleID:   ptrTo("no-existe-" + uuid.NewString()[:8]),
		Status:   domain.UserRoleStatusActive,
	}

	_, err = repo.CreateWithRole(ctx, user, utr)
	assert.ErrorIs(t, err, domain.ErrInvalidRoleID)

	// CreateWithRole hace rollback de toda la transaccion: el usuario no debe quedar huerfano.
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count))
	assert.Equal(t, 0, count, "el rollback debe deshacer tambien el INSERT de users")
}

func TestCreateWithRole_EmailDuplicadoEnElMismoTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)
	tenantUUID, err := uuid.Parse(tenantID)
	require.NoError(t, err)

	email := "crud-dup-" + uuid.NewString()[:8] + "@test.local"
	first := newCrudUser(tenantID, email)
	firstUTR := &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantUUID, RoleID: ptrTo("cliente_operario"), Status: domain.UserRoleStatusActive}
	createdFirst, err := repo.CreateWithRole(ctx, first, firstUTR)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, firstUTR.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, createdFirst.ID)
	})

	second := newCrudUser(tenantID, email)
	secondUTR := &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantUUID, RoleID: ptrTo("cliente_operario"), Status: domain.UserRoleStatusActive}
	_, err = repo.CreateWithRole(ctx, second, secondUTR)
	assert.ErrorIs(t, err, domusers.ErrEmailTaken)
}

package invitations_test

// Este archivo complementa cloaking_test.go (que cubre el cloaking de roles
// globales en ListByTenant, GetByID y GetPendingByEmailAndTenant) con lo que
// ese archivo no toca: Create (happy path + el mapeo del choque contra
// idx_user_invitations_pending a ErrInvitationAlreadyPending),
// ListPendingByEmail (sin cloaking, self-action), UpdateStatus, y el caso
// simple de GetByID con un id que directamente no existe.
//
// Nota: platformTenant ya está declarada en cloaking_test.go (mismo paquete
// invitations_test) — se reutiliza acá en vez de redeclararla.

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	invRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedInviter crea un usuario descartable para usar como invited_by (FK real).
func seedInviter(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := uuid.New().String()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Inviter', 'active')`,
		id, id+"@inv.local")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func TestCreateHappyPath(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@create.local"

	created, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, created.ID)
	})

	assert.Equal(t, "pending", string(created.Status))
	assert.Equal(t, email, created.Email)
}

func TestCreateDuplicadoPendingDevuelveErrInvitationAlreadyPending(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@dup.local"

	first, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, first.ID)
	})

	_, err = repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "operario", InvitedBy: inviterID,
	})
	assert.ErrorIs(t, err, domain.ErrInvitationAlreadyPending)
}

func TestListPendingByEmailCrossTenantSinCloaking(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@pending.local"

	inv, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "super_admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, inv.ID)
	})

	list, err := repo.ListPendingByEmail(context.Background(), email)
	require.NoError(t, err)
	require.Len(t, list, 1, "ListPendingByEmail es self-action: debe ver incluso invitaciones a rol global")
	assert.Equal(t, "super_admin", list[0].RoleID)
}

func TestUpdateStatus(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)
	inviterID := seedInviter(t, pool)
	email := uuid.NewString() + "@status.local"

	inv, err := repo.Create(context.Background(), &domain.UserInvitation{
		TenantID: platformTenant, Email: email, RoleID: "admin", InvitedBy: inviterID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, inv.ID)
	})

	require.NoError(t, repo.UpdateStatus(context.Background(), inv.ID, domain.InvitationStatusAccepted))

	got, err := repo.GetByID(context.Background(), inv.ID, platformTenant, true)
	require.NoError(t, err)
	assert.Equal(t, domain.InvitationStatusAccepted, got.Status)
}

func TestGetByIDInexistenteEsNotFound(t *testing.T) {
	pool := newPool(t)
	repo := invRepo.NewInvitationRepository(pool)

	_, err := repo.GetByID(context.Background(), uuid.NewString(), platformTenant, true)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

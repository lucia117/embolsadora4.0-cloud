package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

func TestGetBySupabaseIDAndGetByID(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()

	supabaseID := "supabase-" + uuid.NewString()[:8]
	email := supabaseID + "@auth.local"
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, email)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	bySupabase, err := repo.GetBySupabaseID(ctx, supabaseID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, bySupabase.ID)
	assert.Equal(t, email, bySupabase.Email)

	byID, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, email, byID.Email)
}

func TestGetBySupabaseID_NotFound(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)

	_, err := repo.GetBySupabaseID(context.Background(), "no-existe-"+uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSetStatus(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()

	supabaseID := "supabase-status-" + uuid.NewString()[:8]
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, supabaseID+"@auth.local")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	require.NoError(t, repo.SetStatus(ctx, created.ID, domain.UserStatusRevoked))

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UserStatusRevoked, got.Status)
}

func TestSetStatus_IDInexistenteNoDaError(t *testing.T) {
	// SetStatus no chequea RowsAffected: un UPDATE que no matchea ninguna fila
	// no es un error, solo un no-op silencioso. Este test documenta ese
	// comportamiento real, no lo que "deberia" hacer.
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)

	err := repo.SetStatus(context.Background(), uuid.NewString(), domain.UserStatusActive)
	assert.NoError(t, err)
}

func TestSetPasswordChangeRequired(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()

	supabaseID := "supabase-pwd-" + uuid.NewString()[:8]
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, supabaseID+"@auth.local")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })
	require.False(t, created.PasswordChangeRequired, "el default de UpsertBySupabaseID es false")

	require.NoError(t, repo.SetPasswordChangeRequired(ctx, created.ID, true))
	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, got.PasswordChangeRequired)

	require.NoError(t, repo.SetPasswordChangeRequired(ctx, created.ID, false))
	got, err = repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, got.PasswordChangeRequired)
}

func TestSetPasswordChangeRequired_IDInexistenteNoDaError(t *testing.T) {
	// Mismo comportamiento que SetStatus: no chequea RowsAffected, un id
	// inexistente es un no-op silencioso, no un error.
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)

	err := repo.SetPasswordChangeRequired(context.Background(), uuid.NewString(), true)
	assert.NoError(t, err)
}

func TestIsActiveMemberOfTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)

	supabaseID := "supabase-member-" + uuid.NewString()[:8]
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, supabaseID+"@auth.local")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	isMember, err := repo.IsActiveMemberOfTenant(ctx, created.ID, tenantID)
	require.NoError(t, err)
	assert.False(t, isMember, "todavia no tiene ninguna membresia")

	utrID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at) VALUES ($1, $2, $3, 'cliente_operario', 'active', NOW())`,
		utrID, created.ID, tenantID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID) })

	isMember, err = repo.IsActiveMemberOfTenant(ctx, created.ID, tenantID)
	require.NoError(t, err)
	assert.True(t, isMember)

	_, err = pool.Exec(ctx, `UPDATE user_tenant_roles SET status = 'revoked' WHERE id = $1`, utrID)
	require.NoError(t, err)

	isMember, err = repo.IsActiveMemberOfTenant(ctx, created.ID, tenantID)
	require.NoError(t, err)
	assert.False(t, isMember, "una membresia revoked no cuenta como activa")
}

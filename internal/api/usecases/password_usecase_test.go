package usecases

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	usersRepoPkg "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// Hallazgo del re-review de Task 5: ForcePasswordChange resolvía el usuario
// objetivo con el repo de auth (users.UserRepository.GetByID), que no tiene
// noción de rol, y solo chequeaba membresía activa (IsActiveMemberOfTenant) sin
// mirar si esa membresía es is_global. Un caller no-superadmin podía usar el 200
// vs 404 como oráculo de existencia de un super_admin oculto, y de paso forzarle
// el flag y dispararle un mail real de reset — un efecto observable y
// disruptivo, mismo patrón que el side door de DeleteUser en Task 5.
//
// El fix resuelve la visibilidad con el repo de management ya cloakeado
// (users.Repository.GetByID, el mismo que usan ListUsers/GetUser/DeleteUser)
// ANTES de tocar el flag o mandar el mail.

const platformTenantForPasswordTest = "11b36b85-033d-4bb3-9e31-4c92161887c0"

type fakeAdminClient struct {
	resetCalls   int
	lastEmail    string
	lastRedirect string
}

func (f *fakeAdminClient) InviteUserByEmail(ctx context.Context, p supabase.InviteParams) error {
	return nil
}

func (f *fakeAdminClient) SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error {
	f.resetCalls++
	f.lastEmail = userEmail
	f.lastRedirect = redirectTo
	return nil
}

func passwordTestPoolOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedSuperAdminMember crea un usuario con supabase_user_id no-nulo (lo exige
// users.UserRepository.GetByID, a diferencia del repo de management que no lo
// toca) y una membresía activa a super_admin en el tenant plataforma.
func seedSuperAdminMember(t *testing.T, pool *pgxpool.Pool) (userID, email string) {
	t.Helper()
	ctx := context.Background()
	userID = uuid.New().String()
	email = userID + "@force-pwd-cloak.local"
	utrID := uuid.New().String()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, name, status, supabase_user_id, password_change_required)
		VALUES ($1, $2, 'Force PWD Cloak Test', 'active', $3, FALSE)`,
		userID, email, "supa-"+userID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		VALUES ($1, $2, $3, 'super_admin', 'active', NOW())`,
		utrID, userID, platformTenantForPasswordTest)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID, email
}

func passwordChangeRequiredFlag(t *testing.T, pool *pgxpool.Pool, userID string) bool {
	t.Helper()
	var flag bool
	err := pool.QueryRow(context.Background(),
		`SELECT password_change_required FROM users WHERE id = $1`, userID).Scan(&flag)
	require.NoError(t, err)
	return flag
}

func TestForcePasswordChange_SuperadminOcultoNoMutaNiEnviaMail(t *testing.T) {
	pool := passwordTestPoolOrSkip(t)
	authRepo := usersRepoPkg.NewUserRepository(pool)
	mgmtRepo := usersRepoPkg.NewPostgresRepository(pool)
	fakeClient := &fakeAdminClient{}
	uc := NewPasswordUsecase(authRepo, mgmtRepo, fakeClient, "http://localhost:3000", zap.NewNop())

	superID, _ := seedSuperAdminMember(t, pool)
	ctx := platform.WithTenantID(context.Background(), platformTenantForPasswordTest)

	// Caller no-superadmin (includeGlobal=false): el superadmin oculto debe ser
	// indistinguible de uno inexistente. Mismo error, cero efectos.
	err := uc.ForcePasswordChange(ctx, superID, false)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrNotFound), "debe dar ErrNotFound, no ErrForbidden — un 403 confirmaría que el usuario existe")

	require.Equal(t, 0, fakeClient.resetCalls, "no debe enviarse ningún mail de reset a un usuario oculto")
	require.False(t, passwordChangeRequiredFlag(t, pool, superID), "el flag no debe mutar para un usuario oculto")
}

func TestForcePasswordChange_SuperadminVisibleParaSuperadminMutaYEnviaMail(t *testing.T) {
	pool := passwordTestPoolOrSkip(t)
	authRepo := usersRepoPkg.NewUserRepository(pool)
	mgmtRepo := usersRepoPkg.NewPostgresRepository(pool)
	fakeClient := &fakeAdminClient{}
	uc := NewPasswordUsecase(authRepo, mgmtRepo, fakeClient, "http://localhost:3000", zap.NewNop())

	superID, email := seedSuperAdminMember(t, pool)
	ctx := platform.WithTenantID(context.Background(), platformTenantForPasswordTest)

	// Caller superadmin (includeGlobal=true): debe poder operar normalmente.
	err := uc.ForcePasswordChange(ctx, superID, true)
	require.NoError(t, err)

	require.Equal(t, 1, fakeClient.resetCalls)
	require.Equal(t, email, fakeClient.lastEmail)
	require.True(t, passwordChangeRequiredFlag(t, pool, superID))
}

package usecases

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	invitationsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// TestCreateInvitation_DuplicadoOcultoDaLaMismaRespuestaQueElVisible ejercita
// contra Postgres real (no fakes) el hallazgo de la revisión: el chequeo
// previo GetPendingByEmailAndTenant está cloakeado, así que para un caller
// no-super_admin no ve una invitación pendiente oculta a super_admin — pero
// idx_user_invitations_pending (único sobre tenant_id+email WHERE
// status='pending') sí la ve, y sin capturar el 23505 en Create() ese choque
// se propagaba como error genérico (500), un status distinto del 409 que
// devuelve el chequeo previo cuando el duplicado SÍ es visible. Eso reabría
// exactamente la distinción que esta tarea existe para cerrar.
//
// La decisión de diseño (indistinguibilidad perfecta con "no existe ninguna
// invitación" no es alcanzable: el índice único bloquea la segunda fila pase
// lo que pase) es que la respuesta sea IDÉNTICA a la del duplicado visible:
// domain.ErrInvitationAlreadyPending, 409. El caller aprende que "existe
// alguna invitación pendiente para este email" — igual que aprendería con
// cualquier duplicado visible — pero no aprende el rol.
func TestCreateInvitation_DuplicadoOcultoDaLaMismaRespuestaQueElVisible(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	// t.Cleanup, NO defer pool.Close(): mismo motivo que en
	// internal/repo/pg/invitations/cloaking_test.go — un defer cierra el pool
	// ANTES que el t.Cleanup de limpieza registrado por seedPendingInvitation,
	// dejando la limpieza fallando en silencio.
	t.Cleanup(pool.Close)

	invRepo := invitationsRepo.NewInvitationRepository(pool)
	roleRepo := rolesRepo.NewPostgresRepository(pool)
	admin := &fakeAdminClientForCreate{}

	uc := NewInvitationUsecase(invRepo, nil, nil, nil, roleRepo, admin, nil, "https://app.example.com", 1000)

	// Sembrar una invitación pendiente a super_admin (rol global) para un
	// email nuevo, directamente por SQL — así el estado de partida es
	// exactamente el que produciría un flujo real de invitación.
	inviterID := uuid.New().String()
	email := uuid.New().String() + "@dup.local"
	pendingInvID := uuid.New().String()

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Inviter', 'active')`,
		inviterID, inviterID+"@inv.local")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO user_invitations (id, tenant_id, email, role_id, status, invited_by)
		 VALUES ($1, $2, $3, 'super_admin', 'pending', $4)`,
		pendingInvID, testTenantID, email, inviterID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, pendingInvID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, inviterID)
	})

	// Caller no-super_admin, mismo tenant plataforma, intenta invitar al mismo
	// email con un rol visible ("admin"). GetPendingByEmailAndTenant (cloaked)
	// no ve la invitación a super_admin, así que el usecase sigue hasta el
	// INSERT — que debe chocar contra idx_user_invitations_pending y volver
	// como el mismo ErrInvitationAlreadyPending que un duplicado visible.
	callerCtx := ctxForCreateInvitation("platform_admin")
	created, err := uc.CreateInvitation(callerCtx, email, "admin")

	require.Nil(t, created)
	require.ErrorIs(t, err, domain.ErrInvitationAlreadyPending,
		"un duplicado oculto por cloaking debe dar la misma respuesta 409 que uno visible, nunca un 500")
	require.False(t, admin.inviteCalled, "un duplicado (visible u oculto) no debe mandar mail")
}

// TestCreateInvitation_DuplicadoVisibleDaElMismoError es el control: dos
// invitaciones al mismo rol visible, mismo email — el camino que YA andaba
// bien antes de este fix (el chequeo previo SÍ ve el duplicado). Confirma que
// ambos caminos (chequeo previo y constraint de DB) convergen al mismo error.
func TestCreateInvitation_DuplicadoVisibleDaElMismoError(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	invRepo := invitationsRepo.NewInvitationRepository(pool)
	roleRepo := rolesRepo.NewPostgresRepository(pool)
	admin := &fakeAdminClientForCreate{}

	uc := NewInvitationUsecase(invRepo, nil, nil, nil, roleRepo, admin, nil, "https://app.example.com", 1000)

	inviterID := uuid.New().String()
	email := uuid.New().String() + "@dup.local"
	pendingInvID := uuid.New().String()

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Inviter', 'active')`,
		inviterID, inviterID+"@inv.local")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO user_invitations (id, tenant_id, email, role_id, status, invited_by)
		 VALUES ($1, $2, $3, 'admin', 'pending', $4)`,
		pendingInvID, testTenantID, email, inviterID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_invitations WHERE id = $1`, pendingInvID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, inviterID)
	})

	callerCtx := ctxForCreateInvitation("platform_admin")
	created, err := uc.CreateInvitation(callerCtx, email, "admin")

	require.Nil(t, created)
	require.ErrorIs(t, err, domain.ErrInvitationAlreadyPending)
	require.False(t, admin.inviteCalled)
}

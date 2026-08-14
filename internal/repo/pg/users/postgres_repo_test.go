package users_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// Problema 5: PATCH /api/v1/users/:id (and DELETE) returned 404 "User not found" for
// global-role users (e.g. super_admin) even though GET /api/v1/users/:id for the exact
// same id/tenant succeeded. Root cause: GetByID's SELECT already accounts for
// auth-provisioned users whose users.tenant_id column is NULL (their real tenant
// membership lives in user_tenant_roles instead — see the comment on GetByID), but
// Update and Delete's queries filtered on a strict `tenant_id = $N` equality with no
// such fallback, so they always affected 0 rows for exactly this class of user.
//
// Requires DATABASE_URL env var. Skip if not set.
func setupNullTenantUser(t *testing.T, db *pgxpool.Pool) (tenantID, userID string) {
	t.Helper()
	ctx := context.Background()

	tenantID = uuid.New().String()
	userID = uuid.New().String()
	utrID := uuid.New().String()

	_, err := db.Exec(ctx, `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Test Tenant', 'Test Tenant SRL', $2)`,
		tenantID, "test-tenant-"+tenantID[:8])
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID) })

	// cliente_admin, no admin: desde la migración 000010 admin es platform-only
	// y no se puede asignar en un tenant cliente como el que este helper crea —
	// irrelevante para lo que estos tests verifican (el bug de tenant_id NULL).
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, email, tenant_id, status, first_name, last_name, role)
		VALUES ($1, $2, NULL, 'active', 'Before', 'Update', 'cliente_admin')`,
		userID, "repro-"+userID[:8]+"@test.local")
	require.NoError(t, err)
	t.Cleanup(func() { db.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID) })

	_, err = db.Exec(ctx, `
		INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		VALUES ($1, $2, $3, 'cliente_admin', 'active', now())`,
		utrID, userID, tenantID)
	require.NoError(t, err)

	return tenantID, userID
}

func TestUpdate_UserWithNullTenantIDColumn_Succeeds(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	repo := users.NewPostgresRepository(db)

	tenantID, userID := setupNullTenantUser(t, db)

	// Sanity check: GetByID must find this user via their user_tenant_roles membership
	// (this is the read path the prior investigation confirmed already works).
	current, err := repo.GetByID(ctx, tenantID, userID, false)
	require.NoError(t, err, "GetByID should find the user via their user_tenant_roles membership")

	current.FirstName = "After"
	updated, err := repo.Update(ctx, current)
	require.NoError(t, err, "Update must succeed for a user whose tenant_id column is NULL but who has an active user_tenant_roles membership")
	require.Equal(t, "After", updated.FirstName)
}

func TestDelete_UserWithNullTenantIDColumn_Succeeds(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	repo := users.NewPostgresRepository(db)

	tenantID, userID := setupNullTenantUser(t, db)

	err = repo.Delete(ctx, tenantID, userID)
	require.NoError(t, err, "Delete must succeed for a user whose tenant_id column is NULL but who has an active user_tenant_roles membership")

	var deletedAt *string
	err = db.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", userID).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt)
}

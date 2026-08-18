package middleware

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestLoadRolePermissions(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	perms, isGlobal, err := loadRolePermissions(context.Background(), pool, "cliente_admin")
	require.NoError(t, err)
	require.False(t, isGlobal)
	require.Contains(t, perms, "perm_users_manage")
	require.NotContains(t, perms, "perm_users")
}

func TestLoadRolePermissionsRolInexistente(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	_, _, err = loadRolePermissions(context.Background(), pool, "rol_que_no_existe")
	require.Error(t, err)
}

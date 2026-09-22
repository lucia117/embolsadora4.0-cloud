package tenants_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

func TestFindAll_IncluyeLosTenantsCreados(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	ctx := context.Background()

	first := newTestTenant()
	require.NoError(t, repo.Create(ctx, first))
	defer func() { _ = repo.Delete(ctx, first.ID) }()

	second := newTestTenant()
	require.NoError(t, repo.Create(ctx, second))
	defer func() { _ = repo.Delete(ctx, second.ID) }()

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)

	ids := make(map[uuid.UUID]bool, len(all))
	for _, tn := range all {
		ids[tn.ID] = true
	}
	assert.True(t, ids[first.ID], "FindAll debe incluir el primer tenant creado")
	assert.True(t, ids[second.ID], "FindAll debe incluir el segundo tenant creado")
}

package users_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestUpsertBySupabaseID_Idempotency verifies that concurrent upserts with the same
// supabase_user_id produce exactly one DB record (SC-005 from spec).
// Requires DATABASE_URL env var. Skip if not set.
func TestUpsertBySupabaseID_Idempotency(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := users.NewUserRepository(db)
	ctx := context.Background()

	const supabaseID = "test-idempotency-user-001"
	const email = "idempotency@test.example.com"

	// Cleanup before test
	db.Exec(ctx, "DELETE FROM users WHERE supabase_user_id = $1", supabaseID)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = repo.UpsertBySupabaseID(ctx, supabaseID, email)
		}()
	}
	wg.Wait()

	// Verify exactly one record exists
	var count int
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE supabase_user_id = $1", supabaseID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "concurrent upserts should produce exactly one record")

	// Cleanup
	db.Exec(ctx, "DELETE FROM users WHERE supabase_user_id = $1", supabaseID)
}

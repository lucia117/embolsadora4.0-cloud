package dbmigrate_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/platform/dbmigrate"
)

// TestRun_AppliesAndIsIdempotent runs the real migrations from migrations/
// against a Postgres pointed to by DATABASE_URL. Skipped when DATABASE_URL
// is unset so the unit suite stays fast.
func TestRun_AppliesAndIsIdempotent(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	logger := zap.NewNop()

	if err := dbmigrate.Run("file://../../../migrations", dbURL, logger); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(context.Background())

	var version int
	var dirty bool
	if err := conn.QueryRow(context.Background(),
		"SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 2 || dirty {
		t.Fatalf("expected version=2 dirty=false, got version=%d dirty=%v", version, dirty)
	}

	// Second run should be a no-op (ErrNoChange handled internally).
	if err := dbmigrate.Run("file://../../../migrations", dbURL, logger); err != nil {
		t.Fatalf("second Run (idempotency): %v", err)
	}
}

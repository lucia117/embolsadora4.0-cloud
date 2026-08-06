package apikeys_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	pgapikeys "github.com/tu-org/embolsadora-api/internal/repo/pg/apikeys"
)

// newPool sigue la convencion del repo (ver internal/repo/pg/tenants/repository_test.go):
// sin DATABASE_URL el test hace skip en vez de fallar.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL no seteada; se omite el test de integracion")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedDevice crea un tenant y un edge device descartables y devuelve sus IDs.
func seedDevice(t *testing.T, pool *pgxpool.Pool) (tenantID, deviceID uuid.UUID, machineID string) {
	t.Helper()
	ctx := context.Background()
	tenantID, deviceID = uuid.New(), uuid.New()
	machineID = "EMB-TEST-" + uuid.NewString()[:8]

	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain, is_active)
		 VALUES ($1, $2, $2, $3, true)`,
		tenantID, "apikeys-test", "apikeys-"+uuid.NewString()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO edge_devices (id, tenant_id, name, machine_id, edge_type, raspberry_base_url, status)
		 VALUES ($1, $2, 'test device', $3, 'RASPBERRY_PLC', 'http://localhost:9000', 'ACTIVE')`,
		deviceID, tenantID, machineID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})
	return tenantID, deviceID, machineID
}

func newKey(tenantID, deviceID uuid.UUID) (*domainapikeys.APIKey, string) {
	plaintext, keyID, hash, err := domainapikeys.Generate()
	if err != nil {
		panic(err)
	}
	name := "clave de test"
	return &domainapikeys.APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		DeviceID:  deviceID,
		KeyID:     keyID,
		KeyHash:   hash,
		Name:      &name,
		CreatedAt: time.Now().UTC(),
	}, plaintext
}

func TestCreateAndGetByKeyID(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, machineID := seedDevice(t, pool)
	key, plaintext := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	cred, err := repo.GetByKeyID(ctx, key.KeyID)
	require.NoError(t, err)

	assert.Equal(t, key.ID, cred.KeyPK)
	assert.Equal(t, tenantID, cred.TenantID)
	assert.Equal(t, deviceID, cred.DeviceID)
	assert.Equal(t, machineID, cred.MachineID, "el JOIN debe traer el machine_id del device")
	assert.Equal(t, "ACTIVE", cred.DeviceStatus)
	assert.Nil(t, cred.RevokedAt)

	// El hash persistido tiene que validar contra el secreto original.
	_, secret, err := domainapikeys.Parse(plaintext)
	require.NoError(t, err)
	assert.True(t, domainapikeys.Matches(secret, cred.KeyHash))
}

func TestGetByKeyIDNotFound(t *testing.T) {
	repo := pgapikeys.NewRepository(newPool(t))
	_, err := repo.GetByKeyID(context.Background(), "deadbeefcafe")
	assert.ErrorIs(t, err, domainapikeys.ErrKeyNotFound)
}

func TestRevokeIsIdempotent(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	key, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	require.NoError(t, repo.Revoke(ctx, tenantID, key.ID))
	first, err := repo.GetByKeyID(ctx, key.KeyID)
	require.NoError(t, err)
	require.NotNil(t, first.RevokedAt)

	// Revocar de nuevo no debe mover el timestamp original.
	require.NoError(t, repo.Revoke(ctx, tenantID, key.ID))
	second, err := repo.GetByKeyID(ctx, key.KeyID)
	require.NoError(t, err)
	assert.Equal(t, first.RevokedAt.UnixNano(), second.RevokedAt.UnixNano())
}

func TestRevokeRejectsForeignTenant(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	key, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	err := repo.Revoke(ctx, uuid.New(), key.ID)
	assert.ErrorIs(t, err, domainapikeys.ErrKeyNotFound)
}

func TestListByDeviceNewestFirst(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	older, _ := newKey(tenantID, deviceID)
	older.CreatedAt = time.Now().UTC().Add(-time.Hour)
	newer, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, older))
	require.NoError(t, repo.Create(ctx, newer))

	list, err := repo.ListByDevice(ctx, tenantID, deviceID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, newer.ID, list[0].ID, "la mas nueva va primero")
}

func TestTouchLastUsed(t *testing.T) {
	pool := newPool(t)
	repo := pgapikeys.NewRepository(pool)
	ctx := context.Background()

	tenantID, deviceID, _ := seedDevice(t, pool)
	key, _ := newKey(tenantID, deviceID)
	require.NoError(t, repo.Create(ctx, key))

	list, err := repo.ListByDevice(ctx, tenantID, deviceID)
	require.NoError(t, err)
	require.Nil(t, list[0].LastUsedAt)

	require.NoError(t, repo.TouchLastUsed(ctx, key.ID))

	list, err = repo.ListByDevice(ctx, tenantID, deviceID)
	require.NoError(t, err)
	assert.NotNil(t, list[0].LastUsedAt)
}

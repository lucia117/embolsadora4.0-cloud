package mongo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/config"
	mongoplatform "github.com/tu-org/embolsadora-api/internal/platform/mongo"
)

func testConfig(t *testing.T) config.MongoConfig {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	return config.MongoConfig{
		URI:      uri,
		Database: "embolsadora_test_platform_client",
		Timeout:  5 * time.Second,
	}
}

func TestConnectPingClose(t *testing.T) {
	cfg := testConfig(t)
	client, err := mongoplatform.Connect(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, client.Database())
	assert.Equal(t, cfg.Database, client.Database().Name())

	require.NoError(t, client.Ping(context.Background()))
	require.NoError(t, client.Close(context.Background()))
}

func TestConnectURIInvalidaDevuelveError(t *testing.T) {
	// No necesita Mongo real levantado: una URI con esquema invalido falla
	// en el propio Connect del driver antes de intentar red.
	_, err := mongoplatform.Connect(context.Background(), config.MongoConfig{
		URI:      "not-a-mongo-uri",
		Database: "x",
		Timeout:  2 * time.Second,
	})
	require.Error(t, err)
}

func TestConnectServidorInalcanzableFallaPorPing(t *testing.T) {
	// URI con esquema valido pero puerto sin nada escuchando: Connect() no
	// falla (conexion perezosa en el driver v2), pero el Ping interno si.
	_, err := mongoplatform.Connect(context.Background(), config.MongoConfig{
		URI:      "mongodb://127.0.0.1:1",
		Database: "x",
		Timeout:  500 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping fallido")
}

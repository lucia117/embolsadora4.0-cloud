package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDashboardsConfigDefaults verifica que DashboardsConfig se cargue con
// los defaults documentados cuando las env vars DASHBOARDS_* no estan
// seteadas. Solo setea las env vars que Load() efectivamente requiere via
// require() (ver internal/config/config.go): DATABASE_URL,
// SUPABASE_JWKS_URL, SUPABASE_JWT_ISSUER, SUPABASE_URL,
// SUPABASE_SERVICE_ROLE_KEY y APP_BASE_URL.
func TestDashboardsConfigDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("SUPABASE_JWKS_URL", "https://x")
	t.Setenv("SUPABASE_JWT_ISSUER", "https://x")
	t.Setenv("SUPABASE_URL", "https://x")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "x")
	t.Setenv("APP_BASE_URL", "https://x")

	cfg, err := Load(EnvLocal)
	require.NoError(t, err)

	assert.Equal(t, 15, cfg.Dashboards.MetricsRateLimitRPM)
	assert.Equal(t, 5, cfg.Dashboards.MetricsRateLimitBurst)
	assert.Equal(t, 10, cfg.Dashboards.MetricsMaxSpecs)
	assert.Equal(t, 1000, cfg.Dashboards.MetricsMaxBuckets)
	assert.Equal(t, 5000, cfg.Dashboards.MetricsMaxRawPoints)
	assert.Equal(t, 200, cfg.Dashboards.MetricsMaxGroups)
	assert.Equal(t, 50, cfg.Dashboards.MetricsMaxBatchQueries)
	assert.Equal(t, 5000, cfg.Dashboards.MetricsMaxTimeMS)
}

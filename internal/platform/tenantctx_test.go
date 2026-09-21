package platform_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestTenantIDRoundTrip(t *testing.T) {
	ctx := platform.WithTenantID(context.Background(), "tenant-abc")
	assert.Equal(t, "tenant-abc", platform.TenantID(ctx))
}

func TestTenantIDSinSetearDevuelveVacio(t *testing.T) {
	assert.Equal(t, "", platform.TenantID(context.Background()))
}

func TestUserIDRoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := platform.WithUserID(context.Background(), id)
	got := platform.UserID(ctx)
	require.NotNil(t, got)
	assert.Equal(t, id, *got)
}

func TestUserIDSinSetearDevuelveNil(t *testing.T) {
	assert.Nil(t, platform.UserID(context.Background()))
}

func TestSupabaseSubRoundTrip(t *testing.T) {
	ctx := platform.WithSupabaseSub(context.Background(), "sub-123")
	assert.Equal(t, "sub-123", platform.SupabaseSub(ctx))
}

func TestUserEmailRoundTrip(t *testing.T) {
	ctx := platform.WithUserEmail(context.Background(), "a@b.com")
	assert.Equal(t, "a@b.com", platform.UserEmail(ctx))
}

func TestDomainUserRoundTrip(t *testing.T) {
	type fakeDomainUser struct{ Name string }
	ctx := platform.WithDomainUser(context.Background(), &fakeDomainUser{Name: "Ana"})
	got, ok := platform.DomainUser(ctx).(*fakeDomainUser)
	require.True(t, ok)
	assert.Equal(t, "Ana", got.Name)
}

func TestDomainUserSinSetearDevuelveNil(t *testing.T) {
	assert.Nil(t, platform.DomainUser(context.Background()))
}

func TestTenantUUIDRoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := platform.WithTenantUUID(context.Background(), id)
	got := platform.TenantUUID(ctx)
	require.NotNil(t, got)
	assert.Equal(t, id, *got)
}

func TestTenantUUIDSinSetearDevuelveNil(t *testing.T) {
	assert.Nil(t, platform.TenantUUID(context.Background()))
}

func TestAppBaseURLRoundTrip(t *testing.T) {
	ctx := platform.WithAppBaseURL(context.Background(), "https://app.example.com")
	assert.Equal(t, "https://app.example.com", platform.AppBaseURL(ctx))
}

func TestAppBaseURLSinSetearDevuelveVacio(t *testing.T) {
	assert.Equal(t, "", platform.AppBaseURL(context.Background()))
}

func TestTenantMatches(t *testing.T) {
	tenantID := uuid.New()

	t.Run("feliz: mismo uuid", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), tenantID.String())
		assert.True(t, platform.TenantMatches(ctx, tenantID))
	})

	t.Run("mismatch: uuid distinto", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), tenantID.String())
		assert.False(t, platform.TenantMatches(ctx, uuid.New()))
	})

	t.Run("contexto sin tenant seteado", func(t *testing.T) {
		assert.False(t, platform.TenantMatches(context.Background(), tenantID))
	})

	t.Run("tenantID en contexto no es un uuid valido", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), "no-es-un-uuid")
		assert.False(t, platform.TenantMatches(ctx, tenantID))
	})

	t.Run("case insensitive: mismo uuid con distinto casing", func(t *testing.T) {
		ctx := platform.WithTenantID(context.Background(), tenantID.String())
		upper, err := uuid.Parse(tenantID.String())
		require.NoError(t, err)
		assert.True(t, platform.TenantMatches(ctx, upper))
	})
}

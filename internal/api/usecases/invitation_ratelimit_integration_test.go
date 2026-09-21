package usecases

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestCheckRateLimit_ContraRedisReal(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit de invitaciones contra Redis real")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	tenantID := fmt.Sprintf("rl-test-%d", time.Now().UnixNano())
	key := fmt.Sprintf("invitations:ratelimit:%s:%s", tenantID, time.Now().UTC().Format("2006-01-02-15"))
	t.Cleanup(func() { rdb.Del(context.Background(), key) })

	// limite bajo (2) para no tener que iterar cientos de veces
	uc := NewInvitationUsecase(nil, nil, nil, nil, nil, nil, rdb, "", 2)

	require.NoError(t, uc.checkRateLimit(context.Background(), tenantID), "1ra llamada, dentro del limite")
	require.NoError(t, uc.checkRateLimit(context.Background(), tenantID), "2da llamada, en el limite")
	err = uc.checkRateLimit(context.Background(), tenantID)
	require.ErrorIs(t, err, domain.ErrInvitationRateLimitExceeded, "3ra llamada, excede el limite de 2")
}

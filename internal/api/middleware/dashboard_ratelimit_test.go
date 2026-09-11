package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDashboardBucketReply(t *testing.T) {
	tests := []struct {
		name           string
		res            any
		wantAllowed    bool
		wantRetryAfter int
		wantOK         bool
	}{
		{name: "permitido", res: []any{int64(1), int64(0)}, wantAllowed: true, wantOK: true},
		{name: "denegado con retry_after", res: []any{int64(0), int64(3)}, wantAllowed: false, wantRetryAfter: 3, wantOK: true},
		{name: "malformado -- debe abrir, no denegar", res: "no-es-slice", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, retryAfter, ok := parseDashboardBucketReply(tt.res)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantAllowed, allowed)
				assert.Equal(t, tt.wantRetryAfter, retryAfter)
			}
		})
	}
}

func TestDashboardRateLimiter_AllowsWhenRedisNil(t *testing.T) {
	limiter := NewDashboardRateLimiter(nil, 15.0/60.0, 5)
	allowed, retryAfter, err := limiter.Allow(t.Context(), "user-1")
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 0, retryAfter)
}

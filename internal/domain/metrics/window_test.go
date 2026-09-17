package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWindow(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	from8h := now.Add(-8 * time.Hour)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name     string
		q        MetricQuery
		wantFrom time.Time
		wantTo   time.Time
		wantCode string
	}{
		{
			name:     "range resuelve a now-range..now",
			q:        MetricQuery{Range: Range8h},
			wantFrom: from8h,
			wantTo:   now,
		},
		{
			name:     "from/to absolutos",
			q:        MetricQuery{From: &past, To: &future},
			wantFrom: past,
			wantTo:   future,
		},
		{
			name:     "range invalido",
			q:        MetricQuery{Range: "3h"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "ni range ni from/to",
			q:        MetricQuery{},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "range y from/to a la vez",
			q:        MetricQuery{Range: Range1h, From: &past, To: &future},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "to no posterior a from",
			q:        MetricQuery{From: &future, To: &past},
			wantCode: CodeInvalidParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, err := ResolveWindow(tt.q, now)
			if tt.wantCode != "" {
				require.Error(t, err)
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
				assert.Equal(t, tt.wantCode, ve.Code)
				return
			}
			require.NoError(t, err)
			assert.True(t, tt.wantFrom.Equal(from))
			assert.True(t, tt.wantTo.Equal(to))
		})
	}
}

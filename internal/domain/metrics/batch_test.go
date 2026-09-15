// internal/domain/metrics/batch_test.go
package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBatchIDs(t *testing.T) {
	limits := Limits{MaxBatchQueries: 2}

	tests := []struct {
		name     string
		ids      []string
		wantCode string
	}{
		{name: "ok", ids: []string{"a", "b"}},
		{name: "vacio", ids: nil, wantCode: CodeInvalidParams},
		{name: "excede el maximo", ids: []string{"a", "b", "c"}, wantCode: CodeTooManyQueries},
		{name: "id vacio", ids: []string{"a", ""}, wantCode: CodeInvalidParams},
		{name: "id duplicado", ids: []string{"a", "a"}, wantCode: CodeInvalidParams},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBatchIDs(tt.ids, limits)
			if tt.wantCode == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, tt.wantCode, ve.Code)
		})
	}
}

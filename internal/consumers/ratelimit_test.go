package consumers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseBucketReply cubre el parseo de la respuesta del script Lua,
// incluida la rama de fail-open ante una respuesta malformada que el propio
// tokenBucketScript nunca produce (por eso esta separada de Allow: probar
// Allow contra un Redis real jamas ejercitaria estos casos).
func TestParseBucketReply(t *testing.T) {
	tests := []struct {
		name           string
		res            any
		wantAllowed    bool
		wantRetryAfter int
		wantOK         bool
	}{
		{
			name:           "permitido bien formado",
			res:            []any{int64(1), int64(0)},
			wantAllowed:    true,
			wantRetryAfter: 0,
			wantOK:         true,
		},
		{
			name:           "denegado bien formado con retry_after",
			res:            []any{int64(0), int64(5)},
			wantAllowed:    false,
			wantRetryAfter: 5,
			wantOK:         true,
		},
		{
			name:   "no es un slice",
			res:    "no-es-una-respuesta-valida",
			wantOK: false,
		},
		{
			name:   "slice de longitud incorrecta",
			res:    []any{int64(1)},
			wantOK: false,
		},
		{
			name: "elementos float64 en vez de int64 -- debe abrir, no denegar",
			res:  []any{float64(1), float64(0)},
			// Antes del fix, `_` descartaba el error de la asercion de tipo,
			// `allowed` quedaba en su zero value (0), `allowed == 1` daba
			// false y la funcion devolvia "denegado, Retry-After: 0" en vez
			// de fail-open. wantOK:false es la asercion que prueba que ya no
			// pasa eso: el llamador (Allow) trata ok=false como fail-open.
			wantOK: false,
		},
		{
			name:   "primer elemento de tipo inesperado",
			res:    []any{"uno", int64(0)},
			wantOK: false,
		},
		{
			name:   "segundo elemento de tipo inesperado",
			res:    []any{int64(1), "cero"},
			wantOK: false,
		},
		{
			name:   "nil",
			res:    nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, retryAfter, ok := parseBucketReply(tt.res)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantAllowed, allowed)
				assert.Equal(t, tt.wantRetryAfter, retryAfter)
			}
		})
	}
}

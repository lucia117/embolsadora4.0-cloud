package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

// TestDecimateUniform_PreservesLastPointDespiteFloatRounding cubre un caso
// donde int(float64(maxPoints-1)*step) pierde precision y trunca por debajo
// del ultimo indice real: para n=16, maxPoints=12, step = 15/11 y
// 11*step evalua en float64 a 14.999999999999998, no 15 -- sin el guard
// explicito del ultimo indice, decimateUniform devolveria points[14] en vez
// del verdadero ultimo punto points[15], violando la garantia documentada de
// preservar siempre el primer y el ultimo punto original.
func TestDecimateUniform_PreservesLastPointDespiteFloatRounding(t *testing.T) {
	base := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	points := make([]domain.RawPoint, 16)
	for i := range points {
		points[i] = domain.RawPoint{Ts: base.Add(time.Duration(i) * time.Second), Value: float64(i)}
	}

	out := decimateUniform(points, 12)

	require.LessOrEqual(t, len(out), 12)
	require.Equal(t, points[0], out[0], "debe preservar el primer punto")
	require.Equal(t, points[len(points)-1], out[len(out)-1], "debe preservar el ultimo punto")
}

func TestDecimateUniform_NoOpWhenUnderLimit(t *testing.T) {
	points := []domain.RawPoint{{Value: 1.0}, {Value: 2.0}}
	out := decimateUniform(points, 5)
	require.Equal(t, points, out)
}

package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidationErrorCarriesCode(t *testing.T) {
	err := newValidationError(CodeInvalidParams, "machineId es requerido")

	assert.Equal(t, "machineId es requerido", err.Error())

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, CodeInvalidParams, ve.Code)
}

func TestAggValid(t *testing.T) {
	assert.True(t, AggAvg.Valid())
	assert.True(t, AggRaw.Valid())
	assert.False(t, Agg("bogus").Valid())
}

func TestBucketDuration(t *testing.T) {
	d, ok := Bucket1h.duration()
	assert.True(t, ok)
	assert.Equal(t, time.Hour, d)

	_, ok = Bucket("bogus").duration()
	assert.False(t, ok)
}

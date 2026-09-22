package logs

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	entry := domain.LogEntry{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	cursor := encodeCursor(entry)
	require.NotEmpty(t, cursor)

	decoded, err := decodeCursor(cursor)
	require.NoError(t, err)
	assert.Equal(t, entry.ID, decoded.ID)
	assert.True(t, entry.CreatedAt.Equal(decoded.CreatedAt))
}

func TestDecodeCursorInvalidoDevuelveErrInvalidCursor(t *testing.T) {
	_, err := decodeCursor("no-es-base64-valido-!!!")
	assert.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestDecodeCursorBase64ValidoPeroJSONInvalido(t *testing.T) {
	// "aG9sYQ==" decodifica a "hola", que no es JSON valido.
	_, err := decodeCursor("aG9sYQ==")
	assert.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestBuildFilterClausesSoloTenantPorDefecto(t *testing.T) {
	tenantID := uuid.New()
	where, args := buildFilterClauses(tenantID, "", "", nil, nil, nil, "")
	require.Len(t, where, 1)
	require.Len(t, args, 1)
	assert.Equal(t, "tenant_id = $1", where[0])
	assert.Equal(t, tenantID, args[0])
}

func TestBuildFilterClausesAcumulaTodosLosFiltrosEnOrden(t *testing.T) {
	tenantID := uuid.New()
	machineID := uuid.New()
	from := time.Now().Add(-time.Hour)
	to := time.Now()

	where, args := buildFilterClauses(tenantID, "system", "critical", &machineID, &from, &to, "falla")

	require.Len(t, where, 7)
	require.Len(t, args, 7)
	assert.Equal(t, "tenant_id = $1", where[0])
	assert.Equal(t, "event_type = $2", where[1])
	assert.Equal(t, "severity = $3", where[2])
	assert.Equal(t, "machine_id = $4", where[3])
	assert.Equal(t, "created_at >= $5", where[4])
	assert.Equal(t, "created_at <= $6", where[5])
	assert.Contains(t, where[6], "plainto_tsquery")
	assert.Equal(t, "falla", args[6])
}

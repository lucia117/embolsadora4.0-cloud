package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// errRepoUnavailable simula un fallo de Postgres (timeout, conexion caida,
// etc.) que no es "key inexistente". Sirve para probar que ese caso NO se
// colapsa en ErrAPIKeyInvalid (I-1): un outage de base no es una credencial
// invalida.
var errRepoUnavailable = errors.New("fakeRepo: postgres no disponible")

// fakeRepo implementa domainapikeys.Repository en memoria. El autenticador se
// testea sin Postgres ni Redis: sus reglas son de decision, no de storage.
type fakeRepo struct {
	byKeyID map[string]*domainapikeys.Credential
	touched []uuid.UUID
	calls   int

	// lookupErr, si esta seteado, hace que GetByKeyID lo devuelva en vez de
	// consultar byKeyID. Deja el comportamiento existente intacto cuando es
	// nil, para que los demas subtests sigan probando lo mismo que hoy.
	lookupErr error
}

func (f *fakeRepo) GetByKeyID(_ context.Context, keyID string) (*domainapikeys.Credential, error) {
	f.calls++
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	c, ok := f.byKeyID[keyID]
	if !ok {
		return nil, domainapikeys.ErrKeyNotFound
	}
	return c, nil
}
func (f *fakeRepo) Create(context.Context, *domainapikeys.APIKey) error { return nil }
func (f *fakeRepo) ListByDevice(context.Context, uuid.UUID, uuid.UUID) ([]*domainapikeys.APIKey, error) {
	return nil, nil
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeRepo) TouchLastUsed(_ context.Context, keyPK uuid.UUID) error {
	f.touched = append(f.touched, keyPK)
	return nil
}

// newAuth arma un autenticador con una key valida y devuelve el texto en claro.
func newAuth(t *testing.T, mutate func(*domainapikeys.Credential)) (*security.APIKeyAuthenticator, string, *fakeRepo) {
	t.Helper()
	plaintext, keyID, hash, err := domainapikeys.Generate()
	require.NoError(t, err)

	cred := &domainapikeys.Credential{
		KeyPK:        uuid.New(),
		TenantID:     uuid.New(),
		DeviceID:     uuid.New(),
		KeyID:        keyID,
		KeyHash:      hash,
		MachineID:    "EMB-DEV-001",
		DeviceStatus: "ACTIVE",
	}
	if mutate != nil {
		mutate(cred)
	}
	repo := &fakeRepo{byKeyID: map[string]*domainapikeys.Credential{keyID: cred}}
	// rdb nil: el autenticador debe funcionar sin Redis (fail-open de la cache).
	auth := security.NewAPIKeyAuthenticator(repo, nil, time.Minute, zap.NewNop())
	return auth, plaintext, repo
}

func TestAuthenticateHappyPath(t *testing.T) {
	auth, plaintext, repo := newAuth(t, nil)

	id, err := auth.Authenticate(context.Background(), plaintext)
	require.NoError(t, err)

	want := repo.byKeyID[id.KeyID]
	assert.Equal(t, want.TenantID, id.TenantID)
	assert.Equal(t, want.DeviceID, id.DeviceID)
	assert.Equal(t, "EMB-DEV-001", id.MachineID)
	assert.Equal(t, want.KeyPK, id.KeyPK)
}

// Todos los modos de fallo colapsan al mismo error. El handler los traduce a un
// 403 identico: si el cliente pudiera distinguir "no existe" de "revocada", la
// respuesta seria un oraculo para enumerar keys.
func TestAuthenticateFailureModes(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour)

	t.Run("formato invalido", func(t *testing.T) {
		auth, _, _ := newAuth(t, nil)
		_, err := auth.Authenticate(context.Background(), "no-es-una-key")
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("key inexistente", func(t *testing.T) {
		auth, _, _ := newAuth(t, nil)
		_, err := auth.Authenticate(context.Background(), "emb_ffffffffffff_secreto")
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("secreto incorrecto", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, nil)
		keyID, _, err := domainapikeys.Parse(plaintext)
		require.NoError(t, err)
		_, err = auth.Authenticate(context.Background(), "emb_"+keyID+"_secreto-que-no-es")
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("key revocada", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.RevokedAt = &past })
		_, err := auth.Authenticate(context.Background(), plaintext)
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("key vencida", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.ExpiresAt = &past })
		_, err := auth.Authenticate(context.Background(), plaintext)
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})

	t.Run("device deshabilitado", func(t *testing.T) {
		auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.DeviceStatus = "DISABLED" })
		_, err := auth.Authenticate(context.Background(), plaintext)
		assert.ErrorIs(t, err, security.ErrAPIKeyInvalid)
	})
}

// Un error de Postgres que no sea ErrKeyNotFound es una falla de infraestructura,
// no una credencial invalida (I-1): debe propagarse intacto para que el
// middleware lo traduzca a 500 y el Edge reintente, en vez de a un 403 que
// haria pasar un outage de base por "key mala". Se comprueban las dos caras:
// que el error real llegue sin envolver, y que no se haya colapsado en
// ErrAPIKeyInvalid.
func TestAuthenticatePropagatesNonNotFoundRepositoryError(t *testing.T) {
	auth, plaintext, repo := newAuth(t, nil)
	repo.lookupErr = errRepoUnavailable

	_, err := auth.Authenticate(context.Background(), plaintext)

	assert.ErrorIs(t, err, errRepoUnavailable)
	assert.NotErrorIs(t, err, security.ErrAPIKeyInvalid)
}

func TestAuthenticateAcceptsFutureExpiry(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	auth, plaintext, _ := newAuth(t, func(c *domainapikeys.Credential) { c.ExpiresAt = &future })

	_, err := auth.Authenticate(context.Background(), plaintext)
	assert.NoError(t, err)
}

func TestDeviceIdentityRoundTripsThroughContext(t *testing.T) {
	want := &domainapikeys.DeviceIdentity{
		TenantID:  uuid.New(),
		DeviceID:  uuid.New(),
		MachineID: "EMB-DEV-001",
	}
	ctx := security.WithDeviceIdentity(context.Background(), want)

	got := security.DeviceIdentityFrom(ctx)
	require.NotNil(t, got)
	assert.Equal(t, want.TenantID, got.TenantID)
	assert.Equal(t, want.MachineID, got.MachineID)

	assert.Nil(t, security.DeviceIdentityFrom(context.Background()))
}

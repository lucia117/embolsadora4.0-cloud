package apikeys_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain/apikeys"
)

func TestGenerateProducesParseableKey(t *testing.T) {
	plaintext, keyID, hash, err := apikeys.Generate()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(plaintext, "emb_"), "la key debe llevar el prefijo emb_")
	assert.Len(t, keyID, apikeys.KeyIDLen)
	assert.Len(t, hash, sha256.Size)

	gotKeyID, secret, err := apikeys.Parse(plaintext)
	require.NoError(t, err)
	assert.Equal(t, keyID, gotKeyID)
	assert.True(t, apikeys.Matches(secret, hash))
}

func TestGenerateIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 500)
	for i := 0; i < 500; i++ {
		_, keyID, _, err := apikeys.Generate()
		require.NoError(t, err)
		_, dup := seen[keyID]
		require.False(t, dup, "key_id repetido en %d iteraciones", i)
		seen[keyID] = struct{}{}
	}
}

// El key_id es hexadecimal justamente para que no contenga "_", que es el
// separador. El secreto es base64url y SI puede contener "_": por eso Parse
// parte en el primer separador y no en el ultimo.
func TestParseHandlesUnderscoreInSecret(t *testing.T) {
	keyID, secret, err := apikeys.Parse("emb_0123456789ab_aa_bb_cc")
	require.NoError(t, err)
	assert.Equal(t, "0123456789ab", keyID)
	assert.Equal(t, "aa_bb_cc", secret)
}

func TestParseRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"vacio":              "",
		"sin prefijo":        "xxx_0123456789ab_secreto",
		"sin separador":      "emb_0123456789absecreto",
		"key_id corto":       "emb_0123_secreto",
		"key_id no hex":      "emb_zzzzzzzzzzzz_secreto",
		"secreto vacio":      "emb_0123456789ab_",
		"solo prefijo":       "emb_",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := apikeys.Parse(input)
			assert.ErrorIs(t, err, apikeys.ErrMalformedKey)
		})
	}
}

func TestMatchesRejectsWrongSecret(t *testing.T) {
	_, _, hash, err := apikeys.Generate()
	require.NoError(t, err)
	assert.False(t, apikeys.Matches("secreto-incorrecto", hash))
	assert.False(t, apikeys.Matches("", hash))
	assert.False(t, apikeys.Matches("x", nil))
}

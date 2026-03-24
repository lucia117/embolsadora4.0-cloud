package security_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/security"
)

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func makeJWKSServer(t *testing.T, key *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	pub := key.Public().(*rsa.PublicKey)
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid, issuer, audience string, expiry time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "user-123",
		"iss": issuer,
		"aud": []string{audience},
		"exp": expiry.Unix(),
	})
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestJWKSVerifier_ValidToken(t *testing.T) {
	key := generateRSAKey(t)
	srv := makeJWKSServer(t, key, "key1")
	defer srv.Close()

	verifier, err := security.NewJWKSVerifier(srv.URL, "https://issuer.example.com/auth/v1", "authenticated")
	require.NoError(t, err)

	tokenStr := signToken(t, key, "key1", "https://issuer.example.com/auth/v1", "authenticated", time.Now().Add(time.Hour))
	tok, err := verifier.Verify(tokenStr)
	require.NoError(t, err)
	assert.True(t, tok.Valid)
}

func TestJWKSVerifier_ExpiredToken(t *testing.T) {
	key := generateRSAKey(t)
	srv := makeJWKSServer(t, key, "key1")
	defer srv.Close()

	verifier, err := security.NewJWKSVerifier(srv.URL, "https://issuer.example.com/auth/v1", "authenticated")
	require.NoError(t, err)

	tokenStr := signToken(t, key, "key1", "https://issuer.example.com/auth/v1", "authenticated", time.Now().Add(-time.Hour))
	_, err = verifier.Verify(tokenStr)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, security.ErrJWKSUnavailable)
}

func TestJWKSVerifier_InvalidSignature(t *testing.T) {
	key1 := generateRSAKey(t)
	key2 := generateRSAKey(t)
	srv := makeJWKSServer(t, key1, "key1")
	defer srv.Close()

	verifier, err := security.NewJWKSVerifier(srv.URL, "https://issuer.example.com/auth/v1", "authenticated")
	require.NoError(t, err)

	// Token signed with key2 but JWKS only has key1
	tokenStr := signToken(t, key2, "key1", "https://issuer.example.com/auth/v1", "authenticated", time.Now().Add(time.Hour))
	_, err = verifier.Verify(tokenStr)
	assert.Error(t, err)
}

func TestJWKSVerifier_InvalidURL(t *testing.T) {
	_, err := security.NewJWKSVerifier("not-a-url", "https://issuer.example.com/auth/v1", "authenticated")
	assert.Error(t, err)
}

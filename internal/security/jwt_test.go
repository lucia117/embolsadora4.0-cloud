package security_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
		_ = json.NewEncoder(w).Encode(jwks)
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

// makeECJWKSServer publica una clave EC P-256, que es lo que usa el proyecto
// de Supabase de producción (alg ES256) desde la migración a claves asimétricas.
func makeECJWKSServer(t *testing.T, key *ecdsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	pub := key.Public().(*ecdsa.PublicKey)
	size := (pub.Curve.Params().BitSize + 7) / 8
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"use": "sig",
				"alg": "ES256",
				"crv": "P-256",
				"kid": kid,
				"x":   base64.RawURLEncoding.EncodeToString(pub.X.FillBytes(make([]byte, size))),
				"y":   base64.RawURLEncoding.EncodeToString(pub.Y.FillBytes(make([]byte, size))),
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func validClaims(issuer, audience string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "user-123",
		"iss": issuer,
		"aud": []string{audience},
		"exp": time.Now().Add(time.Hour).Unix(),
	}
}

const (
	testIssuer   = "https://issuer.example.com/auth/v1"
	testAudience = "authenticated"
)

func TestJWKSVerifier_ES256ValidToken(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	srv := makeECJWKSServer(t, key, "ec1")
	defer srv.Close()

	verifier, err := security.NewJWKSVerifier(srv.URL, testIssuer, testAudience)
	require.NoError(t, err)

	token := jwt.NewWithClaims(jwt.SigningMethodES256, validClaims(testIssuer, testAudience))
	token.Header["kid"] = "ec1"
	signed, err := token.SignedString(key)
	require.NoError(t, err)

	tok, err := verifier.Verify(signed)
	require.NoError(t, err)
	assert.True(t, tok.Valid)
}

// Confusión de algoritmo: el atacante firma con HS256 usando como secreto la
// clave pública (que el JWKS publica). Un verificador que no fija los métodos
// válidos podría aceptarlo.
func TestJWKSVerifier_RejectsHS256AlgorithmConfusion(t *testing.T) {
	key := generateRSAKey(t)
	srv := makeJWKSServer(t, key, "key1")
	defer srv.Close()

	verifier, err := security.NewJWKSVerifier(srv.URL, testIssuer, testAudience)
	require.NoError(t, err)

	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(testIssuer, testAudience))
	token.Header["kid"] = "key1"
	signed, err := token.SignedString(pubPEM)
	require.NoError(t, err)

	_, err = verifier.Verify(signed)
	require.Error(t, err)
	assert.ErrorIs(t, err, jwt.ErrTokenSignatureInvalid)
}

func TestJWKSVerifier_RejectsAlgNone(t *testing.T) {
	key := generateRSAKey(t)
	srv := makeJWKSServer(t, key, "key1")
	defer srv.Close()

	verifier, err := security.NewJWKSVerifier(srv.URL, testIssuer, testAudience)
	require.NoError(t, err)

	token := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims(testIssuer, testAudience))
	token.Header["kid"] = "key1"
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = verifier.Verify(signed)
	require.Error(t, err)
	assert.ErrorIs(t, err, jwt.ErrTokenSignatureInvalid)
}

func TestJWKSVerifier_InvalidURL(t *testing.T) {
	_, err := security.NewJWKSVerifier("not-a-url", "https://issuer.example.com/auth/v1", "authenticated")
	assert.Error(t, err)
}

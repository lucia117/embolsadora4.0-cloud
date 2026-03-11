package security

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	jwt "github.com/golang-jwt/jwt/v5"
)

// ErrJWKSUnavailable is returned when the JWKS endpoint cannot be reached.
var ErrJWKSUnavailable = errors.New("JWKS endpoint unavailable")

// Verifier verifies a JWT string and returns the parsed token.
type Verifier interface {
	Verify(tokenString string) (*jwt.Token, error)
}

// NewJWKSVerifier creates a Verifier backed by the Supabase JWKS endpoint.
// Uses automatic key caching and refresh on unknown kid.
func NewJWKSVerifier(jwksURL, issuer, audience string) (Verifier, error) {
	if _, err := url.ParseRequestURI(jwksURL); err != nil {
		return nil, fmt.Errorf("invalid JWKS URL: %w", err)
	}
	k, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrJWKSUnavailable, err)
	}
	return &jwksVerifier{keyfunc: k, issuer: issuer, audience: audience}, nil
}

type jwksVerifier struct {
	keyfunc  keyfunc.Keyfunc
	issuer   string
	audience string
}

func (v *jwksVerifier) Verify(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, v.keyfunc.Keyfunc,
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if isJWKSFetchError(err) {
			return nil, fmt.Errorf("%w: %s", ErrJWKSUnavailable, err)
		}
		return nil, err
	}
	return token, nil
}

func isJWKSFetchError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range []string{"failed to fetch", "connection refused", "no such host", "dial tcp", "context deadline exceeded", "JWKS"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// StubVerifier returns a no-op verifier placeholder for tests.
func StubVerifier() Verifier {
	return stubVerifier{}
}

type stubVerifier struct{}

func (stubVerifier) Verify(token string) (*jwt.Token, error) {
	return nil, nil
}

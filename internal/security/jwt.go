package security

import (
    jwt "github.com/golang-jwt/jwt/v5"
)

// TODO: Define JWT utilities and middleware wiring (no crypto details).

// Verifier verifies a JWT string and returns the parsed token.
type Verifier interface {
    Verify(token string) (*jwt.Token, error)
}

// StubVerifier returns a no-op verifier placeholder.
func StubVerifier() Verifier {
    return stubVerifier{}
}

type stubVerifier struct{}

func (stubVerifier) Verify(token string) (*jwt.Token, error) {
    // TODO: implement verification using configured public keys, issuer, audience, etc.
    return nil, nil
}

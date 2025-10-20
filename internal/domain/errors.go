package domain

import "errors"

// TODO: Define domain-specific error types and sentinel errors.

// ErrForbidden is returned when the operation lacks a tenant in context or violates tenant access rules.
var ErrForbidden = errors.New("forbidden")

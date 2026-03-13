package users

import "errors"

var (
	// ErrNotFound user not found or soft-deleted
	ErrNotFound = errors.New("user not found")

	// ErrEmailTaken email already exists in tenant
	ErrEmailTaken = errors.New("email already exists in this tenant")

	// ErrInvalidRole invalid role value
	ErrInvalidRole = errors.New("invalid role value")

	// ErrImmutableField attempting to modify immutable field
	ErrImmutableField = errors.New("field cannot be modified")

	// ErrTenantMismatch user doesn't belong to this tenant
	ErrTenantMismatch = errors.New("user does not belong to this tenant")

	// ErrValidation validation error in request
	ErrValidation = errors.New("validation error")
)

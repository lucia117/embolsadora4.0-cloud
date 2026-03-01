package domain

import "errors"

// TODO: Define domain-specific error types and sentinel errors.

// ErrForbidden is returned when the operation lacks a tenant in context or violates tenant access rules.
var ErrForbidden = errors.New("forbidden")

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrUserAlreadyHasActiveRole is returned when a user already has an active role in a tenant.
var ErrUserAlreadyHasActiveRole = errors.New("User already has an active role in this tenant. Use PUT to update.")

// ErrAssignmentNotFound is returned when a user-role assignment does not exist.
var ErrAssignmentNotFound = errors.New("user-role assignment not found")

// ErrInvalidRoleID is returned when the provided role_id does not exist in the roles table.
var ErrInvalidRoleID = errors.New("invalid roleId: role does not exist")

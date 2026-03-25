package dashboard_layouts

import "errors"

// ErrLayoutNotFound is returned when a layout does not exist for the given tenant.
// HTTP status: 404
var ErrLayoutNotFound = errors.New("layout not found")

// ErrDuplicateName is returned when a layout with the same name already exists in the tenant.
// HTTP status: 409 with error code "DUPLICATE_NAME"
var ErrDuplicateName = errors.New("layout name already exists for this tenant")

// ErrLimitReached is returned when a tenant has reached the maximum number of layouts (3).
// HTTP status: 403 with error code "LIMIT_REACHED"
var ErrLimitReached = errors.New("tenant has reached the maximum number of layouts")

// ErrCannotDeleteLastLayout is returned when attempting to delete the tenant's only layout.
// HTTP status: 400
var ErrCannotDeleteLastLayout = errors.New("cannot delete the only remaining layout")

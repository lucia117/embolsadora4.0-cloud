package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNoActiveAssignment is returned when a user has no active (or updatable) UTR in a tenant.
var ErrNoActiveAssignment = errors.New("user has no active role assignment in this tenant")

// UserRoleStatus represents the lifecycle state of a user-tenant-role assignment.
type UserRoleStatus string

const (
	UserRoleStatusActive    UserRoleStatus = "active"
	UserRoleStatusPending   UserRoleStatus = "pending"
	UserRoleStatusRevoked   UserRoleStatus = "revoked"
	UserRoleStatusSuspended UserRoleStatus = "suspended"
)

// UserTenantRole represents a single role assignment for a user within a tenant.
type UserTenantRole struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TenantID   uuid.UUID
	RoleID     *string        // nullable: pending assignments have no role yet
	Status     UserRoleStatus
	AssignedBy *uuid.UUID     // nullable: set when role is assigned
	AssignedAt *time.Time     // nullable: set when role is assigned
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UserTenantRoleDetail is returned by GET /user-roles?tenantId=...
// It embeds UserTenantRole plus role and user display fields resolved via
// JOIN in FindByTenant, so callers don't need a second round-trip to render
// a name (see docs/superpowers/specs/2026-07-21-tenant-user-roles-enrichment-design.md).
type UserTenantRoleDetail struct {
	UserTenantRole
	RoleName      string // "" when RoleID is nil or the role has no name
	UserEmail     string
	UserName      *string
	UserFirstName *string
	UserLastName  *string
}

// UserRoleWithContext is returned by GET /users/:userId/roles.
// It includes tenant and role display names via JOIN.
type UserRoleWithContext struct {
	TenantID   uuid.UUID
	TenantName string
	RoleID     string
	RoleName   string
	Status     UserRoleStatus
}

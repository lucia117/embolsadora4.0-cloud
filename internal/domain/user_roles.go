package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserRoleStatus represents the lifecycle state of a user-tenant-role assignment.
type UserRoleStatus string

const (
	UserRoleStatusActive  UserRoleStatus = "active"
	UserRoleStatusPending UserRoleStatus = "pending"
	UserRoleStatusRevoked UserRoleStatus = "revoked"
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

// UserRoleWithContext is returned by GET /users/:userId/roles.
// It includes tenant and role display names via JOIN.
type UserRoleWithContext struct {
	TenantID   uuid.UUID
	TenantName string
	RoleID     string
	RoleName   string
	Status     UserRoleStatus
}

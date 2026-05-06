package domain

import "time"

// UserStatus represents the lifecycle state of a user account.
type UserStatus string

const (
	UserStatusInvited  UserStatus = "invited"
	UserStatusActive   UserStatus = "active"
	UserStatusRevoked  UserStatus = "revoked"  // manual admin action
	UserStatusDisabled UserStatus = "disabled" // automatic system action
)

// User is the core domain entity for an authenticated application user.
type User struct {
	ID                     string
	SupabaseUserID         string
	Email                  string
	Name                   string
	Status                 UserStatus
	AuthProvider           string
	EmailVerifiedAt        *time.Time
	LastLoginAt            *time.Time
	PasswordChangeRequired bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

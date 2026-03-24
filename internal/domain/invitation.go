package domain

import "time"

// InvitationStatus represents the lifecycle state of a user invitation.
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRevoked  InvitationStatus = "revoked"
	InvitationStatusExpired  InvitationStatus = "expired"
)

// UserInvitation represents an invitation for a user to join a tenant.
type UserInvitation struct {
	ID        string
	TenantID  string
	Email     string
	RoleID    string
	Status    InvitationStatus
	InvitedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired returns true if the invitation has passed its expiration time.
func (i *UserInvitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

package usecases

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// InvitationActivator is an optional hook called during auto-provisioning to
// activate every pending invitation for the user's email (creates the
// user_tenant_roles membership with the invited role and activates the user).
type InvitationActivator interface {
	ActivatePendingInvitations(ctx context.Context, email, userID string) error
}

// AuthUsecase handles identity provisioning for authenticated users.
type AuthUsecase struct {
	userRepo users.UserRepository
}

func NewAuthUsecase(userRepo users.UserRepository) *AuthUsecase {
	return &AuthUsecase{userRepo: userRepo}
}

// ProvisionUser upserts the user record on first (and every) authenticated request.
// It is idempotent: concurrent calls with the same supabaseUserID produce exactly one record.
func (uc *AuthUsecase) ProvisionUser(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	return uc.userRepo.UpsertBySupabaseID(ctx, supabaseUserID, email)
}

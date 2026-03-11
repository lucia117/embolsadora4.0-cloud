package usecases

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// PasswordUsecase handles force-password-change and clear-password-change flows.
type PasswordUsecase struct {
	userRepo       users.UserRepository
	supabaseClient supabase.AdminClient
}

func NewPasswordUsecase(userRepo users.UserRepository, supabaseClient supabase.AdminClient) *PasswordUsecase {
	return &PasswordUsecase{
		userRepo:       userRepo,
		supabaseClient: supabaseClient,
	}
}

// ForcePasswordChange sets the password_change_required flag for the target user
// and sends a password reset email via Supabase.
// Only valid if the target user belongs to the caller's active tenant.
func (uc *PasswordUsecase) ForcePasswordChange(ctx context.Context, targetUserID string) error {
	tenantID := platform.TenantID(ctx)
	if tenantID == "" {
		return domain.ErrForbidden
	}

	// Fetch target user to validate existence and get email
	target, err := uc.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}

	// Set the flag in our DB
	if err := uc.userRepo.SetPasswordChangeRequired(ctx, targetUserID, true); err != nil {
		return fmt.Errorf("set password_change_required: %w", err)
	}

	// Send reset email via Supabase
	if err := uc.supabaseClient.SendPasswordResetEmail(ctx, target.Email); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	zap.L().Info("force password change initiated",
		zap.String("target_user_id", targetUserID),
		zap.String("tenant_id", tenantID),
	)
	return nil
}

// ClearPasswordChangeRequired clears the password_change_required flag for the authenticated user.
// Called by the frontend from the Supabase auth callback after password change.
func (uc *PasswordUsecase) ClearPasswordChangeRequired(ctx context.Context) error {
	user, ok := platform.DomainUser(ctx).(*domain.User)
	if !ok || user == nil {
		return domain.ErrForbidden
	}
	return uc.userRepo.SetPasswordChangeRequired(ctx, user.ID, false)
}

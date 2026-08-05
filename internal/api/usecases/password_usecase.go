package usecases

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// PasswordUsecase handles force-password-change and clear-password-change flows.
type PasswordUsecase struct {
	userRepo       users.UserRepository
	mgmtUserRepo   users.Repository // resolución de visibilidad cloakeada — ver ForcePasswordChange
	supabaseClient supabase.AdminClient
	appBaseURL     string
	log            *zap.Logger
}

func NewPasswordUsecase(userRepo users.UserRepository, mgmtUserRepo users.Repository, supabaseClient supabase.AdminClient, appBaseURL string, log *zap.Logger) *PasswordUsecase {
	if log == nil {
		log = zap.NewNop()
	}
	return &PasswordUsecase{
		userRepo:       userRepo,
		mgmtUserRepo:   mgmtUserRepo,
		supabaseClient: supabaseClient,
		appBaseURL:     appBaseURL,
		log:            log,
	}
}

// ForcePasswordChange sets the password_change_required flag for the target user
// and sends a password reset email via Supabase.
// Only valid if the target user belongs to the caller's active tenant.
//
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals. Se
// verifica DESPUÉS de confirmar existencia/membresía (para no cambiar el 404 vs
// 403 que ya distinguían "no existe" de "existe pero no es de este tenant" para
// el resto de la población) pero SIEMPRE antes de cualquier mutación o envío de
// mail: un miembro cuyo rol activo en este tenant es is_global (super_admin,
// tenant_manager) tiene que ser indistinguible de uno inexistente para un caller
// no-superadmin — mismo ErrNotFound, ningún efecto observable (ni el flag, ni el
// email). Se resuelve con el repo de management ya cloakeado (Task 5) en lugar
// del repo de auth, que no tiene noción de rol.
func (uc *PasswordUsecase) ForcePasswordChange(ctx context.Context, targetUserID string, includeGlobal bool) error {
	tenantID := platform.TenantID(ctx)
	if tenantID == "" {
		return domain.ErrForbidden
	}

	// Fetch target user to validate existence and get email
	target, err := uc.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}

	// Validate that target user belongs to the caller's tenant
	member, err := uc.userRepo.IsActiveMemberOfTenant(ctx, targetUserID, tenantID)
	if err != nil {
		return fmt.Errorf("check tenant membership: %w", err)
	}
	if !member {
		return domain.ErrForbidden
	}

	// Cloaking: si el rol activo del target en este tenant es is_global y el
	// caller no puede verlo, tratarlo como si no existiera. Nada de lo que
	// sigue (SetPasswordChangeRequired, email) debe ejecutarse.
	if _, err := uc.mgmtUserRepo.GetByID(ctx, tenantID, targetUserID, includeGlobal); err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("check target visibility: %w", err)
	}

	// Set the flag in our DB
	if err := uc.userRepo.SetPasswordChangeRequired(ctx, targetUserID, true); err != nil {
		return fmt.Errorf("set password_change_required: %w", err)
	}

	// Send reset email via Supabase
	if err := uc.supabaseClient.SendPasswordResetEmail(ctx, target.Email, callbackURL(ctx, uc.appBaseURL, tenantID)); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}

	uc.log.Info("force password change initiated",
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

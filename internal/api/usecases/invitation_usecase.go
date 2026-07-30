package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
	userRoles "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	"go.uber.org/zap"
)

// Log is the package-level logger for invitation use cases.
var Log *zap.Logger = zap.NewNop()

// InvitationUsecase handles invitation business logic.
type InvitationUsecase struct {
	invRepo        invitations.InvitationRepository
	userRepo       users.UserRepository
	userRoleRepo   userRoles.UserRoleRepository
	tenantRepo     TenantNameLookup
	roleRepo       RoleNameLookup
	supabaseClient supabase.AdminClient
	redis          *redis.Client
	appBaseURL     string
	rateLimitHour  int
}

func NewInvitationUsecase(
	invRepo invitations.InvitationRepository,
	userRepo users.UserRepository,
	userRoleRepo userRoles.UserRoleRepository,
	tenantRepo TenantNameLookup,
	roleRepo RoleNameLookup,
	supabaseClient supabase.AdminClient,
	redisClient *redis.Client,
	appBaseURL string,
	rateLimitHour int,
) *InvitationUsecase {
	return &InvitationUsecase{
		invRepo:        invRepo,
		userRepo:       userRepo,
		userRoleRepo:   userRoleRepo,
		tenantRepo:     tenantRepo,
		roleRepo:       roleRepo,
		supabaseClient: supabaseClient,
		redis:          redisClient,
		appBaseURL:     appBaseURL,
		rateLimitHour:  rateLimitHour,
	}
}

// CreateInvitation creates an invitation record and sends the invite email via Supabase.
func (uc *InvitationUsecase) CreateInvitation(ctx context.Context, email, roleID string) (*domain.UserInvitation, error) {
	tenantID := platform.TenantID(ctx)
	if tenantID == "" {
		return nil, domain.ErrForbidden
	}

	callerUser, ok := platform.DomainUser(ctx).(*domain.User)
	if !ok || callerUser == nil {
		return nil, domain.ErrForbidden
	}

	// Rate limit: max N invitations per tenant per hour using Redis
	if err := uc.checkRateLimit(ctx, tenantID); err != nil {
		return nil, err
	}

	// Check for existing pending invitation
	existing, err := uc.invRepo.GetPendingByEmailAndTenant(ctx, email, tenantID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrInvitationAlreadyPending
	}

	// Create DB record
	inv := &domain.UserInvitation{
		TenantID:  tenantID,
		Email:     email,
		RoleID:    roleID,
		InvitedBy: callerUser.ID,
	}
	created, err := uc.invRepo.Create(ctx, inv)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	// Send invite email via Supabase Admin API
	names := resolveInviteDisplayNames(ctx, uc.tenantRepo, uc.roleRepo, tenantID, roleID)
	inviteErr := uc.supabaseClient.InviteUserByEmail(ctx, supabase.InviteParams{
		Email:       email,
		RedirectTo:  callbackURL(ctx, uc.appBaseURL, tenantID),
		TenantName:  names.TenantName,
		InviterName: callerUser.Name,
		RoleName:    names.RoleName,
	})
	if inviteErr != nil {
		// Rollback: mark invitation as revoked since Supabase failed
		if rbErr := uc.invRepo.UpdateStatus(ctx, created.ID, domain.InvitationStatusRevoked); rbErr != nil {
			Log.Error("failed to rollback invitation after supabase error",
				zap.String("invitation_id", created.ID),
				zap.Error(rbErr),
			)
		}
		return nil, fmt.Errorf("supabase invite failed: %w", inviteErr)
	}

	Log.Info("invitation created",
		zap.String("tenant_id", tenantID),
		zap.String("email_domain", emailDomain(email)),
		zap.String("invitation_id", created.ID),
	)
	return created, nil
}

// ResendInvitation re-sends the invitation email for an existing pending invitation.
func (uc *InvitationUsecase) ResendInvitation(ctx context.Context, invID string) error {
	tenantID := platform.TenantID(ctx)
	inv, err := uc.invRepo.GetByID(ctx, invID, tenantID)
	if err != nil {
		return err
	}
	if inv.Status != domain.InvitationStatusPending {
		return domain.ErrInvitationNotPending
	}

	names := resolveInviteDisplayNames(ctx, uc.tenantRepo, uc.roleRepo, tenantID, inv.RoleID)

	// InviterName sale de quien esta reenviando, no de quien invito originalmente:
	// el nombre del invitador original exigiria una consulta mas por un dato
	// decorativo, y quien reenvia es igual de valido como referencia para el invitado.
	var inviterName string
	if u, ok := platform.DomainUser(ctx).(*domain.User); ok && u != nil {
		inviterName = u.Name
	}

	if err := uc.supabaseClient.InviteUserByEmail(ctx, supabase.InviteParams{
		Email:       inv.Email,
		RedirectTo:  callbackURL(ctx, uc.appBaseURL, tenantID),
		TenantName:  names.TenantName,
		InviterName: inviterName,
		RoleName:    names.RoleName,
	}); err != nil {
		return err
	}
	Log.Info("invitation resent", zap.String("invitation_id", invID), zap.String("email_domain", emailDomain(inv.Email)))
	return nil
}

// RevokeInvitation soft-deletes an invitation by setting its status to revoked.
func (uc *InvitationUsecase) RevokeInvitation(ctx context.Context, invID string) (*domain.UserInvitation, error) {
	tenantID := platform.TenantID(ctx)
	inv, err := uc.invRepo.GetByID(ctx, invID, tenantID)
	if err != nil {
		return nil, err
	}

	if err := uc.invRepo.UpdateStatus(ctx, invID, domain.InvitationStatusRevoked); err != nil {
		return nil, err
	}
	inv.Status = domain.InvitationStatusRevoked
	Log.Info("invitation revoked", zap.String("invitation_id", invID), zap.String("tenant_id", tenantID))
	return inv, nil
}

// ListInvitations returns all invitations for the current tenant, optionally filtered by status.
func (uc *InvitationUsecase) ListInvitations(ctx context.Context, status *string) ([]domain.UserInvitation, error) {
	tenantID := platform.TenantID(ctx)
	return uc.invRepo.ListByTenant(ctx, tenantID, status)
}

// ActivatePendingInvitations is called by JWTAuth after provisioning a user whose
// status is still 'invited'. For every pending invitation matching the email it
// creates the user_tenant_roles membership (with the invited role) and marks the
// invitation accepted; if at least one activates, the user becomes active.
// Implements the InvitationActivator interface.
func (uc *InvitationUsecase) ActivatePendingInvitations(ctx context.Context, email, userID string) error {
	invs, err := uc.invRepo.ListPendingByEmail(ctx, email)
	if err != nil {
		return err
	}
	if len(invs) == 0 {
		return nil // nothing to activate
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id %q: %w", userID, err)
	}

	activated := 0
	for i := range invs {
		inv := &invs[i]
		if inv.IsExpired() {
			if err := uc.invRepo.UpdateStatus(ctx, inv.ID, domain.InvitationStatusExpired); err != nil {
				Log.Warn("failed to expire invitation", zap.String("invitation_id", inv.ID), zap.Error(err))
			}
			continue
		}

		tenantUUID, err := uuid.Parse(inv.TenantID)
		if err != nil {
			Log.Warn("invitation with invalid tenant id", zap.String("invitation_id", inv.ID), zap.Error(err))
			continue
		}

		now := time.Now().UTC()
		utr := &domain.UserTenantRole{
			ID:         uuid.New(),
			UserID:     userUUID,
			TenantID:   tenantUUID,
			RoleID:     &inv.RoleID,
			Status:     domain.UserRoleStatusActive,
			AssignedAt: &now,
		}
		if invitedBy, err := uuid.Parse(inv.InvitedBy); err == nil {
			utr.AssignedBy = &invitedBy
		}

		if _, err := uc.userRoleRepo.Create(ctx, utr); err != nil &&
			!errors.Is(err, domain.ErrUserAlreadyHasActiveRole) {
			Log.Warn("failed to create membership from invitation",
				zap.String("invitation_id", inv.ID),
				zap.String("tenant_id", inv.TenantID),
				zap.Error(err),
			)
			continue
		}

		if err := uc.invRepo.UpdateStatus(ctx, inv.ID, domain.InvitationStatusAccepted); err != nil {
			Log.Warn("failed to mark invitation accepted", zap.String("invitation_id", inv.ID), zap.Error(err))
		}
		activated++
	}

	if activated == 0 {
		return nil
	}

	// Invited users come from Supabase's invite flow and never set a password;
	// once their one-time invite session expires, email+password login is
	// impossible without one. Force the change-password screen on next load
	// so they set one while we know they're still authenticated.
	//
	// Both writes are best-effort side effects of an otherwise-successful
	// activation: run them independently (a failure in one must not skip the
	// other) and join their errors. Like SetStatus below, the caller
	// (JWTAuth) only logs this error and does not abort the request — the
	// user is mid-login and already provisioned; failing the request over a
	// secondary flag write would be worse than a user who occasionally
	// doesn't get prompted to set a password until their next visit.
	statusErr := uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive)
	pwErr := uc.userRepo.SetPasswordChangeRequired(ctx, userID, true)
	return errors.Join(statusErr, pwErr)
}

// emailDomain returns only the domain part of an email for safe logging (e.g. "user@example.com" → "@example.com").
func emailDomain(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[i:]
		}
	}
	return "[invalid]"
}

func (uc *InvitationUsecase) checkRateLimit(ctx context.Context, tenantID string) error {
	if uc.redis == nil {
		// Redis unavailable: fail open (rate limiting disabled)
		return nil
	}
	key := fmt.Sprintf("invitations:ratelimit:%s:%s", tenantID, time.Now().UTC().Format("2006-01-02-15"))
	count, err := uc.redis.Incr(ctx, key).Result()
	if err != nil {
		return nil // fail open
	}
	if count == 1 {
		uc.redis.Expire(ctx, key, time.Hour)
	}
	if int(count) > uc.rateLimitHour {
		return domain.ErrInvitationRateLimitExceeded
	}
	return nil
}

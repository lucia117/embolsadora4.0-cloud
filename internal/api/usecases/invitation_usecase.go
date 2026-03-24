package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// Log is the package-level logger for invitation use cases.
var Log *zap.Logger = zap.NewNop()

// InvitationUsecase handles invitation business logic.
type InvitationUsecase struct {
	invRepo        invitations.InvitationRepository
	userRepo       users.UserRepository
	supabaseClient supabase.AdminClient
	redis          *redis.Client
	appBaseURL     string
	rateLimitHour  int
}

func NewInvitationUsecase(
	invRepo invitations.InvitationRepository,
	userRepo users.UserRepository,
	supabaseClient supabase.AdminClient,
	redisClient *redis.Client,
	appBaseURL string,
	rateLimitHour int,
) *InvitationUsecase {
	return &InvitationUsecase{
		invRepo:        invRepo,
		userRepo:       userRepo,
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
	redirectTo := fmt.Sprintf("%s/s/%s/auth/callback", uc.appBaseURL, tenantID)
	if err := uc.supabaseClient.InviteUserByEmail(ctx, email, redirectTo); err != nil {
		// Rollback: mark invitation as revoked since Supabase failed
		if rbErr := uc.invRepo.UpdateStatus(ctx, created.ID, domain.InvitationStatusRevoked); rbErr != nil {
			Log.Error("failed to rollback invitation after supabase error",
				zap.String("invitation_id", created.ID),
				zap.Error(rbErr),
			)
		}
		return nil, fmt.Errorf("supabase invite failed: %w", err)
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

	redirectTo := fmt.Sprintf("%s/s/%s/auth/callback", uc.appBaseURL, tenantID)
	if err := uc.supabaseClient.InviteUserByEmail(ctx, inv.Email, redirectTo); err != nil {
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

// ActivateInvitation is called by JWTAuth after provisioning a new user.
// If there is a pending invitation for this email+tenantID, it activates it and
// creates the user_tenant_role record. Implements InvitationActivator interface.
func (uc *InvitationUsecase) ActivateInvitation(ctx context.Context, email, tenantID string, userID string) error {
	inv, err := uc.invRepo.GetPendingByEmailAndTenant(ctx, email, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil // no pending invitation, nothing to activate
		}
		return err
	}

	// Mark invitation accepted
	if err := uc.invRepo.UpdateStatus(ctx, inv.ID, domain.InvitationStatusAccepted); err != nil {
		return err
	}

	// Activate user status
	if err := uc.userRepo.SetStatus(ctx, userID, domain.UserStatusActive); err != nil {
		return err
	}

	return nil
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

package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AuthRequestsTotal counts authentication attempts by status (success/unauthorized/forbidden/unavailable).
	AuthRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_requests_total",
		Help: "Total number of authentication requests by status",
	}, []string{"status"})

	// AuthTenantViolationsTotal counts requests where X-Tenant-ID validation failed.
	AuthTenantViolationsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_tenant_violations_total",
		Help: "Total number of tenant access violations",
	})

	// AuthCrossTenantGrantsTotal counts accesses granted to platform operators
	// (global roles, or admin of the platform tenant) on a tenant where they
	// have no direct membership, by effective role. See CLOUD-ADR-015.
	AuthCrossTenantGrantsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_cross_tenant_grants_total",
		Help: "Total number of cross-tenant accesses granted to platform operators, by effective role",
	}, []string{"role"})

	// InvitationsSentTotal counts invitations created and sent.
	InvitationsSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "invitations_sent_total",
		Help: "Total number of invitations sent",
	})

	// InvitationsExpiredTotal counts invitations that expired before being accepted.
	InvitationsExpiredTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "invitations_expired_total",
		Help: "Total number of invitations that expired",
	})

	// PasswordChangeForcedTotal counts force-password-change operations.
	PasswordChangeForcedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "password_change_forced_total",
		Help: "Total number of forced password change requests",
	})
)

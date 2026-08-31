package security

import (
	"context"
	"errors"
	"testing"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestIsCrossTenantRole(t *testing.T) {
	cases := []struct {
		name     string
		isGlobal bool
		want     bool
	}{
		{"is_global=true es cross-tenant", true, true},
		{"is_global=false no es cross-tenant", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := WithRoleContext(context.Background(), RoleContext{IsGlobal: c.isGlobal})
			if got := IsCrossTenantRole(ctx); got != c.want {
				t.Errorf("IsCrossTenantRole() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsCrossTenantRoleSinContexto(t *testing.T) {
	if IsCrossTenantRole(context.Background()) {
		t.Error("sin RoleContext debe devolver false (fail-closed)")
	}
}

func TestEffectiveRole(t *testing.T) {
	cases := []struct {
		name             string
		roleID           string
		isPlatformTenant bool
		want             string
	}{
		{"admin en tenant plataforma asciende", "admin", true, "platform_admin"},
		{"admin en tenant cliente no asciende", "admin", false, "admin"},
		{"super_admin no cambia en plataforma", "super_admin", true, "super_admin"},
		{"super_admin no cambia fuera de plataforma", "super_admin", false, "super_admin"},
		{"tenant_manager no cambia", "tenant_manager", true, "tenant_manager"},
		{"operario no cambia en plataforma", "operario", true, "operario"},
		{"cliente_admin no cambia en plataforma", "cliente_admin", true, "cliente_admin"},
		{"rol vacío no cambia", "", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EffectiveRole(c.roleID, c.isPlatformTenant); got != c.want {
				t.Errorf("EffectiveRole(%q, %v) = %q, want %q", c.roleID, c.isPlatformTenant, got, c.want)
			}
		})
	}
}

func TestCanSeePlatformInternals(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{"super_admin", true},
		{"tenant_manager", false},
		{"platform_admin", false},
		{"admin", false},
		{"operario", false},
		{"", false},
	}
	for _, c := range cases {
		ctx := WithRole(context.Background(), c.role)
		if got := CanSeePlatformInternals(ctx); got != c.want {
			t.Errorf("CanSeePlatformInternals(role=%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

func TestCanSeePlatformInternalsSinRolEnContexto(t *testing.T) {
	if CanSeePlatformInternals(context.Background()) {
		t.Error("sin rol en contexto debe devolver false (fail-closed)")
	}
}

func TestCan(t *testing.T) {
	ctx := WithRoleContext(context.Background(), RoleContext{
		Name:        "cliente_admin",
		Permissions: []string{"perm_dashboard", "perm_users_view"},
	})
	if err := Can(ctx, "perm_dashboard"); err != nil {
		t.Errorf("Can(perm_dashboard) = %v, want nil", err)
	}
	if err := Can(ctx, "perm_users_manage"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("Can(perm_users_manage) = %v, want ErrForbidden", err)
	}
}

func TestCanSinRolEnContexto(t *testing.T) {
	if err := Can(context.Background(), "perm_dashboard"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("Can() sin rol = %v, want ErrForbidden (fail-closed)", err)
	}
}

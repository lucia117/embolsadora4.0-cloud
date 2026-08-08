package security

import (
	"context"
	"testing"
)

func TestIsCrossTenantRole(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{"super_admin", true},
		{"tenant_manager", true},
		{"platform_admin", true},
		{"admin", false},
		{"operario", false},
		{"cliente_admin", false},
		{"cliente_operario", false},
		{"", false},
		{"unknown_role", false},
	}
	for _, c := range cases {
		if got := IsCrossTenantRole(c.role); got != c.want {
			t.Errorf("IsCrossTenantRole(%q) = %v, want %v", c.role, got, c.want)
		}
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

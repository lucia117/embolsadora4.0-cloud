package security

import "testing"

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

package apporigin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
)

const fallback = "https://embolsadora.site"

func TestResolve(t *testing.T) {
	list := apporigin.Parse("https://embolsadora.site,http://localhost:3000,https://*.vercel.app")

	tests := []struct {
		name      string
		candidate string
		want      string
		wantOK    bool
	}{
		{"origin exacto de produccion", "https://embolsadora.site", "https://embolsadora.site", true},
		{"localhost con puerto", "http://localhost:3000", "http://localhost:3000", true},
		{"barra final se normaliza", "https://embolsadora.site/", "https://embolsadora.site", true},
		{"mayusculas se normalizan", "HTTPS://EMBOLSADORA.SITE", "https://embolsadora.site", true},
		{"el path se descarta", "https://embolsadora.site/s/demo/auth/callback", "https://embolsadora.site", true},
		{"espacios alrededor se recortan", "  https://embolsadora.site  ", "https://embolsadora.site", true},
		{"ataque por sufijo se rechaza", "https://embolsadora.site.atacante.com", fallback, false},
		{"ataque por path se rechaza", "https://atacante.com/embolsadora.site", fallback, false},
		{"esquema incorrecto se rechaza", "http://embolsadora.site", fallback, false},
		{"puerto incorrecto se rechaza", "http://localhost:4000", fallback, false},
		{"preview de vercel se acepta", "https://embolsadora-abc123.vercel.app", "https://embolsadora-abc123.vercel.app", true},
		{"dominio pelado del wildcard se rechaza", "https://vercel.app", fallback, false},
		{"wildcard sobre http se rechaza", "http://x.vercel.app", fallback, false},
		{"lookalike del wildcard se rechaza", "https://evilvercel.app", fallback, false},
		{"vacio cae al fallback", "", fallback, false},
		{"url relativa cae al fallback", "/s/demo/auth/callback", fallback, false},
		{"esquema javascript se rechaza", "javascript:alert(1)", fallback, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := list.Resolve(tc.candidate, fallback)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolve_AllowListVaciaSiempreCaeAlFallback(t *testing.T) {
	list := apporigin.Parse("")
	got, ok := list.Resolve("https://embolsadora.site", fallback)
	assert.False(t, ok)
	assert.Equal(t, fallback, got, "sin allow-list configurada no se confia en ningun header")
}

func TestParse_EntradasInvalidasSeIgnoran(t *testing.T) {
	list := apporigin.Parse("no-es-una-url, ,https://embolsadora.site,ftp://embolsadora.site")
	assert.True(t, list.Allows("https://embolsadora.site"))
	assert.False(t, list.Allows("ftp://embolsadora.site"))
}

// TestCounts_ReflejanLoQueRealmenteSeCargo respalda el log de arranque: las
// entradas invalidas no se cuentan, asi que un contador en cero es la señal
// de que la allow-list quedo inerte.
func TestCounts_ReflejanLoQueRealmenteSeCargo(t *testing.T) {
	list := apporigin.Parse("https://embolsadora.site,http://localhost:3000,https://*.vercel.app,no-es-una-url, ")
	assert.Equal(t, 2, list.ExactCount())
	assert.Equal(t, 1, list.WildcardCount())

	vacia := apporigin.Parse("")
	assert.Equal(t, 0, vacia.ExactCount())
	assert.Equal(t, 0, vacia.WildcardCount())
}

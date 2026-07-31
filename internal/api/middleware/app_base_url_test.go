package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
)

const fallbackBase = "https://embolsadora.site"

// runWithHeader ejecuta un request a traves del middleware y devuelve el base
// URL que quedo en el contexto del handler.
func runWithHeader(t *testing.T, header string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	allow := apporigin.Parse("https://embolsadora.site,http://localhost:3000")
	var seen string

	r := gin.New()
	r.Use(apimw.AppBaseURLFromHeader(allow, fallbackBase))
	r.GET("/probe", func(c *gin.Context) {
		seen = platform.AppBaseURL(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if header != "" {
		req.Header.Set("X-App-Base-URL", header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "el middleware nunca debe abortar el request")
	return seen
}

func TestAppBaseURLFromHeader_OriginPermitido(t *testing.T) {
	assert.Equal(t, "http://localhost:3000", runWithHeader(t, "http://localhost:3000"))
}

func TestAppBaseURLFromHeader_OriginRechazadoCaeAlFallback(t *testing.T) {
	assert.Equal(t, fallbackBase, runWithHeader(t, "https://embolsadora.site.atacante.com"))
}

func TestAppBaseURLFromHeader_SinHeaderCaeAlFallback(t *testing.T) {
	assert.Equal(t, fallbackBase, runWithHeader(t, ""))
}

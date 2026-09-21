package me

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/api/usecases"
)

// TestHandle_NoDomainUserInContext_Returns403 es la única rama de GetMe
// testeable sin Postgres real: el usecase valida platform.DomainUser(ctx)
// ANTES de tocar la DB (ver me_usecase.go), así que db=nil es seguro acá. El
// camino feliz (tenant/rol/permissions desde `roles.permissions` -- el mismo
// código del incidente RBAC del 2026-08-18) necesita un test de integración
// con DATABASE_URL, fuera del alcance de este batch.
func TestHandle_NoDomainUserInContext_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := usecases.NewMeUsecase(nil)
	h := NewHandler(uc)
	r := gin.New()
	r.GET("/api/v1/me", h.Handle)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "forbidden")
}

package dashboards

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// HandleError mapea un error de domain/metrics o app/dashboards al envelope
// {success:false, error, code}. Todos los guardrails de la spec son 400;
// QUERY_TIMEOUT (fork/C4) es 504; cualquier otra cosa es 500.
func HandleError(c *gin.Context, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		status := http.StatusBadRequest
		if ve.Code == domain.CodeQueryTimeout {
			status = http.StatusGatewayTimeout
		}
		c.JSON(status, gin.H{"success": false, "error": ve.Message, "code": ve.Code})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno", "code": "INTERNAL_ERROR"})
}

// normalizedTenantID reparsea el X-Tenant-ID del contexto (platform.TenantID,
// el valor crudo del header) como un UUID canonico en minusculas. I1: el
// header pasa JWTAuth/membership/RBAC case-insensitive (columna uuid de
// Postgres), pero el $match de Mongo en repo/mongo/metrics es byte-exacto
// sobre un tenantId guardado siempre en minusculas -- sin esta normalizacion,
// un X-Tenant-ID en mayusculas pasa todos los checks y despues devuelve 200
// con resultados vacios en los 3 endpoints de dashboards.
func normalizedTenantID(ctx context.Context) (string, bool) {
	raw := platform.TenantID(ctx)
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

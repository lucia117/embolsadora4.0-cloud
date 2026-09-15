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

// normalizedTenantID reparsea platform.TenantID(ctx) como un UUID canonico en
// minusculas. I1: middleware.TenantFromHeader ya normaliza el X-Tenant-ID
// antes de guardarlo en contexto (ver comentario ahi), asi que esto es
// revalidacion defensiva, no la fix primaria -- pero repo/mongo/metrics hace
// un $match byte-exacto contra un tenantId guardado siempre en minusculas, y
// esta funcion tambien sirve para rechazar (ok=false) un contexto sin tenant
// valido, algo que los 3 handlers de dashboards necesitan de todos modos.
func normalizedTenantID(ctx context.Context) (string, bool) {
	raw := platform.TenantID(ctx)
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

package dashboards

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
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
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno"})
}

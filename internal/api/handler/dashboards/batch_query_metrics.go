package dashboards

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboards/dto"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

type batchQueryItemRequest struct {
	ID string `json:"id"`
	dto.QueryRequest
}

type batchQueryRequest struct {
	Queries []batchQueryItemRequest `json:"queries"`
}

// BatchQueryMetrics ejecuta N queries en paralelo (app.Service.Batch) y
// correlaciona los resultados por id. Errores a nivel batch (cantidad de
// items, ids duplicados -- domain.ValidateBatchIDs) van por HandleError,
// igual que un error de /query; errores por item van dentro de cada
// elemento de "results", no abortan el resto (spec, "Endpoint batch").
func BatchQueryMetrics(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantID(c.Request.Context())
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID invalido o ausente", "code": "INVALID_PARAMS"})
			return
		}

		var req batchQueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "JSON malformado", "code": "INVALID_PARAMS"})
			return
		}

		items := make([]app.BatchItem, len(req.Queries))
		for i, q := range req.Queries {
			items[i] = app.BatchItem{ID: q.ID, Query: q.QueryRequest.ToDomain()}
		}

		results, err := service.Batch(c.Request.Context(), tenantID, items, time.Now())
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"results": batchResultsToResponse(results)}})
	}
}

func batchResultsToResponse(results []app.BatchItemResult) []gin.H {
	out := make([]gin.H, 0, len(results))
	for _, r := range results {
		if r.Err != nil {
			var ve *domain.ValidationError
			code := "INTERNAL_ERROR"
			message := "error interno"
			if errors.As(r.Err, &ve) {
				code = ve.Code
				message = ve.Message
			}
			out = append(out, gin.H{"id": r.ID, "success": false, "error": message, "code": code})
			continue
		}
		out = append(out, gin.H{"id": r.ID, "success": true, "data": dto.QueryResultToResponse(r.Result)})
	}
	return out
}

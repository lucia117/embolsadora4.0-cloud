package consumers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/consumers/dto"
	"github.com/tu-org/embolsadora-api/internal/security"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// HandlerConfig son los limites de la ingesta.
type HandlerConfig struct {
	MaxBodyBytes int64
	MaxEvents    int
}

// IngestEvents recibe un batch de eventos del Edge Pi Service.
//
// Sobre los codigos de respuesta, que no son intercambiables:
//   - 400 significa "todo el batch es irrecuperable" y el Edge manda hasta 1000
//     eventos a DEAD. Queda reservado a requests rotos a nivel de SOBRE.
//   - Los problemas de eventos individuales van por 200 + errors[] (I-2).
//   - 500 es "no pude guardarlo ahora"; el Edge reintenta con backoff.
func IngestEvents(svc *ingestapp.Service, cfg HandlerConfig, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := security.DeviceIdentityFrom(c.Request.Context())
		if identity == nil {
			// Solo puede pasar si el middleware APIKeyAuth no esta cableado.
			log.Error("IngestEvents sin identidad en contexto: APIKeyAuth no esta en la cadena")
			c.JSON(http.StatusInternalServerError, gin.H{"message": "error interno"})
			return
		}

		// El tope de bytes se aplica ANTES de parsear: un body de 2 GB no debe
		// poder consumir memoria mientras se deserializa.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxBodyBytes)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				log.Warn("batch por encima del tope de bytes",
					zap.String("machine_id", identity.MachineID),
					zap.Int64("limite", cfg.MaxBodyBytes))
				c.JSON(http.StatusBadRequest, gin.H{"message": "body demasiado grande"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"message": "no se pudo leer el body"})
			return
		}

		var req dto.BatchEventsRequest
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "JSON malformado"})
			return
		}

		switch {
		case len(req.Events) == 0:
			c.JSON(http.StatusBadRequest, gin.H{"message": "events es obligatorio y no puede estar vacio"})
			return
		case len(req.Events) > cfg.MaxEvents:
			c.JSON(http.StatusBadRequest, gin.H{"message": "events excede el maximo de elementos"})
			return
		}

		// El Idempotency-Key se registra para trazabilidad pero no decide nada
		// (D-6): la garantia fuerte la da el indice unico sobre eventId. Una
		// cache de respuestas seria una segunda fuente de verdad con casos borde
		// —request en vuelo, Redis caido— para un beneficio acotado.
		idemKey := c.GetHeader("Idempotency-Key")
		if len(idemKey) > 64 {
			idemKey = idemKey[:64]
		}

		res, err := svc.IngestBatch(c.Request.Context(), identity.ToDeviceContext(), req.Events)
		if err != nil {
			// I-1: infraestructura caida es 500, jamas 400 ni INVALID_SCHEMA.
			// Un 400 aca convertiria una caida de diez minutos en perdida
			// permanente de datos.
			telemetry.IngestBatchesTotal.WithLabelValues("error").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "no se pudo persistir el batch"})
			return
		}

		telemetry.IngestBatchesTotal.WithLabelValues("ok").Inc()
		telemetry.IngestEventsAcceptedTotal.Add(float64(res.Accepted))
		for _, e := range res.Errors {
			telemetry.IngestEventsRejectedTotal.WithLabelValues(e.Code).Inc()
		}

		log.Info("batch ingerido",
			zap.String("machine_id", identity.MachineID),
			zap.String("tenant_id", identity.TenantID.String()),
			zap.String("idempotency_key", idemKey),
			zap.Int("recibidos", len(req.Events)),
			zap.Int("aceptados", res.Accepted),
			zap.Int("rechazados", res.Rejected),
		)

		c.JSON(http.StatusOK, dto.BatchEventsResponse{Data: res})
	}
}

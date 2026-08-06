package ingest

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.uber.org/zap"

	domain "github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// Service orquesta la ingesta de un batch.
type Service struct {
	repo         domain.Repository
	log          *zap.Logger
	now          func() time.Time
	mongoTimeout time.Duration
}

// NewService construye el service de ingesta.
//
// mongoTimeout acota CADA llamada a repo.InsertMany (item 6 de la review
// final): sin un limite propio, un primario de Mongo colgado deja el
// goroutine del request esperando indefinidamente en vez de devolver el 500
// que el Edge sabe reintentar (I-1). "Colgado" no es lo mismo que "caido":
// un primario caido falla rapido (conexion rechazada); uno colgado no falla
// nunca por si solo, y es ese caso el que necesita un limite explicito.
func NewService(repo domain.Repository, log *zap.Logger, mongoTimeout time.Duration) *Service {
	return &Service{repo: repo, log: log, now: func() time.Time { return time.Now().UTC() }, mongoTimeout: mongoTimeout}
}

// IngestBatch valida el sobre de cada evento y persiste los validos.
//
// Devuelve error SOLO si la escritura fallo entera (Mongo inalcanzable): ese
// caso lo traduce el handler a HTTP 500 y el Edge reintenta con backoff. Todo
// lo demas —eventos invalidos, duplicados, fallos parciales— viaja dentro del
// Result como un 200 con errors[], porque un 400 mandaria el batch entero a
// DEAD (invariante I-2).
func (s *Service) IngestBatch(ctx context.Context, dev domain.DeviceContext, raw []json.RawMessage) (domain.Result, error) {
	now := s.now()

	valid := make([]domain.Measurement, 0, len(raw))
	// origIndex[j] es la posicion en `raw` del documento que quedo en valid[j].
	// Esta correspondencia es lo unico que sostiene el invariante I-3: el repo
	// reporta indices sobre `valid`, y el Edge espera indices sobre `raw`.
	origIndex := make([]int, 0, len(raw))
	errs := make([]domain.EventError, 0)

	for i, item := range raw {
		m, evErr, skew := ValidateEvent(item, dev, now)
		if evErr != nil {
			evErr.Index = i
			errs = append(errs, *evErr)
			continue
		}
		if skew != domain.SkewReasonNone {
			// Aceptar un kind o schemaVersion que este build no conoce no
			// puede ser silencioso: es la unica senal de que el Edge quedo
			// mas adelante que el cloud desplegado (ver comentario de
			// SkewReason).
			telemetry.IngestVersionSkewTotal.WithLabelValues(string(skew)).Inc()
			s.log.Warn("evento aceptado con version skew",
				zap.String("reason", string(skew)),
				zap.String("tenant_id", dev.TenantID),
				zap.String("machine_id", dev.MachineID),
				zap.String("event_id", m.EventID),
				zap.String("kind", m.Kind),
				zap.Int("schema_version", m.SchemaVersion),
			)
		}
		valid = append(valid, m)
		origIndex = append(origIndex, i)
	}

	if len(valid) > 0 {
		insertCtx := ctx
		if s.mongoTimeout > 0 {
			var cancel context.CancelFunc
			insertCtx, cancel = context.WithTimeout(ctx, s.mongoTimeout)
			defer cancel()
		}

		start := time.Now()
		report, err := s.repo.InsertMany(insertCtx, valid)
		telemetry.IngestBatchDuration.Observe(time.Since(start).Seconds())
		if err != nil {
			s.log.Error("fallo total al persistir el batch",
				zap.Error(err),
				zap.String("tenant_id", dev.TenantID),
				zap.String("machine_id", dev.MachineID),
				zap.Int("eventos", len(valid)),
			)
			return domain.Result{}, err
		}

		for j := range valid {
			if _, dup := report.Duplicated[j]; dup {
				errs = append(errs, domain.EventError{
					Index:   origIndex[j],
					Code:    domain.CodeDuplicate,
					Message: "el evento ya habia sido ingerido",
				})
				continue
			}
			if msg, bad := report.Failed[j]; bad {
				// I-1: esto es infraestructura, no payload. El codigo tiene que
				// ser retriable o el Edge dara el evento por muerto.
				errs = append(errs, domain.EventError{
					Index:   origIndex[j],
					Code:    domain.CodeStorageUnavailable,
					Message: msg,
				})
			}
		}
	}

	// Orden por indice: la respuesta es determinista y el mapa de errores del
	// repo no impone su orden de iteracion, que en Go es aleatorio.
	sort.Slice(errs, func(a, b int) bool { return errs[a].Index < errs[b].Index })

	res := domain.Result{
		// I-4 por construccion: cada evento rechazado aporta exactamente una
		// entrada a errs, asi que accepted + rejected == len(raw) siempre.
		Accepted: len(raw) - len(errs),
		Rejected: len(errs),
	}
	if len(errs) > 0 {
		res.Errors = errs
	}
	return res, nil
}

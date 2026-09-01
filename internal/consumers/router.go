package consumers

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
)

// Deps son las dependencias de la superficie de consumers.
type Deps struct {
	Ingest *ingestapp.Service
	Log    *zap.Logger
}

// Config son los limites de la ingesta.
type Config struct {
	MaxBodyBytes int64
	MaxEvents    int
}

// RegisterConsumerRoutes registra las rutas bajo el grupo dado
// (ej. /api/v1/consumers). El grupo ya trae APIKeyAuth y RateLimit aplicados.
func RegisterConsumerRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {
	g.POST("/events", IngestEvents(deps.Ingest, HandlerConfig{
		MaxBodyBytes: cfg.MaxBodyBytes,
		MaxEvents:    cfg.MaxEvents,
	}, deps.Log))

	// Sigue siendo 501: el feature 002 del Edge no lo usa (events.KindHeartbeat
	// esta reservado pero no se emite). Fuera del alcance de este plan.
	g.POST("/heartbeat", Heartbeat)
}

package consumers

import (
	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/domain/aas"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type Deps struct {
	APIKeys   security.APIKeyLookup
	ShellRepo aas.ShellRepository // nil if MongoDB is unavailable
}

// TODO: fill in configuration as needed.
type Config struct{}

// RegisterConsumerRoutes wires Consumers routes under the provided group (e.g., /api/v1/consumers).
func RegisterConsumerRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {
	// Events batch ingestion
	g.POST("/events", IngestEvents)

	// Device heartbeat
	g.POST("/heartbeat", Heartbeat)

	// AAS shell read — IoT device fetches its own digital twin (API key auth, no JWT).
	// Only registered when APIKeys is configured; otherwise the route is not exposed (fail-secure).
	if deps.APIKeys != nil {
		g.GET("/shells/:id", GetShell(deps))
	}
}

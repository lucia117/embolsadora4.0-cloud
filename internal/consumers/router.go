package consumers

import (
    "github.com/gin-gonic/gin"
    "github.com/tu-org/embolsadora-api/internal/security"
)

// TODO: fill in dependency set as needed.
type Deps struct{
    APIKeys security.APIKeyLookup
}

// TODO: fill in configuration as needed.
type Config struct{}

// RegisterConsumerRoutes wires Consumers routes under the provided group (e.g., /api/v1/consumers).
func RegisterConsumerRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {

    // Events batch ingestion
    g.POST("/events", IngestEvents)

    // Device heartbeat
    g.POST("/heartbeat", Heartbeat)
}

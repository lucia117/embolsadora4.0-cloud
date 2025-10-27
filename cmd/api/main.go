package main

import (
	"github.com/gin-gonic/gin"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/routes"
)

// TODO: Bootstrap config, telemetry, repositories, services, and dependency wiring.
func main() {
	r := gin.New()

	// Global middlewares: RequestID and Logger (stubs for now)
	r.Use(apimw.RequestID())
	r.Use(apimw.Logger())

	// Centralizar registro de rutas
	routes.RegisterURLMappings(r)

	// TODO: get address from config. Hardcoded for dev.
	_ = r.Run(":8080")
}

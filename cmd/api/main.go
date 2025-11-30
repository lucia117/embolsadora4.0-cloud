package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/routes"
)

// TODO: Bootstrap config, telemetry, repositories, services, and dependency wiring.
func main() {
	// Inicializar conexión a base de datos
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/embolsadora?sslmode=disable"
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()

	// Verificar conexión
	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}

	log.Println("Database connection established")

	r := gin.New()

	// Global middlewares: RequestID and Logger (stubs for now)
	r.Use(apimw.RequestID())
	r.Use(apimw.Logger())

	// Centralizar registro de rutas
	routes.RegisterURLMappings(r, db)

	// TODO: get address from config. Hardcoded for dev.
	log.Println("Starting server on :8080")
	_ = r.Run(":8080")
}

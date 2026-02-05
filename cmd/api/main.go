package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/routes"
)

// TODO: Bootstrap config, telemetry, repositories, services, and dependency wiring.
func main() {
	// Cargar variables de entorno desde .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Inicializar conexión a base de datos
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("environment variable DB_URL must be set (ensure it matches your docker-compose.yml or local configuration)")
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

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	_ = r.Run(":" + port)
}

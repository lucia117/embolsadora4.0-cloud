package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/config"
	"github.com/tu-org/embolsadora-api/internal/routes"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// PostgreSQL connection
	db, err := pgxpool.New(context.Background(), cfg.DB.URL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Database connection established")

	// Redis connection (optional — rate limiting fails open if unavailable)
	var redisClient *redis.Client
	if cfg.Redis.URL != "" {
		opt, err := redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			log.Printf("Invalid REDIS_URL, rate limiting disabled: %v", err)
		} else {
			redisClient = redis.NewClient(opt)
			if err := redisClient.Ping(context.Background()).Err(); err != nil {
				log.Printf("Redis unreachable, rate limiting disabled: %v", err)
				redisClient = nil
			}
		}
	}

	r := gin.New()
	r.Use(apimw.RequestID())
	r.Use(apimw.Logger())
	r.Use(apimw.CORS())

	routes.RegisterURLMappings(r, db, cfg, redisClient)

	log.Printf("Starting server on :%s", cfg.HTTP.Port)
	if err := r.Run(":" + cfg.HTTP.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

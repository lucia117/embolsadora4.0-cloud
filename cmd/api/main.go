package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/config"
	"github.com/tu-org/embolsadora-api/internal/platform/dbmigrate"
	"github.com/tu-org/embolsadora-api/internal/routes"
)

func main() {
	// Inicializar logger (debe vivir hasta el cierre de la app)
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	env := config.LoadEnvFile()

	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DB.RunMigrationsOnBoot {
		logger.Info("dbmigrate: enabled, applying pending migrations",
			zap.String("source", cfg.DB.MigrationsSourceURL))
		if err := dbmigrate.Run(cfg.DB.MigrationsSourceURL, cfg.DB.URL, logger); err != nil {
			logger.Error("dbmigrate failed", zap.Error(err))
			_ = logger.Sync()
			os.Exit(1)
		}
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

	// http.Server explicito, no r.Run(): r.Run() no fija ReadTimeout,
	// WriteTimeout ni ReadHeaderTimeout, y el plan que borro el middleware
	// Timeout (no-op) nunca lo reemplazo por esto. A 4 MiB por batch y 200 rps
	// sobre enlaces industriales, un lector lento deja un goroutine colgado
	// indefinidamente; sin ReadHeaderTimeout ni ReadTimeout, eso crece sin
	// limite en vez de fallar rapido. El bound real contra un Mongo colgado
	// esta en internal/app/ingest.Service (mongoTimeout, ver item 6 de la
	// review): esto acota el lado HTTP, no el de Mongo.
	//
	// ReadTimeout/WriteTimeout usan cfg.HTTP.ReadTimeout/WriteTimeout
	// (HTTP_READ_TIMEOUT/HTTP_WRITE_TIMEOUT, default 10s cada uno) en vez de
	// los 30s que sugirio la review textualmente: ese config ya existia en
	// config.go, con su propio default, y estaba muerto — nada lo leia
	// porque r.Run() no lo acepta. Cablearlo aca en vez de hardcodear un
	// numero distinto es preferible a dejar un config field sin uso; ver
	// nota en el reporte final.
	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("Starting server on :%s", cfg.HTTP.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

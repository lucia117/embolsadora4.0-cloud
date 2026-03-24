package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	api "github.com/tu-org/embolsadora-api/internal/api"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/auth"
	consumers "github.com/tu-org/embolsadora-api/internal/consumers"
	consumermw "github.com/tu-org/embolsadora-api/internal/consumers/middleware"
	edgeDevicesApp "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	edgeDevicesHandler "github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices"
	edgeDevicesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform/edgeclient"
	tenantsRepository "github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
	userRolesRepository "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepository "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// RegisterURLMappings configura todas las rutas de la API en un único lugar.
func RegisterURLMappings(r *gin.Engine, db *pgxpool.Pool, logger *zap.Logger) {
	// Health checks
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Métricas Prometheus
	telemetry.RegisterMetrics(r)

	// Rutas de autenticación (sin middleware de autenticación)
	// Mapea los endpoints del pacto auth-service-api.json
	authGroup := r.Group(
		"/api/auth",
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
	)
	auth.RegisterRoutes(authGroup, db)

	// Superficie administrativa / interna (v1)
	v1 := r.Group(
		"/api/v1",
		// TODO: Enable JWT validation middleware in production
		// apimw.JWTAuth(),
		// apimw.TenantFromJWT(),
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
	)

	// Inicializar repositorios
	tenantRepo := tenantsRepository.NewTenantRepository(db)
	userRoleRepo := userRolesRepository.NewUserRoleRepository(db)
	userRepo := usersRepository.NewPostgresRepository(db)

	api.RegisterAdminRoutes(v1, api.Deps{
		TenantRepo:   tenantRepo,
		UserRoleRepo: userRoleRepo,
		Logger:       logger,
		UserRepo:     userRepo,
	}, api.Config{})

	// Superficie para consumidores (IoT / dispositivos, etc.)
	c1 := r.Group(
		"/api/v1/consumers",
		consumermw.APIKeyAuth(),
		consumermw.RateLimit(),
		consumermw.Idempotency(),
		consumermw.NoCORS(),
		consumermw.Timeout(),
	)
	consumers.RegisterConsumerRoutes(c1, consumers.Deps{}, consumers.Config{})

	// Superficie de edge devices (/api/tenants/{tenantId}/edge-devices)
	// Esta ruta sigue el contrato del pact y es parte de la superficie ABM
	edgeDeviceTimeout := time.Duration(0) // usar timeout por defecto (10s)
	edgeDeviceClient := edgeclient.NewHTTPClient(edgeDeviceTimeout)
	edgeDeviceRepository := edgeDevicesRepo.NewPostgresRepository(db)
	edgeDeviceService := edgeDevicesApp.NewService(edgeDeviceRepository, edgeDeviceClient, logger)

	tenantsGroup := r.Group(
		"/api/tenants/:tenantId",
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
		apimw.JWTAuth(),
		apimw.ResolveTenantFromPath(db),
	)
	edgeDevicesHandler.RegisterRoutes(tenantsGroup, edgeDeviceService)
}

package routes

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	api "github.com/tu-org/embolsadora-api/internal/api"
	alarmRulesHandler "github.com/tu-org/embolsadora-api/internal/api/handler/alarm_rules"
	handlerChangePassword "github.com/tu-org/embolsadora-api/internal/api/handler/auth/change_password"
	handlerLogin "github.com/tu-org/embolsadora-api/internal/api/handler/auth/login"
	dashboardLayoutsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts"
	edgeDevicesHandler "github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices"
	handlerCreateInvitation "github.com/tu-org/embolsadora-api/internal/api/handler/invitations/create_invitation"
	handlerListInvitations "github.com/tu-org/embolsadora-api/internal/api/handler/invitations/list_invitations"
	handlerResendInvitation "github.com/tu-org/embolsadora-api/internal/api/handler/invitations/resend_invitation"
	handlerRevokeInvitation "github.com/tu-org/embolsadora-api/internal/api/handler/invitations/revoke_invitation"
	logsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/logs"
	handlerMe "github.com/tu-org/embolsadora-api/internal/api/handler/me"
	notificationsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/notifications"
	permissionsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/permissions"
	rolesHandler "github.com/tu-org/embolsadora-api/internal/api/handler/roles"
	handlerForcePasswordChange "github.com/tu-org/embolsadora-api/internal/api/handler/users/force_password_change"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	alarmRulesApp "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	dashboardLayoutsApp "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	edgeDevicesApp "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	appNotifications "github.com/tu-org/embolsadora-api/internal/app/notifications"
	permissionsApp "github.com/tu-org/embolsadora-api/internal/app/permissions"
	rolesApp "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/config"
	consumers "github.com/tu-org/embolsadora-api/internal/consumers"
	consumermw "github.com/tu-org/embolsadora-api/internal/consumers/middleware"
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
	"github.com/tu-org/embolsadora-api/internal/platform/edgeclient"
	mongoplatform "github.com/tu-org/embolsadora-api/internal/platform/mongo"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
	measurementsRepo "github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
	alarmRulesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/alarm_rules"
	apiKeysRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/apikeys"
	dashboardLayoutsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/dashboard_layouts"
	edgeDevicesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/edge_devices"
	invitationsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/invitations"
	logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
	notificationsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
	permissionsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/permissions"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	tenantsRepository "github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
	userRolesRepository "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	"github.com/tu-org/embolsadora-api/internal/security"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// RegisterURLMappings configures all API routes.
func RegisterURLMappings(r *gin.Engine, db *pgxpool.Pool, cfg *config.Config, redisClient *redis.Client) {
	// Health check
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Public auth
	loginHandler := handlerLogin.NewHandler(cfg.Supabase.URL, cfg.Supabase.AnonKey)
	r.POST("/api/v1/auth/login", loginHandler.Handle)

	// Prometheus metrics
	telemetry.RegisterMetrics(r)

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo := usersRepo.NewUserRepository(db)         // auth: UpsertBySupabaseID, GetBySupabaseID, etc.
	mgmtUserRepo := usersRepo.NewPostgresRepository(db) // user management CRUD
	tenantRepo := tenantsRepository.NewTenantRepository(db)
	userRoleRepo := userRolesRepository.NewUserRoleRepository(db)
	invRepo := invitationsRepo.NewInvitationRepository(db)
	rRepo := rolesRepo.NewPostgresRepository(db)

	// ── External clients ──────────────────────────────────────────────────────
	supabaseClient := supabase.NewAdminClient(cfg.Supabase.URL, cfg.Supabase.ServiceRoleKey)

	// ── Logger ────────────────────────────────────────────────────────────────
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	apimw.SetLogger(logger)
	usecases.SetLogger(logger)

	// ── Use cases ─────────────────────────────────────────────────────────────
	authUC := usecases.NewAuthUsecase(userRepo)
	meUC := usecases.NewMeUsecase(db)
	invUC := usecases.NewInvitationUsecase(invRepo, userRepo, userRoleRepo, tenantRepo, rRepo, supabaseClient, redisClient, cfg.Supabase.AppBaseURL, cfg.Supabase.InviteRateLimitHour)
	passwordUC := usecases.NewPasswordUsecase(userRepo, supabaseClient, cfg.Supabase.AppBaseURL, logger)

	// ── JWT verifier ──────────────────────────────────────────────────────────
	verifier, err := security.NewJWKSVerifier(cfg.Supabase.JWKSUrl, cfg.Supabase.JWTIssuer, cfg.Supabase.JWTAudience)
	if err != nil {
		log.Fatalf("failed to initialize JWKS verifier: %v", err)
	}

	// ── Allow-list de origins para links de mail ──────────────────────────────
	// Se parsea una sola vez y se deja constancia en el arranque: si la env var
	// falta o todas sus entradas son invalidas la lista queda vacia, el header
	// X-App-Base-URL se ignora en todos los requests y los mails vuelven a
	// salir con APP_BASE_URL — exactamente el bug que esta feature arregla.
	// Con los contadores en el log, esa mala config es un hecho observable al
	// arrancar y no un Warn por request que alguien tiene que estar mirando.
	appOrigins := apporigin.Parse(cfg.Supabase.AppAllowedOrigins)
	logger.Info("app origin allow-list",
		zap.Int("exact", appOrigins.ExactCount()),
		zap.Int("wildcard", appOrigins.WildcardCount()),
		zap.String("fallback", cfg.Supabase.AppBaseURL),
	)

	// ── Handlers ──────────────────────────────────────────────────────────────
	meHandler := handlerMe.NewHandler(meUC)
	createInvHandler := handlerCreateInvitation.NewHandler(invUC)
	listInvHandler := handlerListInvitations.NewHandler(invUC)
	resendInvHandler := handlerResendInvitation.NewHandler(invUC)
	revokeInvHandler := handlerRevokeInvitation.NewHandler(invUC)
	forcePasswordHandler := handlerForcePasswordChange.NewHandler(passwordUC)
	changePasswordHandler := handlerChangePassword.NewHandler(passwordUC)

	// ── /api/v1 group — protected by JWTAuth + TenantFromHeader + PasswordChangeGuard ──
	v1 := r.Group(
		"/api/v1",
		apimw.JWTAuth(verifier, authUC, invUC),
		apimw.TenantFromHeader(db),
		apimw.PasswordChangeGuard(),
		apimw.AppBaseURLFromHeader(appOrigins, cfg.Supabase.AppBaseURL),
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
	)

	// GET /api/v1/me — exempt from TenantFromHeader and PasswordChangeGuard (handled inside middleware)
	v1.GET("/me", meHandler.Handle)

	// POST /api/v1/auth/change-password — exempt from TenantFromHeader and PasswordChangeGuard
	v1.POST("/auth/change-password", changePasswordHandler.Handle)

	// Invitations
	v1.GET("/invitations", listInvHandler.Handle)
	v1.POST("/invitations", apimw.RBACCheck("invitations:write"), createInvHandler.Handle)
	v1.POST("/invitations/:id/resend", apimw.RBACCheck("invitations:write"), resendInvHandler.Handle)
	v1.DELETE("/invitations/:id", apimw.RBACCheck("invitations:write"), revokeInvHandler.Handle)

	// Force password change
	v1.POST("/users/:id/force-password-change", apimw.RBACCheck("users:write"), forcePasswordHandler.Handle)

	// Admin routes (tenants, user-roles, etc.)
	api.RegisterAdminRoutes(v1, api.Deps{
		TenantRepo:   tenantRepo,
		UserRoleRepo: userRoleRepo,
		Logger:       logger,
		UserRepo:     mgmtUserRepo,
	}, api.Config{})

	// ── Consumer surface (Edge Pi Service) ────────────────────────────────────
	// La ingesta necesita Mongo. Si no hay conexion, el proceso entero no
	// arranca — log.Fatalf termina toda la API, no solo /api/v1/consumers: un
	// cloud que aparece caido de punta a punta (conexion rechazada en todos
	// los endpoints) es preferible a uno que queda arriba y acepta eventos que
	// nunca se van a poder guardar.
	mongoClient, err := mongoplatform.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		log.Fatalf("no se pudo conectar a MongoDB: %v", err)
	}

	measurementRepo := measurementsRepo.New(mongoClient.Database())
	if err := measurementRepo.EnsureIndexes(context.Background()); err != nil {
		// Sin el indice unico sobre eventId no hay idempotencia, y cada reintento
		// del Pi duplicaria mediciones en silencio. No se arranca sin el.
		log.Fatalf("no se pudieron crear los indices de measurements: %v", err)
	}

	// El health check detallado (a diferencia de /ping) necesita al repo de
	// measurements para chequear Mongo, asi que se registra aca, una vez que
	// measurementRepo ya existe.
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := gin.H{"postgres": "ok", "mongo": "ok", "redis": "ok"}
		status := http.StatusOK

		if err := db.Ping(ctx); err != nil {
			checks["postgres"] = "error"
			status = http.StatusServiceUnavailable
		}
		if err := measurementRepo.Ping(ctx); err != nil {
			checks["mongo"] = "error"
			status = http.StatusServiceUnavailable
		}
		if redisClient == nil {
			checks["redis"] = "no configurado"
		} else if err := redisClient.Ping(ctx).Err(); err != nil {
			// Redis degradado no tumba el servicio: la cache de keys y el rate
			// limit fallan abiertos a proposito.
			checks["redis"] = "error"
		}

		telemetry.SetMongoUp(checks["mongo"] == "ok")
		c.JSON(status, gin.H{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "checks": checks})
	})

	apiKeyRepository := apiKeysRepo.NewRepository(db)
	apiKeyAuth := security.NewAPIKeyAuthenticator(apiKeyRepository, redisClient, cfg.Ingest.APIKeyCacheTTL, logger)
	ingestService := ingestapp.NewService(measurementRepo, logger)
	rateLimiter := consumers.NewRateLimiter(redisClient, cfg.Ingest.RateLimitRPS, cfg.Ingest.RateLimitBurst)

	c1 := r.Group(
		"/api/v1/consumers",
		apimw.RequestID(),
		consumermw.NoCORS(),
		consumermw.APIKeyAuth(apiKeyAuth, logger),
		consumermw.RateLimit(rateLimiter),
	)
	consumers.RegisterConsumerRoutes(c1, consumers.Deps{
		Ingest: ingestService,
		Log:    logger,
	}, consumers.Config{
		MaxBodyBytes: cfg.Ingest.MaxBodyBytes,
		MaxEvents:    cfg.Ingest.MaxEvents,
	})

	// Superficie de edge devices (/api/v1/tenants/{tenantId}/edge-devices)
	edgeDeviceTimeout := time.Duration(0) // usar timeout por defecto (10s)
	edgeDeviceClient := edgeclient.NewHTTPClient(edgeDeviceTimeout)
	edgeDeviceRepository := edgeDevicesRepo.NewPostgresRepository(db)
	edgeDeviceService := edgeDevicesApp.NewService(edgeDeviceRepository, edgeDeviceClient, logger)

	tenantsGroup := r.Group(
		"/api/v1/tenants/:tenantId",
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
		apimw.JWTAuth(verifier, authUC, invUC),
		apimw.ResolveTenantAndCheckMembership(db),
	)
	edgeDevicesHandler.RegisterRoutes(tenantsGroup, edgeDeviceService)

	// Dashboard Layouts surface (/api/v1/dashboard-layouts)
	// tenant_id comes from X-Tenant-ID header, user_id from JWT context
	dlRepo := dashboardLayoutsRepo.NewPostgresRepository(db)
	dlService := dashboardLayoutsApp.NewService(dlRepo, logger)
	dashboardLayoutsHandler.RegisterRoutes(v1, dlService)

	// Roles surface (/api/v1/roles)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado puede listar/ver roles)
	// POST/PUT/DELETE: requieren permiso users:write (solo administradores)
	rService := rolesApp.NewService(rRepo, logger)
	rolesWriteGroup := v1.Group("", apimw.RBACCheck("users:write"))
	rolesHandler.RegisterRoutes(v1, rolesWriteGroup, rService)

	// Alarm Rules surface (/api/v1/alarm-rules)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado del tenant puede listar/ver reglas)
	// POST/PATCH/DELETE: requieren permiso users:write (solo administradores)
	arRepo := alarmRulesRepo.NewPostgresRepository(db)
	arService := alarmRulesApp.NewService(arRepo, logger)
	alarmRulesWriteGroup := v1.Group("", apimw.RBACCheck("users:write"))
	alarmRulesHandler.RegisterRoutes(v1, alarmRulesWriteGroup, arService)

	// Log Service (/api/v1/logs)
	logRepository := logsRepo.New(db)
	logService := appLogs.New(logRepository, logger)
	logsHandler.RegisterRoutes(v1, logService)

	// Notification Service (/api/v1/notifications)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado del tenant puede ver/gestionar sus notificaciones)
	nRepo := notificationsRepo.New(db)
	nService := appNotifications.New(nRepo, logger)
	notificationsHandler.RegisterRoutes(v1, nService)

	// Permissions Service (/api/v1/permissions)
	// GET /permissions y GET /permissions/:id — sin RBAC adicional (cualquier usuario autenticado puede consultar)
	// POST/PUT/DELETE — requieren permiso users:write (solo administradores)
	pRepo := permissionsRepo.NewPostgresRepository(db)
	pService := permissionsApp.NewService(pRepo, logger)
	pHandler := permissionsHandler.NewHandler(pService, logger)
	permissionsWriteGroup := v1.Group("", apimw.RBACCheck("users:write"))
	v1.GET("/permissions", pHandler.ListPermissions)
	v1.GET("/permissions/:id", pHandler.GetPermission)
	permissionsWriteGroup.POST("/permissions", pHandler.CreatePermission)
	permissionsWriteGroup.PUT("/permissions/:id", pHandler.UpdatePermission)
	permissionsWriteGroup.DELETE("/permissions/:id", pHandler.DeletePermission)
}

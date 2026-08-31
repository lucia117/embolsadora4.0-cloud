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
	getPublicTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_public_tenant"
	handlerForcePasswordChange "github.com/tu-org/embolsadora-api/internal/api/handler/users/force_password_change"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	ucGetPublicTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_public_tenant"
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
	domainingest "github.com/tu-org/embolsadora-api/internal/domain/ingest"
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

	// Public tenant lookup — no session, no X-Tenant-ID. Backs the invitation/
	// password-reset callback link (runs before any session exists) and the
	// public tenant landing page. Registered on `r` directly (like POST
	// /api/v1/auth/login above), NOT on the `v1` group, which requires JWTAuth
	// on every route.
	getPublicTenantUC := ucGetPublicTenant.NewUseCase(tenantRepo)
	getPublicTenantHandler := getPublicTenant.NewGetPublicTenantHandler(getPublicTenantUC)
	r.GET("/api/v1/public/tenants/:idOrSubdomain", getPublicTenantHandler.GetPublicTenant)

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
	passwordUC := usecases.NewPasswordUsecase(userRepo, mgmtUserRepo, supabaseClient, cfg.Supabase.AppBaseURL, logger)

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
	v1.POST("/invitations", apimw.RBACCheck("perm_users_manage"), createInvHandler.Handle)
	v1.POST("/invitations/:id/resend", apimw.RBACCheck("perm_users_manage"), resendInvHandler.Handle)
	v1.DELETE("/invitations/:id", apimw.RBACCheck("perm_users_manage"), revokeInvHandler.Handle)

	// Force password change
	v1.POST("/users/:id/force-password-change", apimw.RBACCheck("perm_users_manage"), forcePasswordHandler.Handle)

	// Admin routes (tenants, user-roles, etc.)
	api.RegisterAdminRoutes(v1, api.Deps{
		TenantRepo:   tenantRepo,
		UserRoleRepo: userRoleRepo,
		Logger:       logger,
		UserRepo:     mgmtUserRepo,
		RoleRepo:     rRepo,
	}, api.Config{})

	// ── Consumer surface (Edge Pi Service) ────────────────────────────────────
	// La ingesta necesita Mongo, pero un Mongo inalcanzable AL ARRANCAR ya no
	// tumba el proceso entero: I-1 (Mongo caido -> 500 -> el Edge reintenta
	// con backoff sin perder datos) ya cubre exactamente este caso, y
	// aplicarlo tambien aca es estrictamente mejor que log.Fatalf, que se
	// llevaba puesto login, /me, dashboards y reglas de alarma por un fallo
	// que el propio diseno de la ingesta esta hecho para sobrevivir. Se deja
	// constancia con logger.Error (no Fatalf) y measurementRepo queda en un
	// stub que devuelve error en cada InsertMany/Ping (measurementsRepo.Unavailable).
	measurementRepo, err := connectMeasurementsRepo(context.Background(), cfg.Mongo, logger)
	if err != nil {
		// connectMeasurementsRepo ya distingue "no se pudo crear los indices"
		// (fatal: sin el indice unico no hay idempotencia y cada reintento del
		// Pi duplicaria mediciones en silencio) de "no se pudo conectar"
		// (degradado, no fatal). Si volvio error aca es SIEMPRE el primer caso.
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
	ingestService := ingestapp.NewService(measurementRepo, logger, cfg.Mongo.Timeout)
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
	edgeDeviceService := edgeDevicesApp.NewService(edgeDeviceRepository, edgeDeviceClient, logger, apiKeyRepository, redisClient)

	tenantsGroup := r.Group(
		"/api/v1/tenants/:tenantId",
		apimw.RequestID(),
		apimw.Logger(),
		apimw.CORS(),
		apimw.JWTAuth(verifier, authUC, invUC),
		apimw.ResolveTenantAndCheckMembership(db),
	)
	edgeDevicesWriteGroup := tenantsGroup.Group("", apimw.RBACCheck("machines:write"))
	edgeDevicesHandler.RegisterRoutes(tenantsGroup, edgeDevicesWriteGroup, edgeDeviceService)

	// Dashboard Layouts surface (/api/v1/dashboard-layouts)
	// tenant_id comes from X-Tenant-ID header, user_id from JWT context
	dlRepo := dashboardLayoutsRepo.NewPostgresRepository(db)
	dlService := dashboardLayoutsApp.NewService(dlRepo, logger)
	dashboardLayoutsHandler.RegisterRoutes(v1, dlService)

	// Roles surface (/api/v1/roles)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado puede listar/ver roles)
	// POST/PUT/DELETE: requieren perm_users_manage (solo administradores)
	rService := rolesApp.NewService(rRepo, logger)
	rolesWriteGroup := v1.Group("", apimw.RBACCheck("perm_users_manage"))
	rolesHandler.RegisterRoutes(v1, rolesWriteGroup, rService)

	// Alarm Rules surface (/api/v1/alarm-rules)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado del tenant puede listar/ver reglas)
	// POST/PATCH/DELETE: requieren perm_users_manage (solo administradores)
	arRepo := alarmRulesRepo.NewPostgresRepository(db)
	arService := alarmRulesApp.NewService(arRepo, logger)
	alarmRulesWriteGroup := v1.Group("", apimw.RBACCheck("perm_users_manage"))
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
	// POST/PUT/DELETE — requieren perm_users_manage (solo administradores)
	pRepo := permissionsRepo.NewPostgresRepository(db)
	pService := permissionsApp.NewService(pRepo, logger)
	pHandler := permissionsHandler.NewHandler(pService, logger)
	permissionsWriteGroup := v1.Group("", apimw.RBACCheck("perm_users_manage"))
	v1.GET("/permissions", pHandler.ListPermissions)
	v1.GET("/permissions/:id", pHandler.GetPermission)
	permissionsWriteGroup.POST("/permissions", pHandler.CreatePermission)
	permissionsWriteGroup.PUT("/permissions/:id", pHandler.UpdatePermission)
	permissionsWriteGroup.DELETE("/permissions/:id", pHandler.DeletePermission)
}

// connectMeasurementsRepo conecta a Mongo y crea sus indices.
//
// Si la CONEXION falla, devuelve un repo degradado
// (measurementsRepo.Unavailable) y nil error: el llamador debe seguir
// arrancando. La ingesta ya esta disenada para sobrevivir a Mongo caido —
// I-1: 500 en cada request, el Edge reintenta con backoff sin perder datos —
// y aplicar esa misma garantia a una caida al arrancar es estrictamente
// mejor que tumbar login, /me, dashboards y reglas de alarma con log.Fatalf
// por un problema que no los afecta.
//
// Si la conexion tuvo exito pero EnsureIndexes falla, devuelve el error: ese
// caso SI es fatal y el llamador debe terminar el proceso. Sin el indice
// unico sobre (tenantId, eventId) no hay idempotencia, y cada reintento del
// Pi duplicaria mediciones en silencio.
func connectMeasurementsRepo(ctx context.Context, cfg config.MongoConfig, logger *zap.Logger) (domainingest.Repository, error) {
	mongoClient, err := mongoplatform.Connect(ctx, cfg)
	if err != nil {
		logger.Error("no se pudo conectar a MongoDB al arrancar; la ingesta respondera 500 y el Edge reintentara (I-1); el resto de la API sigue arriba",
			zap.Error(err))
		telemetry.SetMongoUp(false)
		return measurementsRepo.Unavailable(err), nil
	}

	repo := measurementsRepo.New(mongoClient.Database())
	if err := repo.EnsureIndexes(ctx); err != nil {
		return nil, err
	}
	telemetry.SetMongoUp(true)
	return repo, nil
}

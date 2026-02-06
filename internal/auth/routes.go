package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes registra todas las rutas de autenticación
// Mapea exactamente los endpoints definidos en el pacto auth-service-api.json
func RegisterRoutes(router *gin.RouterGroup, db *pgxpool.Pool) {
	// Inicializar repositorios
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)
	tenantRepo := NewTenantRepository(db)
	resetRepo := NewPasswordResetTokenRepository(db)

	// Inicializar servicio de email (mock por ahora)
	emailService := &MockEmailService{}

	// Inicializar servicio de autenticación
	authService := NewAuthService(userRepo, sessionRepo, tenantRepo, resetRepo, emailService)

	// Inicializar handler
	handler := NewHandler(authService)

	// Registrar rutas según el pacto
	// POST /api/auth/callback/credentials - Login con credenciales
	router.POST("/callback/credentials", handler.HandleLogin)

	// GET /api/auth/session - Obtener sesión actual
	router.GET("/session", handler.HandleGetSession)

	// POST /api/auth/signout - Cerrar sesión
	router.POST("/signout", handler.HandleSignOut)

	// POST /api/auth/forgot-password - Solicitar reseteo de contraseña
	router.POST("/forgot-password", handler.HandleForgotPassword)

	// POST /api/auth/reset-password - Resetear contraseña con token
	router.POST("/reset-password", handler.HandleResetPassword)
}

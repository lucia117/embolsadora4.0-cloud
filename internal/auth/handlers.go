package auth

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// Handler contiene las dependencias para los handlers de auth
type Handler struct {
	authService *AuthService
}

// NewHandler crea una nueva instancia de Handler
func NewHandler(authService *AuthService) *Handler {
	return &Handler{
		authService: authService,
	}
}

// HandleLogin maneja POST /api/auth/callback/credentials
func (h *Handler) HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:      "Invalid request",
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	// Autenticar usuario
	session, _, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:      "Invalid credentials",
				StatusCode: http.StatusUnauthorized,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:      "Internal server error",
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	// Setear cookie de sesión según el pacto
	// Cookie name: next-auth.session-token
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	// Configurar secure flag basado en entorno
	secureCookie := os.Getenv("APP_ENV") == "production"
	c.SetCookie(
		"next-auth.session-token", // name
		session.Token,             // value
		maxAge,                    // maxAge (en segundos)
		"/",                       // path
		"",                        // domain (vacío = current domain)
		secureCookie,              // secure (true en producción con HTTPS)
		true,                      // httpOnly
	)

	// Responder según el pacto
	c.JSON(http.StatusOK, LoginResponse{
		URL: "/dashboard",
	})
}

// HandleGetSession maneja GET /api/auth/session
func (h *Handler) HandleGetSession(c *gin.Context) {
	// Leer cookie de sesión
	token, err := c.Cookie("next-auth.session-token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:      "No session found",
			StatusCode: http.StatusUnauthorized,
		})
		return
	}

	// Obtener datos de sesión
	sessionData, err := h.authService.GetSession(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:      "Invalid session",
				StatusCode: http.StatusUnauthorized,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:      "Internal server error",
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	// Responder con datos de sesión según el pacto
	c.JSON(http.StatusOK, sessionData)
}

// HandleSignOut maneja POST /api/auth/signout
func (h *Handler) HandleSignOut(c *gin.Context) {
	var req SignOutRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:      "Invalid request",
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	// Leer cookie de sesión (puede no existir)
	token, _ := c.Cookie("next-auth.session-token")

	// Cerrar sesión
	if token != "" {
		_ = h.authService.SignOut(c.Request.Context(), token)
	}

	// Borrar cookie (setear con maxAge negativo)
	// Configurar secure flag basado en entorno
	secureCookie := os.Getenv("APP_ENV") == "production"
	c.SetCookie(
		"next-auth.session-token", // name
		"",                        // value vacío
		-1,                        // maxAge negativo para borrar
		"/",                       // path
		"",                        // domain
		secureCookie,              // secure
		true,                      // httpOnly
	)

	// Responder según el pacto
	c.JSON(http.StatusOK, SignOutResponse{
		URL:     "/auth/login",
		Success: true,
	})
}

// HandleForgotPassword maneja POST /api/auth/forgot-password
func (h *Handler) HandleForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:      "Invalid request",
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	// Procesar solicitud de reseteo
	// Nota: Siempre retornamos éxito para no revelar si el email existe
	_ = h.authService.ForgotPassword(c.Request.Context(), req.Email)

	// Responder según el pacto (siempre el mismo mensaje)
	c.JSON(http.StatusOK, ForgotPasswordResponse{
		Message: "Password reset email sent",
	})
}

// HandleResetPassword maneja POST /api/auth/reset-password
func (h *Handler) HandleResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:      "Invalid request",
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	// Resetear contraseña
	err := h.authService.ResetPassword(
		c.Request.Context(),
		req.Token,
		req.Password,
		req.PasswordConfirmation,
	)

	if err != nil {
		if errors.Is(err, ErrInvalidToken) || errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:      "Invalid or expired token",
				StatusCode: http.StatusBadRequest,
			})
			return
		}

		if errors.Is(err, ErrPasswordMismatch) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:      "Passwords do not match",
				StatusCode: http.StatusBadRequest,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:      "Internal server error",
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	// Responder según el pacto
	c.JSON(http.StatusOK, ResetPasswordResponse{
		Message: "Password updated successfully",
	})
}

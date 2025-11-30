package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials se retorna cuando las credenciales son inválidas
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrSessionNotFound se retorna cuando no se encuentra la sesión
	ErrSessionNotFound = errors.New("session not found")
	// ErrInvalidToken se retorna cuando el token es inválido
	ErrInvalidToken = errors.New("invalid token")
	// ErrPasswordMismatch se retorna cuando las contraseñas no coinciden
	ErrPasswordMismatch = errors.New("passwords do not match")
)

// AuthService maneja la lógica de autenticación
type AuthService struct {
	userRepo     *UserRepository
	sessionRepo  *SessionRepository
	tenantRepo   *TenantRepository
	resetRepo    *PasswordResetTokenRepository
	emailService EmailService
}

// NewAuthService crea una nueva instancia de AuthService
func NewAuthService(
	userRepo *UserRepository,
	sessionRepo *SessionRepository,
	tenantRepo *TenantRepository,
	resetRepo *PasswordResetTokenRepository,
	emailService EmailService,
) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		tenantRepo:   tenantRepo,
		resetRepo:    resetRepo,
		emailService: emailService,
	}
}

// Login autentica un usuario y crea una sesión
func (s *AuthService) Login(ctx context.Context, email, password string) (*Session, *User, error) {
	// Buscar usuario por email
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// Verificar contraseña
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// Crear sesión
	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, user, nil
}

// GetSession obtiene la sesión y datos del usuario
func (s *AuthService) GetSession(ctx context.Context, token string) (*SessionResponse, error) {
	// Buscar sesión
	session, err := s.sessionRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, ErrSessionNotFound
	}

	// Buscar usuario
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Buscar tenant
	tenant, err := s.tenantRepo.FindByID(ctx, user.TenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Construir respuesta
	response := &SessionResponse{
		User: SessionUser{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Image: user.Image,
		},
		Expires: session.ExpiresAt.Format(time.RFC3339),
		Tenant: SessionTenant{
			ID:          tenant.ID,
			Name:        tenant.Name,
			CompanyName: tenant.CompanyName,
			Subdomain:   tenant.Subdomain,
		},
	}

	return response, nil
}

// SignOut cierra la sesión del usuario
func (s *AuthService) SignOut(ctx context.Context, token string) error {
	if token == "" {
		return nil // No hay sesión que cerrar
	}

	return s.sessionRepo.Delete(ctx, token)
}

// ForgotPassword genera un token de reseteo y envía email
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	// Buscar usuario (pero no revelar si existe o no)
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// Siempre retornar éxito para no revelar si el email existe
		return nil
	}

	// Invalidar tokens anteriores
	if err := s.resetRepo.InvalidateUserTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to invalidate old tokens: %w", err)
	}

	// Generar token aleatorio
	tokenStr, err := generateSecureToken(32)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Crear token de reseteo (válido por 1 hora)
	resetToken := &PasswordResetToken{
		ID:        generateUUID(),
		UserID:    user.ID,
		Token:     tokenStr,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.resetRepo.Create(ctx, resetToken); err != nil {
		return fmt.Errorf("failed to create reset token: %w", err)
	}

	// Enviar email
	if err := s.emailService.SendPasswordResetEmail(ctx, user.Email, tokenStr); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// ResetPassword resetea la contraseña usando un token
func (s *AuthService) ResetPassword(ctx context.Context, token, password, passwordConfirmation string) error {
	// Verificar que las contraseñas coincidan
	if password != passwordConfirmation {
		return ErrPasswordMismatch
	}

	// Buscar token válido
	resetToken, err := s.resetRepo.FindValidByToken(ctx, token)
	if err != nil {
		return ErrInvalidToken
	}

	// Hashear nueva contraseña
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Actualizar contraseña
	if err := s.userRepo.UpdatePassword(ctx, resetToken.UserID, string(passwordHash)); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Marcar token como usado
	if err := s.resetRepo.MarkAsUsed(ctx, resetToken.ID); err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	return nil
}

// createSession crea una nueva sesión para un usuario
func (s *AuthService) createSession(ctx context.Context, userID string) (*Session, error) {
	// Generar token aleatorio
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	// Crear sesión (válida por 30 días)
	session := &Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// generateSecureToken genera un token aleatorio seguro
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateUUID genera un UUID simple (para IDs de tokens)
func generateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:])
}

// EmailService define la interfaz para envío de emails
type EmailService interface {
	SendPasswordResetEmail(ctx context.Context, email, token string) error
}

// MockEmailService es una implementación mock para desarrollo
type MockEmailService struct{}

// SendPasswordResetEmail simula el envío de email
func (m *MockEmailService) SendPasswordResetEmail(ctx context.Context, email, token string) error {
	// TODO: Implementar con proveedor real (SendGrid, SES, SMTP)
	fmt.Printf("[EMAIL] Password reset for %s with token: %s\n", email, token)
	return nil
}

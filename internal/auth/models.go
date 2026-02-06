package auth

import "time"

// User representa un usuario en el sistema
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"` // No exponer en JSON
	Image        *string   `json:"image,omitempty"`
	TenantID     string    `json:"-"`
	Status       string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

// Tenant representa una organización/tenant
type Tenant struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CompanyName string    `json:"companyName"`
	Subdomain   string    `json:"subdomain"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

// Session representa una sesión de usuario
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// PasswordResetToken representa un token de reseteo de contraseña
type PasswordResetToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// DTOs para requests/responses según el pacto

// LoginRequest representa la petición de login (form-urlencoded)
type LoginRequest struct {
	Email       string `form:"email" binding:"required,email"`
	Password    string `form:"password" binding:"required"`
	Redirect    string `form:"redirect"`
	CSRFToken   string `form:"csrfToken"`
	CallbackURL string `form:"callbackUrl"`
	JSON        string `form:"json"`
}

// LoginResponse representa la respuesta de login exitoso
type LoginResponse struct {
	URL string `json:"url"`
}

// ErrorResponse representa una respuesta de error
type ErrorResponse struct {
	Error      string `json:"error"`
	StatusCode int    `json:"statusCode,omitempty"`
}

// SessionResponse representa la respuesta de GET /api/auth/session
type SessionResponse struct {
	User    SessionUser   `json:"user"`
	Expires string        `json:"expires"` // ISO8601
	Tenant  SessionTenant `json:"tenant"`
}

// SessionUser representa el usuario en la respuesta de sesión
type SessionUser struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email string  `json:"email"`
	Image *string `json:"image,omitempty"`
}

// SessionTenant representa el tenant en la respuesta de sesión
type SessionTenant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CompanyName string `json:"companyName"`
	Subdomain   string `json:"subdomain"`
}

// SignOutRequest representa la petición de logout (form-urlencoded)
type SignOutRequest struct {
	CSRFToken   string `form:"csrfToken"`
	CallbackURL string `form:"callbackUrl"`
	JSON        string `form:"json"`
}

// SignOutResponse representa la respuesta de logout
type SignOutResponse struct {
	URL     string `json:"url"`
	Success bool   `json:"success"`
}

// ForgotPasswordRequest representa la petición de olvido de contraseña
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPasswordResponse representa la respuesta de olvido de contraseña
type ForgotPasswordResponse struct {
	Message string `json:"message"`
}

// ResetPasswordRequest representa la petición de reseteo de contraseña
type ResetPasswordRequest struct {
	Token                string `json:"token" binding:"required"`
	Password             string `json:"password" binding:"required,min=8"`
	PasswordConfirmation string `json:"passwordConfirmation" binding:"required"`
}

// ResetPasswordResponse representa la respuesta de reseteo de contraseña
type ResetPasswordResponse struct {
	Message string `json:"message"`
}

package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/api/handler/users/dto"
	"github.com/tu-org/embolsadora-api/internal/app/users"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
)

// Handler handles user HTTP requests
type Handler struct {
	service *users.Service
	logger  *zap.Logger
}

// NewHandler creates a new user handler
func NewHandler(service *users.Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// ListUsers handles GET /api/v1/users - list paginated users
func (h *Handler) ListUsers(c *gin.Context) {
	tenantID := c.GetString("tenant_id") // Set by middleware
	// Middleware ensures tenant_id is present, no need to check here

	// Parse pagination params
	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	// Validate pagination
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	h.logger.Debug("list users request", zap.String("tenant_id", tenantID), zap.Int("limit", limit), zap.Int("offset", offset))

	users, total, err := h.service.ListUsers(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		h.logger.Error("list users failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	// Convert to response DTOs
	userResponses := make([]dto.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = userToResponse(user)
	}

	response := dto.ListUsersResponse{
		Data: userResponses,
		Pagination: dto.PaginationMeta{
			Total:  total,
			Count:  len(userResponses),
			Limit:  limit,
			Offset: offset,
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetUser handles GET /api/v1/users/:id - get a specific user
func (h *Handler) GetUser(c *gin.Context) {
	tenantID := c.GetString("tenant_id") // Set by middleware

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAM",
			Message: "User ID is required",
			Status:  http.StatusBadRequest,
		})
		return
	}

	h.logger.Debug("get user request", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	user, err := h.service.GetUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("get user failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, userToResponse(user))
}

// CreateUser handles POST /api/v1/users - create a new user
func (h *Handler) CreateUser(c *gin.Context) {
	tenantID := c.GetString("tenant_id") // Set by middleware

	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid create user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
			Status:  http.StatusBadRequest,
		})
		return
	}

	h.logger.Debug("create user request", zap.String("tenant_id", tenantID), zap.String("email", req.Email))

	cmd := &domainUsers.CreateUserCommand{
		TenantID:  tenantID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Role:      req.Role,
		Image:     req.Image,
	}

	user, err := h.service.CreateUser(c.Request.Context(), tenantID, cmd)
	if err != nil {
		h.logger.Error("create user failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, userToResponse(user))
}

// UpdateUser handles PATCH /api/v1/users/:id - update a user
func (h *Handler) UpdateUser(c *gin.Context) {
	tenantID := c.GetString("tenant_id") // Set by middleware

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAM",
			Message: "User ID is required",
			Status:  http.StatusBadRequest,
		})
		return
	}

	var req struct {
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
		Role      *string `json:"role"`
		Image     *string `json:"image"`
		Email     *string `json:"email"`      // Should not be allowed
		TenantID  *string `json:"tenantId"`   // Should not be allowed
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid update user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
			Status:  http.StatusBadRequest,
		})
		return
	}

	// Check for immutable field attempts
	if req.Email != nil || req.TenantID != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "IMMUTABLE_FIELD",
			Message: "Email and tenantId cannot be modified",
			Status:  http.StatusBadRequest,
		})
		return
	}

	h.logger.Debug("update user request", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	cmd := &domainUsers.UpdateUserCommand{
		TenantID:  tenantID,
		UserID:    userID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      req.Role,
		Image:     req.Image,
	}

	user, err := h.service.UpdateUser(c.Request.Context(), tenantID, userID, cmd)
	if err != nil {
		h.logger.Error("update user failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, userToResponse(user))
}

// DeleteUser handles DELETE /api/v1/users/:id - soft delete a user
func (h *Handler) DeleteUser(c *gin.Context) {
	tenantID := c.GetString("tenant_id") // Set by middleware

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAM",
			Message: "User ID is required",
			Status:  http.StatusBadRequest,
		})
		return
	}

	h.logger.Debug("delete user request", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	err := h.service.DeleteUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("delete user failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// userToResponse converts a domain user to a response DTO
func userToResponse(user *domainUsers.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Role:      user.Role,
		TenantID:  user.TenantID,
		Image:     user.Image,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		DeletedAt: user.DeletedAt,
	}
}

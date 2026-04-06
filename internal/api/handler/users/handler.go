package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/api/handler/users/dto"
	"github.com/tu-org/embolsadora-api/internal/app/users"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	"github.com/tu-org/embolsadora-api/internal/platform"
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
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 || parsed > 100 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: "limit must be an integer between 1 and 100",
				Status:  http.StatusBadRequest,
			})
			return
		}
		limit = parsed
	}

	if o := c.Query("offset"); o != "" {
		parsed, err := strconv.Atoi(o)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: "offset must be a non-negative integer",
				Status:  http.StatusBadRequest,
			})
			return
		}
		offset = parsed
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

	// If include=roles is requested, fetch user with role data
	if c.Query("include") == "roles" {
		uwr, err := h.service.GetUserWithRoles(c.Request.Context(), tenantID, userID)
		if err != nil {
			h.logger.Error("get user with roles failed", zap.Error(err))
			HandleError(c, err)
			return
		}
		c.JSON(http.StatusOK, userWithRolesToResponse(uwr))
		return
	}

	user, err := h.service.GetUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("get user failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, userToResponse(user))
}

// ListPendingUsers handles GET /api/v1/users/pending - list users pending activation
func (h *Handler) ListPendingUsers(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	h.logger.Debug("list pending users request", zap.String("tenant_id", tenantID))

	users, err := h.service.ListPendingUsers(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.Error("list pending users failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	data := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		data = append(data, userToResponse(u))
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  data,
		"total": len(data),
	})
}

// UpdateUserStatus handles PATCH /api/v1/users/:id/status - change user participation status
func (h *Handler) UpdateUserStatus(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAM",
			Message: "User ID is required",
			Status:  http.StatusBadRequest,
		})
		return
	}

	// Extract caller ID from JWT context — fail explicitly if unavailable
	callerUUID := platform.UserID(c.Request.Context())
	if callerUUID == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Authenticated user identity not available",
			Status:  http.StatusUnauthorized,
		})
		return
	}
	callerID := callerUUID.String()

	var req dto.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_BODY",
			Message: "Request body is invalid: " + err.Error(),
			Status:  http.StatusBadRequest,
		})
		return
	}

	h.logger.Debug("update user status request",
		zap.String("tenant_id", tenantID),
		zap.String("user_id", userID),
		zap.String("status", req.Status))

	user, err := h.service.UpdateUserStatus(c.Request.Context(), tenantID, userID, callerID, req.Status)
	if err != nil {
		h.logger.Error("update user status failed", zap.Error(err))
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

	var req dto.UpdateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid update user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
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

// userWithRolesToResponse converts a UserWithRoles domain object to a response DTO.
func userWithRolesToResponse(uwr *domainUsers.UserWithRoles) dto.UserWithRolesResponse {
	roles := make([]dto.RoleInfo, 0, len(uwr.Roles))
	for _, r := range uwr.Roles {
		perms := r.Permissions
		if perms == nil {
			perms = []string{}
		}
		roles = append(roles, dto.RoleInfo{
			ID:          r.ID,
			Name:        r.Name,
			Permissions: perms,
		})
	}
	return dto.UserWithRolesResponse{
		UserResponse: userToResponse(&uwr.User),
		Roles:        roles,
	}
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

package shells

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/domain/aas"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// Handler handles CRUD operations for AAS shells.
type Handler struct {
	repo aas.ShellRepository
}

func NewHandler(repo aas.ShellRepository) *Handler {
	return &Handler{repo: repo}
}

// RegisterRoutes registers AAS shell routes on the given group.
// The group must already have JWTAuth and TenantFromHeader middleware applied.
func RegisterRoutes(group *gin.RouterGroup, repo aas.ShellRepository) {
	h := NewHandler(repo)
	group.GET("/aas/shells", h.List)
	group.POST("/aas/shells", h.Create)
	group.GET("/aas/shells/:id", h.Get)
	group.PUT("/aas/shells/:id", h.Update)
	group.DELETE("/aas/shells/:id", h.Delete)
}

// List handles GET /api/v1/aas/shells
func (h *Handler) List(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	shells, total, err := h.repo.ListByTenant(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   shells,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type createRequest struct {
	GlobalAssetID  string                   `json:"globalAssetId" binding:"required"`
	AssetKind      string                   `json:"assetKind" binding:"required"`
	AssetType      string                   `json:"assetType" binding:"required"`
	Description    *string                  `json:"description"`
	Administration *aas.Administration      `json:"administration"`
	SubmodelRefs   []aas.SubmodelRef        `json:"submodelRefs"`
}

// Create handles POST /api/v1/aas/shells
func (h *Handler) Create(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shell := &aas.AssetAdministrationShell{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		GlobalAssetID:  req.GlobalAssetID,
		AssetKind:      req.AssetKind,
		AssetType:      req.AssetType,
		Description:    req.Description,
		Administration: req.Administration,
		SubmodelRefs:   req.SubmodelRefs,
	}
	if shell.SubmodelRefs == nil {
		shell.SubmodelRefs = []aas.SubmodelRef{}
	}

	created, err := h.repo.Create(c.Request.Context(), shell)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "shell with this globalAssetId already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// Get handles GET /api/v1/aas/shells/:id
func (h *Handler) Get(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	shell, err := h.repo.GetByID(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "shell not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, shell)
}

type updateRequest struct {
	Description    *string             `json:"description"`
	Administration *aas.Administration `json:"administration"`
	AssetKind      *string             `json:"assetKind"`
	AssetType      *string             `json:"assetType"`
	SubmodelRefs   []aas.SubmodelRef   `json:"submodelRefs"`
}

// Update handles PUT /api/v1/aas/shells/:id
func (h *Handler) Update(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.repo.Update(c.Request.Context(), tenantID, c.Param("id"), &aas.ShellUpdate{
		Description:    req.Description,
		Administration: req.Administration,
		AssetKind:      req.AssetKind,
		AssetType:      req.AssetType,
		SubmodelRefs:   req.SubmodelRefs,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "shell not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/aas/shells/:id
func (h *Handler) Delete(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	err := h.repo.Delete(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "shell not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func parseTenantID(c *gin.Context) (uuid.UUID, bool) {
	raw := platform.TenantID(c.Request.Context())
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Tenant-ID header"})
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return uuid.UUID{}, false
	}
	return id, true
}

func queryInt(c *gin.Context, key string, def int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil || v < 0 {
		return def
	}
	return v
}

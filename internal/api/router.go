package api

import (
    "context"
    "github.com/gin-gonic/gin"
    "github.com/tu-org/embolsadora-api/internal/security"
)

// TODO: fill in dependency set as needed.
type Deps struct{
    JWTVerifier security.Verifier
    RBACCan     func(ctx context.Context, perm string) error
}

// TODO: fill in configuration as needed.
type Config struct{}

// RegisterAdminRoutes wires API surface routes under the provided group (e.g., /api/v1).
func RegisterAdminRoutes(g *gin.RouterGroup, deps Deps, cfg Config) {
    // Users
    g.GET("/users", ListUsers)
    g.POST("/users", CreateUser)

    // Machines
    g.GET("/machines", ListMachines)
    g.POST("/machines", CreateMachine)

    // Tenants
    g.GET("/tenants", ListTenants)
    g.POST("/tenants", CreateTenant)
}

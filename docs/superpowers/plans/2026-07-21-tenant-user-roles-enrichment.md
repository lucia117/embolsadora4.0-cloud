# Tenant User-Roles Enrichment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `GET /api/v1/user-roles?tenantId=` return a resolved role name and the assigned user's email/name directly, so callers don't need a second request that silently drops auto-provisioned users.

**Architecture:** `FindByTenant`'s SQL gains `JOIN users` (on the real FK relation, `user_id`) and `LEFT JOIN roles` (nullable, mirroring the existing `FindByUserQuery` pattern in the same file). A new `domain.UserTenantRoleDetail` type carries the extra fields through the usecase to a new `UserSummary` nested object in the JSON response. All existing response fields are unchanged.

**Tech Stack:** Go 1.24+, Gin, pgx/v5, testify — see `CLAUDE.md` for exact Docker-wrapped commands (Go is not installed on the host).

## Global Constraints

- No frontend changes (this plan is `embolsadora4.0-cloud` only).
- No change to `GET /api/v1/users?tenantId=` (`users` repo/handler) — out of scope.
- No change to `assign_user_role`, `update_user_role`, `revoke_user_role`, `bulk_assign_user_roles`, or `get_user_roles` — each owns its own response model and none share code with `list_user_roles` beyond `domain.UserTenantRole`.
- Every existing field on `list_user_roles`'s `UserRoleResponse` (`id`, `userId`, `tenantId`, `roleId`, `status`, `assignedBy`, `assignedAt`, `createdAt`, `updatedAt`) must keep its exact JSON key and semantics — additive changes only.
- The user JOIN must be `JOIN users u ON u.id = utr.user_id` (the real FK relation) — never filter or join on `users.tenant_id`, which is the bug this plan fixes.
- The role JOIN must be a `LEFT JOIN` (role_id is nullable for pending assignments).

---

### Task 1: Repository layer — JOIN query, domain type, usecase plumbing

**Files:**
- Modify: `internal/domain/user_roles.go`
- Modify: `internal/repo/pg/user_roles/resources.go`
- Modify: `internal/repo/pg/user_roles/repository.go`
- Modify: `internal/api/usecases/user_roles/list_user_roles/usecase.go`
- Test: `internal/repo/pg/user_roles/repository_test.go`

**Interfaces:**
- Consumes: existing `domain.UserTenantRole` struct (unchanged), existing `userRoleRepository` struct and `NewUserRoleRepository` constructor (unchanged).
- Produces: `domain.UserTenantRoleDetail` struct (embeds `UserTenantRole`, adds `RoleName string`, `UserEmail string`, `UserName *string`, `UserFirstName *string`, `UserLastName *string`) — Task 2 consumes this exact type and its exact field names in `FromDomain`. `UserRoleRepository.FindByTenant` and `list_user_roles.UseCase.Execute` both now return `([]domain.UserTenantRoleDetail, error)` instead of `([]domain.UserTenantRole, error)`.

- [ ] **Step 1: Add the `UserTenantRoleDetail` domain type**

Open `internal/domain/user_roles.go`. After the existing `UserTenantRole` struct (ends at the closing `}` before `// UserRoleWithContext...`), add:

```go
// UserTenantRoleDetail is returned by GET /user-roles?tenantId=...
// It embeds UserTenantRole plus role and user display fields resolved via
// JOIN in FindByTenant, so callers don't need a second round-trip to render
// a name (see docs/superpowers/specs/2026-07-21-tenant-user-roles-enrichment-design.md).
type UserTenantRoleDetail struct {
	UserTenantRole
	RoleName      string // "" when RoleID is nil or the role has no name
	UserEmail     string
	UserName      *string
	UserFirstName *string
	UserLastName  *string
}
```

**Step 2: Write the failing integration test**

Create `internal/repo/pg/user_roles/repository_test.go`:

```go
package user_roles_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// mrgPlatformTenantID is the fixed MRG platform tenant seeded by migration
// 000002 (migrations/000002_seed_essentials.up.sql). Reused here instead of
// creating a throwaway tenant because it always exists, and the 'operario'
// role used below (tenant-scoped, is_global=false) is always assignable to
// it under the migration 000004 platform-role rule.
const mrgPlatformTenantID = "11b36b85-033d-4bb3-9e31-4c92161887c0"

// TestFindByTenant_ResolvesUserAndRoleAcrossJoin verifies the fix for the
// root cause in the design spec: a user auto-provisioned with
// users.tenant_id = NULL (see internal/api/usecases/auth_usecase.go's
// ProvisionUser) must still resolve correctly through the real
// user_tenant_roles relation, and role_id must resolve to a role name.
func TestFindByTenant_ResolvesUserAndRoleAcrossJoin(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	repo := user_roles.NewUserRoleRepository(db)
	tenantID := uuid.MustParse(mrgPlatformTenantID)

	autoProvisionedUserID := uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, tenant_id, first_name, last_name) VALUES ($1, $2, NULL, $3, $4)`,
		autoProvisionedUserID, "join-fix-test@example.com", "Join", "Fixture",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM users WHERE id = $1`, autoProvisionedUserID)
	})

	activeUTRID := uuid.New()
	const roleID = "operario"
	_, err = db.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status) VALUES ($1, $2, $3, $4, 'active')`,
		activeUTRID, autoProvisionedUserID, tenantID, roleID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM user_tenant_roles WHERE id = $1`, activeUTRID)
	})

	pendingUTRID := uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status) VALUES ($1, $2, $3, NULL, 'pending')`,
		pendingUTRID, autoProvisionedUserID, tenantID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Exec(ctx, `DELETE FROM user_tenant_roles WHERE id = $1`, pendingUTRID)
	})

	var expectedRoleName string
	err = db.QueryRow(ctx, `SELECT name FROM roles WHERE id = $1`, roleID).Scan(&expectedRoleName)
	require.NoError(t, err)

	results, err := repo.FindByTenant(ctx, tenantID, nil)
	require.NoError(t, err)

	var active, pending *domain.UserTenantRoleDetail
	for i := range results {
		switch results[i].ID {
		case activeUTRID:
			active = &results[i]
		case pendingUTRID:
			pending = &results[i]
		}
	}

	require.NotNil(t, active, "active assignment should be present in FindByTenant results")
	assert.Equal(t, expectedRoleName, active.RoleName)
	assert.Equal(t, "join-fix-test@example.com", active.UserEmail)
	require.NotNil(t, active.UserFirstName)
	assert.Equal(t, "Join", *active.UserFirstName)
	require.NotNil(t, active.UserLastName)
	assert.Equal(t, "Fixture", *active.UserLastName)

	require.NotNil(t, pending, "pending assignment should be present in FindByTenant results")
	assert.Equal(t, "", pending.RoleName)
	assert.Equal(t, "join-fix-test@example.com", pending.UserEmail)
}
```

- [ ] **Step 3: Run the test to verify it fails to compile**

Requires a local Postgres. Start one and apply migrations:

```bash
docker compose down db -v 2>/dev/null
docker compose up -d db
until [ "$(docker inspect --format='{{.State.Health.Status}}' embolsadora_db)" = "healthy" ]; do sleep 2; done
export DBURL="postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable"
migrate -path migrations/ -database "$DBURL" up
```

Run the test via Docker (per `CLAUDE.md`), pointing `DATABASE_URL` at the host Postgres from inside the container:

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app \
  -e DATABASE_URL="postgres://embolsadora_user:embolsadora_password@host.docker.internal:5432/embolsadora_dev?sslmode=disable" \
  golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/user_roles/... -run TestFindByTenant_ResolvesUserAndRoleAcrossJoin -v"
```

Expected: **build failure** — `repo.FindByTenant` still returns `[]domain.UserTenantRole`, which has no `RoleName`/`UserEmail`/`UserFirstName`/`UserLastName` fields, and `domain.UserTenantRoleDetail` doesn't exist as a distinct type from `UserTenantRole` in a way the test's field access can satisfy. The compiler error will point at the missing fields.

- [ ] **Step 4: Update the queries in `resources.go`**

In `internal/repo/pg/user_roles/resources.go`, replace `FindByTenantQuery` and `FindByTenantWithStatusQuery`:

```go
	// FindByTenantQuery retrieves all UTR assignments for a tenant, ordered by creation date.
	// Joins users on the real membership relation (user_id, not users.tenant_id — see
	// docs/superpowers/specs/2026-07-21-tenant-user-roles-enrichment-design.md) so
	// auto-provisioned users (users.tenant_id IS NULL) resolve correctly. Joins roles
	// (LEFT, since role_id is nullable for pending assignments) for the display name.
	FindByTenantQuery = `
		SELECT utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at,
		       COALESCE(r.name, '') AS role_name,
		       u.email AS user_email,
		       u.name AS user_name,
		       u.first_name AS user_first_name,
		       u.last_name AS user_last_name
		FROM user_tenant_roles utr
		JOIN users u ON u.id = utr.user_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.tenant_id = $1
		ORDER BY utr.created_at DESC
	`

	// FindByTenantWithStatusQuery retrieves UTR assignments for a tenant filtered by status.
	// Same JOINs as FindByTenantQuery — see comment there.
	FindByTenantWithStatusQuery = `
		SELECT utr.id, utr.user_id, utr.tenant_id, utr.role_id, utr.status, utr.assigned_by, utr.assigned_at, utr.created_at, utr.updated_at,
		       COALESCE(r.name, '') AS role_name,
		       u.email AS user_email,
		       u.name AS user_name,
		       u.first_name AS user_first_name,
		       u.last_name AS user_last_name
		FROM user_tenant_roles utr
		JOIN users u ON u.id = utr.user_id
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE utr.tenant_id = $1 AND utr.status = $2
		ORDER BY utr.created_at DESC
	`
```

Leave every other query in this file (`CreateQuery`, `FindByIDQuery`, `UpdateQuery`, `RevokeQuery`, `UpdateStatusQuery`, `FindByUserQuery`) exactly as-is.

- [ ] **Step 5: Update `repository.go`: interface, `FindByTenant`, and add the detail scanner**

In `internal/repo/pg/user_roles/repository.go`:

Change the interface method signature (inside `UserRoleRepository`):

```go
	FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRoleDetail, error)
```

Replace the `FindByTenant` method body:

```go
func (r *userRoleRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRoleDetail, error) {
	var rows pgx.Rows
	var err error

	if status != nil {
		rows, err = r.db.Query(ctx, FindByTenantWithStatusQuery, tenantID, *status)
	} else {
		rows, err = r.db.Query(ctx, FindByTenantQuery, tenantID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.UserTenantRoleDetail
	for rows.Next() {
		d, err := scanUTRDetailFromRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []domain.UserTenantRoleDetail{}
	}
	return result, nil
}
```

Add a new scanner function next to `scanUTRFromRow` (do not modify `scanUTRFromRow` itself — it's still used by every other query in this file):

```go
// scanUTRDetailFromRow scans a single UTR row plus its joined role name and
// user display fields, from the FindByTenant / FindByTenantWithStatus queries.
func scanUTRDetailFromRow(rows pgx.Rows) (*domain.UserTenantRoleDetail, error) {
	var d domain.UserTenantRoleDetail
	var roleID *string
	var assignedBy *uuid.UUID
	err := rows.Scan(
		&d.ID, &d.UserID, &d.TenantID, &roleID, &d.Status,
		&assignedBy, &d.AssignedAt, &d.CreatedAt, &d.UpdatedAt,
		&d.RoleName, &d.UserEmail, &d.UserName, &d.UserFirstName, &d.UserLastName,
	)
	if err != nil {
		return nil, err
	}
	d.RoleID = roleID
	d.AssignedBy = assignedBy
	return &d, nil
}
```

- [ ] **Step 6: Update the usecase**

In `internal/api/usecases/user_roles/list_user_roles/usecase.go`, change both the interface and the implementation:

```go
package list_user_roles

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for listing user-role assignments for a tenant.
type UseCase interface {
	Execute(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRoleDetail, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new list_user_roles use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute returns all UTR assignments for a tenant, optionally filtered by status.
func (uc *useCase) Execute(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRoleDetail, error) {
	return uc.repo.FindByTenant(ctx, tenantID, status)
}
```

- [ ] **Step 7: Build to confirm the rest of the codebase still compiles**

`list_user_roles`'s handler (`internal/api/handler/user_roles/list_user_roles/list_user_roles.go`) calls `models.FromDomain(results)` — this will fail to compile until Task 2 updates `FromDomain`'s signature. That is expected and resolved in Task 2; for this task, only confirm the packages you touched build in isolation:

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./internal/domain/... ./internal/repo/pg/user_roles/... ./internal/api/usecases/user_roles/list_user_roles/..."
```

Expected: succeeds (these three packages have no remaining reference to the old `[]domain.UserTenantRole` return type).

- [ ] **Step 8: Run the integration test to verify it passes**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app \
  -e DATABASE_URL="postgres://embolsadora_user:embolsadora_password@host.docker.internal:5432/embolsadora_dev?sslmode=disable" \
  golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/user_roles/... -run TestFindByTenant_ResolvesUserAndRoleAcrossJoin -v"
```

Expected: `--- PASS: TestFindByTenant_ResolvesUserAndRoleAcrossJoin`.

Tear down the local Postgres:

```bash
docker compose down db -v
```

- [ ] **Step 9: Commit**

```bash
git add internal/domain/user_roles.go internal/repo/pg/user_roles/resources.go internal/repo/pg/user_roles/repository.go internal/repo/pg/user_roles/repository_test.go internal/api/usecases/user_roles/list_user_roles/usecase.go
git commit -m "feat: resolve role name and user via JOIN in FindByTenant"
```

---

### Task 2: Response model — expose `roleName` and `user` in the JSON response

**Files:**
- Modify: `internal/api/handler/user_roles/list_user_roles/models/response.go`
- Test: `internal/api/handler/user_roles/list_user_roles/models/response_test.go`

**Interfaces:**
- Consumes: `domain.UserTenantRoleDetail` from Task 1 — exact fields `RoleName string`, `UserEmail string`, `UserName *string`, `UserFirstName *string`, `UserLastName *string`, plus the embedded `UserTenantRole` fields (`ID`, `UserID`, `TenantID`, `RoleID`, `Status`, `AssignedBy`, `AssignedAt`, `CreatedAt`, `UpdatedAt`).
- Produces: `UserRoleResponse` gains `RoleName string` (JSON `roleName`) and `User *UserSummary` (JSON `user`); new exported type `UserSummary{Email string, Name *string, FirstName *string, LastName *string}`. `FromDomain` now takes `[]domain.UserTenantRoleDetail` instead of `[]domain.UserTenantRole`. Nothing outside this file consumes `UserSummary` or the new `FromDomain` signature yet — no other task depends on it.

- [ ] **Step 1: Write the failing unit tests**

Create `internal/api/handler/user_roles/list_user_roles/models/response_test.go`:

```go
package models_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/api/handler/user_roles/list_user_roles/models"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_MapsRoleNameAndUser(t *testing.T) {
	now := time.Now()
	roleID := "operario"
	firstName := "Join"
	lastName := "Fixture"

	utr := domain.UserTenantRoleDetail{
		UserTenantRole: domain.UserTenantRole{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TenantID:  uuid.New(),
			RoleID:    &roleID,
			Status:    domain.UserRoleStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
		RoleName:      "Operario",
		UserEmail:     "join-fix-test@example.com",
		UserFirstName: &firstName,
		UserLastName:  &lastName,
	}

	result := models.FromDomain([]domain.UserTenantRoleDetail{utr})

	require.Len(t, result, 1)
	resp := result[0]
	assert.Equal(t, "Operario", resp.RoleName)
	require.NotNil(t, resp.User)
	assert.Equal(t, "join-fix-test@example.com", resp.User.Email)
	require.NotNil(t, resp.User.FirstName)
	assert.Equal(t, "Join", *resp.User.FirstName)
	require.NotNil(t, resp.User.LastName)
	assert.Equal(t, "Fixture", *resp.User.LastName)
	assert.Nil(t, resp.User.Name)
}

func TestFromDomain_PendingAssignmentHasEmptyRoleName(t *testing.T) {
	now := time.Now()
	utr := domain.UserTenantRoleDetail{
		UserTenantRole: domain.UserTenantRole{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TenantID:  uuid.New(),
			RoleID:    nil,
			Status:    domain.UserRoleStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		},
		RoleName:  "",
		UserEmail: "pending@example.com",
	}

	result := models.FromDomain([]domain.UserTenantRoleDetail{utr})

	require.Len(t, result, 1)
	assert.Equal(t, "", result[0].RoleName)
	assert.Nil(t, result[0].RoleID)
	require.NotNil(t, result[0].User)
	assert.Equal(t, "pending@example.com", result[0].User.Email)
}
```

- [ ] **Step 2: Run the tests to verify they fail to compile**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/handler/user_roles/list_user_roles/... -v"
```

Expected: **build failure** — `domain.UserTenantRoleDetail` has no field the test can assign into a `FromDomain` call whose signature still expects `[]domain.UserTenantRole`, and `models.UserSummary` doesn't exist yet.

- [ ] **Step 3: Update `response.go`**

Replace the full contents of `internal/api/handler/user_roles/list_user_roles/models/response.go`:

```go
package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// UserSummary is the minimal user identity embedded in a UserRoleResponse,
// resolved via the JOIN in FindByTenant so callers can render a name or
// email without a second request.
type UserSummary struct {
	Email     string  `json:"email"`
	Name      *string `json:"name"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}

// UserRoleResponse is the JSON shape for a single UTR assignment in list responses.
type UserRoleResponse struct {
	ID         string       `json:"id"`
	UserID     string       `json:"userId"`
	TenantID   string       `json:"tenantId"`
	RoleID     *string      `json:"roleId"`
	RoleName   string       `json:"roleName"`
	Status     string       `json:"status"`
	AssignedBy *string      `json:"assignedBy"`
	AssignedAt *string      `json:"assignedAt"`
	CreatedAt  string       `json:"createdAt"`
	UpdatedAt  string       `json:"updatedAt"`
	User       *UserSummary `json:"user"`
}

// FromDomain converts a slice of domain.UserTenantRoleDetail to a slice of UserRoleResponse.
func FromDomain(utrs []domain.UserTenantRoleDetail) []UserRoleResponse {
	result := make([]UserRoleResponse, 0, len(utrs))
	for _, utr := range utrs {
		resp := UserRoleResponse{
			ID:        utr.ID.String(),
			UserID:    utr.UserID.String(),
			TenantID:  utr.TenantID.String(),
			RoleID:    utr.RoleID,
			RoleName:  utr.RoleName,
			Status:    string(utr.Status),
			CreatedAt: utr.CreatedAt.Format(time.RFC3339),
			UpdatedAt: utr.UpdatedAt.Format(time.RFC3339),
			User: &UserSummary{
				Email:     utr.UserEmail,
				Name:      utr.UserName,
				FirstName: utr.UserFirstName,
				LastName:  utr.UserLastName,
			},
		}
		if utr.AssignedBy != nil {
			s := utr.AssignedBy.String()
			resp.AssignedBy = &s
		}
		if utr.AssignedAt != nil {
			s := utr.AssignedAt.Format(time.RFC3339)
			resp.AssignedAt = &s
		}
		result = append(result, resp)
	}
	return result
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/handler/user_roles/list_user_roles/... -v"
```

Expected: both `TestFromDomain_MapsRoleNameAndUser` and `TestFromDomain_PendingAssignmentHasEmptyRoleName` `PASS`.

- [ ] **Step 5: Build and vet the whole module**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./... && go vet ./..."
```

Expected: no errors — this confirms the handler (`list_user_roles.go`, which calls `models.FromDomain(results)`) now compiles end-to-end with Task 1's usecase change.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/user_roles/list_user_roles/models/response.go internal/api/handler/user_roles/list_user_roles/models/response_test.go
git commit -m "feat: expose roleName and user in GET /user-roles response"
```

---

## Self-Review Notes

- **Spec coverage:** Query change (§1) → Task 1 Steps 4-5. Domain type (§2) → Task 1 Step 1. Response shape, additive-only (§3) → Task 2 Step 3. Testing section's three assertions (role name resolves, user resolves for the `tenant_id IS NULL` case, pending → empty role name) → Task 1's integration test covers all three directly against a real Postgres; Task 2's unit tests re-verify the same mapping at the pure-function level. No spec section is left uncovered.
- **Placeholder scan:** none — every step has complete, runnable code.
- **Type consistency:** `domain.UserTenantRoleDetail`'s field names (`RoleName`, `UserEmail`, `UserName`, `UserFirstName`, `UserLastName`) are identical between Task 1 (where the type is defined and scanned) and Task 2 (where `FromDomain` reads them) — checked by direct comparison.
- **Pre-verification:** the exact JOIN query in Task 1 Step 4 was hand-run against a fresh local Postgres (all 5 migrations applied) with a `users.tenant_id IS NULL` fixture row before being written into this plan — confirmed it returns the expected `role_name`/`user_email`/`user_first_name`/`user_last_name` for an active assignment and an empty `role_name` for a pending one.

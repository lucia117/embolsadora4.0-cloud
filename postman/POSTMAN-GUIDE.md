# Embolsadora API - Postman Complete Guide

Master collection for testing all Embolsadora 4.0 Cloud API endpoints across all features.

## 📦 Collection Files

### Primary Collection (USE THIS)
- **`Embolsadora-API-Complete.postman_collection.json`** — Master collection with 20+ endpoints organized in 4 folders

### Legacy Collections (for reference only)
- `User-Management-API.postman_collection.json` — Users CRUD (included in master)
- `Edge-Device-Management-API.postman_collection.json` — Edge Devices (included in master)
- `tenants.postman_collection.json` — Tenant management (included in master)
- `user-role-assignments.postman_collection.json` — User role assignments (included in master)

### Environment Files
- `Edge-Device-Management-API.postman_environment.json` — For edge device testing
- (Generic environment from collection variables)

## 🚀 Quick Start

### Step 1: Import Master Collection

```
Postman → File → Import → Select "Embolsadora-API-Complete.postman_collection.json"
```

### Step 2: Set Collection Variables

Click collection **Variables** tab and update:

| Variable | Example Value | Purpose |
|----------|---------------|---------|
| `baseUrl` | `http://localhost:8080/api/v1` | Base API URL |
| `jwt_token` | `eyJhbGciOiJIUzI1NiIs...` | Bearer token for auth |
| `tenant_id` | `acme` | Tenant subdomain (edge devices) |
| `tenantId` | `550e8400-e29b...` | Tenant UUID (users, roles) |
| `user_id` | `323e4567-e89b...` | User UUID for testing |
| `userId` | `00000000-0000...` | User UUID for role assignment |
| `device_id` | `550e8400-e29b...` | Device UUID for status checks |
| `utrId` | (auto-set) | User-Tenant-Role assignment UUID |

### Step 3: Start API Server

```bash
go run cmd/api/main.go
# Output: Listening on :8080
```

### Step 4: Execute Requests

Navigate to each folder in the collection:
1. **Tenants** — Create and manage tenants
2. **Users** — CRUD operations on users
3. **User Roles** — Assign and manage roles
4. **Edge Devices** — Register and monitor devices

---

## 📋 Folder Structure

### Tenants (5 endpoints)
- `GET All Tenants` — List all tenants
- `POST Create Tenant` — Create new tenant with admin user
- `GET Tenant by ID` — Get tenant details
- `PATCH Update Tenant` — Update tenant info
- `DELETE Tenant` — Delete tenant

**Headers**: Bearer JWT token

**Example**: Create a tenant and the collection auto-extracts `tenantId` for later use.

---

### Users (5 endpoints)
- `List Users (with pagination)` — Paginated user list for tenant
- `Get User by ID` — Retrieve user profile
- `Create User` — Create new user (admin only)
- `Update User` — Partial update (admin only)
- `Delete User` — Soft-delete user (admin only)

**Headers**:
- `X-Tenant-ID: {{tenant_id}}` (required)
- `Authorization: Bearer {{jwt_token}}`

**Flow**:
```
1. List Users → See current users
2. Create User → POST with firstName, lastName, email, role
3. Get User by ID → Verify creation
4. Update User → Change role or profile
5. Delete User → Soft-delete (soft delete = user marked deleted, record preserved)
```

---

### User Roles (7 endpoints)
- `POST Assign Role` — Assign role to user in tenant
- `GET List Assignments` — List all role assignments
- `GET List Assignments (filtered by status)` — Filter by active/pending/revoked
- `PUT Update Role` — Change assigned role
- `DELETE Revoke Role` — Revoke role (soft delete)
- `POST Bulk Assign Roles` — Assign same role to multiple users
- `GET User Roles (cross-tenant)` — View user's roles in all tenants

**Key Features**:
- Auto-sets `{{utrId}}` after POST Assign Role (used by PUT/DELETE)
- Built-in Postman tests validate response structure
- 409 conflict if user already has active role in tenant

**Flow**:
```
1. Assign Role (userId, tenantId, roleId)
   ↓ Auto-extracts utrId
2. List Assignments (verify creation)
3. Update Role (change roleId via utrId)
4. Try duplicate assign → 409 Conflict (expected)
5. Bulk Assign multiple users at once
6. Revoke Role (soft delete)
```

---

### Edge Devices (3 endpoints)
- `US1 - List Edge Devices` — GET all devices for tenant
- `US2 - Create Edge Device` — POST new device
- `US6 - Status Check` — POST connectivity check

**Key Details**:
- Uses tenant subdomain slug in path: `/tenants/{{tenant_id}}/edge-devices`
- Device status check performs health check against Raspberry Pi
- Response includes version, reachability, response time

**Flow**:
```
1. Create Device → machineId must be unique per tenant
2. List Devices → Verify creation
3. Status Check → Connectivity + version info
   ├─ 200 OK (device online)
   └─ 200 OK with ERROR (device offline but recorded)
```

---

## 🔐 Authentication

### Getting a JWT Token

#### Option 1: Generate Mock Token
Use any JWT from jwt.io with claims like:
```json
{
  "sub": "admin-user-id",
  "name": "Admin User",
  "role": "admin",
  "iat": 1677000000
}
```

#### Option 2: Use Actual Auth Endpoint
If your API has login:
```bash
POST http://localhost:8080/api/v1/login
Body: {"email": "admin@example.com", "password": "..."}
# Copy "access_token" from response → {{jwt_token}}
```

---

## 🧪 Common Testing Workflows

### Workflow 1: Complete User Management

```
1. SET Variables
   - baseUrl = http://localhost:8080/api/v1
   - tenant_id = "acme" (for edge devices)
   - tenantId = "550e8400..." (from DB or create tenant)
   - jwt_token = "eyJ..."

2. Users/List Users → 200 OK
3. Users/Create User → 201 Created (copies id to user_id)
4. Users/Get User by ID → 200 OK (verify creation)
5. Users/Update User → 200 OK (change role)
6. Users/Delete User → 204 No Content
7. Users/List Users → Verify user is soft-deleted (doesn't appear)
```

### Workflow 2: Role Assignment Workflow

```
1. SET Variables (same as above + userId)

2. User Roles/POST Assign Role → 201 Created
   (auto-extracts utrId)

3. User Roles/GET List Assignments → 200 OK
   (verify role is assigned)

4. User Roles/PUT Update Role → 200 OK
   (change roleId)

5. User Roles/DELETE Revoke Role → 200 OK
   (soft revoke - status=revoked)

6. User Roles/GET List Assignments (filtered)
   → Only "active" roles returned (revoked excluded)
```

### Workflow 3: Edge Device Monitoring

```
1. SET Variables
   - tenant_id = "acme" (subdomain)
   - device_id = "550e8400..." (from create response)

2. Edge Devices/US2 - Create Device → 201 Created
   (copies device id to device_id)

3. Edge Devices/US1 - List Devices → 200 OK
   (verify device registered)

4. Edge Devices/US6 - Status Check → 200 OK
   (check connectivity to Raspberry Pi at raspberryBaseUrl)
   Returns:
   - checkType: "STATUS"
   - overallStatus: "OK" or "ERROR"
   - summary: "Device reachable, version 1.2.3"
   - details: { version, reachable, responseTime }
```

---

## ⚠️ Error Handling

| Status | Error Code | Cause | Resolution |
|--------|-----------|-------|-----------|
| 400 | `MISSING_HEADER` | Missing X-Tenant-ID | Add header in collection |
| 400 | `VALIDATION_ERROR` | Invalid JSON or required fields missing | Check request body |
| 400 | `EDGE_DEVICE_DISABLED` | Device status is DISABLED | Device must be ACTIVE |
| 400 | `IMMUTABLE_FIELD` | Attempted to change email/tenantId | Only update mutable fields |
| 401 | `UNAUTHORIZED` | Invalid or missing JWT | Get fresh token |
| 403 | `INSUFFICIENT_PERMISSIONS` | User doesn't have admin role | POST/PATCH/DELETE need admin |
| 404 | `NOT_FOUND` | Resource doesn't exist | Verify ID and tenant |
| 409 | `CONFLICT` | machineId already exists | Use unique machineId per tenant |
| 409 | `CONFLICT` | User already has active role | Use PUT to update, DELETE to revoke |
| 500 | Internal error | Server error | Check API logs |

---

## 🔍 Built-In Postman Tests

All requests include **Tests** tab with assertions:

### User Roles Examples:
```javascript
// POST Assign Role
✓ Status 201
✓ success = true
✓ data has id and status active

// PUT Update Role
✓ Status 200
✓ roleId actualizado a operario

// DELETE Revoke Role
✓ Status 200
✓ status = revoked
```

Run tests: **Send** → view **Test Results** tab

---

## 📊 Multi-Tenant Isolation

**CRITICAL**: All operations are tenant-scoped.

- **Users endpoint**: Requires `X-Tenant-ID` header
- **User Roles endpoint**: Scoped by `tenantId` query param
- **Edge Devices endpoint**: Scoped by `:tenantId` in path
- **Emails are unique PER TENANT** — same email can exist in different tenants
- **Roles are PER TENANT** — user can have admin in tenant A, operario in tenant B

---

## 🛠️ Troubleshooting

### "Unknown Variable {{tenantId}}"
**Cause**: Variable not set in collection
**Fix**: Click collection → Variables → Set `tenantId` to actual UUID

### "401 Unauthorized"
**Cause**: JWT expired or missing
**Fix**: Get fresh token from auth endpoint or jwt.io

### "409 Conflict - machineId already exists"
**Cause**: Creating device with duplicate machineId in same tenant
**Fix**: Use unique machineId or change tenant

### "Test script not running"
**Cause**: Tests tab exists but not executing
**Fix**: Click **Send** and view **Test Results** tab (bottom)

### "X-Tenant-ID header is required"
**Cause**: Missing multi-tenant header
**Fix**: Verify header is set in Users requests

---

## 📚 Additional Resources

- **Specification**: `specs/003-edge-device-management/spec.md`
- **Plan**: `specs/003-edge-device-management/plan.md`
- **Data Model**: `specs/003-edge-device-management/data-model.md`
- **OpenAPI Contract**: `specs/003-edge-device-management/contracts/`
- **User Guide**: `POSTMAN-GUIDE.md` (this file)

---

## 📝 Version History

- **2026-03-11**: Master collection created — consolidated all 4 separate collections
- **2026-03-02**: User Management API collection + docs
- **2026-03-11**: Edge Device Management collection + README
- **2026-02-28**: User Role Assignments collection
- **2026-02-27**: Tenants collection

---

**Status**: Production-Ready
**Collection Version**: 2.0 (Consolidated)
**Last Updated**: 2026-03-11

# API Contract: user-role-service-api

**Pact Version**: 2.0.0
**Consumer**: `embolsadora-frontend-bff`
**Provider**: `user-role-service-api` (this service)
**Source**: `user-role-service-api.json` (Pact file)
**Auth**: JWT Bearer Token (`Authorization: Bearer <token>`) — required on all endpoints
**Base Path**: `/api/v1`
**Response Envelope**: `{ "success": bool, "data": <payload> }` on success; `{ "success": false, "error": "<message>" }` on error

---

## Endpoints

### 1. GET /api/v1/user-roles

List user-role assignments for a tenant, with optional status filter.

**Query Parameters**:
| Param | Required | Type | Description |
|-------|----------|------|-------------|
| `tenantId` | yes | UUID string | Filter assignments to this tenant |
| `status` | no | `active` \| `pending` \| `revoked` | Filter by assignment status |

**Response 200**:
```json
{
  "success": true,
  "data": [
    {
      "id": "<uuid>",
      "userId": "<uuid>",
      "tenantId": "<uuid>",
      "roleId": "admin",
      "status": "active",
      "assignedBy": "<uuid>",
      "assignedAt": "2026-02-08T10:00:00Z",
      "createdAt": "2026-02-08T10:00:00Z",
      "updatedAt": "2026-02-08T10:00:00Z"
    }
  ]
}
```

**Matching Rules** (from Pact):
- `$.body.data`: min 0 items (empty array is valid)
- `$.body.data[*].id`: type match (any string)
- `$.body.data[*].status`: regex `active|pending|revoked`

**Response 400**: `tenantId` missing or not a valid UUID.

---

### 2. POST /api/v1/user-roles

Assign a role to a user within a tenant.

**Request Body**:
```json
{
  "userId": "<uuid>",
  "tenantId": "<uuid>",
  "roleId": "admin"
}
```

**Response 201** — assignment created:
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "userId": "<uuid>",
    "tenantId": "<uuid>",
    "roleId": "admin",
    "status": "active",
    "assignedBy": "<uuid>",
    "assignedAt": "2026-02-08T10:00:00Z",
    "createdAt": "2026-02-08T10:00:00Z",
    "updatedAt": "2026-02-08T10:00:00Z"
  }
}
```

**Matching Rules** (from Pact):
- `$.body.data.id`: type match
- `$.body.data.assignedBy`: type match

**Response 409** — user already has active role in this tenant:
```json
{
  "success": false,
  "error": "User already has an active role in this tenant. Use PUT to update."
}
```

---

### 3. PUT /api/v1/user-roles/:id

Update the role on an existing assignment (full replacement of roleId).

**Path Parameters**: `id` — UUID of the UTR record

**Request Body**:
```json
{
  "roleId": "operario"
}
```

**Response 200**:
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "userId": "<uuid>",
    "tenantId": "<uuid>",
    "roleId": "operario",
    "status": "active",
    "assignedBy": "<uuid>",
    "assignedAt": "2026-02-08T10:30:00Z",
    "createdAt": "2026-02-08T10:00:00Z",
    "updatedAt": "2026-02-08T10:30:00Z"
  }
}
```

**Response 404**: Assignment not found.

---

### 4. DELETE /api/v1/user-roles/:id

Revoke a role assignment (soft delete — record preserved with status `revoked`).

**Path Parameters**: `id` — UUID of the UTR record

**Response 200**:
```json
{
  "success": true,
  "data": {
    "id": "<uuid>",
    "status": "revoked"
  }
}
```

**Response 404**: Assignment not found.

---

### 5. POST /api/v1/user-roles/bulk

Assign the same role to multiple users within a tenant. **All-or-nothing**: if any user already has an active role, the entire operation is rejected.

**Request Body**:
```json
{
  "userIds": ["<uuid>", "<uuid>", "<uuid>"],
  "tenantId": "<uuid>",
  "roleId": "operario"
}
```

**Response 201** — all assignments created:
```json
{
  "success": true,
  "data": {
    "assigned": 3,
    "failed": 0,
    "assignments": [
      { "id": "<uuid>", "userId": "<uuid>", "roleId": "operario", "status": "active" },
      { "id": "<uuid>", "userId": "<uuid>", "roleId": "operario", "status": "active" },
      { "id": "<uuid>", "userId": "<uuid>", "roleId": "operario", "status": "active" }
    ]
  }
}
```

**Response 409** — one or more users already have an active role:
```json
{
  "success": false,
  "error": "User already has an active role in this tenant. Use PUT to update."
}
```

---

### 6. GET /api/v1/users/:userId/roles

Get all role assignments for a specific user across all tenants. Platform-admin endpoint.

**Path Parameters**: `userId` — UUID of the user

**Response 200**:
```json
{
  "success": true,
  "data": [
    {
      "tenantId": "<uuid>",
      "tenantName": "Acme Corp",
      "roleId": "admin",
      "roleName": "Admin",
      "status": "active"
    },
    {
      "tenantId": "<uuid>",
      "tenantName": "Demo Tenant",
      "roleId": "operario",
      "roleName": "Operario",
      "status": "active"
    }
  ]
}
```

---

## Provider States (for Pact verification)

| Provider State | Setup Required |
|----------------|----------------|
| `tenant acme exists with user-role assignments` | Seed tenant + UTR rows |
| `tenant acme has pending user assignments` | Seed tenant + UTR with status=pending |
| `user user_xyz and tenant acme and role admin exist` | Seed user, tenant, role |
| `user user_xyz already has active role in tenant acme` | Seed active UTR for user+tenant |
| `user-role assignment utr_abc123 exists` | Seed specific UTR row |
| `user-role assignment utr_abc123 exists with status active` | Seed active UTR |
| `users user_1, user_2, user_3 exist and tenant acme exists` | Seed 3 users + tenant |
| `user user_xyz has roles in multiple tenants` | Seed user with UTR rows in ≥2 tenants |

# Quickstart: User Management API

## Overview

This quickstart demonstrates how to test the User Management CRUD API end-to-end after implementation.

## Prerequisites

- Running embolsadora-api backend (`go run ./cmd/api`)
- PostgreSQL database initialized with migrations
- Valid JWT bearer token for a tenant admin
- Tenant ID (UUID)

## Environment Setup

```bash
# Tenant ID (replace with actual UUID)
TENANT_ID="550e8400-e29b-41d4-a716-446655440000"

# Admin JWT token (obtained from login endpoint)
JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# API base URL
API_URL="http://localhost:8080"
```

## Test Scenarios

### 1. List Users (GET /api/v1/users)

**Happy Path**: Retrieve paginated list of users

```bash
curl -X GET "$API_URL/api/v1/users?limit=20&offset=0" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json"
```

**Expected Response (200 OK)**:
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "firstName": "Admin",
      "lastName": "User",
      "email": "admin@example.com",
      "role": "admin",
      "tenantId": "550e8400-e29b-41d4-a716-446655440000",
      "image": null,
      "createdAt": "2026-03-01T10:00:00Z",
      "updatedAt": "2026-03-01T10:00:00Z",
      "deletedAt": null
    }
  ],
  "pagination": {
    "total": 5,
    "count": 1,
    "limit": 20,
    "offset": 0
  }
}
```

**Error Case**: Missing X-Tenant-ID header

```bash
curl -X GET "$API_URL/api/v1/users?limit=20&offset=0" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json"
```

**Expected Response (400 Bad Request)**:
```json
{
  "error": "MISSING_HEADER",
  "message": "X-Tenant-ID header is required",
  "status": 400
}
```

### 2. Create User (POST /api/v1/users)

**Happy Path**: Create a new user in the tenant

```bash
curl -X POST "$API_URL/api/v1/users" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "John",
    "lastName": "Doe",
    "email": "john.doe@example.com",
    "role": "user",
    "image": "https://example.com/avatar.jpg"
  }'
```

**Expected Response (201 Created)**:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440002",
  "firstName": "John",
  "lastName": "Doe",
  "email": "john.doe@example.com",
  "role": "user",
  "tenantId": "550e8400-e29b-41d4-a716-446655440000",
  "image": "https://example.com/avatar.jpg",
  "createdAt": "2026-03-01T10:05:00Z",
  "updatedAt": "2026-03-01T10:05:00Z",
  "deletedAt": null
}
```

**Error Case**: Duplicate email in same tenant

```bash
curl -X POST "$API_URL/api/v1/users" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "Jane",
    "lastName": "Smith",
    "email": "john.doe@example.com",
    "role": "user"
  }'
```

**Expected Response (409 Conflict)**:
```json
{
  "error": "DUPLICATE_EMAIL",
  "message": "Email already exists in this tenant",
  "status": 409
}
```

### 3. Get User (GET /api/v1/users/:id)

**Happy Path**: Retrieve a specific user

```bash
USER_ID="550e8400-e29b-41d4-a716-446655440002"

curl -X GET "$API_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json"
```

**Expected Response (200 OK)**: Full user profile

**Error Case**: User not found or soft-deleted

```bash
curl -X GET "$API_URL/api/v1/users/550e8400-e29b-41d4-a716-446655440999" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json"
```

**Expected Response (404 Not Found)**:
```json
{
  "error": "USER_NOT_FOUND",
  "message": "User not found",
  "status": 404
}
```

### 4. Update User (PATCH /api/v1/users/:id)

**Happy Path**: Update user first name and role

```bash
USER_ID="550e8400-e29b-41d4-a716-446655440002"

curl -X PATCH "$API_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "Jonathan",
    "role": "admin"
  }'
```

**Expected Response (200 OK)**: Updated user profile with new `updatedAt` timestamp

**Error Case**: Attempt to modify immutable field (email)

```bash
curl -X PATCH "$API_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "new.email@example.com"
  }'
```

**Expected Response (400 Bad Request)**:
```json
{
  "error": "IMMUTABLE_FIELD",
  "message": "Field 'email' cannot be modified",
  "status": 400
}
```

### 5. Delete User (DELETE /api/v1/users/:id)

**Happy Path**: Soft-delete a user

```bash
USER_ID="550e8400-e29b-41d4-a716-446655440002"

curl -X DELETE "$API_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json"
```

**Expected Response (204 No Content)**: No response body

**Verify Soft Delete**: Try to retrieve the deleted user

```bash
curl -X GET "$API_URL/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json"
```

**Expected Response (404 Not Found)**: User appears deleted, but data is retained in DB

## Multi-Tenant Isolation Testing

### Cross-Tenant Access Attempt

Attempt to list users from a different tenant:

```bash
OTHER_TENANT_ID="550e8400-e29b-41d4-a716-446655440999"

curl -X GET "$API_URL/api/v1/users?limit=20&offset=0" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $OTHER_TENANT_ID" \
  -H "Content-Type: application/json"
```

**Expected Response (403 Forbidden)**:
```json
{
  "error": "ACCESS_DENIED",
  "message": "User does not belong to this tenant",
  "status": 403
}
```

## Authorization Testing

### Non-Admin User Attempt to Create

Non-admin users cannot create, update, or delete users. Only listing and viewing their own data is allowed:

```bash
# Using JWT token for a 'user' role
curl -X POST "$API_URL/api/v1/users" \
  -H "Authorization: Bearer $NON_ADMIN_JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "Test",
    "lastName": "User",
    "email": "test@example.com",
    "role": "user"
  }'
```

**Expected Response (403 Forbidden)**:
```json
{
  "error": "INSUFFICIENT_PERMISSIONS",
  "message": "Only admin users can create users",
  "status": 403
}
```

## Integration Testing Checklist

- [ ] List users returns paginated results (limit/offset working)
- [ ] Create user generates correct ID, timestamps, and sets deleted_at=null
- [ ] Duplicate email in same tenant returns 409 Conflict
- [ ] Same email in different tenant allowed without conflict
- [ ] Get user returns 404 for soft-deleted users
- [ ] Update user prevents modification of email and tenantId
- [ ] Delete user sets deleted_at and excludes from future queries
- [ ] Missing X-Tenant-ID header returns 400
- [ ] Cross-tenant access returns 403
- [ ] Non-admin users cannot write (create, update, delete)
- [ ] Admin users can perform all operations
- [ ] All responses include correct HTTP status codes
- [ ] All error responses follow standard error schema

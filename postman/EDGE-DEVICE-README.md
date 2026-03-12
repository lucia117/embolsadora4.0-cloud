# Edge Device Management API - Postman Collection

Complete testing guide for Edge Device Management API MVP (US1, US2, US6).

## 📦 Files

- `Edge-Device-Management-API.postman_collection.json` - 3 endpoint requests with examples
- `Edge-Device-Management-API.postman_environment.json` - Environment variables (base_url, tenant_id, jwt_token, device_id)

## 🚀 Quick Start

### 1. Import Collection & Environment

```
Postman → File → Import → Select both JSON files
```

### 2. Set Environment Variables

Click the eye icon → Select "Edge Device Management API" environment

Update these variables:
- `base_url`: `http://localhost:8080` (default)
- `tenant_id`: Your tenant subdomain slug (e.g., "acme")
- `jwt_token`: Valid JWT Bearer token from auth endpoint

### 3. Start the API Server

```bash
go run cmd/api/main.go
```

Wait for: `Listening on :8080`

## 📋 API Endpoints (MVP)

### Endpoint 1: List Edge Devices (GET)

**Route:** `GET /api/tenants/:tenantId/edge-devices`

**Description:** Retrieve all edge devices for a tenant

**Request:**
```bash
GET http://localhost:8080/api/tenants/acme/edge-devices
Authorization: Bearer {{jwt_token}}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
      "name": "RaspPi-01",
      "machineId": "M-EQ-001",
      "edgeType": "RASPBERRY_PLC",
      "status": "ACTIVE",
      "createdAt": "2026-03-11T10:00:00Z"
    }
  ]
}
```

**Error Responses:**
- `401 Unauthorized`: Missing or invalid JWT
- `404 Not Found`: Tenant not found
- `500 Internal Server Error`: Database error

---

### Endpoint 2: Create Edge Device (POST)

**Route:** `POST /api/tenants/:tenantId/edge-devices`

**Description:** Register a new edge device with initial ACTIVE status

**Request Body (Required Fields):**
```json
{
  "name": "RaspPi-02",
  "machineId": "M-EQ-002",
  "edgeType": "RASPBERRY_PLC",
  "raspberryBaseUrl": "http://192.168.1.101:8080"
}
```

**Request Body (Optional Fields):**
```json
{
  "name": "RaspPi-02",
  "machineId": "M-EQ-002",
  "edgeType": "RASPBERRY_PLC",
  "raspberryBaseUrl": "http://192.168.1.101:8080",
  "description": "Secondary edge gateway",
  "plcAddress": "192.168.1.51"
}
```

**Field Validation Rules:**
- `name`: Required, max 255 chars
- `machineId`: Required, max 100 chars, unique per tenant
- `edgeType`: Required, must be "RASPBERRY_PLC"
- `raspberryBaseUrl`: Required, valid HTTP/HTTPS URL
- `plcAddress`: Optional, valid IP or hostname

**Success Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "tenantId": "123e4567-e89b-12d3-a456-426614174000",
    "name": "RaspPi-02",
    "machineId": "M-EQ-002",
    "edgeType": "RASPBERRY_PLC",
    "status": "ACTIVE",
    "lastHealthStatus": "UNKNOWN",
    "createdAt": "2026-03-11T16:00:00Z",
    "updatedAt": "2026-03-11T16:00:00Z"
  }
}
```

**Error Responses:**
- `400 Bad Request`: Validation error (missing or invalid fields)
  ```json
  {"success": false, "error": "name, machineId, edgeType, and raspberryBaseUrl are required"}
  ```
- `409 Conflict`: machineId already exists for tenant
  ```json
  {"success": false, "error": "CONFLICT: machineId ya existe en el tenant"}
  ```
- `401 Unauthorized`: Invalid JWT
- `500 Internal Server Error`: Database error

**Testing Scenarios:**
1. ✅ Create device with all required fields
2. ✅ Create device with optional fields (description, plcAddress)
3. ❌ Create device without required field → 400
4. ❌ Create device with invalid edgeType → 400
5. ❌ Create device with duplicate machineId → 409

---

### Endpoint 3: Status Check (POST)

**Route:** `POST /api/tenants/:tenantId/edge-devices/:deviceId/status`

**Description:** Perform connectivity + version check on an ACTIVE device

**Path Parameters:**
- `tenantId`: Tenant subdomain slug
- `deviceId`: Device UUID (from create response)

**Request:**
```bash
POST http://localhost:8080/api/tenants/acme/edge-devices/550e8400-e29b-41d4-a716-446655440000/status
Authorization: Bearer {{jwt_token}}
```

**Success Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "checkType": "STATUS",
    "checkedAt": "2026-03-11T16:15:00Z",
    "overallStatus": "OK",
    "summary": "Device reachable, version 1.2.3",
    "details": {
      "version": "1.2.3",
      "reachable": true,
      "responseTime": 120
    }
  }
}
```

**Error Response - Device Disabled (400):**
```json
{
  "success": false,
  "error": "EDGE_DEVICE_DISABLED"
}
```

**Error Response - Device Not Found (404):**
```json
{
  "success": false,
  "error": "Not found"
}
```

**Error Responses:**
- `400 Bad Request`: Device is disabled
- `404 Not Found`: Device not found
- `401 Unauthorized`: Invalid JWT
- `500 Internal Server Error`: Network/device error

**Testing Scenarios:**
1. ✅ Status check on ACTIVE device → 200 with result
2. ❌ Status check on DISABLED device → 400 EDGE_DEVICE_DISABLED
3. ❌ Status check on non-existent device → 404
4. ❌ Status check with invalid UUID → 400

---

## 🔄 Full Test Workflow

### Step 1: List Devices
```bash
[1] GET /api/tenants/acme/edge-devices
Expected: 200 OK with empty array (or existing devices)
```

### Step 2: Create Device
```bash
[2] POST /api/tenants/acme/edge-devices
Body: {
  "name": "TestDevice-01",
  "machineId": "TEST-001",
  "edgeType": "RASPBERRY_PLC",
  "raspberryBaseUrl": "http://192.168.1.100:8080"
}
Expected: 201 Created
Note: Copy device ID from response → save to {{device_id}}
```

### Step 3: Verify Device in List
```bash
[3] GET /api/tenants/acme/edge-devices
Expected: 200 OK with device in array
```

### Step 4: Status Check
```bash
[4] POST /api/tenants/acme/edge-devices/{{device_id}}/status
Expected: 200 OK (device reachable) or 200 with ERROR status (device unreachable)
```

---

## 🔐 Authentication Setup

### Option 1: Existing Auth Endpoint
If your API has an auth endpoint (e.g., `POST /api/auth/login`):

```bash
POST http://localhost:8080/api/auth/login
Body: {
  "email": "user@example.com",
  "password": "password"
}
```

Copy `access_token` → Update `{{jwt_token}}` in environment

### Option 2: Mock JWT Token
For testing, you can use a pre-generated JWT:

```
Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

---

## 📊 Expected Database State

### After Running All Tests

**edge_devices table:**
```
id                                   | tenant_id  | name            | machine_id | status
550e8400-e29b-41d4-a716-446655440001 | acme-uuid  | TestDevice-01  | TEST-001   | ACTIVE
```

**device_events table (if status check succeeded):**
```
id                                   | device_id | check_type | overall_status
660e8400-e29b-41d4-a716-446655440001 | device-id | STATUS     | OK or ERROR
```

---

## 🛠️ Troubleshooting

### 401 Unauthorized
**Problem:** JWT token invalid or expired
**Solution:**
1. Get fresh token from auth endpoint
2. Update `{{jwt_token}}` in environment
3. Verify token is in request header: `Authorization: Bearer <token>`

### 404 Tenant Not Found
**Problem:** Tenant slug doesn't exist in database
**Solution:**
1. Check tenants table: `SELECT subdomain, id FROM tenants;`
2. Use existing tenant subdomain
3. Update `{{tenant_id}}` in environment

### 409 Conflict - machineId Already Exists
**Problem:** Trying to create device with existing machineId
**Solution:**
1. Use unique machineId: `TEST-001`, `TEST-002`, etc.
2. Or delete existing device first (not implemented in MVP)

### Device Status Check Returns ERROR
**Problem:** Edge device at raspberryBaseUrl is unreachable
**Solution:**
1. Verify device IP is correct
2. Ping the device: `ping 192.168.1.100`
3. Check device is running on specified port: `http://192.168.1.100:8080`
4. Check firewall rules

---

## 📝 Example cURL Commands

### List Devices
```bash
curl -X GET \
  http://localhost:8080/api/tenants/acme/edge-devices \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json"
```

### Create Device
```bash
curl -X POST \
  http://localhost:8080/api/tenants/acme/edge-devices \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "RaspPi-01",
    "machineId": "M-001",
    "edgeType": "RASPBERRY_PLC",
    "raspberryBaseUrl": "http://192.168.1.100:8080"
  }'
```

### Status Check
```bash
curl -X POST \
  http://localhost:8080/api/tenants/acme/edge-devices/550e8400-e29b-41d4-a716-446655440000/status \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json"
```

---

## ✅ MVP Validation Checklist

- [ ] Server running: `go run cmd/api/main.go`
- [ ] Environment variables set (base_url, tenant_id, jwt_token)
- [ ] [1] List Devices: 200 OK
- [ ] [2] Create Device: 201 Created + save device_id
- [ ] [3] Verify Device in List: 200 OK with device
- [ ] [4] Status Check: 200 OK (success or device error)
- [ ] Database queries work: Check edge_devices, device_events tables
- [ ] Error handling works: Test 404, 400, 409 scenarios
- [ ] Multi-tenant isolation: Create device in different tenant_id

---

## 📚 Additional Resources

- OpenAPI Contract: `specs/003-edge-device-management/contracts/edge-device-service-api.openapi.yaml`
- Database Schema: `specs/003-edge-device-management/plan/data-model.md`
- Implementation Plan: `specs/003-edge-device-management/plan.md`
- Feature Spec: `specs/003-edge-device-management/spec.md`

---

**Created:** 2026-03-11
**API Version:** v1
**Collection Version:** 1.0

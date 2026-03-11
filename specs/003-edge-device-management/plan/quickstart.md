# Quickstart: Edge Device Management API

## Running Locally

```bash
# 1. Start PostgreSQL and Redis
docker-compose up -d

# 2. Run migrations
go run cmd/migrate/main.go up

# 3. Start the API server
go run cmd/api/main.go
```

## Testing the API

All endpoints require a valid JWT Bearer token. Obtain one via the auth surface:

```bash
POST /api/auth/login
{
  "email": "admin@acme.com",
  "password": "your-password"
}
```

Then use the token in all subsequent requests:

```bash
Authorization: Bearer <token>
```

The `tenantId` path parameter is the tenant's **subdomain** (e.g., `acme`), not the UUID.

## Example Flows

### Register a device

```bash
curl -X POST http://localhost:8080/api/tenants/acme/edge-devices \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Embolsadora Línea 1",
    "machineId": "EMB-L1-001",
    "edgeType": "RASPBERRY_PLC",
    "raspberryBaseUrl": "http://192.168.1.10:8080",
    "plcAddress": "192.168.1.20"
  }'
```

### Trigger a health check

```bash
curl -X POST http://localhost:8080/api/tenants/acme/edge-devices/<device-uuid>/health-check \
  -H "Authorization: Bearer <token>"
```

### Get telemetry

```bash
curl http://localhost:8080/api/tenants/acme/edge-devices/<device-uuid>/telemetry \
  -H "Authorization: Bearer <token>"
```

## Running Tests

```bash
# Unit tests
go test ./internal/...

# Integration tests (requires running DB)
go test ./tests/integration/...
```

## Key Configuration

| Config Key | Description | Default |
|-----------|-------------|---------|
| `EDGE_CLIENT_TIMEOUT` | HTTP timeout for device calls | `10s` |
| `DATABASE_URL` | PostgreSQL connection string | — |
| `REDIS_URL` | Redis connection string | — |
| `JWT_SECRET` | JWT signing secret | — |

# Quickstart: Alarm Rules Service API (008)

**Objetivo**: Validar los 10 contratos Pact de `alarm-rules-service-api` contra el servidor local.  
**Prerequisitos**: Servidor corriendo en `http://localhost:8080`, usuario admin autenticado, migración 000014 aplicada.

---

## Setup: Variables

```bash
BASE_URL="http://localhost:8080/api/v1"
JWT="<token del POST /api/v1/auth/login>"
TENANT_ID="<UUID del tenant>"
RULE_ID=""  # se llena después de crear la primera regla
```

---

## Pact 1 — GET /alarm-rules → 401 (sin auth)

```bash
curl -s "$BASE_URL/alarm-rules" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (401):
```json
{"success": false, "error": "UNAUTHORIZED", "message": "token de autenticación requerido", "status": 401}
```

---

## Pact 2 — GET /alarm-rules → 200 (lista vacía)

```bash
curl -s "$BASE_URL/alarm-rules" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (200):
```json
{"success": true, "data": []}
```

---

## Pact 3 — POST /alarm-rules → 201 (crear regla)

```bash
RULE_ID=$(curl -s -X POST "$BASE_URL/alarm-rules" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Temperatura alta",
    "description": "Alerta cuando la temperatura supera el umbral de seguridad",
    "metric": "temperature",
    "operator": "gt",
    "threshold": 80.0,
    "severity": "critical",
    "enabled": true
  }' | jq -r '.data.id')

echo "RULE_ID=$RULE_ID"
```

**Esperado** (201): objeto con `id`, `tenantId`, `name`, `metric`, `operator`, `threshold`, `severity`, `enabled`, `createdAt`, `updatedAt`.

---

## Pact 4 — POST /alarm-rules → 400 (validación — campo faltante)

```bash
curl -s -X POST "$BASE_URL/alarm-rules" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Regla incompleta",
    "operator": "gt",
    "threshold": 50.0
  }' | jq .
```

**Esperado** (400):
```json
{"success": false, "error": "VALIDATION_ERROR", "message": "...", "status": 400}
```

---

## Pact 5 — GET /alarm-rules → 200 (lista con una regla)

```bash
curl -s "$BASE_URL/alarm-rules" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (200): array con la regla creada anteriormente.

---

## Pact 6 — GET /alarm-rules/:id → 200 (obtener regla)

```bash
curl -s "$BASE_URL/alarm-rules/$RULE_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (200): detalle de la regla.

---

## Pact 7 — GET /alarm-rules/:id → 404 (ID inexistente)

```bash
curl -s "$BASE_URL/alarm-rules/00000000-0000-0000-0000-000000000000" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (404):
```json
{"success": false, "error": "NOT_FOUND", "message": "regla de alarma no encontrada", "status": 404}
```

---

## Pact 8 — PATCH /alarm-rules/:id → 200 (actualizar regla)

```bash
curl -s -X PATCH "$BASE_URL/alarm-rules/$RULE_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"threshold": 85.0, "severity": "warning"}' | jq .
```

**Esperado** (200): regla con `threshold=85.0`, `severity=warning`, `updatedAt` actualizado.

---

## Pact 9 — PATCH /alarm-rules/:id → 404 (ID inexistente)

```bash
curl -s -X PATCH "$BASE_URL/alarm-rules/00000000-0000-0000-0000-000000000000" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"threshold": 90.0}' | jq .
```

**Esperado** (404).

---

## Pact 10 — DELETE /alarm-rules/:id → 200

```bash
curl -s -X DELETE "$BASE_URL/alarm-rules/$RULE_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (200):
```json
{"success": true}
```

---

## Pact 11 — DELETE /alarm-rules/:id → 404 (ya eliminada)

```bash
curl -s -X DELETE "$BASE_URL/alarm-rules/$RULE_ID" \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado** (404).

---

## Checklist de validación Pact

| # | Interacción Pact | Estado |
|---|---|---|
| 1 | GET /alarm-rules → 401 sin auth | ⬜ |
| 2 | GET /alarm-rules → 200 lista | ⬜ |
| 3 | POST /alarm-rules → 201 crear | ⬜ |
| 4 | POST /alarm-rules → 400 validación | ⬜ |
| 5 | GET /alarm-rules → 200 con datos | ⬜ |
| 6 | GET /alarm-rules/{id} → 200 | ⬜ |
| 7 | GET /alarm-rules/{id} → 404 | ⬜ |
| 8 | PATCH /alarm-rules/{id} → 200 | ⬜ |
| 9 | PATCH /alarm-rules/{id} → 404 | ⬜ |
| 10 | DELETE /alarm-rules/{id} → 200 | ⬜ |
| 11 | DELETE /alarm-rules/{id} → 404 | ⬜ |

Marcar ✅ cuando el curl retorne el response esperado.

# Quickstart: Notification Service API (010)

**Propósito**: Validar los 6 contratos Pact del `notification-service-api` contra servidor local.

## Prerequisitos

```bash
# 1. Aplicar migración
migrate -path migrations/ -database $DATABASE_URL up 1

# 2. Levantar servidor
docker-compose up -d

# 3. Insertar notificaciones de prueba (seed)
psql $DATABASE_URL -c "
INSERT INTO notifications (id, tenant_id, title, message, severity, status, alarm_rule_id, machine_id)
VALUES
  ('a1b2c3d4-e5f6-7890-abcd-ef1234567890',
   'b2c3d4e5-f6a7-8901-bcde-f12345678901',
   'Temperatura crítica detectada',
   'La métrica temperature superó el umbral de 80°C en la máquina M-001',
   'critical', 'unread',
   'c3d4e5f6-a7b8-9012-cdef-123456789012',
   'd4e5f6a7-b8c9-0123-defa-234567890123'),
  ('b2c3d4e5-f6a7-8901-bcde-f12345678901',
   'b2c3d4e5-f6a7-8901-bcde-f12345678901',
   'Presión fuera de rango',
   'La métrica pressure bajó del umbral mínimo de 2 bar',
   'warning', 'acknowledged',
   NULL, NULL);
"

# Variables
export BASE_URL=http://localhost:8080/api/v1
export JWT_TOKEN=<tu-jwt-válido>
export TENANT_ID=b2c3d4e5-f6a7-8901-bcde-f12345678901
export NOTIF_ID=a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

---

## Pact 1 — GET /notifications (lista con datos) → 200

```bash
curl -s -X GET "$BASE_URL/notifications" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` con `{ data: [...], total: 2, limit: 20, offset: 0 }`

---

## Pact 2 — GET /notifications (sin auth) → 401

```bash
curl -s -X GET "$BASE_URL/notifications" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `401 UNAUTHORIZED` con `{ error: "...", code: "UNAUTHORIZED" }`

---

## Pact 3 — GET /notifications/count → 200

```bash
curl -s -X GET "$BASE_URL/notifications/count" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` con `{ "unread": 1 }` (solo la notificación con status='unread')

---

## Pact 4 — GET /notifications/:id (existente) → 200

```bash
curl -s -X GET "$BASE_URL/notifications/$NOTIF_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` con todos los campos de la notificación, `status: "unread"`

---

## Pact 5 — GET /notifications/:id (inexistente) → 404

```bash
curl -s -X GET "$BASE_URL/notifications/00000000-0000-0000-0000-000000000000" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `404 NOT_FOUND` con `{ error: "notificación no encontrada", code: "NOT_FOUND" }`

---

## Pact 6 — POST /notifications/:id/ack → 200

```bash
curl -s -X POST "$BASE_URL/notifications/$NOTIF_ID/ack" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` con `status: "acknowledged"` y `acknowledged_at` seteado

```bash
# Idempotencia: segunda llamada también retorna 200
curl -s -X POST "$BASE_URL/notifications/$NOTIF_ID/ack" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` con mismo `acknowledged_at` (no se modifica)

---

## Pact 7 — POST /notifications/:id/close → 200

```bash
curl -s -X POST "$BASE_URL/notifications/$NOTIF_ID/close" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` con `status: "closed"` y `closed_at` seteado

```bash
# Verificar que el conteo de unread se decrementó
curl -s -X GET "$BASE_URL/notifications/count" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `{ "unread": 0 }`

---

## Pact 8 — GET /alarm-rules → 200 (ya implementado en 008)

```bash
curl -s -X GET "$BASE_URL/alarm-rules" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

**Esperado**: `200 OK` — este endpoint ya existe desde feature 008. Solo verificar que responde correctamente.

---

## Filtros opcionales

```bash
# Filtrar por status=unread
curl -s "$BASE_URL/notifications?status=unread" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .

# Filtrar por severity=critical
curl -s "$BASE_URL/notifications?severity=critical" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .

# Paginación
curl -s "$BASE_URL/notifications?limit=5&offset=0" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" | jq .
```

---

## Checklist de validación

- [ ] Pact 1: GET /notifications → 200 con lista paginada
- [ ] Pact 2: GET /notifications sin auth → 401
- [ ] Pact 3: GET /notifications/count → 200 con conteo correcto
- [ ] Pact 4: GET /notifications/:id (existente) → 200
- [ ] Pact 5: GET /notifications/:id (inexistente) → 404
- [ ] Pact 6: POST /notifications/:id/ack → 200 + idempotente
- [ ] Pact 7: POST /notifications/:id/close → 200 + idempotente
- [ ] Pact 8: GET /alarm-rules → 200 (verificación de 008)
- [ ] Aislamiento multi-tenant: notificaciones de otro tenant no visibles

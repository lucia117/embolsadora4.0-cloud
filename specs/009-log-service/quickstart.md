# Quickstart: Log Service API (009)

**Objetivo**: Validar los 14 contratos Pact de `log-service-api` contra el servidor local.  
**Prerequisitos**: Servidor corriendo en `http://localhost:8080`, usuario admin autenticado, migración 000004 aplicada.

---

## Setup: Variables

```bash
BASE_URL="http://localhost:8080/api/v1"
TOKEN="<jwt_token_del_usuario_admin>"
TENANT_ID="550e8400-e29b-41d4-a716-446655440001"
MACHINE_ID="<uuid_de_una_maquina_del_tenant>"
```

### Insertar logs de prueba

```sql
INSERT INTO log_entries (tenant_id, severity, event_type, machine_id, message, metadata)
VALUES
  ('550e8400-e29b-41d4-a716-446655440001', 'warning',  'alarm_triggered',    '<machine_id>', 'Temperatura supera umbral', '{"threshold": 80}'),
  ('550e8400-e29b-41d4-a716-446655440001', 'info',     'device_connected',   '<machine_id>', 'Dispositivo conectado', '{}'),
  ('550e8400-e29b-41d4-a716-446655440001', 'critical', 'alarm_triggered',    '<machine_id>', 'Falla crítica en motor', '{"component": "motor"}'),
  ('550e8400-e29b-41d4-a716-446655440001', 'info',     'user_action',        NULL,           'Usuario cambió configuración', '{"user": "admin"}'),
  ('550e8400-e29b-41d4-a716-446655440001', 'error',    'device_disconnected','<machine_id>', 'Desconexión inesperada', '{}');

-- Guardar un ID para las pruebas de detalle
LOG_ID=$(psql -c "SELECT id FROM log_entries WHERE tenant_id='550e8400-e29b-41d4-a716-446655440001' LIMIT 1" -t | tr -d ' ')
```

---

## Pact 1 — List Logs 401 (sin auth)

```bash
curl -s -o /dev/null -w "%{http_code}" \
  "$BASE_URL/logs"
# Esperado: 401
```

---

## Pact 2 — List Logs con filtros (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs?severity=warning&event_type=alarm_triggered"
# Esperado: 200, data con logs filtrados, next_cursor, total
```

---

## Pact 3 — List Logs búsqueda por texto (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs?q=Temperatura"
# Esperado: 200, data con logs que contienen "Temperatura" en message
```

---

## Pact 4 — List Logs por máquina (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs?machine_id=$MACHINE_ID"
# Esperado: 200, data solo con logs de esa máquina
```

---

## Pact 5 — List Logs paginación por cursor (200)

```bash
# Paso 1: obtener primera página
RESPONSE=$(curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs?limit=2")
CURSOR=$(echo $RESPONSE | jq -r '.next_cursor')

# Paso 2: usar cursor para siguiente página
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs?cursor=$CURSOR&limit=2"
# Esperado: 200, resultados sin superposición con primera página
```

---

## Pact 6 — List Logs sin resultados (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs?event_type=system&from=2000-01-01T00:00:00Z&to=2000-01-02T00:00:00Z"
# Esperado: 200, { "data": [], "next_cursor": null, "total": 0 }
```

---

## Pact 7 — Get Log por ID (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs/$LOG_ID"
# Esperado: 200, objeto LogEntry completo
```

---

## Pact 8 — Get Log por ID inexistente (404)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs/00000000-0000-0000-0000-000000000000"
# Esperado: 404, { "success": false, "error": "NOT_FOUND" }
```

---

## Pact 9 — Get Log Context (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs/$LOG_ID/context?window_size=3"
# Esperado: 200, { "before": [...], "anchor": {...}, "after": [...] }
```

---

## Pact 10 — Get Retention Policy (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs/retention"
# Esperado: 200, { "tenant_id": "...", "retention_days": 90, "next_purge_at": "...", "updated_at": "..." }
```

---

## Pact 11 — Update Retention Policy (200)

```bash
curl -s -X PATCH \
     -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     -H "Content-Type: application/json" \
     -d '{"retention_days": 30}' \
     "$BASE_URL/logs/retention"
# Esperado: 200, política actualizada con retention_days: 30
```

---

## Pact 12 — Stream SSE (200)

```bash
curl -s -N \
     -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     -H "Accept: text/event-stream" \
     "$BASE_URL/logs/stream" &
SSE_PID=$!

# Insertar un log mientras el stream está abierto
sleep 1
# (insertar log via SQL o endpoint interno)

sleep 2
kill $SSE_PID
# Esperado: líneas "data: {...}" con el nuevo log
```

---

## Pact 13 — Export Logs normal (200)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs/export?severity=warning"
# Esperado: 200, { "data": [...], "truncated": false, "exported_count": N, "total_available": N }
```

---

## Pact 14 — Export Logs truncado (200)

```bash
# Requiere más de 50000 logs en la BD del tenant para activar truncado
# En tests: puede verificarse con mock del servicio o seed de datos masivo
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "X-Tenant-ID: $TENANT_ID" \
     "$BASE_URL/logs/export"
# Esperado: 200, { "truncated": true, "exported_count": 50000, "total_available": >50000 }
```

---

## Verificación rápida

```bash
# Verificar que la tabla existe y tiene datos
psql -c "SELECT COUNT(*), MIN(created_at), MAX(created_at) FROM log_entries WHERE tenant_id='550e8400-e29b-41d4-a716-446655440001';"

# Verificar retención
psql -c "SELECT * FROM log_retention_policies WHERE tenant_id='550e8400-e29b-41d4-a716-446655440001';"
```

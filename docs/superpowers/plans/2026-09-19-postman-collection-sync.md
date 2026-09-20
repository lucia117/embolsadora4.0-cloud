# Postman Collection Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring `postman/Embolsadora-API-Complete.postman_collection.json` and `postman/POSTMAN-GUIDE.md` back in sync with the API surface actually registered in `internal/routes/url_mappings.go` and `internal/api/router.go` — add the endpoints that shipped since the collection was last touched, and fix the one folder whose description is now wrong.

**Architecture:** The collection is a single pretty-printed (indent=2, `ensure_ascii=False`) JSON file. Round-tripping it through `json.load` → `json.dump(..., indent=2, ensure_ascii=False)` reproduces the file byte-for-byte (verified below), so every mutation in this plan is a small Python script that loads the file, mutates the in-memory `dict`/`list`, and writes it back — never a manual multi-line JSON hand-edit. This keeps diffs minimal and avoids trailing-comma/bracket mistakes in a 3900+ line file. The environment file (`postman/env-local.postman_environment.json`) round-trips the same way.

**Tech Stack:** Postman Collection Format v2.1.0 (schema `https://schema.getpostman.com/json/collection/v2.1.0/collection.json`), Python 3 (`json` stdlib only, already on PATH per `python3 --version`), git.

**Spec:** No formal spec doc — this plan is scoped directly from a gap analysis done in conversation (diffing the collection against `internal/routes/url_mappings.go`, `internal/api/router.go`, and the relevant handler/DTO files). The "Global Constraints" below are the facts that analysis established; each task cites the exact source file it verified against.

## Global Constraints

- Base branch is `develop`, not `main` — `main` (7c94b27) predates the Dashboard Metrics feature, dynamic RBAC permissions, and the edge-device API-key endpoints; `develop` (359bad0) has all of it and is the actual merge-base of the currently checked-out branch.
- Every JSON mutation must go through `json.load` → mutate → `json.dump(data, f, indent=2, ensure_ascii=False)` + trailing `\n`, matching the file's existing serialization exactly (verified round-trip-identical for both `postman/Embolsadora-API-Complete.postman_collection.json` and `postman/env-local.postman_environment.json`).
- New folders are plain `{"name": ..., "item": [...]}` dicts — no folder ever carries its own `description` key in this collection (checked: all 14 existing top-level folders have exactly `["name", "item"]` as keys).
- Request items follow the existing two header conventions found in the collection:
  - `/api/v1/*` routes behind the `v1` JWT+tenant-header group: `Authorization: Bearer {{token}}` + `X-Tenant-ID: {{tenant_id}}` (+ `Content-Type: application/json` for bodies).
  - `/api/v1/tenants/:tenantId/edge-devices/*` routes (tenant comes from the URL, not a header): `Authorization: Bearer {{token}}` only — no `X-Tenant-ID`.
  - Public/no-auth routes: no `Authorization`, no `X-Tenant-ID`.
- Every new/modified request keeps a `description` field explaining the response envelope and any RBAC permission it requires, matching the existing style (see `US2 - Create Layout`, `Pact 1 - List Roles (200)` items).
- Do not touch folders not named in this plan (Auth beyond the one addition, Invitations, Tenants, Users beyond the one addition, User Roles, Roles, Alarm Rules, Notifications, Logs, Permissions) — they were checked against the code and are already accurate.

---

## File Structure

| File | Change |
|---|---|
| `postman/Embolsadora-API-Complete.postman_collection.json` | Fix `Consumers` folder (name + `POST Ingest Events` item); add `Public` folder (1 item); add `PATCH Update Me` to `Auth` folder; add `Dashboard Metrics` folder (3 items); add 3 API-key items to `Edge Devices` folder; update top-level `info.description`. |
| `postman/env-local.postman_environment.json` | Add `edge_api_key_id` variable (auto-set by the new "Create Device API Key" request's test script). |
| `postman/POSTMAN-GUIDE.md` | Rewrite the folder list/endpoint counts, fix the Consumers description, add two workflow sections (Dashboard Metrics, Public Tenant Lookup), bump the version-history table. |

No Go code changes — this is a docs/tooling-only branch.

---

## Task 0: Branch setup

**Files:** none (git only)

- [ ] **Step 1: Confirm the working tree is clean and branch from `develop`**

```bash
git status --short
git fetch origin
git switch develop
git pull --ff-only origin develop
git switch -c chore/postman-collection-sync
```

Expected: `git status --short` prints nothing before switching; `git switch -c` reports the new branch checked out from `develop`.

- [ ] **Step 2: Confirm the branch point is what the plan assumes**

```bash
git log --oneline -1
```

Expected: top commit is `359bad0 feat: Dashboard Metrics Query API (query/catalog/batch) (#78)` (or a descendant of it on `develop` at time of execution).

---

## Task 1: Fix the `Consumers` folder — events is a real endpoint, not a 501 stub

**Files:**
- Modify: `postman/Embolsadora-API-Complete.postman_collection.json`

**Context:** `internal/consumers/router.go:26` registers `POST /events` on the real, fully implemented `IngestEvents` handler (`internal/consumers/events_handler.go`) — it is part of the frozen contract with the Edge Pi Service documented in `docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md`. Only `POST /heartbeat` (`internal/consumers/router.go:30`, `internal/consumers/heartbeat_handler.go:13`) still returns `501`. The collection currently has the folder named `"Consumers (IoT ingest - stubs, 501)"` and the events request named `"POST Ingest Events (stub, esperar 501)"` with a test asserting `501` and a body shape (`machine_id`, `event_type: cycle_completed`) that doesn't match the real contract. The real request body shape (validated against `internal/consumers/testdata/last-batch.json` and `internal/consumers/dto/events.go`) is `{"events": [{"eventId", "machineId", "ts", "seq", "kind", "schemaVersion", "payload": {"aasPath", "unit", "value", "valueType"}}]}`, and a successful call returns `200` with `{"data": {"accepted": N, "rejected": N, "errors": [...]}}` (`internal/consumers/dto/events.go`, `internal/domain/ingest/measurement.go:106`).

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_consumers.py` in the repo root (temporary, deleted in the last step):

```python
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))

folder = next(f for f in data["item"] if "Consumers" in f["name"])
assert folder["name"] == "Consumers (Edge Pi ingest)", f"folder name is still: {folder['name']!r}"

events_item = next(i for i in folder["item"] if "Ingest Events" in i["name"])
assert events_item["name"] == "POST Ingest Events (200 - contrato real)", f"item name is still: {events_item['name']!r}"

body = json.loads(events_item["request"]["body"]["raw"])
assert "eventId" in body["events"][0], "body still uses the old machine_id/event_type shape"

test_script = "\n".join(events_item["event"][0]["script"]["exec"])
assert "to.have.status(200)" in test_script, "test script still asserts 501"

print("OK: Consumers folder already fixed")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_consumers.py
```

Expected: `AssertionError: folder name is still: 'Consumers (IoT ingest - stubs, 501)'`

- [ ] **Step 3: Write and run the mutation script**

```bash
python3 - <<'EOF'
import json
path = "postman/Embolsadora-API-Complete.postman_collection.json"
with open(path, encoding="utf-8") as f:
    data = json.load(f)
folder = next(f for f in data["item"] if "Consumers" in f["name"])
folder["name"] = "Consumers (Edge Pi ingest)"
events_item = next(i for i in folder["item"] if "Ingest Events" in i["name"])
events_item["name"] = "POST Ingest Events (200 - contrato real)"
events_item["request"]["description"] = (
    "Endpoint real de ingesta (no un stub). Contrato congelado con el Edge Pi "
    "Service - ver docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md.\n\n"
    "Body: {events: [{eventId, machineId, ts, seq, kind, schemaVersion, payload}]}. "
    "eventId es la clave de idempotencia (unique index tenantId+eventId en Mongo): "
    "reenviar el mismo eventId no duplica la medicion.\n\n"
    "Respuesta 200 SIEMPRE (incluso con rechazos parciales, invariante I-2): "
    "{data: {accepted, rejected, errors: [{index, code, message}]}}. "
    "400 solo si el body entero es invalido (JSON malformado, events vacio/ausente, "
    "excede el maximo de elementos)."
)
events_item["request"]["body"]["raw"] = json.dumps(
    {
        "events": [
            {
                "eventId": "postman-{{$timestamp}}-1",
                "machineId": "{{machine_id}}",
                "ts": "2026-07-31T01:06:37.147166Z",
                "seq": 0,
                "kind": "metric",
                "schemaVersion": 1,
                "payload": {
                    "aasPath": "Operativos/Pesada/peso",
                    "unit": "kg",
                    "value": 1,
                    "valueType": "xs:float",
                },
            }
        ]
    },
    indent=2,
)
events_item["event"] = [
    {
        "listen": "test",
        "script": {
            "exec": [
                "pm.test('Status 200', () => pm.response.to.have.status(200));",
                "pm.test('acepta el evento', () => {",
                "    const body = pm.response.json();",
                "    pm.expect(body.data.accepted).to.eql(1);",
                "    pm.expect(body.data.rejected).to.eql(0);",
                "});",
            ]
        },
    }
]
with open(path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("mutation applied")
EOF
```

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_consumers.py
```

Expected: `OK: Consumers folder already fixed`

- [ ] **Step 5: Validate the whole file is still well-formed JSON**

```bash
python3 -m json.tool postman/Embolsadora-API-Complete.postman_collection.json > /dev/null && echo "valid JSON"
```

Expected: `valid JSON`

- [ ] **Step 6: Commit**

```bash
git add postman/Embolsadora-API-Complete.postman_collection.json
git commit -m "fix(postman): Consumers events ya no es un stub 501, es el contrato real"
```

---

## Task 2: Add the `Public` folder (unauthenticated tenant lookup)

**Files:**
- Modify: `postman/Embolsadora-API-Complete.postman_collection.json`

**Context:** `internal/routes/url_mappings.go:99` registers `GET /api/v1/public/tenants/:idOrSubdomain` directly on `r` (not the `v1` group), so it needs no `Authorization` and no `X-Tenant-ID` header — confirmed in `internal/api/handler/tenants/get_public_tenant/handler.go` and its tests. It resolves by UUID or subdomain (`internal/api/usecases/tenants/get_public_tenant/usecase.go`) and returns a branding-safe subset (`internal/api/handler/tenants/get_public_tenant/models/models.go`): `{id, subdomain, name, companyName, isActive, theme: {...}, settings: {...}}` — no address, no contactEmail. `404` if the tenant doesn't exist or is inactive. This backs the invitation/password-reset callback link, which runs before any session exists.

**Interfaces:**
- Produces: a new top-level folder named `"Public"`, inserted immediately after `"Auth"` in `data["item"]`.

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_public.py`:

```python
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))

names = [f["name"] for f in data["item"]]
assert "Public" in names, f"no Public folder yet, top-level folders: {names}"
assert names.index("Public") == names.index("Auth") + 1, "Public should sit right after Auth"

folder = next(f for f in data["item"] if f["name"] == "Public")
item = next(i for i in folder["item"] if "Public Tenant" in i["name"])
assert item["request"]["method"] == "GET"
assert "public/tenants" in item["request"]["url"]["raw"]
assert item["request"]["header"] == [], "public lookup must not send auth headers"

print("OK: Public folder already added")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_public.py
```

Expected: `AssertionError: no Public folder yet, top-level folders: [...]`

- [ ] **Step 3: Write and run the mutation script**

```bash
python3 - <<'EOF'
import json

path = "postman/Embolsadora-API-Complete.postman_collection.json"
with open(path, encoding="utf-8") as f:
    data = json.load(f)

public_folder = {
    "name": "Public",
    "item": [
        {
            "name": "GET Public Tenant (sin auth, por id o subdominio)",
            "request": {
                "method": "GET",
                "header": [],
                "url": {
                    "raw": "{{baseUrl}}/api/v1/public/tenants/{{tenantSubdomain}}",
                    "host": ["{{baseUrl}}"],
                    "path": ["api", "v1", "public", "tenants", "{{tenantSubdomain}}"],
                },
                "description": (
                    "Lookup PUBLICO (sin Authorization, sin X-Tenant-ID) por UUID o "
                    "subdominio del tenant. Backea el link de invitacion/reset de "
                    "password (corre antes de que exista sesion) y la landing page "
                    "publica del tenant.\n\n"
                    "Respuesta 200: {id, subdomain, name, companyName, isActive, "
                    "theme: {...}, settings: {locale, timezone, dateFormat, "
                    "timeFormat, currency}} -- subconjunto seguro para exponer sin "
                    "auth (sin address, sin contactEmail).\n\n"
                    "404 si el tenant no existe O esta inactivo (los dos casos se "
                    "devuelven igual, a proposito: no distingue 'no existe' de "
                    "'existe pero inactivo')."
                ),
            },
            "event": [
                {
                    "listen": "test",
                    "script": {
                        "exec": [
                            "pm.test('Status 200', () => pm.response.to.have.status(200));",
                            "pm.test('trae subdomain', () => {",
                            "    pm.expect(pm.response.json()).to.have.property('subdomain');",
                            "});",
                        ]
                    },
                }
            ],
        }
    ],
}

idx = next(i for i, f in enumerate(data["item"]) if f["name"] == "Auth")
data["item"].insert(idx + 1, public_folder)

with open(path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("Public folder inserted")
EOF
```

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_public.py
```

Expected: `OK: Public folder already added`

- [ ] **Step 5: Validate JSON**

```bash
python3 -m json.tool postman/Embolsadora-API-Complete.postman_collection.json > /dev/null && echo "valid JSON"
```

- [ ] **Step 6: Commit**

```bash
git add postman/Embolsadora-API-Complete.postman_collection.json
git commit -m "feat(postman): agregar folder Public con GET /public/tenants/:idOrSubdomain"
```

---

## Task 3: Add `PATCH Update Me` to the `Auth` folder

**Files:**
- Modify: `postman/Embolsadora-API-Complete.postman_collection.json`

**Context:** `internal/api/router.go:87` registers `PATCH /api/v1/users/me` with **no RBAC check** — the user ID comes from the JWT (`platform.DomainUser(ctx)`), never from the URL, so any authenticated user can update their own `firstName`/`lastName` regardless of role (`internal/api/handler/users/handler.go:268`, confirmed by the regression test `TestUpdateMe_SinPermisoRBAC_Funciona` in `internal/api/handler/users/update_me_test.go`). It still needs `X-Tenant-ID` (the route sits behind `TenantFromHeader`/`ExtractTenantID`). Response is the standard user object (`userToResponse`, 200).

**Interfaces:**
- Produces: a new item appended to the existing `"Auth"` folder's `item` array.

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_update_me.py`:

```python
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))

auth_folder = next(f for f in data["item"] if f["name"] == "Auth")
names = [i["name"] for i in auth_folder["item"]]
assert "PATCH Update Me (self-service)" in names, f"Auth items: {names}"

item = next(i for i in auth_folder["item"] if i["name"] == "PATCH Update Me (self-service)")
assert item["request"]["method"] == "PATCH"
assert item["request"]["url"]["raw"] == "{{baseUrl}}/api/v1/users/me"
header_keys = {h["key"] for h in item["request"]["header"]}
assert header_keys == {"Authorization", "X-Tenant-ID", "Content-Type"}, header_keys

print("OK: PATCH Update Me already added")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_update_me.py
```

Expected: `AssertionError: Auth items: ['POST Login', 'GET Me', 'GET Me (con tenant)', 'POST Change Password']`

- [ ] **Step 3: Write and run the mutation script**

```bash
python3 - <<'EOF'
import json

path = "postman/Embolsadora-API-Complete.postman_collection.json"
with open(path, encoding="utf-8") as f:
    data = json.load(f)

auth_folder = next(f for f in data["item"] if f["name"] == "Auth")

item = {
    "name": "PATCH Update Me (self-service)",
    "request": {
        "method": "PATCH",
        "header": [
            {"key": "Authorization", "value": "Bearer {{token}}"},
            {"key": "X-Tenant-ID", "value": "{{tenant_id}}"},
            {"key": "Content-Type", "value": "application/json"},
        ],
        "body": {
            "mode": "raw",
            "raw": json.dumps({"firstName": "Nuevo", "lastName": "Apellido"}, indent=2),
        },
        "url": {
            "raw": "{{baseUrl}}/api/v1/users/me",
            "host": ["{{baseUrl}}"],
            "path": ["api", "v1", "users", "me"],
        },
        "description": (
            "Self-service: el usuario actualiza SU PROPIO firstName/lastName. "
            "El userId sale del JWT, nunca de la URL -- NO tiene RBACCheck, "
            "cualquier rol autenticado puede usarlo (ver "
            "TestUpdateMe_SinPermisoRBAC_Funciona). Registrada antes de "
            "PATCH /users/:id en el router para que Gin no la trate como "
            "'id=me'.\n\nRespuesta 200: el user actualizado."
        ),
    },
    "event": [
        {
            "listen": "test",
            "script": {
                "exec": [
                    "pm.test('Status 200', () => pm.response.to.have.status(200));",
                ]
            },
        }
    ],
}
auth_folder["item"].append(item)

with open(path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("PATCH Update Me added to Auth")
EOF
```

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_update_me.py
```

Expected: `OK: PATCH Update Me already added`

- [ ] **Step 5: Validate JSON**

```bash
python3 -m json.tool postman/Embolsadora-API-Complete.postman_collection.json > /dev/null && echo "valid JSON"
```

- [ ] **Step 6: Commit**

```bash
git add postman/Embolsadora-API-Complete.postman_collection.json
git commit -m "feat(postman): agregar PATCH /users/me (self-service) al folder Auth"
```

---

## Task 4: Add the `Dashboard Metrics` folder (query / catalog / query/batch)

**Files:**
- Modify: `postman/Embolsadora-API-Complete.postman_collection.json`

**Context:** `internal/routes/url_mappings.go:337-341` mounts `dashboardsHandler.RegisterRoutes` on `v1.Group("/dashboards/metrics", RBACCheck("perm_metrics_view"), DashboardRateLimit(...))`, which registers exactly 3 routes (`internal/api/handler/dashboards/routes.go`): `POST /query`, `GET /catalog`, `POST /query/batch`. This whole surface (PR #78, `359bad0`) has zero coverage in the collection today.

Request/response shapes, verified against the DTOs:
- `POST /query` body (`internal/api/handler/dashboards/dto/request.go`): `{machineId, range?, from?, to?, bucket?, metrics: [{aasPath, agg}], groupBy?, filter?, maxPoints?}`. `range` (e.g. `"1h"`) and `from`/`to` are mutually exclusive (`internal/domain/metrics/window.go`). Valid `Bucket` values: `1m 5m 15m 1h 6h 1d` (`internal/domain/metrics/metrics.go:34`). Valid `Range` values: `15m 30m 1h 8h 24h 7d 30d` (`internal/domain/metrics/metrics.go:65`). Valid `Agg` values: `avg sum min max last count delta raw`. Response envelope: `{success: true, data: {mode, machineId, from, to, ...}}` on success; on a guardrail violation, `{success: false, error, code}` with `400` (or `504` for `QUERY_TIMEOUT`) — see `internal/api/handler/dashboards/errors.go`.
- `GET /catalog?machineId=...` (`internal/api/handler/dashboards/catalog_metrics.go`): returns `{success: true, data: {machineId, aasPaths: [...]}}`.
- `POST /query/batch` body (`internal/api/handler/dashboards/batch_query_metrics.go`): `{queries: [{id, ...same fields as /query}]}`. Per-item errors don't abort the batch — response is always `200` with `{success: true, data: {results: [{id, success, data|error, code}]}}`.

**Interfaces:**
- Produces: a new top-level folder named `"Dashboard Metrics"`, inserted immediately after `"Dashboard Layouts"` in `data["item"]`.

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_dashboard_metrics.py`:

```python
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))

names = [f["name"] for f in data["item"]]
assert "Dashboard Metrics" in names, f"top-level folders: {names}"
assert names.index("Dashboard Metrics") == names.index("Dashboard Layouts") + 1

folder = next(f for f in data["item"] if f["name"] == "Dashboard Metrics")
item_names = [i["name"] for i in folder["item"]]
assert item_names == [
    "US1 - Query Metrics (single)",
    "US2 - Catalog Metrics",
    "US3 - Batch Query Metrics",
], item_names

query_item = folder["item"][0]
assert query_item["request"]["url"]["raw"] == "{{baseUrl}}/api/v1/dashboards/metrics/query"
body = json.loads(query_item["request"]["body"]["raw"])
assert body["metrics"][0]["agg"] == "avg"

catalog_item = folder["item"][1]
assert "catalog" in catalog_item["request"]["url"]["raw"]

batch_item = folder["item"][2]
batch_body = json.loads(batch_item["request"]["body"]["raw"])
assert batch_body["queries"][0]["id"] == "peso-avg"

print("OK: Dashboard Metrics folder already added")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_dashboard_metrics.py
```

Expected: `AssertionError: top-level folders: [...]` (no `Dashboard Metrics` in the list)

- [ ] **Step 3: Write and run the mutation script**

```bash
python3 - <<'EOF'
import json

path = "postman/Embolsadora-API-Complete.postman_collection.json"
with open(path, encoding="utf-8") as f:
    data = json.load(f)

common_headers = [
    {"key": "Authorization", "value": "Bearer {{token}}"},
    {"key": "X-Tenant-ID", "value": "{{tenant_id}}"},
]

query_body = {
    "machineId": "{{machine_id}}",
    "range": "1h",
    "bucket": "5m",
    "metrics": [{"aasPath": "Operativos/Pesada/peso", "agg": "avg"}],
}

batch_body = {
    "queries": [
        {
            "id": "peso-avg",
            "machineId": "{{machine_id}}",
            "range": "1h",
            "bucket": "5m",
            "metrics": [{"aasPath": "Operativos/Pesada/peso", "agg": "avg"}],
        }
    ]
}

dashboard_metrics_folder = {
    "name": "Dashboard Metrics",
    "item": [
        {
            "name": "US1 - Query Metrics (single)",
            "request": {
                "method": "POST",
                "header": common_headers + [{"key": "Content-Type", "value": "application/json"}],
                "body": {"mode": "raw", "raw": json.dumps(query_body, indent=2)},
                "url": {
                    "raw": "{{baseUrl}}/api/v1/dashboards/metrics/query",
                    "host": ["{{baseUrl}}"],
                    "path": ["api", "v1", "dashboards", "metrics", "query"],
                },
                "description": (
                    "Requiere perm_metrics_view + esta gateado por DashboardRateLimit "
                    "(por tenant). range y from/to son mutuamente excluyentes. Bucket "
                    "valido: 1m|5m|15m|1h|6h|1d. Agg valido: avg|sum|min|max|last|"
                    "count|delta|raw.\n\n"
                    "200: {success: true, data: {mode, machineId, from, to, ...}} "
                    "-- la forma exacta de data depende de mode (scalar/series/raw/"
                    "grouped).\n"
                    "400 (INVALID_PARAMS u otro guardrail de domain/metrics), "
                    "504 (QUERY_TIMEOUT)."
                ),
            },
            "event": [
                {
                    "listen": "test",
                    "script": {
                        "exec": [
                            "pm.test('Status 200', () => pm.response.to.have.status(200));",
                            "pm.test('success true', () => {",
                            "    pm.expect(pm.response.json().success).to.eql(true);",
                            "});",
                        ]
                    },
                }
            ],
        },
        {
            "name": "US2 - Catalog Metrics",
            "request": {
                "method": "GET",
                "header": common_headers,
                "url": {
                    "raw": "{{baseUrl}}/api/v1/dashboards/metrics/catalog?machineId={{machine_id}}",
                    "host": ["{{baseUrl}}"],
                    "path": ["api", "v1", "dashboards", "metrics", "catalog"],
                    "query": [{"key": "machineId", "value": "{{machine_id}}"}],
                },
                "description": (
                    "Devuelve los aasPath descubiertos para el machineId dado -- "
                    "util para armar el body de /query sin adivinar rutas AAS.\n\n"
                    "200: {success: true, data: {machineId, aasPaths: [...]}}."
                ),
            },
            "event": [
                {
                    "listen": "test",
                    "script": {
                        "exec": [
                            "pm.test('Status 200', () => pm.response.to.have.status(200));",
                        ]
                    },
                }
            ],
        },
        {
            "name": "US3 - Batch Query Metrics",
            "request": {
                "method": "POST",
                "header": common_headers + [{"key": "Content-Type", "value": "application/json"}],
                "body": {"mode": "raw", "raw": json.dumps(batch_body, indent=2)},
                "url": {
                    "raw": "{{baseUrl}}/api/v1/dashboards/metrics/query/batch",
                    "host": ["{{baseUrl}}"],
                    "path": ["api", "v1", "dashboards", "metrics", "query", "batch"],
                },
                "description": (
                    "Ejecuta N queries en paralelo, correlacionadas por id. Un error "
                    "en UN item (id duplicado a nivel batch aparte) NO aborta el "
                    "resto -- va dentro de results[i].error/code, la respuesta "
                    "sigue siendo 200. Limite: MaxBatchQueries (default 50).\n\n"
                    "200: {success: true, data: {results: [{id, success, "
                    "data|error, code}]}}."
                ),
            },
            "event": [
                {
                    "listen": "test",
                    "script": {
                        "exec": [
                            "pm.test('Status 200', () => pm.response.to.have.status(200));",
                            "pm.test('primer resultado ok', () => {",
                            "    const body = pm.response.json();",
                            "    pm.expect(body.data.results[0].success).to.eql(true);",
                            "});",
                        ]
                    },
                }
            ],
        },
    ],
}

idx = next(i for i, f in enumerate(data["item"]) if f["name"] == "Dashboard Layouts")
data["item"].insert(idx + 1, dashboard_metrics_folder)

with open(path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("Dashboard Metrics folder inserted")
EOF
```

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_dashboard_metrics.py
```

Expected: `OK: Dashboard Metrics folder already added`

- [ ] **Step 5: Validate JSON**

```bash
python3 -m json.tool postman/Embolsadora-API-Complete.postman_collection.json > /dev/null && echo "valid JSON"
```

- [ ] **Step 6: Commit**

```bash
git add postman/Embolsadora-API-Complete.postman_collection.json
git commit -m "feat(postman): agregar folder Dashboard Metrics (query/catalog/query-batch)"
```

---

## Task 5: Add edge device API-key requests to the `Edge Devices` folder

**Files:**
- Modify: `postman/Embolsadora-API-Complete.postman_collection.json`
- Modify: `postman/env-local.postman_environment.json`

**Context:** `internal/api/handler/edge_devices/routes.go:63-65` registers `POST/GET /edge-devices/:deviceId/api-keys` and `DELETE /edge-devices/:deviceId/api-keys/:keyId` — issuing/listing/revoking the credentials a Pi uses to authenticate against `/api/v1/consumers/events`. None of the three are in the collection. Per `internal/api/handler/edge_devices/api_keys.go` and `internal/api/handler/edge_devices/dto/api_keys.go`:
- `POST .../api-keys` body: `{name?, expiresAt?}` (body itself is optional). `201` response is the **only** place the plaintext secret ever appears: `{success: true, data: {id, keyId, name, createdAt, expiresAt, revokedAt, lastUsedAt, active, deviceStatus, key}}`. Requires `perm_edge_devices_manage`.
- `GET .../api-keys`: `200` with `{success: true, data: [{...same fields, no "key"...}]}`. Requires `perm_edge_devices_view`.
- `DELETE .../api-keys/:keyId`: `204 No Content`, idempotent. Requires `perm_edge_devices_manage`.

These routes sit under `tenantsGroup` (`internal/routes/url_mappings.go:261-269`), same as the rest of `Edge Devices` — tenant comes from the `:tenantSubdomain` path segment, so (matching every other item already in this folder, e.g. `US2 - Create Edge Device`) there is **no** `X-Tenant-ID` header, only `Authorization`.

**Interfaces:**
- Consumes: env variable `{{device_id}}` (already exists, used by every other Edge Devices item).
- Produces: new env variable `{{edge_api_key_id}}`, auto-set by the "Create Device API Key" request's test script, consumed by "Revoke Device API Key".

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_edge_api_keys.py`:

```python
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))
edge_folder = next(f for f in data["item"] if f["name"] == "Edge Devices")
item_names = [i["name"] for i in edge_folder["item"]]

expected_new = [
    "API Keys - Create Device API Key",
    "API Keys - List Device API Keys",
    "API Keys - Revoke Device API Key",
]
for name in expected_new:
    assert name in item_names, f"missing {name!r}; Edge Devices items: {item_names}"

create_item = next(i for i in edge_folder["item"] if i["name"] == "API Keys - Create Device API Key")
assert create_item["request"]["method"] == "POST"
assert "api-keys" in create_item["request"]["url"]["raw"]
header_keys = {h["key"] for h in create_item["request"]["header"]}
assert "X-Tenant-ID" not in header_keys, "edge-devices routes take tenant from the URL, not a header"

revoke_item = next(i for i in edge_folder["item"] if i["name"] == "API Keys - Revoke Device API Key")
assert revoke_item["request"]["method"] == "DELETE"
assert "{{edge_api_key_id}}" in revoke_item["request"]["url"]["raw"]

env = json.load(open("postman/env-local.postman_environment.json", encoding="utf-8"))
env_keys = {v["key"] for v in env["values"]}
assert "edge_api_key_id" in env_keys, "env is missing edge_api_key_id"

print("OK: edge device API-key requests already added")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_edge_api_keys.py
```

Expected: `AssertionError: missing 'API Keys - Create Device API Key'; Edge Devices items: [...]`

- [ ] **Step 3: Write and run the mutation script**

```bash
python3 - <<'EOF'
import json

collection_path = "postman/Embolsadora-API-Complete.postman_collection.json"
with open(collection_path, encoding="utf-8") as f:
    data = json.load(f)

edge_folder = next(f for f in data["item"] if f["name"] == "Edge Devices")

auth_only_header = [{"key": "Authorization", "value": "Bearer {{token}}"}]

new_items = [
    {
        "name": "API Keys - Create Device API Key",
        "request": {
            "method": "POST",
            "header": auth_only_header + [{"key": "Content-Type", "value": "application/json"}],
            "body": {
                "mode": "raw",
                "raw": json.dumps({"name": "Pi produccion", "expiresAt": None}, indent=2),
            },
            "url": {
                "raw": "{{baseUrl}}/api/v1/tenants/{{tenantSubdomain}}/edge-devices/{{device_id}}/api-keys",
                "host": ["{{baseUrl}}"],
                "path": [
                    "api", "v1", "tenants", "{{tenantSubdomain}}",
                    "edge-devices", "{{device_id}}", "api-keys",
                ],
            },
            "description": (
                "Requiere perm_edge_devices_manage. name y expiresAt son "
                "opcionales (body vacio {} tambien es valido).\n\n"
                "201: unica respuesta que incluye el secreto en claro (campo "
                "'key', formato emb_<keyId>_<secreto>) -- no se persiste, no se "
                "puede volver a consultar despues de esta llamada."
            ),
        },
        "event": [
            {
                "listen": "test",
                "script": {
                    "exec": [
                        "pm.test('Status 201', () => pm.response.to.have.status(201));",
                        "const body = pm.response.json();",
                        "pm.environment.set('edge_api_key_id', body.data.id);",
                    ]
                },
            }
        ],
    },
    {
        "name": "API Keys - List Device API Keys",
        "request": {
            "method": "GET",
            "header": auth_only_header,
            "url": {
                "raw": "{{baseUrl}}/api/v1/tenants/{{tenantSubdomain}}/edge-devices/{{device_id}}/api-keys",
                "host": ["{{baseUrl}}"],
                "path": [
                    "api", "v1", "tenants", "{{tenantSubdomain}}",
                    "edge-devices", "{{device_id}}", "api-keys",
                ],
            },
            "description": (
                "Requiere perm_edge_devices_view. Nunca incluye el secreto -- "
                "solo metadata (id, keyId, name, createdAt, expiresAt, "
                "revokedAt, lastUsedAt, active, deviceStatus)."
            ),
        },
        "event": [
            {
                "listen": "test",
                "script": {
                    "exec": [
                        "pm.test('Status 200', () => pm.response.to.have.status(200));",
                    ]
                },
            }
        ],
    },
    {
        "name": "API Keys - Revoke Device API Key",
        "request": {
            "method": "DELETE",
            "header": auth_only_header,
            "url": {
                "raw": "{{baseUrl}}/api/v1/tenants/{{tenantSubdomain}}/edge-devices/{{device_id}}/api-keys/{{edge_api_key_id}}",
                "host": ["{{baseUrl}}"],
                "path": [
                    "api", "v1", "tenants", "{{tenantSubdomain}}",
                    "edge-devices", "{{device_id}}", "api-keys", "{{edge_api_key_id}}",
                ],
            },
            "description": (
                "Requiere perm_edge_devices_manage. Idempotente: revocar una "
                "key ya revocada tambien devuelve 204."
            ),
        },
        "event": [
            {
                "listen": "test",
                "script": {
                    "exec": [
                        "pm.test('Status 204', () => pm.response.to.have.status(204));",
                    ]
                },
            }
        ],
    },
]

edge_folder["item"].extend(new_items)

with open(collection_path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")

env_path = "postman/env-local.postman_environment.json"
with open(env_path, encoding="utf-8") as f:
    env = json.load(f)

env["values"].append({
    "key": "edge_api_key_id",
    "value": "",
    "type": "default",
    "enabled": True,
})

with open(env_path, "w", encoding="utf-8") as f:
    json.dump(env, f, indent=2, ensure_ascii=False)
    f.write("\n")

print("edge device API-key requests + env var added")
EOF
```

Note: check the existing entries in `postman/env-local.postman_environment.json` (`python3 -c "import json; print(json.load(open('postman/env-local.postman_environment.json', encoding='utf-8'))['values'][0])"`) before running this — match the exact key set (`key`, `value`, `type`, `enabled`) used by the other entries so the new one isn't structurally different.

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_edge_api_keys.py
```

Expected: `OK: edge device API-key requests already added`

- [ ] **Step 5: Validate both JSON files**

```bash
python3 -m json.tool postman/Embolsadora-API-Complete.postman_collection.json > /dev/null && echo "collection valid"
python3 -m json.tool postman/env-local.postman_environment.json > /dev/null && echo "env valid"
```

- [ ] **Step 6: Commit**

```bash
git add postman/Embolsadora-API-Complete.postman_collection.json postman/env-local.postman_environment.json
git commit -m "feat(postman): agregar API keys de edge devices (create/list/revoke)"
```

---

## Task 6: Update the collection's top-level `info.description`

**Files:**
- Modify: `postman/Embolsadora-API-Complete.postman_collection.json`

**Context:** `data["info"]["description"]` (the collection-level description shown in Postman's sidebar) still lists only the original feature set and doesn't mention Dashboard Metrics, the Public folder, `users/me`, edge device API keys, or that Consumers is a real endpoint now.

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_info.py`:

```python
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))
desc = data["info"]["description"]
for marker in ["Dashboard Metrics Query", "Public Tenant Lookup", "users/me", "Edge Device API Keys"]:
    assert marker in desc, f"info.description missing {marker!r}"
print("OK: info.description already updated")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_info.py
```

Expected: `AssertionError: info.description missing 'Dashboard Metrics Query'`

- [ ] **Step 3: Write and run the mutation script**

```bash
python3 - <<'EOF'
import json

path = "postman/Embolsadora-API-Complete.postman_collection.json"
with open(path, encoding="utf-8") as f:
    data = json.load(f)

data["info"]["description"] += (
    "\n\n**Dashboard Metrics Query API (added 2026-09):**\n"
    "- POST /api/v1/dashboards/metrics/query -- single query\n"
    "- GET /api/v1/dashboards/metrics/catalog -- discovered aasPaths for a machine\n"
    "- POST /api/v1/dashboards/metrics/query/batch -- up to MaxBatchQueries in parallel\n"
    "- Requires perm_metrics_view + is rate-limited (DashboardRateLimit)\n\n"
    "**Public Tenant Lookup (added 2026-09):**\n"
    "- GET /api/v1/public/tenants/:idOrSubdomain -- no auth, backs invitation/reset links\n\n"
    "**Self-service (added 2026-09):**\n"
    "- PATCH /api/v1/users/me -- no RBAC, userId comes from the JWT\n\n"
    "**Edge Device API Keys (added 2026-09):**\n"
    "- POST/GET /api/v1/tenants/:tenantId/edge-devices/:deviceId/api-keys\n"
    "- DELETE .../api-keys/:keyId\n\n"
    "**Consumers surface note:** POST /api/v1/consumers/events is a REAL, fully "
    "implemented endpoint (frozen contract with the Edge Pi Service) -- only "
    "POST /api/v1/consumers/heartbeat is still a 501 stub."
)

with open(path, "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("info.description updated")
EOF
```

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_info.py
```

Expected: `OK: info.description already updated`

- [ ] **Step 5: Validate JSON**

```bash
python3 -m json.tool postman/Embolsadora-API-Complete.postman_collection.json > /dev/null && echo "valid JSON"
```

- [ ] **Step 6: Commit**

```bash
git add postman/Embolsadora-API-Complete.postman_collection.json
git commit -m "docs(postman): actualizar info.description con los endpoints nuevos"
```

---

## Task 7: Rewrite `POSTMAN-GUIDE.md`

**Files:**
- Modify: `postman/POSTMAN-GUIDE.md`

**Context:** The guide (last touched 2026-03-11) describes only 4 folders / ~20 endpoints; the collection now has 17 folders. This task brings the folder list, the "Quick Start" step 4 list, and the version-history table in line with what Tasks 1-6 produced, and adds two workflow sections.

- [ ] **Step 1: Write the failing verification script**

Save as `scratch_verify_guide.py`:

```python
text = open("postman/POSTMAN-GUIDE.md", encoding="utf-8").read()
for marker in [
    "Dashboard Metrics",
    "Public",
    "PATCH /api/v1/users/me",
    "Edge Device API Keys",
    "POST /api/v1/consumers/events` es el endpoint real",
    "2026-09-19",
]:
    assert marker in text, f"guide missing {marker!r}"
assert "20+ endpoints organized in 4 folders" not in text, "stale endpoint count still present"
print("OK: guide already rewritten")
```

- [ ] **Step 2: Run it, confirm it fails**

```bash
python3 scratch_verify_guide.py
```

Expected: `AssertionError: guide missing 'Dashboard Metrics'`

- [ ] **Step 3: Rewrite the guide**

Replace the full contents of `postman/POSTMAN-GUIDE.md` with:

```markdown
# Embolsadora API - Postman Complete Guide

Master collection for testing all Embolsadora 4.0 Cloud API endpoints across all features.

## 📦 Collection Files

### Primary Collection (USE THIS)
- **`Embolsadora-API-Complete.postman_collection.json`** — Master collection with 17 folders covering every route registered in `internal/routes/url_mappings.go` and `internal/api/router.go`.

### Environment Files
- **`env-local.postman_environment.json`** — Variables for a local `go run cmd/api/main.go` server (`baseUrl=http://localhost:8080`).

## 🚀 Quick Start

### Step 1: Import Master Collection

```
Postman → File → Import → Select "Embolsadora-API-Complete.postman_collection.json"
```

### Step 2: Import the Environment

```
Postman → File → Import → Select "env-local.postman_environment.json"
```

Set `token` (a Supabase JWT — see Authentication below), `tenant_id` / `tenant_uuid` (a tenant UUID), and `tenantSubdomain` (that same tenant's subdomain, used by the Edge Devices and Public folders, which take the tenant from the URL instead of a header).

### Step 3: Start API Server

```bash
go run cmd/api/main.go
# Output: Listening on :8080
```

### Step 4: Execute Requests

Folders, in collection order:

1. **Auth** — Login, `GET /me`, `PATCH /users/me` (self-service), change password
2. **Public** — Unauthenticated tenant lookup (invitation/reset callback links)
3. **Invitations** — Create/list/resend/revoke
4. **Users (Auth)** — Force password change (admin-triggered)
5. **Tenants** — Tenant CRUD (admin)
6. **Users** — User CRUD, status, pending list, roles-across-tenants
7. **User Roles** — Assign/list/update/revoke/bulk-assign
8. **Dashboard Layouts** — Per-user dashboard layout CRUD
9. **Dashboard Metrics** — Query/catalog/batch-query time-series metrics
10. **Edge Devices** — Device CRUD, enable/disable, status/health checks, telemetry, events, API keys
11. **Consumers (Edge Pi ingest)** — `POST /events` (real, frozen contract), `POST /heartbeat` (still a 501 stub)
12. **Roles** — Role CRUD (Pact-style contract tests)
13. **Alarm Rules** — Alarm rule CRUD
14. **Notifications** — List/count/ack/close
15. **Logs** — List/filter/export/stream, retention policy
16. **Permissions** — Permission catalog CRUD

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

#### Option 2: Use the Actual Login Endpoint
```bash
POST http://localhost:8080/api/v1/auth/login
Body: {"email": "admin@example.com", "password": "..."}
# Copy "access_token" from response → {{token}}
```

---

## 🧪 Common Testing Workflows

### Workflow 1: Complete User Management

```
1. SET Variables
   - baseUrl, token, tenant_id, tenantSubdomain

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

2. User Roles/POST Assign Role → 201 Created (auto-extracts utrId)
3. User Roles/GET List Assignments → 200 OK
4. User Roles/PUT Update Role → 200 OK
5. User Roles/DELETE Revoke Role → 200 OK (soft revoke)
```

### Workflow 3: Edge Device Monitoring + Ingest Credentials

```
1. SET Variables
   - tenantSubdomain, device_id (from create response)

2. Edge Devices/US2 - Create Edge Device → 201 Created
3. Edge Devices/US1 - List Edge Devices → 200 OK
4. Edge Devices/US6 - Status Check → 200 OK
5. Edge Devices/API Keys - Create Device API Key → 201 Created
   (auto-extracts edge_api_key_id; the "key" field in the response is the
   ONLY time the plaintext secret is ever shown — copy it now if you plan
   to actually authenticate a Pi with it)
6. Edge Devices/API Keys - List Device API Keys → 200 OK (no secret in the list)
7. Edge Devices/API Keys - Revoke Device API Key → 204 No Content
```

### Workflow 4: Ingest a Metric, Then Query It

```
1. SET Variables
   - consumer_api_key (an active edge device API key's plaintext "key")
   - machine_id (the machineId of an existing, ACTIVE edge device)

2. Consumers (Edge Pi ingest)/POST Ingest Events → 200 OK, accepted=1
3. Dashboard Metrics/US2 - Catalog Metrics → 200 OK, confirm the aasPath appears
4. Dashboard Metrics/US1 - Query Metrics (single) → 200 OK
```

### Workflow 5: Public Tenant Lookup (no auth)

```
1. SET Variables
   - tenantSubdomain (or a tenant UUID)

2. Public/GET Public Tenant → 200 OK, no Authorization header sent
   404 if the tenant is inactive or doesn't exist
```

---

## ⚠️ Error Handling

| Status | Error Code | Cause | Resolution |
|--------|-----------|-------|-----------|
| 400 | `MISSING_HEADER` | Missing X-Tenant-ID | Add header in collection |
| 400 | `VALIDATION_ERROR` | Invalid JSON or required fields missing | Check request body |
| 400 | `INVALID_PARAMS` | Dashboard Metrics guardrail (bad range/bucket/agg) | Check `internal/domain/metrics/validate.go` |
| 400 | `EDGE_DEVICE_DISABLED` | Device status is DISABLED | Device must be ACTIVE |
| 400 | `IMMUTABLE_FIELD` | Attempted to change email/tenantId | Only update mutable fields |
| 401 | `UNAUTHORIZED` | Invalid or missing JWT | Get fresh token |
| 403 | `INSUFFICIENT_PERMISSIONS` | Missing the RBAC permission for that route | Check the request's `description` for the required `perm_*` |
| 404 | `NOT_FOUND` | Resource doesn't exist | Verify ID and tenant |
| 409 | `CONFLICT` | `machineId` already exists, or user already has an active role | Use PUT/PATCH to update instead |
| 504 | `QUERY_TIMEOUT` | Dashboard Metrics query took too long | Narrow the range/bucket |
| 500 | Internal error | Server error | Check API logs |

Notes:
- **`POST /api/v1/consumers/events` is a real endpoint**, not a stub — it always returns `200` with `{data: {accepted, rejected, errors}}`, even on partial rejection. `400` only for a malformed/oversized batch. `POST /api/v1/consumers/heartbeat` is the one still returning `501`.
- **`PATCH /api/v1/users/me`** has no RBAC check by design — any authenticated user can update their own name.
- **`GET /api/v1/public/tenants/:idOrSubdomain`** sends no `Authorization`/`X-Tenant-ID` at all.

---

## 📊 Multi-Tenant Isolation

**CRITICAL**: Almost every operation is tenant-scoped, but the mechanism differs by surface:

- **`/api/v1/*` under the main `v1` group** (Users, Invitations, Roles, Dashboard Layouts, Dashboard Metrics, Alarm Rules, Notifications, Logs, Permissions, User Roles): `X-Tenant-ID` header (UUID).
- **`/api/v1/tenants/:tenantId/edge-devices/*`** (Edge Devices, incl. API keys): tenant comes from the **URL path segment**, no header.
- **`/api/v1/consumers/*`**: no tenant header or path segment at all — the tenant is resolved server-side from the API key.
- **`/api/v1/public/*`**: no tenant scoping, no auth — deliberately safe to call before any session exists.

---

## 🛠️ Troubleshooting

### "Unknown Variable {{tenant_id}}" / "{{tenantSubdomain}}"
**Fix**: Click collection → environment → set the variable to an actual UUID/subdomain.

### "401 Unauthorized"
**Fix**: Get a fresh token from `POST /api/v1/auth/login` or jwt.io.

### "403 INSUFFICIENT_PERMISSIONS"
**Fix**: Check the request's `description` field for the exact `perm_*` id it requires, then confirm your test user's role has it (`GET /api/v1/roles` or `GET /api/v1/permissions`).

### "409 Conflict - machineId already exists"
**Fix**: Use a unique `machineId` or a different tenant.

---

## 📚 Additional Resources

- **Ingest contract**: `docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md`
- **Edge Device Management spec**: `specs/003-edge-device-management/spec.md`
- **Dashboard Layout spec**: `specs/005-dashboard-layouts` (see `specs/`)
- **OpenAPI Contract**: `docs/openapi.yaml`

---

## 📝 Version History

- **2026-09-19**: Synced against the actual routes in `internal/routes/url_mappings.go` / `internal/api/router.go` — added Dashboard Metrics (query/catalog/batch), Public tenant lookup, `PATCH /users/me`, edge device API keys; corrected the Consumers folder (events is a real endpoint, not a 501 stub).
- **2026-03-11**: Master collection created — consolidated all 4 separate collections
- **2026-03-02**: User Management API collection + docs
- **2026-03-11**: Edge Device Management collection + README
- **2026-02-28**: User Role Assignments collection
- **2026-02-27**: Tenants collection

---

**Status**: Production-Ready
**Collection Version**: 3.0 (Synced 2026-09-19)
**Last Updated**: 2026-09-19
```

- [ ] **Step 4: Run the verification script, confirm it passes**

```bash
python3 scratch_verify_guide.py
```

Expected: `OK: guide already rewritten`

- [ ] **Step 5: Commit**

```bash
git add postman/POSTMAN-GUIDE.md
git commit -m "docs(postman): reescribir POSTMAN-GUIDE.md con los 17 folders reales"
```

---

## Task 8: Final cross-check against the route table, cleanup

**Files:** none modified (verification + scratch-file cleanup only)

- [ ] **Step 1: Re-list every path in the collection and diff against the known route table**

```bash
python3 - <<'EOF'
import json

data = json.load(open("postman/Embolsadora-API-Complete.postman_collection.json", encoding="utf-8"))

def walk(items):
    for it in items:
        if "item" in it:
            yield from walk(it["item"])
        else:
            method = it["request"]["method"]
            url = it["request"]["url"]
            # The Permissions folder pre-dates this plan and stores url as a
            # plain string instead of the {raw, host, path} object -- both
            # are valid Postman schema, so handle both here.
            raw = url if isinstance(url, str) else url["raw"]
            yield method, raw

seen = sorted(set(walk(data["item"])))
for method, raw in seen:
    print(method, raw)
print(f"\n{len(seen)} unique (method, path) pairs")
EOF
```

Manually confirm the output includes at minimum:
- `GET {{baseUrl}}/api/v1/public/tenants/{{tenantSubdomain}}`
- `PATCH {{baseUrl}}/api/v1/users/me`
- `POST {{baseUrl}}/api/v1/dashboards/metrics/query`
- `GET {{baseUrl}}/api/v1/dashboards/metrics/catalog?machineId={{machine_id}}`
- `POST {{baseUrl}}/api/v1/dashboards/metrics/query/batch`
- `POST {{baseUrl}}/api/v1/tenants/{{tenantSubdomain}}/edge-devices/{{device_id}}/api-keys`
- `GET {{baseUrl}}/api/v1/tenants/{{tenantSubdomain}}/edge-devices/{{device_id}}/api-keys`
- `DELETE {{baseUrl}}/api/v1/tenants/{{tenantSubdomain}}/edge-devices/{{device_id}}/api-keys/{{edge_api_key_id}}`

And that the events request name no longer says "stub":

```bash
python3 -c "
import json
data = json.load(open('postman/Embolsadora-API-Complete.postman_collection.json', encoding='utf-8'))
names = [i['name'] for f in data['item'] for i in f.get('item', [])]
assert not any('stub, esperar 501' in n and 'Ingest Events' in n for n in names)
print('OK: no stray stub language on Ingest Events')
"
```

- [ ] **Step 2: Run `go build ./...` to confirm nothing on the Go side was touched by mistake**

```bash
go build ./...
```

Expected: exits 0, no output (this branch is docs/tooling only — this is a safety check, not a real test of the plan's changes).

- [ ] **Step 3: Remove the scratch verification scripts**

```bash
rm -f scratch_verify_consumers.py scratch_verify_public.py scratch_verify_update_me.py \
      scratch_verify_dashboard_metrics.py scratch_verify_edge_api_keys.py scratch_verify_info.py \
      scratch_verify_guide.py
git status --short
```

Expected: `git status --short` shows nothing (the scratch files were never committed, per Step 6 of each task only ever adding the two `postman/` files).

- [ ] **Step 4: Review the full diff one last time**

```bash
git log --oneline develop..HEAD
git diff develop..HEAD --stat
```

Expected: 6 commits (Tasks 1-3, 4-6 as committed), touching only `postman/Embolsadora-API-Complete.postman_collection.json`, `postman/env-local.postman_environment.json`, and `postman/POSTMAN-GUIDE.md`.

Do not push or open a PR as part of this plan — confirm with whoever is driving before pushing `chore/postman-collection-sync` or opening a PR against `develop`.

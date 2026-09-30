---
id: 003
title: "Gestión de edge devices"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-03-09
repos: [embolsadora4.0-cloud, embolsadora-frontend, embolsadora-edge]
origin: speckit
issues: []
prs: [17, 69, 71, 73, 77]
adrs: [CLOUD-ADR-015, CLOUD-ADR-016]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 003 — Gestión de edge devices

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-03-09, en
> [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. Las decisiones de formato
> (el path usa el subdominio del tenant, respetar el pact) están en [`research.md`](research.md).
> `origen: derivado` se reconstruyó del código y de los PRs #69, #71, #73 y #77.
> **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

Cada planta tiene un Edge Pi (una Raspberry Pi con el Edge Pi Service) asociado a una
máquina. El ABM necesita registrarlos, habilitarlos o deshabilitarlos, y consultarlos en el
momento: un chequeo de estado y de salud, la última telemetría y el historial de chequeos.
El cloud consulta al Edge Pi por HTTP con su `raspberryBaseUrl`.

## Alcance

**Entra:**

- CRUD de devices, habilitar y deshabilitar.
- Chequeos a demanda y telemetría.
- Historial de chequeos.

**No entra:**

- La autenticación del Pi hacia el cloud con API keys y la gestión de esas keys, que están
  en [022](../022-cloud-ingest-endpoint/spec.md) (RF-024).
- Las mediciones de la ingesta.

## Requisitos

### Acceso y tenant

- **RF-001** `origen: original` (FR-001) — Todo endpoint DEBE exigir un JWT válido.
- **RF-002** `origen: original` (FR-002) — Las operaciones DEBEN estar acotadas al tenant
  del path `/api/v1/tenants/:tenantId/edge-devices`. **Derivado (research, decisión 2):**
  `:tenantId` es el **subdominio** del tenant, no su UUID.
  → `ResolveTenantAndCheckMembership` (`internal/api/middleware/resolve_tenant_path.go`).
- **RF-003** `origen: original` (FR-003) — El acceso a un tenant no autorizado DEBE
  responder 403. **Derivado:**
  - un subdominio inexistente responde 404;
  - los operadores de plataforma tienen el fallback cross-tenant de CLOUD-ADR-015 (PR #73).
- **RF-020** `origen: derivado` (PR #69) — Cada ruta DEBE exigir un permiso fino:
  - `perm_edge_devices_view` para leer (listado, detalle, telemetría, eventos, listado de
    keys);
  - `perm_edge_devices_create` para dar de alta (migración `000013`);
  - `perm_edge_devices_manage` para editar, habilitar, deshabilitar, crear y revocar keys;
  - `perm_edge_devices_check` para los chequeos.
  → `internal/api/handler/edge_devices/routes.go`, `routes_rbac_test.go`.

### Registro y ciclo de vida

- **RF-004** `origen: original` (FR-004, FR-009, FR-010) — DEBE poder listarse los devices
  del tenant y obtenerse uno por ID; un ID inexistente en el tenant responde 404.
- **RF-005** `origen: original` (FR-005) — El alta DEBE exigir `name`, `machineId`,
  `edgeType` y `raspberryBaseUrl`, con `description` y `plcAddress` opcionales.
- **RF-006** `origen: original` (FR-006, FR-007) — `machineId` DEBE ser único dentro del
  tenant (409) y puede repetirse entre tenants.
  → `create_device.go` (409), restricción `uq_edge_devices_tenant_machine`.
- **RF-008** `origen: original` (FR-008, FR-018, FR-019) — Un device nuevo DEBE quedar
  con UUID del servidor, estado `ACTIVE`, salud `UNKNOWN`, `lastSeenAt` y
  `lastHealthCheckAt` nulos, y `createdAt`/`updatedAt`.
  → `edge_devices.Service` (`LastHealthStatus: "UNKNOWN"`).
- **RF-011** `origen: original` (FR-011), **ampliado** — DEBE poder editarse `name` y
  `description`. **Derivado (PR #71):** también `raspberryBaseUrl` y `plcAddress`.
  → `dto.UpdateDeviceRequest`.
- **RF-012** `origen: original` (FR-012) — DEBE poder pasarse un device entre `ACTIVE` y
  `DISABLED` con acciones dedicadas (`/enable` y `/disable`).

### Chequeos y telemetría

- **RF-013** `origen: original` (FR-013) — Los chequeos y la telemetría sobre un device
  `DISABLED` DEBEN responder 400 `EDGE_DEVICE_DISABLED`.
  → `status_check.go`, `health_check.go`, `get_telemetry.go`.
- **RF-014** `origen: original` (FR-014) — `POST …/status` DEBE consultar al Pi y
  devolver `checkType: STATUS`, `checkedAt`, `overallStatus`, `summary` y la versión.
  → `edgeclient.HTTPClient` (`GET {raspberryBaseUrl}/status`).
- **RF-015** `origen: original` (FR-015) — `POST …/health-check` DEBE devolver
  `checkType: HEALTH_CHECK` con métricas de hardware (CPU, RAM, disco).
  → `GET {raspberryBaseUrl}/health`.
- **RF-016** `origen: original` (FR-016) — `GET …/telemetry` DEBE devolver la última
  telemetría del Pi: CPU, RAM, disco, temperatura, uptime y estado del PLC.
  → `GET {raspberryBaseUrl}/telemetry`.
- **RF-017** `origen: original` (FR-017) — `GET …/events` DEBE devolver el historial de
  chequeos con quién los disparó (`userId`, `userEmail`).
  → tabla `device_events`.
- **RF-021** `origen: derivado` (PR #77) — `lastSeenAt` DEBE refrescarse también cuando
  el Pi entrega un batch a la ingesta, no solo con los chequeos del ABM.
  → `TouchLastSeen`, llamado desde la ingesta.

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-007** | Ningún device se expone fuera de su tenant | original (SC-007) | `resolve_tenant_path_test.go`; `internal/repo/pg/edge_devices/repository_test.go` |
| **CE-008** | Los requests inválidos (auth, campos faltantes, device `DISABLED`) reciben un error con código | original (SC-008) | handlers de `edge_devices` |
| **CE-009** | Todo alta queda con UUID, `ACTIVE`, `UNKNOWN` y timestamps | original (SC-009) | `internal/app/edge_devices/service_test.go` |
| **CE-010** | `machineId` único por tenant y repetible entre tenants | original (SC-010) | restricción única por `(tenant_id, machine_id)` |
| **CE-011** | Cada ruta exige su permiso fino | derivado | `routes_rbac_test.go` |

SC-001 a SC-006 eran umbrales de latencia sin medición: no se trasladan.

## Decisiones

| Decisión | Alternativa descartada | Por qué | Fuente |
|---|---|---|---|
| Tenant por subdominio en el path | UUID | El pact usa el slug; no exponer UUIDs en URLs | research, decisión 2 |
| Rutas bajo `/api/v1/tenants/:tenantId/…` | `/api/tenants/…` sin versión | Alinearse con el prefijo `/api/v1` | derivado |
| El cloud consulta al Pi a demanda | Que el Pi empuje su estado | Chequeos disparados por el usuario, con historial | design |

## Riesgos y preguntas abiertas

- **Contrato con el Edge sin enlazar:** el cloud consume `/status`, `/health` y
  `/telemetry` del Pi sin una versión fijada del contrato del repo `embolsadora-edge`. La
  copia en `contracts/` de esta spec puede estar desactualizada.
- **Dos formatos de tenant en la API** (UUID en el header, slug en este path); ver
  CLOUD-ADR-016.
- El cloud necesita alcanzar la `raspberryBaseUrl` de cada Pi por la red.

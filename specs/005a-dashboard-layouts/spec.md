---
id: 005a
title: "Layouts de dashboard"
tier: feature
status: done
veredicto: A
rf_incumplidos: [RF-013]
owner: Lucia Scharff
date: 2026-03-24
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [20, 32]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 005a — Layouts de dashboard

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-03-24, en
> [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. `origen: derivado` se
> reconstruyó del código y del esquema. **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

El frontend deja armar dashboards con widgets posicionados en una grilla. Hacía falta
persistir esas configuraciones ("layouts") con un tope y nombres únicos, sin quedarse
nunca sin ninguno. Los datos que muestran los widgets los resuelve
[024](../024-dashboard-metrics-query/spec.md).

## Alcance

**Entra:** el CRUD de `/api/v1/dashboard-layouts`.

**No entra:** los datos de los widgets.

## Requisitos

- **RF-001** `origen: original` (FR-001) — Todo endpoint DEBE exigir un JWT válido.
- **RF-002** `origen: original` (FR-002), **alcance cambiado** — Los layouts DEBEN estar
  aislados por el tenant de `X-Tenant-ID`. **Derivado:** además son **por usuario**:
  cada consulta, el tope y la unicidad de nombre se evalúan sobre `(tenant_id, user_id)`,
  con `user_id` tomado del JWT. Dos usuarios del mismo tenant no ven los layouts del otro.
  → `internal/repo/pg/dashboard_layouts/repository.go`,
  índice `idx_dashboard_layouts_tenant_user_name_active`.
- **RF-003** `origen: original` (FR-003) — El listado DEBE devolver `id`, `name`,
  `widgets`, `createdAt` y `updatedAt` de cada layout, más `meta: {total, limit}`.
  → `dto.ListLayoutsResponse`.
- **RF-004** `origen: original` (FR-004, FR-007) — Crear DEBE aceptar `name` y `widgets`
  opcionales; el servidor asigna ID y timestamps. **Derivado:** responde 200, no 201.
- **RF-005** `origen: original` (FR-005) — Hay un máximo de 3 layouts (por usuario y
  tenant, ver RF-002); superarlo responde 403 `LIMIT_REACHED`.
- **RF-006** `origen: original` (FR-006) — El nombre DEBE ser único (por usuario y
  tenant); un duplicado responde 409 `DUPLICATE_NAME`.
- **RF-008** `origen: original` (FR-008) — `GET /:id` DEBE devolver el layout o 404
  `LAYOUT_NOT_FOUND`.
- **RF-009** `origen: original` (FR-009, FR-012) — Actualizar DEBE permitir cambiar
  `name` y/o `widgets` (la lista se reemplaza completa) y renovar `updatedAt`.
- **RF-010** `origen: original` (FR-010) — Borrar el último layout DEBE responder 400
  (`CANNOT_DELETE_LAST_LAYOUT`). **Derivado:** el chequeo toma un lock de fila
  (`FOR UPDATE`) para que dos borrados concurrentes no dejen cero layouts; el borrado es
  lógico (`deleted_at`) y responde 200 `{}`.
- **RF-011** `origen: original` (FR-011) — Cada widget lleva `id`, `type`, `name`,
  `title`, `description`, `category`, `icon` y `position: {x, y, w, h, i}`. Se guarda
  como documento JSON.
- **RF-013** `origen: original` (FR-013) — **Ya no se cumple, por decisión posterior.**
  La spec pedía el envelope `{success, data}`. La estandarización
  [016](../016-api-standardization/spec.md) (PR #32) lo reemplazó por el struct directo y
  errores `{error, message, status}`. El 401 sigue con `{success: false, error}` porque
  lo arma `JWTAuth`.

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-006** | Ninguna operación lee ni modifica layouts de otro tenant (ni de otro usuario) | `WHERE tenant_id AND user_id`; `internal/repo/pg/dashboard_layouts/repository_test.go` |
| **CE-007** | Sin token, 401 | `JWTAuth` |
| **CE-008** | El tope de 3 no bloquea leer ni actualizar | solo se cuenta al crear |
| **CE-009** | El mismo nombre puede existir en tenants (y usuarios) distintos | índice único parcial |
| **CE-010** | Nunca se borra el último layout | `CANNOT_DELETE_LAST_LAYOUT` con lock |

SC-001 a SC-005 eran umbrales de latencia sin medición: no se trasladan.

## Decisiones

| Decisión | Por qué | Fuente |
|---|---|---|
| Tope de 3 y siempre al menos 1 | Mantener la UI simple y que siempre haya un dashboard | design |
| Layouts por usuario | Cada operario arma su propia vista | derivado (esquema `000001`) |
| Widgets como JSON | La forma del widget la define el frontend | design (FR-012) |

## Riesgos y preguntas abiertas

- **Layouts por usuario frente a por tenant:** la spec y el frontend pueden asumir que un
  layout es compartido por el tenant. Hay que confirmar la intención.
- `metrics` y `aasPath` de los widgets no se validan contra el catálogo (ver
  `aaspath-check` en el harness, que corresponde al frontend).

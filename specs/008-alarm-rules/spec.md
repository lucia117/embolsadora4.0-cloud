---
id: 008
title: "Reglas de alarma (CRUD)"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-06
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [25]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 008 — Reglas de alarma (CRUD)

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-04-06, en
> [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. **Verificado contra `develop`
> el 2026-09-29.**
>
> ⚠️ **Esta spec se cumple, pero el objetivo que la motivaba no.** La spec cubre solo la
> **configuración** de reglas. El motor que las evalúa contra las mediciones y genera
> alertas quedó fuera de alcance como "feature futura" y nunca se especificó ni se
> construyó. El objetivo específico #5 del anteproyecto (alertas automáticas) **no se
> cumple**. Ver la [bitácora de alcance](../../docs/_process/bitacora-de-alcance.md#objetivo-específico-5-alertas-automáticas).

## Contexto y problema

Un tenant necesita definir reglas del tipo "si `metric` `operator` `threshold`, disparar
una alarma de `severity`". La spec las trata como **configuración, no eventos**: "no
generan notificaciones directamente (eso corresponde a `notification-service-api`, feature
futura)" (design, *Assumptions*).

## Alcance

**Entra:** el CRUD de `/api/v1/alarm-rules`.

**No entra:**

- Evaluar las reglas.
- Generar alarmas o notificaciones.
- Paginación y borrado lógico (MVP).

## Requisitos

- **RF-001** `origen: original` (FR-001) — DEBE poder listarse las reglas del tenant.
- **RF-002** `origen: original` (FR-002) — DEBE poder crearse una regla con nombre,
  descripción, métrica, operador, umbral numérico y severidad. **Derivado:** hay además un
  campo `enabled`.
  → `internal/app/alarm_rules/service.go`, `internal/repo/pg/alarm_rules/repository.go`.
- **RF-003** `origen: original` (FR-003, FR-004, FR-005) — DEBE poder obtenerse,
  modificarse en forma parcial (`PATCH`) y borrarse una regla. El borrado es físico
  (supuesto de la spec).
- **RF-006** `origen: original` (FR-006) — Sin autenticación, 401.
- **RF-007** `origen: original` (FR-007, FR-009) — Una regla inexistente o de otro tenant
  DEBE responder 404 `ALARM_RULE_NOT_FOUND`: todas las consultas filtran por `tenant_id`.
- **RF-008** `origen: original` (FR-008) — Los datos inválidos responden 400
  `VALIDATION_ERROR`, indicando el campo. `operator` debe estar en
  `{gt, lt, gte, lte, eq}` y `severity` en `{info, warning, critical}`.
- **RF-010** `origen: original` (FR-010) — Cada regla registra `createdAt` y `updatedAt`.
- **RF-011** `origen: derivado` — Crear, modificar y borrar exige `perm_users_manage`;
  leer no exige un permiso adicional.
  → `internal/routes/url_mappings.go` (`alarmRulesWriteGroup`).

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-002** | Toda operación sobre reglas de otro tenant responde 404 | filtro por tenant; `internal/repo/pg/alarm_rules/repository_test.go` |
| **CE-003** | Sin autenticación, 401 | `JWTAuth` |
| **CE-005** | Los errores tienen un código estable y un mensaje | `internal/api/handler/alarm_rules/errors.go`; `internal/app/alarm_rules/service_test.go` |

SC-001 (cada operación en una interacción) se cumple por diseño REST. SC-004 (10 pacts)
no se verifica en este repo.

## Riesgos y preguntas abiertas

- **Motor de evaluación inexistente:** es la brecha principal del proyecto respecto del
  anteproyecto. Registrada como decisión (objetivo no cumplido) el 2026-09-29.
- **El permiso de escritura es `perm_users_manage`**, un permiso de usuarios que se
  reutiliza para reglas de alarma. No existe un `perm_alerts_manage`: cualquier
  administrador de usuarios puede cambiar las reglas.
- `metric` es texto libre, sin validar contra los `aasPath` que emite el Edge.

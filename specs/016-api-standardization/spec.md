---
id: 016
title: "Estandarización de errores y respuestas (P1)"
tier: change
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-16
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: []
prs: [32]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 016 — Estandarización de errores y respuestas (P1)

> **Procedencia.** `origen: original` viene del diseño del 2026-04-16, en
> [`design.md`](design.md). El plan de ejecución está en [`plan.md`](plan.md).
> **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

Cada módulo de la API respondía los errores con una forma distinta, y algunos envolvían el
éxito en `{"success": true, "data": …}` y otros no. Los pacts del frontend esperaban una
forma concreta por módulo. La estandarización fijó un patrón único **para los módulos de
su alcance**.

## Alcance

**Entra:** `alarm_rules`, `roles`, `dashboard_layouts`, `notifications`, `logs`,
`permissions`, `tenants` y `users`.

**No entra:**

- La lógica de negocio.
- Las firmas de servicio.
- Los módulos creados después: `edge_devices`, `user_roles`, `dashboards` e ingesta.

## Requisitos

- **RF-001** `origen: original` (Patrón estándar) — Cada módulo DEBE tener su
  `errors.go`, con `ErrorResponse{error, message, status}` y un `HandleError` que traduzca
  los errores de dominio a HTTP, con códigos en SCREAMING_SNAKE_CASE.
  → `internal/api/handler/{alarm_rules,roles,dashboard_layouts,notifications,logs,users}/errors.go`;
  `tenants/errors/`.
- **RF-002** `origen: original` (Success responses) — Las respuestas exitosas DEBEN ser el
  struct directo (o un array en los listados), sin `{"success": true, "data": …}`.
- **RF-003** `origen: original` (excepción logs) — `logs` mantiene el campo `data` porque
  el pact lo pide, pero sin `success`.
- **RF-004** `origen: original` (tabla por módulo, `permissions`) — `permissions` DEBE
  alinear su manejo de errores al struct tipado. **Derivado:** usa su propio
  `errorResponse{error}` en `handler.go`, sin `errors.go`; sigue siendo la forma que pide
  su pact.
- **RF-005** `origen: original` (tabla por módulo, `users`) — `users` DEBE tomar el tenant
  de `platform.TenantID` y no de `c.GetString`.

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | Ningún módulo del alcance envuelve el éxito en `success` | búsqueda de `"success"` en `internal/api/handler/{alarm_rules,roles,dashboard_layouts,notifications,logs,tenants,users}` (sin resultados). Única excepción: el `DELETE` de `permissions` responde `{success: true}` porque lo exige su pact (`deleteSuccessResponse`) |
| **CE-002** | El build pasa después de cada módulo | restricción del diseño; `go build ./...` |

## Riesgos y preguntas abiertas

- **Hoy conviven dos convenciones.** Los módulos posteriores a esta estandarización
  (`edge_devices`, `user_roles`, `dashboards`) y los middlewares (`JWTAuth`,
  `TenantFromHeader`) responden `{"success": …, "error": …}`. La ingesta tiene un
  contrato propio (`{"data": …}`, ver [022](../022-cloud-ingest-endpoint/spec.md)). Un
  cliente tiene que conocer las dos formas. Queda por decidir si se completa la
  estandarización.
- Las specs escritas antes que esta (006 y 005a) exigían el envelope y quedaron con ese
  requisito incumplido (`rf_incumplidos`).

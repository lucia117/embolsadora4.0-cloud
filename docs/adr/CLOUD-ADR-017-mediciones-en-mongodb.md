---
id: CLOUD-ADR-017
title: "Mediciones de la ingesta en MongoDB; identidad en Postgres"
status: propuesta
date: 2026-09-29
owner: Lucia Scharff
last_reviewed: 2026-09-29
supersedes: [CLOUD-ADR-003]
superseded_by: []
---

# CLOUD-ADR-017 — Mediciones de la ingesta en MongoDB; identidad en Postgres

> ADR retrospectivo. La decisión se tomó en el diseño de la ingesta
> (`embolsadora-edge/docs/superpowers/specs/2026-07-30-cloud-ingest-endpoint-design.md`,
> 2026-07-30, "Aprobado"), se implementó en el PR #56 (2026-08-31) y nunca se registró
> como ADR en este repo.

## Contexto

[CLOUD-ADR-003](CLOUD-ADR-003-eventos-en-postgres.md) (2025-10-17) decidió guardar los
eventos de las máquinas en Postgres: tabla `machine_events` con `payload JSONB`,
particionado mensual y retención de 90 días.

Esa tabla nunca existió en `migrations/`. Cuando se diseñó la ingesta real, el Edge Pi
Service ya enviaba batches con un contrato congelado cuyo `payload` es un objeto de forma
libre, que cambia cada vez que cambia el catálogo AAS del historian.

La spec `005-plc-events` (2026-03-24) también pedía persistir eventos de PLC en Postgres.
Se implementó distinto (veredicto B de la reconciliación de specs).

## Decisión

Los números entre paréntesis son las decisiones del diseño de la ingesta.

- **Las mediciones se guardan en MongoDB**, colección `measurements` (D-1). `payload` es
  de forma libre, y un document store lo absorbe sin migraciones cuando cambia el
  catálogo AAS.
- **La identidad queda en Postgres** (D-2): tenants, edge devices y API keys. Son datos
  transaccionales con integridad referencial que el ABM ya administra.
- **La idempotencia la garantiza la base, no la aplicación** (D-5): escritura síncrona
  con `insertMany(ordered:false)` y un índice único. Implementado como
  `uq_tenant_eventId` sobre `(tenantId, eventId)`
  (`internal/repo/mongo/measurements/repository.go`). El diseño decía `eventId` solo. Se
  corrigió porque `eventId` no incluye el tenant y dos tenants con el mismo `machineId`
  colisionaban: el segundo recibía `DUPLICATE` y el Edge borraba una medición que nunca
  se había guardado.
- **Sin TTL: retención indefinida** (D-9), por decisión explícita de producto.
- **El `payload` se guarda tal cual llega**, sin validar el contenido (D-8).
- Todo el conocimiento de Mongo queda en `internal/platform/mongo/` y `internal/repo/mongo/`
  detrás de interfaces del dominio.

## Alternativas consideradas

1. **Postgres con `JSONB` y particionado** (CLOUD-ADR-003). Para este ADR no se encontró
   una comparación escrita contra esta opción. La razón que da el diseño es la
   flexibilidad del `payload` (D-1).
2. **Caché de idempotencia por batch en Redis** (D-6). Descartada: sería una segunda
   fuente de verdad con casos borde, y el índice único ya da la garantía fuerte.

## Consecuencias

- La API depende de tres stores (Postgres, MongoDB, Redis). Mongo caído al arrancar no
  tumba el proceso: la ingesta y las métricas de dashboards responden 500 y el Edge
  reintenta sin perder datos. Si falla la creación del índice único, el proceso termina.
- No hay retención: el volumen crece sin límite. Agregar un TTL después no es destructivo.
- La tabla `device_events` de `000001` guarda los resultados de los chequeos de status y
  health de edge devices (`internal/repo/pg/edge_devices`), no las mediciones.
- Producción necesita un Mongo administrado. El diseño sugiere Atlas; el proveedor real
  está pendiente de documentar en `docs/operations.md`.

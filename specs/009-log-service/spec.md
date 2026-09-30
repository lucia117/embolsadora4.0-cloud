---
id: 009
title: "Servicio de logs: consulta, streaming, exportación y retención"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-07
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [26]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 009 — Servicio de logs: consulta, streaming, exportación y retención

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-04-07, en
> [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. `origen: derivado` se
> reconstruyó del código. **Verificado contra `develop` el 2026-09-29.**

> ⚠️ **Nada escribe logs.** La spec define un servicio "de **consulta**, no de ingesta":
> supone que "los logs son generados por otros servicios/workers" (design,
> *Assumptions*). Existe el punto de escritura (`logwriter.LogWriter`, implementado por
> `logs.Service.Write`, que además publica al stream), pero **ningún componente lo
> llama**. En producción `log_entries` solo tiene lo que se cargue a mano o por seed. Ver la
> [bitácora de alcance](../../docs/_process/bitacora-de-alcance.md#logs-y-notificaciones-sin-productor).

## Contexto y problema

El frontend tiene un visor de eventos del tenant: filtros, búsqueda, contexto temporal,
exportación y tiempo real. La spec define ese lado de lectura y la configuración de
retención.

## Alcance

**Entra:** `GET /logs`, `GET /logs/:id`, `GET /logs/:id/context`, `GET /logs/export`,
`GET /logs/stream` (SSE) y `GET|PATCH /logs/retention`.

**No entra:** la generación de logs, delegada a "otros servicios" que no existen (ver el
aviso de arriba).

## Requisitos

- **RF-001** `origen: original` (FR-001) — El listado DEBE filtrar por `event_type`,
  `severity`, `machine_id`, `from`/`to` y texto libre `q`, en cualquier combinación.
  → `internal/repo/pg/logs/repository.go`, `cursor_filters_test.go`.
- **RF-002** `origen: original` (FR-002) — La paginación DEBE ser por cursor
  (`next_cursor`), no por offset.
- **RF-003** `origen: original` (FR-003) — Los logs DEBEN estar aislados por tenant.
- **RF-004** `origen: original` (FR-004, FR-005) — DEBE poder obtenerse un log por ID y
  la ventana de logs contiguos a un evento.
- **RF-006** `origen: original` (FR-006) — La exportación DEBE aplicar los filtros y
  truncar con aviso al superar el máximo. **Derivado:** el máximo es de 50.000 filas; se
  piden 50.001 para detectar el truncamiento.
  → `logs.Service` (`maxExport = 50000`).
- **RF-007** `origen: original` (FR-007) — DEBE existir un stream SSE de eventos nuevos,
  restringido al tenant. **Derivado:** es un pub/sub **en memoria del proceso**, con un
  latido cada 30 s; un suscriptor lento pierde eventos (`default: drop`).
  → `stream_logs.go`, `logs.Service.Publish`.
- **RF-008** `origen: original` (FR-008, FR-010) — DEBE poder consultarse y actualizarse
  la política de retención del tenant; actualizarla exige permiso de administrador.
  **Derivado:** ese permiso es `perm_logs_admin`.
  → `internal/api/handler/logs/routes.go`.
- **RF-009** `origen: original` (FR-009) — Todos los endpoints exigen JWT.
- **RF-011** `origen: derivado` — **La política de retención se guarda pero no se
  aplica:** ningún proceso borra los logs vencidos. `next_purge_at` se calcula y no se usa.

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-002** | La paginación por cursor no produce duplicados ni saltos | `internal/repo/pg/logs/cursor_filters_test.go` |
| **CE-004** | Los logs de un tenant son invisibles para otros | filtro por tenant; `internal/repo/pg/logs/repository_test.go` |

- SC-001, SC-005 y SC-006 eran umbrales de rendimiento sin medición.
- SC-003 (14 pacts) no se verifica en este repo.

## Riesgos y preguntas abiertas

- **Sin productor de logs**, la feature no tiene datos reales.
- **SSE en memoria:** en Cloud Run con más de una instancia, un cliente conectado a la
  instancia A no ve los eventos escritos en la B. Mientras no haya productor no se nota.
- **Retención no aplicada** (RF-011).

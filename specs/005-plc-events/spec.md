# Feature Specification: PLC Events Ingestion & Query API

**Feature Branch**: `005-plc-events`
**Created**: 2026-03-24
**Status**: Draft
**Input**: Almacenar eventos pre-procesados de un PLC (alarmas, mediciones, estados) para visualización en el frontend via widgets del dashboard.

---

## Contexto de negocio

Cada embolsadora tiene un PLC (controlador lógico programable) que genera datos continuamente: mediciones de sensores, cambios de estado operativo y alarmas. Esos datos se almacenan en **InfluxDB** (externo, ya existente). Una **Processing API** externa lee InfluxDB, pre-procesa los datos y los envía a esta API cloud para persistencia y consulta.

El frontend del dashboard visualiza estos datos a través de widgets configurables. Cada widget apunta a un datasource específico (un EdgeDevice + tipo de evento + tags de filtro).

### Flujo de datos

```
PLC
 │ escribe en tiempo real
 ▼
InfluxDB (externo)
 │ polling / lectura programada
 ▼
Processing API (externo, pequeño servicio)
 │ pre-procesa: normaliza unidades, aplica reglas, clasifica
 │ POST /api/tenants/:tenantId/edge-devices/:deviceId/plc-events (batch)
 ▼
Cloud API — este feature
 │ valida, persiste en MongoDB
 ▼
MongoDB (colección plc_events)
 │
 ▼ GET /api/tenants/:tenantId/edge-devices/:deviceId/plc-events?filters
Frontend Dashboard (widgets)
```

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Ingerir un batch de eventos del PLC (Priority: P1)

La Processing API envía un batch de eventos pre-procesados de un PLC asociado a un EdgeDevice. La API cloud los valida y persiste en MongoDB.

**Por qué P1**: Sin ingestión no hay datos. Es el punto de entrada de toda la funcionalidad.

**Test independiente**: Puede testearse enviando un batch de eventos válidos y verificando que se persisten correctamente consultando el endpoint de listado.

**Escenarios de aceptación**:

1. **Given** un EdgeDevice activo en el tenant, **When** la Processing API envía un batch de eventos válidos (alarmas, mediciones y/o estados), **Then** el sistema persiste todos los eventos y retorna 201 con el conteo de eventos creados.
2. **Given** un batch con un evento con campos requeridos faltantes, **When** se envía, **Then** el sistema retorna 400 Bad Request con detalle del campo faltante; los eventos válidos del batch NO se persisten (fail-fast todo el batch).
3. **Given** un EdgeDevice inexistente o de otro tenant, **When** se envía un batch, **Then** el sistema retorna 404 o 403 respectivamente.
4. **Given** un batch vacío (`events: []`), **When** se envía, **Then** el sistema retorna 400 Bad Request.
5. **Given** un batch con más de 1000 eventos, **When** se envía, **Then** el sistema retorna 400 con error `BATCH_TOO_LARGE`.

---

### User Story 2 — Consultar eventos recientes de un PLC (Priority: P1)

Un widget del frontend consulta los eventos más recientes de un EdgeDevice para mostrar el estado actual o el historial inmediato.

**Por qué P1**: Es la operación de lectura principal que alimenta los widgets del dashboard.

**Test independiente**: Puede testearse insertando eventos con distintos tipos y tags, y verificando que el listado retorna los eventos correctos en orden cronológico descendente.

**Escenarios de aceptación**:

1. **Given** un EdgeDevice con eventos persistidos, **When** el frontend consulta sin filtros, **Then** el sistema retorna los eventos más recientes paginados (default: últimos 100, orden: timestamp DESC).
2. **Given** eventos de tipo `ALARM`, `MEASUREMENT` y `STATE`, **When** se filtra por `eventType=ALARM`, **Then** el sistema retorna solo alarmas.
3. **Given** eventos con distintos tags, **When** se filtra por `tags=station:filling`, **Then** el sistema retorna solo los eventos que tienen ese tag exacto.
4. **Given** eventos en un rango temporal, **When** se filtra con `from` y `to` (ISO 8601), **Then** el sistema retorna solo los eventos dentro de ese rango.
5. **Given** múltiples filtros simultáneos (`eventType` + `tags` + rango de tiempo), **When** se consulta, **Then** se aplican todos con AND lógico.

---

### User Story 3 — Obtener el último valor de una medición (Priority: P1)

Un widget de tipo "gauge" o "indicador" del frontend necesita el último valor de una medición específica (ej: `bag_weight`) para mostrar el estado actual de la máquina.

**Por qué P1**: Los widgets de estado en tiempo real son el caso de uso más frecuente del dashboard.

**Test independiente**: Puede testearse insertando múltiples mediciones del mismo `measurement` y verificando que `/latest` retorna solo la más reciente.

**Escenarios de aceptación**:

1. **Given** múltiples eventos `MEASUREMENT` del mismo `measurement` (ej: `bag_weight`), **When** el frontend solicita `GET .../plc-events/latest?measurement=bag_weight`, **Then** el sistema retorna el evento más reciente de esa medición.
2. **Given** eventos de distintos `measurement`, **When** se solicita latest sin filtrar por measurement, **Then** el sistema retorna el último evento de cada measurement único (mapa measurement → último valor).
3. **Given** no hay eventos para el `measurement` solicitado, **When** se consulta, **Then** el sistema retorna 404 Not Found.

---

### User Story 4 — Consultar alarmas activas (Priority: P2)

Un widget de "alarmas activas" muestra las alarmas que no han sido resueltas, ordenadas por severidad y tiempo.

**Por qué P2**: Importante para operaciones, pero la funcionalidad base de ingestión y consulta (P1) es prerequisito.

**Test independiente**: Puede testearse insertando alarmas con `status: ACTIVE` y `RESOLVED`, y verificando que el endpoint de activas solo retorna las `ACTIVE`.

**Escenarios de aceptación**:

1. **Given** alarmas con distintos `status` (`ACTIVE`, `RESOLVED`), **When** se consulta `GET .../plc-events?eventType=ALARM&status=ACTIVE`, **Then** el sistema retorna solo las alarmas activas.
2. **Given** alarmas activas con distintas `severity`, **When** se consulta, **Then** el sistema retorna las alarmas ordenadas por severidad (`CRITICAL` > `HIGH` > `MEDIUM` > `LOW`) y luego por timestamp DESC.
3. **Given** una alarma fue resuelta (nuevo evento `ALARM` con mismo `code` y `status: RESOLVED`), **When** se consulta alarmas activas, **Then** esa alarma no aparece en la lista.

---

### User Story 5 — Agregar tags a eventos existentes (Priority: P3)

Un operario enriquece manualmente un evento con tags adicionales (p.ej. marcar una alarma con la causa raíz identificada) para mejorar el análisis posterior.

**Por qué P3**: Útil para análisis, pero no es operativamente crítico en MVP.

**Test independiente**: Puede testearse agregando tags a un evento existente y verificando que aparecen en la consulta siguiente.

**Escenarios de aceptación**:

1. **Given** un evento existente, **When** un usuario autenticado hace `PATCH .../plc-events/:eventId/tags` con nuevos tags, **Then** los tags se agregan al evento (merge, no reemplazo) y retorna el evento actualizado.
2. **Given** tags duplicados en el PATCH, **When** se procesa, **Then** el sistema deduplica y persiste un solo valor por clave.

---

### Edge Cases

- ¿Qué pasa si el mismo evento se envía dos veces (reintento de la Processing API)? → Deduplicación por `externalId` (campo opcional enviado por la Processing API). Si ya existe un evento con ese `externalId` en el device, se ignora silenciosamente y se retorna 201 con el conteo real de insertados.
- ¿Qué pasa si `timestamp` del evento está en el futuro? → Se acepta con warning en el log; no se rechaza (el reloj del PLC puede estar desincronizado).
- ¿Qué pasa si `timestamp` tiene más de 30 días de antigüedad? → Se acepta pero se registra métrica `plc_events_stale_total`.
- ¿Qué pasa si se consultan eventos de un device que no pertenece al tenant del JWT? → 403 Forbidden.
- ¿Qué pasa con tags con caracteres especiales (`:`, `/`, espacios)? → Se aceptan tal cual; los valores de tags son strings libres.
- ¿Qué pasa si `value` es nulo en una `MEASUREMENT`? → Se acepta; puede ocurrir cuando el sensor no reporta valor.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE requerir autenticación JWT en todos los endpoints.
- **FR-002**: El sistema DEBE aislar todos los eventos por `tenantId` y `deviceId`; ninguna consulta expone eventos de otro tenant o device.
- **FR-003**: El sistema DEBE aceptar un endpoint de ingestión en batch: `POST /api/tenants/:tenantId/edge-devices/:deviceId/plc-events`.
- **FR-004**: El batch DEBE soportar los tres tipos de eventos: `ALARM`, `MEASUREMENT` y `STATE`.
- **FR-005**: El batch DEBE tener un máximo de 1000 eventos por request.
- **FR-006**: Si cualquier evento del batch es inválido, el batch completo DEBE rechazarse (fail-fast, sin inserción parcial).
- **FR-007**: El sistema DEBE soportar deduplicación de eventos por campo `externalId` opcional.
- **FR-008**: El sistema DEBE exponer endpoint de consulta: `GET /api/tenants/:tenantId/edge-devices/:deviceId/plc-events`.
- **FR-009**: El endpoint de consulta DEBE soportar filtros: `eventType`, `tags` (key:value), `from` (ISO 8601), `to` (ISO 8601), `status` (para alarmas), `measurement` (para mediciones), `limit` (default 100, max 1000), `cursor` (paginación).
- **FR-010**: El sistema DEBE exponer endpoint de último valor: `GET /api/tenants/:tenantId/edge-devices/:deviceId/plc-events/latest`.
- **FR-011**: El endpoint `/latest` DEBE soportar filtro por `measurement` (retornar último valor) y sin filtro (retornar último valor de cada measurement único).
- **FR-012**: Los resultados del endpoint de consulta DEBEN ordenarse por `timestamp DESC` por defecto.
- **FR-013**: El sistema DEBE soportar adición de tags a eventos existentes via `PATCH .../plc-events/:eventId/tags`.
- **FR-014**: El sistema DEBE validar que el `deviceId` pertenece al `tenantId` de la URL antes de insertar o consultar eventos.
- **FR-015**: Todos los eventos persistidos DEBEN incluir `receivedAt` (timestamp del servidor al momento de la ingestión), diferente del `timestamp` del evento (timestamp del PLC).

### Key Entities

- **PLCEvent**: Evento generado por un PLC, pre-procesado y enviado por la Processing API. Campos comunes: `id`, `tenantId`, `deviceId` (FK a EdgeDevice), `eventType` (`ALARM` | `MEASUREMENT` | `STATE`), `timestamp` (del PLC), `receivedAt` (del servidor), `tags` (mapa clave-valor libre), `externalId` (opcional, para deduplicación).

- **Alarm** (subtipo de PLCEvent con `eventType: ALARM`): Campos adicionales: `code` (código de alarma del PLC), `message` (descripción legible), `severity` (`CRITICAL` | `HIGH` | `MEDIUM` | `LOW`), `status` (`ACTIVE` | `RESOLVED`).

- **Measurement** (subtipo de PLCEvent con `eventType: MEASUREMENT`): Campos adicionales: `measurement` (nombre de la variable medida, ej: `bag_weight`), `value` (número o null), `unit` (ej: `kg`, `°C`, `rpm`).

- **StateChange** (subtipo de PLCEvent con `eventType: STATE`): Campos adicionales: `stateName` (nombre del estado, ej: `machine_mode`), `previousValue` (valor anterior), `currentValue` (valor nuevo).

---

## Success Criteria *(mandatory)*

- **SC-001**: `POST .../plc-events` (batch de 100 eventos) completa en menos de 500ms p95.
- **SC-002**: `GET .../plc-events` (sin filtros, últimos 100) retorna en menos de 200ms p95.
- **SC-003**: `GET .../plc-events/latest` retorna en menos de 100ms p95.
- **SC-004**: La deduplicación por `externalId` funciona en el 100% de los casos; ningún evento duplicado se persiste.
- **SC-005**: El 100% de las consultas cross-tenant retornan 403.
- **SC-006**: Los filtros combinados (`eventType` + `tags` + rango de tiempo) retornan resultados correctos en el 100% de los casos.
- **SC-007**: Un batch de 1000 eventos se persiste en menos de 2 segundos p95.
- **SC-008**: Los eventos tienen siempre `receivedAt` del servidor, independientemente del `timestamp` del PLC.

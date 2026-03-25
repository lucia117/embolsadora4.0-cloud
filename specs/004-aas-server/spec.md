# Feature Specification: AAS Server — Asset Administration Shell

**Feature Branch**: `004-aas-server`
**Created**: 2026-03-24
**Status**: Draft
**Standard**: IDTA-01002-3-1 (AAS Part 2: API) | IEC 63278-1:2023

---

## Contexto de negocio

La plataforma Embolsadora 4.0 necesita un gemelo digital estandarizado para cada máquina embolsadora registrada en el sistema. Este gemelo digital sigue el estándar **Asset Administration Shell (AAS)** de la Industria 4.0, permitiendo interoperabilidad con sistemas externos, auditabilidad de la configuración de la máquina y visibilidad del estado operativo en tiempo real.

### Flujo de datos (end-to-end)

```
PLC (controlador físico)
    │
    ▼ escribe datos en tiempo real
InfluxDB (externo, ya existente)
    │
    ▼ polling / consulta programada
Processing API (nueva — lee InfluxDB, normaliza y transforma)
    │
    ▼ PATCH $value en submodelo
AAS Server (este feature — almacena en MongoDB)
    │
    ▼
Frontend / sistemas externos consultan el gemelo digital
```

El **AAS Server** es el destino final de los datos procesados y el punto de consulta del gemelo digital. No se conecta directamente al PLC ni a InfluxDB: recibe los datos ya procesados desde la Processing API.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Consultar el gemelo digital de una máquina (Priority: P1)

Un operario o administrador consulta el Asset Administration Shell completo de una máquina registrada para ver su estructura, configuración técnica y estado operativo actual.

**Por qué P1**: Es la operación de lectura fundamental del sistema. Sin ella, ningún sistema externo ni el frontend pueden consumir el gemelo digital.

**Test independiente**: Se puede probar registrando un shell y consultándolo por ID, verificando que todos los submodelos y sus elementos retornan con los valores correctos.

**Escenarios de aceptación**:

1. **Given** una máquina con AAS registrada, **When** un usuario autenticado solicita el shell por su AAS-ID, **Then** el sistema retorna el shell completo con todas las referencias a submodelos.
2. **Given** un AAS-ID inexistente, **When** se solicita el shell, **Then** el sistema retorna 404 Not Found.
3. **Given** un usuario autenticado de un tenant distinto al del AAS, **When** solicita el shell, **Then** el sistema retorna 403 Forbidden.

---

### User Story 2 — Registrar el gemelo digital de una máquina (Priority: P1)

Un administrador crea el AAS de una nueva máquina, vinculándolo al `MachineID` del `EdgeDevice` existente para establecer el gemelo digital.

**Por qué P1**: Sin registro de un AAS no hay gemelo digital. Es el punto de entrada de toda la estructura de la máquina.

**Test independiente**: Se puede probar creando un AAS con un `globalAssetId` válido y verificando que el sistema retorna el shell creado con ID único asignado por el servidor.

**Escenarios de aceptación**:

1. **Given** un EdgeDevice con `MachineID` registrado, **When** un administrador crea un AAS con `globalAssetId` igual al `MachineID`, **Then** el sistema crea el shell, asigna un server-ID y retorna el AAS creado.
2. **Given** ya existe un AAS con el mismo `globalAssetId` en el tenant, **When** se intenta crear otro, **Then** el sistema retorna 409 Conflict.
3. **Given** campos requeridos faltantes (`globalAssetId`), **When** se intenta la creación, **Then** el sistema retorna 400 Bad Request.

---

### User Story 3 — Consultar un submodelo específico (Priority: P1)

Un sistema externo o el frontend consulta un submodelo específico (p.ej. `OperationalData`) para obtener el estado actual de la máquina sin cargar todo el shell.

**Por qué P1**: Los submodelos son la unidad de consumo principal. Sistemas externos (MES, SCADA) consultarán submodelos individuales, no shells completos.

**Test independiente**: Se puede probar consultando el submodelo `OperationalData` y verificando que todos sus `SubmodelElements` retornan con sus valores actuales.

**Escenarios de aceptación**:

1. **Given** un submodelo registrado, **When** se solicita por su Submodel-ID, **Then** el sistema retorna el submodelo completo con todos sus elementos y valores actuales.
2. **Given** un Submodel-ID inexistente, **When** se solicita, **Then** el sistema retorna 404 Not Found.
3. **Given** un submodelo con colecciones anidadas (estaciones), **When** se solicita, **Then** el sistema retorna la jerarquía completa de `SubmodelElementCollection`.

---

### User Story 4 — Actualizar el valor de un elemento del submodelo (Priority: P1)

La **Processing API** actualiza el valor de una `Property` dentro de un submodelo (p.ej. `CurrentBagWeight`, `AlarmState`) luego de procesar datos provenientes de InfluxDB.

**Por qué P1**: Es el mecanismo central de actualización del gemelo digital. Sin él, el AAS queda estático y no refleja el estado real de la máquina.

**Test independiente**: Se puede probar haciendo `PATCH $value` sobre `OperationalData/CurrentBagWeight` y verificando que el valor se actualiza correctamente, incluyendo el timestamp de modificación.

**Escenarios de aceptación**:

1. **Given** una `Property` existente en un submodelo, **When** la Processing API hace `PATCH .../submodel-elements/{idShortPath}/$value` con un nuevo valor, **Then** el sistema actualiza el valor, registra `updatedAt` y retorna 204 No Content.
2. **Given** un `idShortPath` que no existe en el submodelo, **When** se intenta actualizar, **Then** el sistema retorna 404 Not Found.
3. **Given** un valor de tipo incorrecto (p.ej. string donde se espera `xs:float`), **When** se envía, **Then** el sistema retorna 400 Bad Request.
4. **Given** una actualización sobre una `SubmodelElementCollection` anidada (ruta compuesta: `Stations.FillingStation.FlowRate`), **When** se hace el PATCH, **Then** el sistema navega la jerarquía y actualiza el valor correcto.

---

### User Story 5 — Registrar y buscar un AAS en el Registry (Priority: P2)

Un sistema externo registra el descriptor de un AAS en el Registry para que otros sistemas puedan descubrirlo por su `globalAssetId`.

**Por qué P2**: El Registry es necesario para la interoperabilidad con sistemas externos que siguen el estándar IDTA. No es bloqueante para el MVP interno.

**Test independiente**: Se puede probar registrando un descriptor y buscándolo por `globalAssetId`, verificando que se retorna el endpoint correcto.

**Escenarios de aceptación**:

1. **Given** un AAS registrado en el Repository, **When** se registra su descriptor en el Registry con `globalAssetId` y endpoint, **Then** el sistema almacena el descriptor y lo retorna en búsquedas.
2. **Given** un `globalAssetId` registrado, **When** se consulta el Discovery Service con ese ID, **Then** el sistema retorna el AAS-ID correspondiente.
3. **Given** un `globalAssetId` desconocido, **When** se consulta el Discovery Service, **Then** el sistema retorna una lista vacía (no error).

---

### User Story 6 — Listar todos los AAS de un tenant (Priority: P2)

Un administrador lista todos los gemelos digitales registrados para su tenant para tener una vista general del parque de máquinas.

**Por qué P2**: Necesario para el panel de administración, pero no bloquea las operaciones operativas individuales.

**Test independiente**: Se puede probar creando múltiples AAS en un tenant y verificando que el listado retorna todos con paginación correcta.

**Escenarios de aceptación**:

1. **Given** un tenant con múltiples AAS registrados, **When** se solicita el listado, **Then** el sistema retorna todos los shells con paginación (cursor + limit).
2. **Given** un tenant sin AAS, **When** se solicita el listado, **Then** el sistema retorna lista vacía.
3. **Given** AAS de diferentes tenants en el mismo servidor, **When** un usuario de un tenant lista sus shells, **Then** solo ve los shells de su tenant.

---

### User Story 7 — Consultar historial de cambios de valor (Priority: P3)

Un auditor o ingeniero de proceso consulta el historial de valores de una `Property` específica (p.ej. `CurrentBagWeight`) para analizar tendencias o investigar incidentes.

**Por qué P3**: Útil para análisis, pero no es operacionalmente crítico. El historial puede consultarse directamente en InfluxDB para análisis de series de tiempo; aquí se trata de cambios al modelo, no de series de alta frecuencia.

**Test independiente**: Se puede probar actualizando un valor varias veces y verificando que el historial retorna todos los cambios con timestamp y valor anterior/nuevo.

**Escenarios de aceptación**:

1. **Given** una `Property` con múltiples actualizaciones, **When** se consulta su historial, **Then** el sistema retorna una lista cronológica de cambios con: timestamp, valor anterior, valor nuevo y origen del cambio (Processing API).
2. **Given** una `Property` nunca actualizada, **When** se consulta su historial, **Then** el sistema retorna lista con el valor inicial de creación.

---

### Edge Cases

- ¿Qué pasa si la Processing API envía un batch de actualizaciones simultáneas sobre el mismo elemento? → Última escritura gana (last-write-wins) con timestamp del servidor; se registra en historial.
- ¿Qué pasa si se elimina un submodelo referenciado en un AAS? → El AAS conserva la referencia; la consulta del submodelo retorna 404 hasta que se recree.
- ¿Qué pasa si el `globalAssetId` del AAS no coincide con ningún `MachineID` en `edge_devices`? → El sistema acepta la creación (no valida la existencia del EdgeDevice en MVP); la validación cruzada es responsabilidad de la capa de negocio.
- ¿Qué pasa si se intenta actualizar un elemento de tipo `SubmodelElementCollection` directamente (no una `Property`)? → 405 Method Not Allowed; solo `Property` acepta `$value`.
- ¿Qué pasa si se crea un submodelo con un `idShort` duplicado dentro del mismo AAS? → 409 Conflict.
- ¿Qué pasa con tenants que no tienen MongoDB configurado? → Error de configuración en startup, no en runtime.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE requerir un Bearer JWT válido en todos los endpoints; las solicitudes sin token DEBEN retornar 401.
- **FR-002**: El sistema DEBE aislar todos los datos por `tenantId`; ninguna operación puede exponer datos de otro tenant.
- **FR-003**: El sistema DEBE implementar el **AAS Repository Interface** (IDTA-01002): `GET/POST /shells`, `GET/PUT/DELETE /shells/{aasId}`.
- **FR-004**: El sistema DEBE implementar el **Submodel Repository Interface**: `GET/POST /submodels`, `GET/PUT/DELETE /submodels/{smId}`.
- **FR-005**: El sistema DEBE implementar la actualización de valor de elemento: `PATCH /submodels/{smId}/submodel-elements/{idShortPath}/$value`.
- **FR-006**: El sistema DEBE soportar rutas compuestas en `idShortPath` (separadas por `.`) para navegar jerarquías de `SubmodelElementCollection`.
- **FR-007**: El sistema DEBE implementar el **AAS Registry Interface**: `GET/POST /registry/shell-descriptors`, `GET/PUT/DELETE /registry/shell-descriptors/{aasId}`.
- **FR-008**: El sistema DEBE implementar el **Discovery Service**: `GET /lookup/shells?assetIds=` para buscar AAS-IDs por `globalAssetId`.
- **FR-009**: Los identificadores en path params DEBEN estar codificados en **Base64URL** (cumplimiento IDTA).
- **FR-010**: El sistema DEBE soportar **paginación por cursor** en todos los endpoints de listado (`cursor` + `limit`).
- **FR-011**: El sistema DEBE validar tipos de dato (`xs:float`, `xs:int`, `xs:boolean`, `xs:string`, `xs:date`, `xs:dateTime`) al actualizar valores.
- **FR-012**: El sistema DEBE registrar un historial de cambios por cada actualización de `Property` (timestamp, valor anterior, valor nuevo, origen).
- **FR-013**: El sistema DEBE retornar `updatedAt` actualizado en toda operación de escritura.
- **FR-014**: El sistema DEBE asignar IDs en formato URN al crear shells y submodelos si el cliente no provee un ID.
- **FR-015**: El sistema DEBE serializar y deserializar en **JSON** (principal) y **XML** (secundario, según `Content-Type`).

### Key Entities

- **AssetAdministrationShell**: Gemelo digital de un asset. Campos clave: `id` (URN), `globalAssetId` (= `EdgeDevice.MachineID`), `assetKind` (Instance/Type), `submodels[]` (referencias), `tenantId`, timestamps.
- **Submodel**: Aspecto específico del asset. Campos clave: `id` (URN), `idShort`, `semanticId`, `submodelElements[]`, `tenantId`, `shellId` (FK al AAS), timestamps.
- **SubmodelElement**: Unidad de dato. Tipos: `Property` (valor escalar), `SubmodelElementCollection` (agrupación), `File`, `Operation`, `ReferenceElement`.
- **Property**: SubmodelElement con valor escalar. Campos clave: `idShort`, `valueType` (XSD type), `value` (string), `unit`, `updatedAt`.
- **SubmodelElementCollection**: Agrupación con hijos. Campos clave: `idShort`, `value[]` (elementos anidados).
- **ShellDescriptor**: Registro en el Registry. Campos clave: `id` (AAS-ID), `globalAssetId`, `endpoints[]` (URL donde se sirve el AAS).
- **PropertyChangeEvent**: Entrada en historial. Campos: `propertyPath`, `oldValue`, `newValue`, `changedAt`, `source` ("processing-api").

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `GET /shells/{aasId}` retorna en menos de 200ms p95 para shells con hasta 5 submodelos y 100 elementos.
- **SC-002**: `PATCH .../submodel-elements/{idShortPath}/$value` (actualización de valor) retorna en menos de 100ms p95.
- **SC-003**: `GET /submodels/{smId}` retorna en menos de 150ms p95 para submodelos con hasta 50 elementos.
- **SC-004**: El Discovery Service retorna resultados en menos de 100ms p95.
- **SC-005**: 100% de las solicitudes sin token o con token inválido retornan 401.
- **SC-006**: 100% de las operaciones cross-tenant retornan 403 sin exponer datos del otro tenant.
- **SC-007**: Las actualizaciones concurrentes sobre la misma `Property` no producen datos corruptos (last-write-wins con timestamp del servidor).
- **SC-008**: El historial de cambios registra el 100% de las actualizaciones `$value` con timestamp y valor anterior correcto.
- **SC-009**: Los identificadores en path params son correctamente codificados/decodificados en Base64URL en el 100% de las operaciones.
- **SC-010**: La Processing API puede actualizar 50 propiedades por segundo sin degradación observable del servidor.

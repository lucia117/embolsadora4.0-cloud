# Feature Specification: MongoDB Infrastructure Layer

**Feature Branch**: `006-mongo-infra`
**Created**: 2026-04-02
**Status**: Draft
**Input**: Agregar soporte MongoDB al proyecto: capa repositorio y servicio lista para uso, con CRUD de ejemplo para AAS shells y submodelos

---

## Contexto de negocio

La plataforma Embolsadora 4.0 necesita almacenar el gemelo digital de cada máquina (Asset Administration Shell) en una base de datos documental que se ajuste a la naturaleza jerárquica y semiestructurada del estándar AAS. El stack actual (PostgreSQL) cubre usuarios, tenants, roles y eventos de dispositivos; MongoDB es la capa complementaria elegida para los documentos AAS (ADR-005 pendiente de formalizar).

Esta feature no expone nuevos endpoints al frontend: establece la infraestructura de acceso a MongoDB dentro del proyecto — conexión, cliente reutilizable, repositorios base y un CRUD de ejemplo sobre las colecciones `asset_administration_shells` y `submodels` — de modo que las features siguientes puedan consumir MongoDB sin repetir código de plomería.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Conectar y desconectar MongoDB de forma controlada (Priority: P1)

El sistema levanta la conexión a MongoDB al arrancar y la cierra limpiamente al apagarse, respetando el ciclo de vida ya establecido para PostgreSQL y Redis.

**Why this priority**: Sin una conexión estable y gestionada, ningún repositorio MongoDB puede funcionar. Es el prerequisito de todo lo demás.

**Independent Test**: Se puede verificar arrancando el servidor con `MONGO_URI` configurado y observando que el log muestra "MongoDB connected" al inicio y "MongoDB disconnected" al cierre graceful (Ctrl+C / SIGTERM), sin errores ni goroutine leaks.

**Acceptance Scenarios**:

1. **Given** `MONGO_URI` y `MONGO_DB` están configurados, **When** el servidor arranca, **Then** el cliente MongoDB se inicializa, se valida con ping y el log registra la conexión exitosa.
2. **Given** el servidor recibe señal de apagado, **When** se ejecuta el shutdown graceful, **Then** la conexión MongoDB se cierra limpiamente antes de que el proceso termine.
3. **Given** `MONGO_URI` está ausente, **When** el servidor arranca, **Then** el servidor inicia normalmente con un log de advertencia (`WARN mongo disabled — MONGO_URI not set`); los repositorios MongoDB no se inicializan y retornan error si se invocan.
4. **Given** `MONGO_URI` está presente pero es inválido o inaccesible, **When** el servidor arranca, **Then** el servidor inicia con log de advertencia indicando que MongoDB no pudo conectarse; el healthcheck reporta `"status": "degraded"` para la sección `mongo`.

---

### User Story 2 — CRUD de Asset Administration Shell (Priority: P1)

Un desarrollador o proceso interno puede crear, leer, actualizar y eliminar un AAS shell en MongoDB usando el repositorio provisto, sin escribir código de acceso a MongoDB desde cero.

**Why this priority**: El repositorio AAS es la primera superficie concreta de MongoDB en el proyecto. Sirve de referencia (example CRUD) para los repositorios que vendrán y valida que la conexión, los índices y el multi-tenant funcionan correctamente.

**Independent Test**: Se puede probar en un test de integración creando un shell, leyéndolo por ID, actualizando un campo y borrándolo, verificando cada respuesta contra el estado esperado en MongoDB.

**Acceptance Scenarios**:

1. **Given** datos válidos de un AAS shell (con `tenantId` y `globalAssetId`), **When** se llama al método Create del repositorio, **Then** el documento se persiste en MongoDB y se retorna el shell creado con su ID asignado.
2. **Given** un shell existente con un AAS-ID conocido, **When** se llama a GetByID del repositorio, **Then** se retorna el documento completo o `nil` si no existe.
3. **Given** un shell existente, **When** se llama a Update, **Then** solo los campos provistos se modifican; `updatedAt` se actualiza automáticamente.
4. **Given** un shell existente, **When** se llama a Delete, **Then** el documento se elimina de MongoDB.
5. **Given** dos tenants distintos con shells cuyos `globalAssetId` coinciden, **When** cada uno consulta su shell, **Then** cada tenant solo ve sus propios documentos (aislamiento multi-tenant por `tenantId`).

---

### User Story 3 — CRUD de Submodelos (Priority: P2)

Un desarrollador puede persistir, leer, actualizar y eliminar submodelos AAS de una máquina usando el repositorio de submodelos, vinculándolos al shell padre mediante `shellId`.

**Why this priority**: Los submodelos son la unidad de dato más importante del AAS en producción (el frontend y sistemas externos consultan submodelos, no shells completos), pero requieren que el CRUD de shells (P1) esté operativo primero.

**Independent Test**: Se puede probar en un test de integración independiente del repositorio de shells, usando un `shellId` de prueba sin requerir que el shell padre exista en la misma pasada.

**Acceptance Scenarios**:

1. **Given** un `shellId` y datos válidos de submodelo, **When** se llama a Create del repositorio de submodelos, **Then** el documento se persiste con el vínculo correcto al shell padre.
2. **Given** un `shellId` conocido, **When** se llama a ListByShell, **Then** el repositorio retorna todos los submodelos de ese shell (y solo los de ese shell).
3. **Given** un submodelo existente, **When** se llama a UpsertElement para actualizar el valor de un `SubmodelElement`, **Then** solo el elemento indicado cambia; el resto del submodelo permanece intacto.
4. **Given** un submodelo inexistente, **When** se llama a GetByID, **Then** el repositorio retorna `nil` sin error de sistema.

---

### User Story 4 — Healthcheck de MongoDB (Priority: P3)

El endpoint de health del servidor incluye el estado de la conexión a MongoDB, permitiendo monitorear su disponibilidad junto con PostgreSQL y Redis.

**Why this priority**: Operacionalmente importante pero no bloquea el uso de los repositorios. Se puede agregar una vez que la conexión esté establecida (P1 completo).

**Independent Test**: Se puede probar llamando al endpoint de health con MongoDB arriba (OK) y con MongoDB caído o inalcanzable (DEGRADED), verificando que la respuesta refleja el estado real.

**Acceptance Scenarios**:

1. **Given** MongoDB está accesible, **When** se consulta el endpoint de health, **Then** la sección `mongo` reporta `"status": "ok"`.
2. **Given** MongoDB no está accesible, **When** se consulta el endpoint de health, **Then** la sección `mongo` reporta `"status": "degraded"` sin que el endpoint retorne 5xx (falla open).

---

### Edge Cases

- ¿Qué ocurre si MongoDB está caído pero PostgreSQL y Redis están operativos? → El servidor debe arrancar y servir el resto de los endpoints normalmente; solo las rutas que usen MongoDB deben retornar error cuando se integren.
- ¿Qué ocurre si se intenta crear un AAS shell con `globalAssetId` duplicado dentro del mismo tenant? → El repositorio debe retornar un error de conflicto distinguible (no un error genérico).
- ¿Qué ocurre si el documento a actualizar no existe? → El repositorio debe retornar un error `ErrNotFound` consistente con el patrón de errores de dominio ya usado en el proyecto.
- ¿Qué ocurre si `tenantId` no se incluye en una consulta? → El repositorio debe exigir `tenantId` como parámetro obligatorio y rechazar queries sin él.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE establecer una única conexión a MongoDB al arrancar el servidor, reutilizable por todos los repositorios.
- **FR-002**: El sistema DEBE cerrar la conexión a MongoDB durante el shutdown graceful.
- **FR-003**: MongoDB es **opcional**. Si `MONGO_URI` no está configurado, el servidor DEBE arrancar normalmente emitiendo un log de advertencia (`WARN mongo disabled`); los repositorios MongoDB retornan error si se invocan sin conexión. El servidor no falla al arrancar por ausencia de MongoDB.
- **FR-004**: El repositorio de AAS shells DEBE exponer operaciones: Create, GetByID, Update, Delete, ListByTenant. `ListByTenant` acepta parámetros de paginación `limit` y `offset` (límite por defecto: 100 resultados).
- **FR-005**: El repositorio de submodelos DEBE exponer operaciones: Create, GetByID, ListByShell, UpsertElement, Delete. `ListByShell` acepta parámetros de paginación `limit` y `offset` (límite por defecto: 100 resultados).
- **FR-006**: Todos los repositorios DEBEN filtrar por `tenantId` en cada operación de lectura y escritura; ninguna query puede ser cross-tenant.
- **FR-007**: Los repositorios DEBEN crear los índices requeridos (`tenantId + globalAssetId` único para shells; `tenantId + shellId + idShort` único para submodelos) al inicializarse, de forma idempotente.
- **FR-008**: El sistema DEBE exponer el estado de la conexión MongoDB en el healthcheck existente del servidor.
- **FR-009**: Los errores del driver MongoDB DEBEN traducirse a errores de dominio (`ErrNotFound`, `ErrConflict`) antes de salir del repositorio, sin exponer tipos del driver en capas superiores.
- **FR-010**: La configuración de conexión (`MONGO_URI`, `MONGO_DB`) DEBE integrarse en la struct `Config` existente del proyecto.
- **FR-011**: Las operaciones de repositorio MongoDB DEBEN emitir métricas Prometheus: histograma de latencia y contador de errores por operación y colección, siguiendo el patrón de `internal/telemetry/`.

### Key Entities

- **AssetAdministrationShell**: Gemelo digital de una máquina. Atributos clave: identificador propio del servidor, `tenantId`, `globalAssetId` (vínculo con `EdgeDevice.MachineID`), tipo de asset, referencias a submodelos, timestamps de creación y modificación.
- **Submodel**: Aspecto específico del AAS (TechnicalData, OperationalData, EdgeInfrastructure, Maintenance). Atributos clave: identificador propio, `tenantId`, `shellId` (vínculo al AAS padre), nombre corto (`idShort`), referencia semántica opcional, colección de elementos, timestamp de modificación.
- **SubmodelElement**: Unidad de dato dentro de un submodelo. Puede ser un valor escalar (Property), una agrupación (SubmodelElementCollection) u otros tipos del estándar AAS v3. Se persiste embebido dentro del documento del submodelo padre.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un desarrollador puede crear un nuevo repositorio MongoDB en el proyecto escribiendo solo la lógica de negocio específica, sin repetir código de conexión o manejo de errores del driver.
- **SC-002**: Los tests de integración del CRUD de shells y submodelos pasan de forma reproducible contra una instancia MongoDB local (docker-compose), con tiempo de ejecución total menor a 30 segundos.
- **SC-003**: El servidor arranca sin errores en menos de 5 segundos tanto con MongoDB configurado como sin él; en este último caso emite exactamente un log de advertencia visible.
- **SC-004**: Ninguna operación de repositorio puede retornar datos de un tenant distinto al provisto como argumento, validable mediante test de aislamiento con dos tenants de prueba.
- **SC-005**: El endpoint de healthcheck refleja el estado real de MongoDB dentro de los primeros 2 intentos de consulta (menos de 2 segundos) después de que el servicio MongoDB cambia de estado.

---

## Clarifications

### Session 2026-04-02

- Q: ¿MongoDB es obligatorio al arrancar o opcional con degraded mode? → A: Opcional con warning — el servidor arranca sin MongoDB; log de advertencia si `MONGO_URI` no está; repositorios retornan error si se invocan sin conexión.
- Q: ¿Los repositorios MongoDB deben emitir métricas Prometheus? → A: Sí — histograma de latencia + contador de errores por operación y colección, siguiendo el patrón de `internal/telemetry/`.
- Q: ¿Los listados (ListByTenant, ListByShell) deben soportar paginación desde el inicio? → A: Sí — limit/offset simple; límite por defecto 100 resultados; firma de interfaz incluye ambos parámetros.
- Q: ¿La instancia MongoDB de desarrollo/CI debe correr con autenticación? → A: No — sin auth en local/CI; `MONGO_URI` local es `mongodb://localhost:27017`; credenciales solo en entornos reales vía URI.
- Q: ¿Cómo se aíslan los tests de integración de repositorios MongoDB? → A: Base de datos efímera por test — cada test crea una DB con nombre único (`test_<uuid>`) y la borra al terminar vía `t.Cleanup`.

---

## Assumptions

- MongoDB se levanta como servicio adicional en `docker-compose.yml` (imagen `mongo:7`). El equipo acepta la nueva dependencia de infraestructura.
- La conexión es a una instancia MongoDB standalone (Community Edition). No se requiere replica set ni Atlas en esta fase.
- Los repositorios de esta feature no se exponen en ningún endpoint HTTP todavía; la integración con rutas viene en features posteriores (AAS Server, Consumer events).
- El patrón de repositorios sigue la misma interfaz de Go ya usada en `internal/repo/pg/` (interfaz en dominio o usecase, implementación en infra).
- La instancia MongoDB de desarrollo y CI corre sin autenticación (sin `MONGO_INITDB_ROOT_USERNAME/PASSWORD`); el `MONGO_URI` local es `mongodb://localhost:27017`. En entornos reales las credenciales viajan dentro del URI; no se agrega gestión de secretos separada en esta fase.
- Esta feature no incluye TimescaleDB ni InfluxDB; esas capas se especifican por separado cuando el volumen de telemetría lo justifique.
- Los tests de integración de repositorios MongoDB usan una base de datos efímera por test (nombre único `test_<uuid>`, borrada en `t.Cleanup`); no comparten estado entre tests.

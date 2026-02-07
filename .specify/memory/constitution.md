# Constitución de la API Embolsadora

## Principios Fundamentales

### I. Arquitectura Hexagonal Limpia con Separación de Superficies

La API Embolsadora implementa un monolito modular en Go con principios de Clean Architecture / Hexagonal. Dos superficies HTTP distintas con responsabilidades aisladas:

- **Superficie ABM** (`/api/v1/**`): Administración de usuarios, máquinas y tenants
- **Superficie de Ingesta** (`/api/v1/consumers/**`): Ingesta de telemetría y eventos desde máquinas

Cada superficie tiene su propio esquema de autenticación, stack de middleware y contrato de API. Flujo de dependencias: `transport → app → domain ← repo | security | telemetry | platform`. Sin lógica de negocio entre superficies; la funcionalidad compartida pertenece a los paquetes `domain/` o `platform/`.

**Justificación**: Permite escalado independiente, posturas de seguridad distintas, y extracción futura de servicios sin romper contratos (según ADR-001).

### II. Aislamiento Prioritario en Seguridad (NO NEGOCIABLE)

Cada superficie impone autenticación apropiada a su modelo de amenaza:

- **Superficie ABM**: Tokens JWT bearer + RBAC requerido; CORS habilitado; todas las mutaciones requieren validación de roles
- **Superficie de Ingesta**: Autenticación por API Key por tenant; rate limiting mediante token bucket Redis; middleware de idempotencia obligatorio para mutaciones de eventos

El aislamiento de tenants es obligatorio: todas las consultas de base de datos y firmas de repositorio incluyen `context.Context` y verifican `tenant_id` mediante `platform.TenantID`. Ninguna operación puede cruzar límites de tenants.

**Justificación**: Previene acceso no autorizado, abuso de rate limiting y fuga de datos entre tenants.

### III. Observabilidad e Instrumentación (NO NEGOCIABLE)

Cada feature debe incluir observabilidad estructurada:

- **Logging**: Logger estructurado Zap; sin datos sensibles en logs; niveles apropiados (debug/info/warn/error)
- **Métricas**: Contadores/histogramas Prometheus en `/metrics`; instrumentación de latencia de request, tasas de error, conteos específicos del dominio
- **Tracing**: Propagación de contexto de traza compatible con OpenTelemetry (pendiente decisión de implementación)

La debuggabilidad en producción depende de observabilidad; es no-opcional.

**Justificación**: Permite troubleshooting rápido y optimización de performance basada en datos.

### IV. Testing de Integración Dirigido por Contrato

Tests de integración son requeridos para cualquier punto de integración nuevo:

- Cambios de schema de base de datos deben incluir tests de migración contra contenedores Postgres de test
- Consumidores de eventos deben verificar deserialización correcta y semántica de reintentos
- Integraciones con APIs externas deben validar contratos request/response contra spec OpenAPI

Tests unitarios solos son insuficientes; tests de contrato detectan bugs de integración antes de producción.

**Justificación**: Previene violaciones silenciosas de contrato y reduce incidentes en producción.

### V. Versionado Semántico y Compatibilidad Hacia Atrás

Todos los APIs siguen versionado semántico (MAJOR.MINOR.PATCH):

- **MAJOR**: Cambios de contrato rotos (esquema de auth, campos requeridos, remoción de endpoint) — requiere período de deprecación y guía de migración
- **MINOR**: Nueva funcionalidad, nuevos campos opcionales, nuevos endpoints — backward compatible
- **PATCH**: Correcciones de bug, mejoras no-funcionales — backward compatible

Endpoints deprecados deben soportar clientes por ≥2 versiones menores antes de remoción. Remociones documentadas en ADR.

**Justificación**: Evolución segura de clientes; previene ruptura sorpresa.

## Definiciones de Arquitectura y Superficies

### Superficie ABM (`/api/v1`)

Operaciones administrativas: gestión de usuarios, máquinas y tenants.

- **Autenticación**: JWT (token Bearer) + middleware RBAC
- **CORS**: Habilitado para clientes navegador
- **Idempotencia**: No requerida (operaciones de escritura admin-iniciadas)
- **Rate Limiting**: Cuota por usuario (implementación pendiente)

**Endpoints** (estado stub):

- `GET/POST /users`
- `GET/POST /machines`
- `GET/POST /tenants`

### Superficie de Ingesta (`/api/v1/consumers`)

Ingesta de alto volumen asincrónica de eventos y heartbeats desde máquinas desplegadas.

- **Autenticación**: API Key (header-based, por-tenant)
- **Rate Limiting**: Token bucket por tenant (respaldado en Redis)
- **Idempotencia**: Requerida; claves de idempotencia almacenadas en Redis con TTL

**Endpoints** (estado stub):

- `POST /events`
- `POST /heartbeat`

## Stack de Tecnología y Estándares de Plataforma

- **Lenguaje**: Go 1.24+; gestión de módulos vía `go.mod`; pin versiones mayores
- **BD Principal**: PostgreSQL (schema vía migraciones en `migrations/`)
- **Cache/Sesiones**: Redis (almacén de idempotencia, tokens de rate limit, sesiones)
- **Framework HTTP**: Gin con wiring custom de middleware
- **Logging**: Zap (estructurado, JSON en producción)
- **Métricas**: Prometheus; scrape en `/metrics`
- **Telemetría**: Tracing pendiente análisis de performance
- **Deployment**: Docker + Docker Compose (dev); Cloud Run o ECS (producción)
- **Testing**: Paquete `testing` Go; contenedores Docker para tests de integración

## Proceso de Desarrollo y Revisión

### Compuertas de Revisión de Código (OBLIGATORIO)

Cada PR debe satisfacer:

1. **Arquitectura**: Mantiene capas hexagonales; sin lógica de negocio entre superficies
2. **Seguridad**: Aislamiento de tenants preservado; sin logging de credenciales; rate limiting aplicado
3. **Observabilidad**: Logs estructurados agregados; endpoints nuevos instrumentados; métricas registradas
4. **Contrato**: Spec OpenAPI actualizado si endpoints cambiaron; cambios rotos documentados en ADR
5. **Tests**: Integraciones externas cubiertas por tests de contrato/integración; coverage ≥70% para código nuevo

### Requerimiento de ADR

Nuevas superficies, cambios de auth, migraciones de schema, integraciones externas y optimizaciones de performance requieren ADRs en `docs/adr/`.

### Versionado en Release

1. Actualizar constante de versión
2. Tagear commit como `v{MAJOR}.{MINOR}.{PATCH}`
3. Actualizar `docs/openapi.yaml`
4. Para cambios MAJOR: documentar en `MIGRATION_v*.md`

## Gobernanza

Esta Constitución supersede toda otra guía a menos que sea explícitamente enmendada.

**Procedimiento de Enmienda**:

1. Abrir issue o ADR proponiendo enmienda
2. Discusión de equipo (sincrónica o asincrónica)
3. Documentar justificación en ADR
4. Actualizar este archivo
5. Tagear release: `docs: amend constitution to v{VERSION}`

**Revisión de Cumplimiento**: Checkpoints trimestrales revisan PRs contra esta Constitución. Violaciones requieren resolución antes de merge.

**Versión**: 1.1.0 | **Ratificada**: 2026-02-07 | **Última Enmienda**: 2026-02-07

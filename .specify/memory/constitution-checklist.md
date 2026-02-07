# Checklist de Cumplimiento de Constitución

Para verificar que nuevas PRs cumplen con la Constitución de Embolsadora API v1.1.0.

## Antes de Abrir una PR

### ✓ Arquitectura Hexagonal Limpia
- [ ] El código respeta capas: `transport → app → domain ← repo | security | telemetry | platform`
- [ ] No hay lógica de negocio compartida entre superficies ABM e Ingesta
- [ ] Los nuevos paquetes respetan la estructura: `internal/{api,consumers,domain,repo,security,telemetry,platform,config}`

### ✓ Aislamiento de Seguridad
- [ ] Superficie ABM usa JWT + RBAC (autorización verificada en handlers/middleware)
- [ ] Superficie de Ingesta usa API Key por tenant (no hay cross-tenant access)
- [ ] Todos los repository calls incluyen `context.Context` para extracción de `tenant_id`
- [ ] No hay credenciales (passwords, tokens, API keys) en logs
- [ ] Rate limiting aplicado a Ingesta (Redis token bucket)

### ✓ Observabilidad
- [ ] Se agregaron logs estructurados con Zap en niveles apropiados (debug/info/warn/error)
- [ ] Se registraron nuevas métricas Prometheus si el feature incrementa actividad observable
- [ ] Endpoints nuevos aparecen en `/metrics` con nombre descriptivo
- [ ] No hay datos sensibles en logs de ningún nivel

### ✓ Testing de Integración Conducido por Contrato
- [ ] Si es cambio de schema: tests de migración contra contenedor Postgres de test
- [ ] Si es nuevo consumer: verificación de deserialización de eventos y retry semantics
- [ ] Si es integración externa: validación de contrato request/response contra OpenAPI spec
- [ ] Coverage de tests de integración ≥70% para código nuevo

### ✓ Versionado Semántico & Compatibilidad Hacia Atrás
- [ ] Se identificó el tipo de cambio (MAJOR/MINOR/PATCH)
- [ ] Si MAJOR: documentación de período de deprecación en ADR
- [ ] Endpoints removidos: documentados y deprecados por ≥2 versiones menores
- [ ] OpenAPI spec actualizado si endpoints cambiaron

## Durante Code Review

### ✓ Compuertas de Revisión Obligatorias

1. **Arquitectura**: 
   - [ ] ¿Mantiene separación de superficies?
   - [ ] ¿No hay violaciones de capas?

2. **Seguridad**: 
   - [ ] ¿Aislamiento de tenants verificado?
   - [ ] ¿Sin credentials en logs?
   - [ ] ¿Rate limiting presente donde corresponde?

3. **Observabilidad**: 
   - [ ] ¿Logs estructurados en nivel correcto?
   - [ ] ¿Nuevas métricas registradas?

4. **Contrato**: 
   - [ ] ¿OpenAPI spec actualizado?
   - [ ] ¿Cambios rotos documentados en ADR?

5. **Tests**: 
   - [ ] ¿Tests de integración cobertura ≥70%?
   - [ ] ¿Migraciones de schema testeadas?

## Si Hay Violaciones

### Requiere ADR

- [ ] Cambio de auth scheme
- [ ] Nueva superficie
- [ ] Migración de schema
- [ ] Integración externa
- [ ] Optimización de performance significativa

**Acción**: Abrir ADR en `docs/adr/ADR-00X.md` documentando decisión y justificación.

### Requiere Enmienda de Constitución

- [ ] Cambio de requisitos de observabilidad
- [ ] Eliminación de principio
- [ ] Cambio de compuertas de revisión
- [ ] Cambio de política de versionado

**Acción**: Levantar issue proponiendo enmienda. Discutir con equipo. Actualizar constitución. Tag release: `docs: amend constitution to vX.Y.Z`.

## Quick Reference

**5 Principios Fundamentales**:
1. Arquitectura Hexagonal con Separación de Superficies
2. Aislamiento Prioritario en Seguridad (NO NEGOCIABLE)
3. Observabilidad e Instrumentación (NO NEGOCIABLE)
4. Testing de Integración Conducido por Contrato
5. Versionado Semántico & Compatibilidad Hacia Atrás

**Dos Superficies**:
- **ABM** (`/api/v1/**`): JWT + RBAC
- **Ingesta** (`/api/v1/consumers/**`): API Key + Rate Limit + Idempotencia

**Stack**: Go 1.24+ | PostgreSQL | Redis | Gin | Zap | Prometheus | OpenTelemetry (pending)

**Ciclo de Revisión**: 5 compuertas obligatorias (Arquitectura, Seguridad, Observabilidad, Contrato, Tests)

---

**Ratificado**: 2026-02-07 | **Versión**: 1.1.0 | **Última actualización**: 2026-02-07

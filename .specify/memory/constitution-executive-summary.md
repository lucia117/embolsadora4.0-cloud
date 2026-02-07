# Resumen Ejecutivo: Constitución de Embolsadora API

**Documento**: Constitución de Embolsadora API v1.1.0  
**Ratificado**: 2026-02-07  
**Equipo**: Desarrollo de Embolsadora

## ¿Qué es?

La Constitución es el documento de gobernanza que define los **principios no-negociables**, **compuertas de revisión**, y **estándares técnicos** para el desarrollo del Embolsadora API.

## Los 5 Principios

| # | Principio | Descripción Breve | Por Qué |
|---|-----------|-------------------|--------|
| **I** | Arquitectura Hexagonal Limpia | Separación clara de capas y superficies (ABM e Ingesta) | Facilita escalado independiente, futura extracción de servicios |
| **II** | Aislamiento Prioritario en Seguridad 🔐 | JWT+RBAC en ABM; API Key+RateLimit en Ingesta; sin cross-tenant access | Previene unauthorized access, rate abuse, multi-tenant data leakage |
| **III** | Observabilidad e Instrumentación 📊 | Logs estructurados (Zap), métricas (Prometheus), tracing (OTel-ready) | Troubleshooting rápido, data-driven optimization en producción |
| **IV** | Testing de Integración Dirigido por Contrato | Tests de migración, deserialización, contratos OpenAPI | Detecta bugs pre-producción, previene contract violations silenciosas |
| **V** | Versionado Semántico & Backward Compatibility | MAJOR/MINOR/PATCH; período de deprecación ≥2 versiones | Safe client evolution, no surprise breakage |

## Las Dos Superficies

```
┌─────────────────────────────────────────────────────┐
│              Embolsadora API (Go 1.24+)             │
├──────────────────────┬──────────────────────────────┤
│  ABM Surface         │  Ingestion Surface           │
│  /api/v1/**          │  /api/v1/consumers/**        │
├──────────────────────┼──────────────────────────────┤
│ Auth: JWT + RBAC     │ Auth: API Key per-tenant     │
│ CORS: Enabled        │ Rate Limit: Redis token-buf  │
│ Users, Machines,     │ Idempotency: Required        │
│ Tenants (admin)      │ Events, Heartbeats (ingest)  │
├──────────────────────┼──────────────────────────────┤
│ DB: PostgreSQL       │ Cache: Redis                 │
│ Logging: Zap         │ Metrics: Prometheus          │
│ Framework: Gin       │ Deploy: Docker + Cloud Run   │
└──────────────────────┴──────────────────────────────┘
```

## Compuertas de Revisión de Código (5 OBLIGATORIAS)

Cada PR **MUST** pasar estas 5 compuertas antes de merge:

1. **Arquitectura** 
   - ¿Mantiene capas hexagonales?
   - ¿Sin lógica compartida entre superficies?

2. **Seguridad** 
   - ¿Aislamiento de tenants preservado?
   - ¿Sin credentials en logs?

3. **Observabilidad** 
   - ¿Logs estructurados agregados?
   - ¿Nuevas métricas registradas?

4. **Contrato** 
   - ¿OpenAPI spec actualizado?
   - ¿Cambios rotos documentados?

5. **Tests** 
   - ¿Coverage ≥70% código nuevo?
   - ¿Integraciones testeadas?

## Stack de Tecnología

| Componente | Stack |
|-----------|-------|
| **Lenguaje** | Go 1.24+ |
| **DB** | PostgreSQL (migraciones en `migrations/`) |
| **Cache/Queue** | Redis (idempotencia, rate limit, sesiones) |
| **HTTP Framework** | Gin con middleware custom |
| **Logging** | Zap (structured, JSON en prod) |
| **Métricas** | Prometheus (`/metrics`) |
| **Telemetry** | OpenTelemetry (pending implementation decision) |
| **Testing** | Go `testing` + Docker containers |
| **Deployment** | Docker + Docker Compose (dev); Cloud Run/ECS (prod) |

## Gobernanza: Cuándo Crear un ADR

**Estos cambios requieren Architecture Decision Record**:
- ✍️ Nueva superficie HTTP
- ✍️ Cambio de auth scheme (ej., cambiar de API Key a OAuth)
- ✍️ Migración de schema significativa
- ✍️ Integración con servicio externo
- ✍️ Optimización de performance que afecte SLA

**No necesitan ADR**: bug fixes, refactoring, nuevos endpoints siguiendo patrón existente.

## Versioning Policy

```
MAJOR.MINOR.PATCH

MAJOR → Breaking change     (v1.0.0 → v2.0.0)
        ⚠️ Requiere: deprecation period, migration guide, ADR

MINOR → New feature, compatible   (v1.0.0 → v1.1.0)
        ✅ Backward compatible

PATCH → Bug fix             (v1.1.0 → v1.1.1)
        ✅ Backward compatible
```

**Deprecation**: ≥2 versiones menores antes de remover.

## Quick Links

- 📄 **Constitución Completa**: [.specify/memory/constitution.md](constitution.md)
- ✅ **Checklist de Compliance**: [.specify/memory/constitution-checklist.md](constitution-checklist.md)
- 🏗️ **ADRs Existentes**: [docs/adr/](../../../docs/adr/)
- 📋 **OpenAPI Spec**: [docs/openapi.yaml](../../../docs/openapi.yaml)

---

**Próximas Acciones para el Equipo**:

1. ✅ **Leer** esta Constitución (15 min)
2. ✅ **Guardar** el [checklist de compliance](constitution-checklist.md) en favoritos
3. ⏳ **Aplicar** las 5 compuertas en PRs a partir de hoy
4. ⏳ **Documentar** nuevas decisiones arquitectónicas con ADRs
5. ⏳ **Revisar trimestralmente** cumplimiento contra esta Constitución

**¿Preguntas?** Abre un issue proponiendo enmienda. Discutamos como equipo.

---

*Versión 1.1.0 • Ratificada: 2026-02-07 • Localización: Español*

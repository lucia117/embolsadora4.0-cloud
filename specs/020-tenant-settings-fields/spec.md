---
id: 020
title: "Campos de contacto y localización del tenant"
tier: feature
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-07-23
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: []
prs: [48]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 020 — Campos de contacto y localización del tenant

> **Procedencia.** `origen: original` viene del diseño aprobado del 2026-07-23, en
> [`design.md`](design.md), citado por sección. `origen: derivado` se reconstruyó del
> código. **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

El formulario `/settings` del frontend ya enviaba siete campos del tenant (`contactEmail`,
`companyWebsite`, `locale`, `timezone`, `dateFormat`, `timeFormat` y `currency`) que el
backend ignoraba. No había nada en `internal/` que los persistiera: no era un bug, era
funcionalidad nunca implementada (§Contexto).

## Alcance

**Entra:** columnas, dominio, repositorio, DTOs de request y respuesta, validación y
defaults al crear un tenant.

**No entra** (§Non-goals):
- Cambios en el frontend.
- Consolidar los 4 `TenantResponse` duplicados.
- Generar `tenants.json` automáticamente.
- Editar el `subdomain`.

## Requisitos

- **RF-001** `origen: original` (§1, decisión 1) — Los siete campos DEBEN guardarse como
  columnas planas en `tenants`, no en JSONB, con `NOT NULL DEFAULT` iguales a los defaults
  del frontend. **Derivado:** la migración es la `000006`, no la `000005` que decía el
  diseño, porque ese número lo tomó la 018.
  → `migrations/000006_add_tenant_settings.up.sql` (7 columnas).
- **RF-002** `origen: original` (§1, decisión 4) — `locale`, `timezone`, `dateFormat`,
  `timeFormat` y `currency` DEBEN validarse contra un catálogo fijo, igual al del
  frontend, tanto con `CHECK` en la base como en el handler. Un valor fuera de catálogo
  responde 400 `"<campo> inválido"`.
  → `CHECK` en `000006`; `validLocales` y siguientes en
  `internal/api/handler/tenants/update_tenant/update_tenant.go`.
- **RF-003** `origen: original` (§1, decisión 2) — En el dominio, los siete campos se
  agrupan en `domain.TenantSettings`.
- **RF-004** `origen: original` (§Nuance) — En el JSON, `contactEmail` y `companyWebsite`
  quedan en la raíz del request y la respuesta; los cinco de localización van en
  `settings`.
- **RF-005** `origen: original` (§3, "Creación de tenants") — Crear un tenant DEBE aplicar
  los defaults de localización (`locale: es-AR`, etc.).
  → `create_tenant/models/request.go` (`Parse`).
- **RF-006** `origen: original` (§4) — Los 4 DTOs de respuesta de tenants DEBEN incluir
  los campos nuevos.
  → `models/response.go` de `create_tenant`, `get_tenant`, `get_all_tenants` y
  `update_tenant`.

## Criterios de éxito

Vienen de la sección Testing del diseño.

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | Un `PATCH` con `settings.locale = "xx-XX"` responde 400 | `update_tenant_test.go` (`TestUpdateTenantHandler_InvalidSettings`) |
| **CE-002** | Crear un tenant deja los defaults de localización | `create_tenant/models/request_test.go` (`TestParse_SetsSettingsDefaults`) |
| **CE-003** | Las respuestas incluyen los campos nuevos | `TestFromDomain_IncludesSettings` en los `response_test.go` de cada DTO |
| **CE-004** | La migración crea las 7 columnas con sus defaults y `CHECK` | verificación manual de la migración (§5); sin registro |

## Decisiones

| Decisión | Alternativa descartada | Por qué |
|---|---|---|
| Columnas planas | JSONB | Sigue el patrón existente de `Theme` y `Address` |
| Catálogo fijo validado en la base y en el backend | Texto libre | Mismo catálogo en frontend y backend; la base garantiza aunque falle la app |
| Replicar el cambio en los 4 DTOs | Consolidarlos | No mezclar un refactor con la feature |

## Riesgos y preguntas abiertas

- **Catálogo duplicado** en tres lugares (`CHECK` en la base, handler Go y frontend): se
  desincroniza con facilidad.
- Los 4 `TenantResponse` duplicados siguen pendientes de consolidar.

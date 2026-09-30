---
id: 018
title: "Seed de roles y permisos: traducción y asignación de permisos a roles"
tier: change
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-07-21
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: []
prs: [46]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 018 — Seed de roles y permisos: traducción y asignación de permisos a roles

> **Procedencia.** `origen: original` viene del diseño aprobado del 2026-07-21, en
> [`design.md`](design.md). `origen: derivado` se reconstruyó de las migraciones
> posteriores. **Verificado contra `develop` el 2026-09-29.** Es `tier: change`: corrige
> datos sembrados, sin funcionalidad nueva.

## Contexto y problema

Las páginas `/roles` y `/permissions` del frontend mostraban los 6 roles del sistema
**con cero permisos** y el catálogo de permisos en inglés. Eran problemas de datos, no del
frontend: la migración `000002` sembraba `roles.permissions` vacío y los nombres del
catálogo en inglés (§Problem).

## Alcance

**Entra:**

- Traducir al español nombre y descripción de los 17 permisos del sistema.
- Cargar `roles.permissions` de los 6 roles del sistema.

**No entra** (§Non-goals):

- La autorización del backend, que entonces era un mapa fijo en Go.
- Cambios en el frontend.
- Renombrar ids del catálogo.

## Requisitos

- **RF-001** `origen: original` (§Design 1) — Los cambios DEBEN ir en una migración nueva
  con `UPDATE` idempotentes y su `down`, no editando `000002`: esa migración ya estaba
  aplicada en producción y sus `INSERT … ON CONFLICT DO NOTHING` no cambiarían nada.
  → `migrations/000005_translate_permissions_and_seed_role_permissions.{up,down}.sql`.
- **RF-002** `origen: original` (§Design 3) — Nombre y descripción de los 17 permisos del
  sistema DEBEN quedar en español.
  → `000005`.
- **RF-003** `origen: original` (§Design 2) — `roles.permissions` de los 6 roles del
  sistema DEBE cargarse según la tabla de mapeo del diseño.
  → `000005`.
- **RF-004** `origen: derivado` — **Reemplazado después.** El mapeo de RF-003 ya no es el
  estado vigente. Lo modificaron:
  - `000008`, que eliminó `perm_all_tenants`;
  - `000009`, que dio `perm_logs_view` a `admin`;
  - `000011`, que resembró los 7 roles con el catálogo fino y reemplazó `perm_users` y
    `perm_tenants`;
  - `000013` y `000015`.

  El estado vigente está verificado por `TestSeedPermissionsMatchDesign` (ver
  [011](../011-permissions-management/spec.md)).

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | Después de migrar, los roles del sistema tienen permisos y el catálogo está en español | `000005` aplicada; estado actual verificado por `internal/repo/pg/roles/seed_test.go` |
| **CE-002** | La migración es reversible | `000005_…down.sql` |

## Decisiones

| Decisión | Alternativa descartada | Por qué |
|---|---|---|
| Migración nueva con `UPDATE` | Editar `000002` | `000002` ya estaba aplicada y usa `ON CONFLICT DO NOTHING` |
| Mapeo de `super_admin`, `tenant_manager`, `admin` y `operario` desde el fallback del frontend; `cliente_*` desde el mapa Go | Inventar un mapeo nuevo | Mantener coherencia con lo que ya mostraba la UI y con lo que se autorizaba |

## Riesgos y preguntas abiertas

- Ninguno vigente. La premisa del diseño ("la autorización del backend no se toca") dejó de
  valer con [011](../011-permissions-management/spec.md): desde el PR #62, lo que hay en
  `roles.permissions` **sí** autoriza.

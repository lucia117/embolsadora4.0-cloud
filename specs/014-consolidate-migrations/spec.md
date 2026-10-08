---
id: 014
title: "Consolidación de migraciones"
tier: change
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-05-08
repos: [embolsadora4.0-cloud]
origin: speckit
issues: []
prs: [36]
adrs: [CLOUD-ADR-014]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 014 — Consolidación de migraciones

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-05-08, en
> [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. La decisión está en
> [CLOUD-ADR-014](../../docs/adr/CLOUD-ADR-014-consolidar-migraciones.md). **Verificado
> contra `develop` el 2026-09-29.**

## Contexto y problema

El repo acumulaba 20 migraciones. Dos compartían el prefijo `000019`, y la `000007` hacía
un `CREATE TABLE IF NOT EXISTS` sobre una tabla existente. La cadena no podía aplicarse
sobre una base vacía, y hacía falta un primer deploy reproducible. La base de producción
no tenía datos que preservar, así que se colapsó el historial.

La spec apuntaba a Koyeb; el deploy real terminó en Google Cloud Run con Postgres en
Supabase (ver `docs/operations.md`). La plataforma no cambia la decisión.

## Alcance

**Entra:**

- Esquema inicial único, seeds esenciales y seeds opcionales fuera del flujo.
- Documentación del procedimiento.

**No entra:** cambios en el código de la aplicación.

## Requisitos

- **RF-001** `origen: original` (FR-001, FR-002) — DEBE existir una migración inicial que
  deje el esquema completo sobre una base vacía, con su `down` que lo revierta sin
  residuos.
  → `migrations/000001_initial_schema.{up,down}.sql`.
- **RF-003** `origen: original` (FR-003) — Los seeds esenciales DEBEN estar separados del
  esquema y ser idempotentes: catálogo de permisos, roles del sistema y tenant MRG.
  → `migrations/000002_seed_essentials.up.sql` (`ON CONFLICT DO NOTHING`).
- **RF-004** `origen: original` (FR-004) — Los seeds de prueba (tenants de ciudades y sus
  usuarios) DEBEN poder ejecutarse a mano en entornos no productivos y nunca aplicarse
  solos.
  → `scripts/seed_test_city_tenants.sql`.
- **RF-005** `origen: original` (FR-005) — No DEBE haber prefijos duplicados ni archivos
  huérfanos.
  → `migrations/` hoy: `000001` a `000015`, uno por número.
- **RF-006** `origen: original` (FR-006), **plataforma cambiada** — La documentación DEBE
  describir el procedimiento para producción. La spec decía Koyeb; hoy está en
  `migrations/README.md` ("Aplicar en producción") y `docs/operations.md`, para Supabase y
  Cloud Run.
- **RF-007** `origen: original` (FR-007) — Las credenciales del admin de plataforma NO
  DEBEN estar en el repo: el admin se crea en Supabase Auth, se auto-provisiona en el
  primer login y se le asigna `super_admin` por SQL.
  → `migrations/README.md` ("Activación del admin MRG").
- **RF-008** `origen: original` (FR-008) — La consolidación NO DEBE requerir cambios de
  código.

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-001** | Esquema y seeds esenciales se aplican sobre una base vacía sin errores | original (SC-001) | CLOUD-ADR-014, sección Verification (`migrate up` en 3,4 s) |
| **CE-003** | Los tests existentes pasan sin cambios de código | original (SC-003) | CLOUD-ADR-014 ("cero archivos `*.go` modificados") |
| **CE-004** | La cantidad de archivos de migración baja al menos un 80 % | original (SC-004) | de 40 a 4 archivos en el momento del cambio |
| **CE-006** | Aplicar solo los pasos esenciales no crea tenants ni usuarios de prueba | original (SC-006) | seeds de prueba fuera de `migrations/` |

SC-002 y SC-005 medían tiempos de un operador sobre Koyeb: no aplican a la plataforma
actual y no se trasladan.

## Riesgos y preguntas abiertas

- La historia granular previa solo queda en `git log`.
- El parche local a la `000007` que se usó para generar el dump no quedó commiteado
  (CLOUD-ADR-014, consecuencias negativas).

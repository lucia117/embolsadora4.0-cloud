---
title: Plan de aplicación del harness documental — embolsadora4.0-cloud
status: en-curso
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Plan de aplicación del harness documental

Referencias: `harness-documental.md` (spec del harness, 2026-09-29) y
[`auditoria-2026-09-29.md`](auditoria-2026-09-29.md). Un PR por fase.

## Fase 1 — Layout base e higiene (rama `docs/harness-documental`)

- [x] `AGENTS.md` como fuente única de reglas, con la sección "Specs" (§8); `CLAUDE.md`
      reducido a adaptador.
- [x] `docs/architecture.md` extraído de `CLAUDE.md` y verificado contra el código.
- [x] `docs/development.md` (reemplaza `QUICKSTART.md`, `SETUP_MIGRATIONS.md` y
      `TESTING_AUTH.md`, que describían el auth propio eliminado y no tenían nada vigente).
- [x] `docs/operations.md`: deploy a Cloud Run, rollback (pendiente de definir),
      troubleshooting, observabilidad, incidente 2026-08-18 y pasos manuales.
- [x] `README.md` con las seis secciones de §4.1.
- [x] `.env.example` completo contra `internal/config/config.go`.
- [x] Raíz: solo `README.md`, `AGENTS.md`, `CLAUDE.md`. `DEUDA-TECNICA.md`,
      `PACTS_ANALYSIS.md` y `docs/EMAIL_SETUP.md` → `docs/_process/`.
- [x] `api.exe` y `.claude/settings.local.json` fuera del índice; `*.exe` en `.gitignore`.
- [x] `specs/develop/` eliminado (veredicto E: duplicado de `013`).
- [x] `migrations/README.md`: tabla hasta `000015`, sección Koyeb → producción actual.
- [x] `postman/env-local.postman_environment.json` revisado: sin secretos (las variables
      `secret` están vacías), se queda versionado. `mrg_tenant_id` corregido al UUID real.

## Fase 2 — ADRs

- [x] Renombrar a `docs/adr/CLOUD-ADR-NNN-slug.md` y resolver la colisión de `002`
      (`002-replace-auth-system` → `CLOUD-ADR-005`; se conservan los números, huecos 006–013).
- [x] Front-matter MADR sin tocar el cuerpo; `docs/adr/index.md` y plantilla.
- [x] ADRs de reemplazo (aceptados el 2026-09-29) para ADR-002 (tenant por header),
      ADR-003 (mediciones en MongoDB) y la parte contradicha de ADR-004; marcar
      `superseded_by`.
- [x] ADR de `004-aas-server` (AAS resuelto por FA³ST en el edge).

## Fase 3 — Specs

Decisiones (2026-09-29): los planes solo reciben front-matter, sin traducirse a `T-NNN`.
Los planes de superpowers sin diseño se asocian a una spec existente o se archivan en
`docs/_process/`. El motor de alarmas se registra como **objetivo no cumplido**. Todo en
esta rama, sin push.

### 3a — Índice y orden

- [x] Colisiones: `002a`/`002b`, `005a`/`005b`.
- [x] `004-aas-server` y `005b-plc-events` recuperadas de `origin/docs/nosql-aas-plc-specs`
      y sacadas de `.gitignore`; `015-aas-shells` incorporada desde el PR #76.
- [x] Front-matter en todos los `spec.md` (con `veredicto`) y `plan.md`.
- [x] `specs/README.md` y `docs/_process/bitacora-de-alcance.md`.
- [x] Veredictos corregidos respecto del harness: 008 y 010 son **A**, porque las dos specs
      dejaron el motor fuera de alcance; el hueco se registra como objetivo #5 no cumplido.
      005b pedía MongoDB, no Postgres; sigue siendo **B** por la forma de la ingesta.

### 3b — Absorber `docs/superpowers/`

- [ ] 9 diseños → `specs/016…/spec.md`, con su plan como `plan.md`.
- [ ] Planes sin diseño: asociar a una spec existente o archivar en `docs/_process/`.
- [ ] Borrar `docs/superpowers/` y actualizar las referencias (el contrato congelado de la
      ingesta pasa a la spec correspondiente).

### 3c — Reconciliación y traducción

- [ ] Traducir los `spec.md` al formato unificado (`RF-NNN` con procedencia `original` o
      `derivado`, `CE-NNN`), verificando requisito por requisito contra el código.
- [ ] Unificar `001` (`spec.md` + `spec.es.md`) en un solo idioma.

## Fase 4 — CI del harness

- [ ] `harness-layout`, `markdownlint`, `links` (lychee), `docs-freshness`.
- [ ] `openapi-drift` (swag + diff, copiado del edge) y agregar
      `PATCH /api/v1/users/{id}/status` al contrato mientras tanto.

## Fuera de este repo o con decisión pendiente

- Issues de `DEUDA-TECNICA.md` vigentes (graceful shutdown, RBAC de lectura en
  `GET /users`) y labels del estándar: requieren OK para crear en GitHub.
- Release-please / `CHANGELOG.md`: en conflicto con la sección "Versionado en Release" de
  la constitution (ver auditoría).

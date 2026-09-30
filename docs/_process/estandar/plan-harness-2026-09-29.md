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

- [x] Diseños y planes → `specs/016…024/`. El diseño de `create-user-with-initial-role`
      pasa a `013/design.md`; la ingesta (022) tiene su diseño en `embolsadora-edge`.
- [x] Planes sin diseño: 5 asociados (011, 021) y 8 archivados en `docs/_process/plans/`.
- [x] `docs/superpowers/` eliminado. Referencias vivas (README, arquitectura, Postman,
      comentarios Go) actualizadas; tabla de rutas anteriores en `specs/README.md` para
      migraciones y planes históricos, que no se editan.
- [x] PR #76 cerrado como reemplazado por `015-aas-shells` y CLOUD-ADR-019.

### 3c — Reconciliación y traducción

- [x] Traducir los `spec.md` al formato unificado (`RF-NNN` con procedencia `original` o
      `derivado`, `CE-NNN`), verificando requisito por requisito contra el código.
      Completo (2026-09-29): 22 specs traducidas; 004, 005b y 015 sin traducción (C o B
      reemplazada). Originales conservados como `design.md`.
- [x] Unificar `001` (`spec.md` + `spec.es.md`) en un solo idioma.
- [ ] Observación de 016: conviven dos convenciones de respuesta (`{"success":...}` y
      struct directo). Decidir si se completa la estandarización.

## Fase 4 — CI del harness

- [x] `harness-layout` (`scripts/harness-check.sh`), `markdownlint`
      (`.markdownlint-cli2.jsonc`), `links` (lychee offline) y `docs-freshness` (semanal,
      abre un issue) en `.github/workflows/docs.yml`.
- [x] Drift de OpenAPI como test de Go (`internal/routes/openapi_drift_test.go`): compara
      las rutas del router con los paths de `docs/openapi.yaml`. Se agregó
      `PATCH /api/v1/users/{userId}/status`, la única ruta sin documentar.
- [ ] **Decisión pendiente:** migrar el OpenAPI a generación con `swag`, como el edge.
      Hoy son unas 83 rutas y un 3.1 escrito a mano con el contrato congelado de la
      ingesta; el test de drift cubre la presencia de rutas, no los schemas.
- [ ] **Pendiente:** `redocly lint docs/openapi.yaml` reporta 48 errores previos; no se
      suma a CI hasta limpiarlos.

## Fuera de este repo o con decisión pendiente

- [x] Labels del estándar creados (2026-09-30): `feature`, `tech-debt`, `risk`, `docs`,
      `docs-user`, `contract-change`, `breaking`.
- [x] Issues creados (2026-09-30):
      #90 graceful shutdown · #91 RBAC de lectura en `GET /users` · #92 `PATCH /users`
      ignora campos inmutables · #93 validar permisos de roles custom · #94 fijar RS256 ·
      #95 configuración de producción pendiente · #96 cold start de Cloud Run ·
      #97 convención de respuesta · #98 layouts por usuario o por tenant · #99 redocly lint ·
      #100 swag · #101 release-please frente a la constitution.
- Sin issue a propósito: el motor de evaluación de alarmas y el productor de logs y
  notificaciones. La decisión fue "objetivo no cumplido" (veredicto C), registrada en la
  bitácora de alcance.
- [x] Issues #33 y #34 (RBAC super-admin / cross-tenant) verificados y cerrados el
      2026-09-30: resueltos por CLOUD-ADR-015 y los PRs #55, #62 y #73. La auditoría de los
      accesos cross-tenant concedidos, único punto sin implementar del #34, pasó a #102.

# Auditoría de estándar de documentación — embolsadora4.0-cloud — 2026-09-29

> Estándar aplicado: `ESTANDAR-DOCUMENTACION-REPOS.md` v1.0.0.
> Marcas: **[H]** hecho verificado · **[I]** inferencia · **[R]** recomendación.
> Restricción de esta corrida (pedida por la dueña del repo): **sin commits y sin push**. La regla 7 del estándar (un commit por ítem en `docs/estandar-documentacion`) queda suspendida hasta nueva indicación; ver "Conflictos".

## Perfil del repo

| Atributo | Valor | Evidencia |
|---|---|---|
| Nombre del repo y dueño del remoto | `embolsadora4.0-cloud`, dueña `lucia117` | [H] `git remote -v` → `github.com/lucia117/embolsadora4.0-cloud` |
| Prefijo de ADR | `CLOUD` | [H] tabla del estándar |
| Stack principal y versión | Go 1.24 (Gin, pgx/v5, Zap, Prometheus, mongo-driver v2, go-redis v8) | [H] `go.mod`, `Dockerfile` (`golang:1.24-alpine`), `ci.yml` (`go-version: "1.24"`) |
| Rama de integración / rama de release | `develop` / `main` | [H] `ci.yml` corre en ambas; `deploy-cloud-run.yml` dispara en push a `main`; PRs `develop→main` (#85, #79) |
| ¿Expone una API HTTP? Contrato | **sí** — `docs/openapi.yaml` (OpenAPI 3.1, `info.version: 2.0.0-alpha`), **escrito a mano** | [H] `docs/openapi.yaml:1-4`; no hay generador (no hay `swag`/`oapi-codegen` en `go.mod`) |
| ¿Consume una API de otro repo? | **sí** — HTTP GET al Edge Pi (`/status`, `/health`, telemetría) sin contrato enlazado; Supabase Auth (externo) | [H] `internal/platform/edgeclient/http_client.go`; `internal/platform/supabase/admin_client.go`; `internal/api/handler/auth/login` |
| ¿Tiene base de datos propia / migraciones? | **sí** — Postgres (`migrations/` 000001–000015, golang-migrate) + MongoDB (mediciones, índices creados en código) + Redis (rate limit/cache) | [H] `migrations/`, `internal/repo/mongo/measurements`, `internal/routes/url_mappings.go:connectMeasurementsRepo` |
| ¿Dónde corre en producción y cómo se despliega? | Google Cloud Run (`embolsadora-api`, `us-east1`), build Docker → Artifact Registry → `gcloud run deploy`, disparado por push a `main`, auth por Workload Identity Federation; smoke test `GET /ping` | [H] `.github/workflows/deploy-cloud-run.yml`. Postgres de prod en Supabase: [I] (memoria del agente, fuente externa; no hay evidencia en el repo). Mongo de prod: ❔ |
| ¿Corre en hardware on-prem / de planta? | no (el Edge Pi es otro repo) | [H] solo Cloud Run en workflows |
| ¿Métricas, logs o health checks expuestos? | **sí** — `GET /metrics` (Prometheus), `GET /ping`, `GET /health` (postgres/mongo/redis); logs Zap | [H] `internal/telemetry/metrics.go:11`, `internal/routes/url_mappings.go:76,208` |
| Herramientas de proceso presentes | spec-kit (`.specify/`, `specs/`, `.claude/commands/speckit.*`, `.github/agents`, `.github/prompts`), superpowers (`docs/superpowers/`, `.superpowers/` ignorado), Postman (`postman/`) | [H] árbol del repo |
| Conventional Commits en uso (últimos 30) | **parcial** | [H] `git log --oneline -30`: 28/30 CC; no-CC: `2afe507 Merge pull request #81…`, `457941d Merge branch 'main' into develop`. Títulos de PR no-CC: #85 y #79 ("Develop") |

## Resultado por ítem

| ID | Ítem | Aplica | Estado | Evidencia | Acción propuesta | Lote | Riesgo |
|---|---|---|---|---|---|---|---|
| E01 | README | sí | ❌ | `README.md` no sigue la estructura; describe stubs 501 que ya no lo son (`internal/api/` "Rutas ABM (stubs 501)"), `docker-compose.dev.yml` (no existe), vars `DB_URL`/`DB_HOST`/`REDIS_HOST` (no las lee el código), `launch.json` con vars que ya no tiene, colecciones Postman inexistentes (`User-Management-API…`, `tenants…`), `POST /auth/login` sin `/api/v1`, "ADR-001..004" | Reescribir con la estructura E01 | B | bajo |
| E02 | Índice `docs/README.md` | sí | ❌ | no existe | Crear con las 7 secciones | A | bajo |
| E03 | `docs/architecture.md` | sí | ❌ | no existe; el contenido está disperso en `CLAUDE.md` (Architecture/Key Patterns), `README.md`, ADR-001, constitution | Crear desde el código con las 8 secciones | A | medio (volumen) |
| E04 | ADRs | sí | ⚠️ | 7 archivos con 3 formatos distintos; **colisión 002** (`002-replace-auth-system.md` y `ADR-002.md`); sin índice ni plantilla; 3 ADRs `Accepted` contradichos por el código (ver Contradicciones) | Migrar a `CLOUD-ADR-NNN-*` + front-matter; índice; plantilla; proponer ADRs de reemplazo | A (índice/plantilla) + C (renombrar) + B (front-matter) | medio |
| E05 | Archivos para agentes | sí | ⚠️ | `CLAUDE.md` es la única fuente de: capas/paquetes, cadena de middlewares, auto-provisioning, fail-open de Redis, JWKS→503, incidente 2026-08-18 de orden de deploy, "Pending Manual Steps" | Copiar a `docs/architecture.md` / `docs/operations/` y dejar enlaces en `CLAUDE.md` (sin borrar instrucciones) | B | bajo |
| E06 | Variables de entorno | sí | ⚠️ | Faltan en `.env.example`: `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `DB_MAX_CONNS`, `DB_MIN_CONNS`, `DB_CONN_MAX_LIFETIME`, `RUN_MIGRATIONS_ON_BOOT`, `MIGRATIONS_SOURCE_URL`, `MONGO_DB` (fallback legacy). Sobra: `CORS_ALLOWED_ORIGINS` (el código no la lee: `middleware.go:370-372` hardcodea `*`). Faltan comentarios de obligatoria/opcional/default en la mayoría. Comentario "en Koyeb/PaaS" desactualizado | Completar y comentar | B | bajo |
| E07 | Higiene del repo | sí | ❌ | `api.exe` versionado (39,5 MB); `.claude/settings.local.json` versionado aunque `.gitignore` lo excluye; `postman/env-local.postman_environment.json` versionado (**no abierto**: puede tener tokens — revisar) | `git rm --cached api.exe .claude/settings.local.json`; agregar `*.exe` a `.gitignore`; revisar a mano el environment de Postman | C | medio |
| E08 | Contrato OpenAPI | sí | ⚠️ | Router↔OpenAPI: 83 rutas vs 79 paths. En router y no en OpenAPI: `GET /ping`, `GET /health`, `GET /metrics`, **`PATCH /api/v1/users/{id}/status`**. En OpenAPI y no en router: ninguna. `info.version` semver ✅. `servers` apunta a `https://api.tu-dominio.com` (placeholder). **No hay job de CI de lint** (`Makefile lint` usa redocly opcional, no corre en CI). Contrato congelado de ingesta (`errors[].code`) vive en `docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md` — verificar si ya está en `openapi.yaml` | Agregar `PATCH /users/{id}/status`; decidir si documentar `/ping` `/health` `/metrics`; job `spectral lint` en CI; mover el contrato congelado a descripciones/`examples` del OpenAPI. [R] pasar a generación a futuro | A (CI) + B (YAML) | medio |
| E09 | Integraciones | sí | ❌ | no existe `docs/integrations.md`. Fronteras reales: Frontend→Cloud, Edge Pi→Cloud (ingesta), Cloud→Edge Pi (status/health/telemetría), Cloud↔Supabase Auth. Copia de contrato: `specs/003-edge-device-management/contracts/edge-device-service-api.openapi.yaml` (spec-kit, no se mueve). Pacts del frontend (`PACTS_ANALYSIS.md`) sin verificación en CI | Crear con una sección por frontera | A | medio |
| E10 | Guía de desarrollo | sí | ❌ | no existe `docs/development.md`; hay `QUICKSTART.md`, `SETUP_MIGRATIONS.md`, `TESTING_AUTH.md` en la raíz, **los tres obsoletos** (auth propio, tablas `sessions`, `docker-compose.dev.yml`, `/api/auth/register`) | Crear `docs/development.md` desde `CLAUDE.md`/Makefile/compose/`scripts/ci-check.sh`; los tres sueltos no se fusionan (no hay nada vigente que rescatar) → proponer borrarlos o moverlos a `_process/` | A + C | bajo |
| E11 | Operación | sí | ❌ | no existe `docs/operations/`. Deploy real en `deploy-cloud-run.yml`; `migrations/README.md` y `CLAUDE.md` describen Koyeb. `docs/EMAIL_SETUP.md` es un runbook ya ejecutado | Crear `deploy.md` + runbooks: `mongo-no-disponible`, `jwks-no-disponible`, `redis-no-disponible` (evidencia en código). Rollback: ❔ (no definido salvo notas de 000010) | A | medio |
| E12 | Material de proceso | sí | ⚠️ | Carpetas de herramientas (no se tocan): `specs/`, `.specify/`, `docs/superpowers/`. Sueltos de proceso: `PACTS_ANALYSIS.md`, `docs/EMAIL_SETUP.md` (runbook completado). **Duplicado**: `specs/develop/` = `specs/013-user-create-with-role/` (`spec.md` idéntico; canónico 013, que tiene `tasks.md`). Prefijo repetido `specs/002-*` (dos features) — no se toca (regla 6) | Listar en índice como Proceso; mover `PACTS_ANALYSIS.md` y `docs/EMAIL_SETUP.md` a `docs/_process/`; borrar `specs/develop/` | C | bajo |
| E13 | Pendientes en Issues | sí | ❌ | `DEUDA-TECNICA.md` (ver tabla abajo); `CLAUDE.md` "Pending Manual Steps" | Crear issues de los vigentes; reemplazar el archivo por un enlace | D + C | bajo |
| E14 | Labels | sí | ❌ | Existen: `bug`(d73a4a ✅), `question`(d876e3 ✅), `documentation`, `duplicate`, `enhancement`, `good first issue`, `help wanted`, `invalid`, `wontfix`. Faltan: `feature`, `tech-debt`, `risk`, `docs`, `docs-user`, `contract-change`, `breaking` | Crear los 7 faltantes (no borrar los existentes) | D | bajo |
| E15 | Plantilla de PR | sí | ❌ | no existe `.github/pull_request_template.md` | Crear con el bloque exacto | A | bajo |
| E16 | Plantillas de issue | sí | ❌ | no existe `.github/ISSUE_TEMPLATE/` | Crear las cuatro | A | bajo |
| E17 | CODEOWNERS | sí | ❌ | no existe | Crear **solo con handles confirmados** (ver Preguntas) | A | bajo |
| E18 | Versionado y changelog | sí | ❌ | sin tags (`git tag` vacío), sin releases, sin CHANGELOG. El repo permite merge commit, squash y rebase; `develop→main` se mergea con merge commit (#85, #79 con título "Develop") | Agregar release-please (`release-type: go`, rama `main`). Versión inicial: ver Preguntas | A | medio |
| E19 | CI de documentación | sí | ❌ | no existen `.markdownlint.jsonc` ni `docs.yml`. markdownlint/lychee no corridos todavía (se corren en Fase 4) | Crear; limitar a `README.md` + `docs/**` excluyendo proceso | A | bajo |
| E20 | Metadatos | sí | ❌ | ningún documento alcanzado existe todavía | Front-matter `owner`/`last_reviewed` en los nuevos | A | bajo |
| E21 | Cero contradicciones | sí | ❌ | ver sección Contradicciones | Corregir docs; avisos `⚠️ Desactualizado` donde falte decisión | B | medio |

### E13 — Ítems de `DEUDA-TECNICA.md`

| Ítem | ¿Vigente? (evidencia) | Repo destino | Título del issue | Labels |
|---|---|---|---|---|
| RBAC en `GET /users` (pendiente decisión) | **Parcial** [H]: `GET /users` y `GET /users/:id` siguen sin `RBACCheck` (`internal/api/router.go:74-75`); `GET /users/:id/roles` ya tiene `perm_users_view` (`router.go:81`) | cloud | Definir RBAC de lectura en `GET /users` y `GET /users/:id` | `question`, `tech-debt` |
| 1. JWT middleware stub | **Resuelto** [H]: `JWTAuth(verifier, …)` en `url_mappings.go` (grupo v1 y grupo edge-devices) | — | — | — |
| 2. Auditoría con usuarios aleatorios | **Resuelto** [H]: `status_check.go:34,39`, `health_check.go:26` usan `platform.UserID/UserEmail` | — | — | — |
| 3. Error handling demasiado amplio | **Resuelto** [I]: el service solo mapea `ErrDeviceNotFound`, el repo solo mapea `pgx.ErrNoRows` (`repository.go:71,119,142`) | — | — | — |
| 4. Postman URLs edge devices | **Resuelto** [H]: colección usa `/api/v1/tenants/{{tenantSubdomain}}/edge-devices`, que coincide con el router | — | — | — |
| 5. Down migration 0005 | **Obsoleto** [H]: migraciones consolidadas (ADR-014), `0005_create_edge_devices_tables` ya no existe | — | — | — |
| 6/7. Timestamps en Create/Update | **Resueltos** [H]: `repository.go:85` (`RETURNING created_at, updated_at`), `:109-111` (`CURRENT_TIMESTAMP`) | — | — | — |
| 8. Docs con comandos incorrectos | **Resuelto** [H]: sin coincidencias de `cmd/main.go`/`cmd/migrate` en `postman/*.md` ni `specs/003-*` | — | — | — |
| 9. Telemetry DTO vs contrato | **Resuelto** [I]: `dto.go:58-66` usa objetos `CPU/RAM/Disk` | — | — | — |
| 10. Sin shutdown gracioso | **Vigente** [H]: sin `signal.Notify`/`Shutdown(` en `cmd/` ni `internal/` | cloud | Agregar graceful shutdown del servidor HTTP (SIGTERM en Cloud Run) | `tech-debt`, `risk` |
| 11. 403 distingue header ausente de key inválida | **Aceptado, no es pendiente** [H] (PR #56) | — | (candidato a ADR retrospectivo o nota en `docs/integrations.md`) | — |
| CLAUDE.md "Pending Manual Steps": provisionar Mongo/`MONGO_URI` | ❔ no verificable desde el repo | cloud | Confirmar Mongo de producción y `MONGO_URI` en Cloud Run | `question` |
| CLAUDE.md "Pending Manual Steps": deploy a Koyeb | **Obsoleto** [H] (deploy es Cloud Run) | — | — | — |
| CLAUDE.md "Pending Manual Steps": activar admin MRG | ❔ | cloud | Confirmar alta del admin MRG en producción | `question` |
| CLAUDE.md: promover `feat/rbac-dynamic-permissions-backend` → `main` | **Probablemente resuelto** [I]: `000011` y `perm_*` finos están en `develop` y hubo varios merges `develop→main` después (#70, #72, #74, #85) | — | — | — |

Issues abiertos existentes: #33 y #34 (RBAC super-admin/tenant-manager, cross-tenant). [I] Parecen resueltos por ADR-015 y `000011`; proponer verificarlos y cerrarlos (lote D).

## Inventario de documentación existente

| Archivo | Qué resuelve | Vigente | Contradicciones con el código | Destino según el estándar |
|---|---|---|---|---|
| `README.md` | Entrada al repo | ❌ | muchas (ver E01) | Reescribir (E01) |
| `CLAUDE.md` | Guía para agentes + estado de prod | ⚠️ | "Deploy a Koyeb"; árbol omite `dashboards`, `edge_devices`, `logs`, etc. [I] | Se queda; enlaces a docs humanos (E05) |
| `QUICKSTART.md` | Levantar con Docker | ❌ | "tres contenedores" (hoy 4 con mongo); `/api/auth/register`; enlaza docs obsoletos | Borrar o `_process/` (C) |
| `SETUP_MIGRATIONS.md` | Instalar migrate | ❌ | tablas `sessions`/`password_reset_tokens` (eliminadas por ADR 002); `docker-compose.dev.yml` | Borrar o `_process/` (C) |
| `TESTING_AUTH.md` | Probar auth propio | ❌ | endpoints `/api/auth/*` que no existen | Borrar o `_process/` (C) |
| `DEUDA-TECNICA.md` | Deuda técnica | ⚠️ | 9 de 11 ítems resueltos/obsoletos | Issues (E13) → borrar (C) |
| `PACTS_ANALYSIS.md` | Cobertura de pacts del frontend | ❔ (leído parcialmente) | se contradice a sí mismo (`permissions-service-api` 10/10 y 0/10) | `docs/_process/` (C); candidato a repo central |
| `docs/openapi.yaml` | Contrato HTTP | ⚠️ | falta `PATCH /users/{id}/status`; `servers` placeholder | Se queda (E08) |
| `docs/EMAIL_SETUP.md` | Runbook DNS/Resend/SMTP (completado) | ⚠️ | "Env vars en Cloud Run" pendiente sin verificar | `docs/_process/` (C); candidato a repo central |
| `docs/adr/*.md` (7) | Decisiones | ⚠️ | ver Contradicciones | `docs/adr/CLOUD-ADR-*` (E04) |
| `migrations/README.md` | Migraciones | ⚠️ | tabla llega a `000012` (existen hasta `000015`); "Deploy a Koyeb"; `version=2` | Actualizar (B); pasos de deploy → `docs/operations/deploy.md` |
| `seeds/README.md` | Seed de prueba | ❔ | no verificado contra el schema actual | Listar en índice (Desarrollo) |
| `emails/README.md` | Plantillas de mail | ✅ [I] | `cmd/renderemails` y `scripts/publish-email-templates.sh` existen | Listar en índice (Operación) |
| `postman/README.md`, `TESTING-GUIDE.md` | Colección "User Management" | ❌ | referencian `User-Management-API.postman_collection.json` (no existe), `POST /api/v1/login` | Listar en índice con aviso, o `_process/` |
| `postman/POSTMAN-GUIDE.md` | Colección maestra | ✅ [I] | — | Índice (Desarrollo) |
| `postman/EDGE-DEVICE-README.md` | Colección edge devices | ❌ | rutas `/api/tenants/...` sin `/v1`; archivos de colección que no existen | `_process/` o borrar (C) |
| `.specify/memory/constitution*.md` | Gobernanza spec-kit | ⚠️ | endpoints "estado stub"; "Idempotencia requerida en Redis" (hoy es por `eventId` en Mongo); `MIGRATION_v*.md` | No se toca (herramienta); reportado |
| `specs/**`, `docs/superpowers/**` | Proceso | — | — | Índice → Proceso |

## Contradicciones documento ↔ código

1. **Plataforma de deploy.** `CLAUDE.md` ("Deploy a Koyeb"), `migrations/README.md` ("Deploy a Koyeb (producción)"), `.env.example:2`, `.gitignore:9`, ADR-014 (título y cuerpo) dicen Koyeb ↔ [H] `.github/workflows/deploy-cloud-run.yml` despliega a Cloud Run.
2. **ADR-002 contradicho.** Dice que el tenant sale "exclusivamente del claim `tenant_id` del JWT" y que `X-Tenant-Id` se ignora ↔ [H] `TenantFromHeader` (`internal/api/middleware`) usa `X-Tenant-ID`, y edge devices resuelve el tenant por subdominio en el path (`resolve_tenant_path.go`). El propio ADR 002-replace-auth dice "Nuevo header requerido: `X-Tenant-ID`".
3. **ADR-003 contradicho.** Eventos en Postgres (`machine_events`, particionado mensual, 90 días) ↔ [H] mediciones en MongoDB (`internal/repo/mongo/measurements`); no existe tabla `machine_events` en `migrations/`.
4. **ADR-004 parcialmente contradicho.** Tamaño máximo 2 MB ↔ [H] `INGEST_MAX_BODY_BYTES` default 4194304 (`config.go`, con justificación). `Idempotency-Key` "TTL 10 min" ↔ [H] `events_handler.go:73-77` lo registra "pero no decide nada".
5. **ADR-014** "`version=2`" y "4 archivos en `migrations/`" ↔ [H] hoy hay 15 migraciones (histórico: es correcto a su fecha; no es error del ADR, pero `migrations/README.md` repite el `version=2` como verificación vigente).
6. **ADR-015** nombra permisos `tenants:write`/`users:write` ↔ [H] catálogo actual `perm_tenants_manage`/`perm_users_manage` (`000011`). [I] superado en nomenclatura, no en decisión.
7. **README**: stubs 501, `docker-compose.dev.yml`, vars `DB_*`/`REDIS_HOST`/`REDIS_ADDR`/`AUTH_JWT_*`, colecciones inexistentes, `make migrate # placeholder` ↔ [H] Makefile tiene `migrate-up`; `make docker` usa `docker-compose.yml`.
8. **`.env.example`**: `CORS_ALLOWED_ORIGINS` ↔ [H] CORS hardcodeado a `*` (`middleware.go:372`).
9. **`LOG_LEVEL`** documentada ↔ [H] se lee en `config.go` pero nada usa `cfg.Observability.LogLevel`; el logger es `zap.NewDevelopment()` fijo (`cmd/api/main.go`, `url_mappings.go`). `internal/telemetry/logger.go:11` compara `APP_ENV == "dev"`, valor que `LoadEnvFile` no acepta.
10. **Constitution**: "Zap JSON en producción" ↔ logger de desarrollo; "Idempotencia en Redis con TTL" ↔ idempotencia por índice único `(tenantId, eventId)` en Mongo; endpoints ABM "estado stub" ↔ implementados. No se corrige (herramienta), se reporta.
11. **`migrations/README.md`** tabla de migraciones termina en `000012` ↔ existen `000013`–`000015`; no menciona `RUN_MIGRATIONS_ON_BOOT` (`internal/platform/dbmigrate`).
12. **`docs/openapi.yaml`** omite `PATCH /api/v1/users/{id}/status` (registrado en `internal/api/router.go:89`).
13. **`CLAUDE.md`** "Pending Manual Steps" con pasos obsoletos (Koyeb).

## Conflictos con reglas propias del repo

- **Pedido explícito de la dueña (esta sesión): "no hagas commits y push".** Gana sobre la regla 7 del estándar. Propuesta: aplicar los lotes en el working tree (en la rama `docs/estandar-documentacion` si la aprobás, sin commitear) y dejar los commits a tu cargo, o autorizar commits más adelante.
- **Constitution → "Versionado en Release"** (actualizar constante de versión, tag manual `v…`, `MIGRATION_v*.md`) vs **E18** (release-please). La constitution declara que "supersede toda otra guía". Por regla 3 gana la constitution: E18 queda en suspenso hasta que decidas enmendarla (vía ADR, como pide su procedimiento).
- **Constitution → "Requerimiento de ADR" en `docs/adr/`**: compatible con E04; el formato de nombre cambia, no la ubicación.
- **Constitution → Compuerta "Contrato: Spec OpenAPI actualizado"**: compatible con E08/E15.

## Preguntas abiertas

1. **CODEOWNERS (E17):** ¿qué handles de GitHub? ¿Solo `@lucia117`, o también Federico (handle?) para `docs/openapi.yaml` y `docs/adr/`?
2. **Versión inicial de release-please (E18):** no hay tags → el estándar dice `0.1.0`, pero `docs/openapi.yaml` declara `2.0.0-alpha` y el ADR de Supabase habla de "v2.0.0". ¿`0.1.0`, `2.0.0-alpha` o posponer E18 por el conflicto con la constitution?
3. **ADRs contradichos (ADR-002, ADR-003, parte de ADR-004):** ¿redacto ADRs de reemplazo propuestos (`estado: Propuesta`) — "tenant por header `X-Tenant-ID` + fallback de plataforma" y "mediciones en MongoDB"? Si la razón no está escrita, van con "Razón no documentada".
4. **Numeración de ADRs:** con la regla "la más nueva toma el siguiente número libre", `002-replace-auth-system` (2026-03-07) pasa a `CLOUD-ADR-005`; ADR-014 y ADR-015 conservan su número y quedan huecos 006–013 (preexistentes). ¿Ok, o preferís renumerar 001–007 corrido?
5. **Postgres/Mongo de producción:** ¿Postgres de prod es Supabase? ¿Dónde está Mongo de prod (Atlas?) y está seteado `MONGO_URI` en Cloud Run? Solo tengo evidencia de Cloud Run.
6. **Rollback:** no hay procedimiento definido (salvo notas de `000010`/`000011`). ¿Se hace redeploy de una revisión anterior en Cloud Run? Si no está decidido queda `> Pendiente`.
7. **`postman/env-local.postman_environment.json`:** no lo abrí por la regla de secretos. ¿Contiene tokens reales? Si sí: sacar del repo y rotar.
8. **Pacts del frontend:** `PACTS_ANALYSIS.md` analiza 13 pacts de `embolsadora-frontend/pacts/` que **nadie verifica** en CI de este repo. ¿Se piensa verificar (pact provider verification) o se abandonan?
9. **Issues #33 y #34:** ¿los cierro (lote D) tras verificar que ADR-015 + `000011` los resuelven?
10. **`GET /ping`, `/health`, `/metrics` en OpenAPI:** ¿se documentan (tag `Ops`) o se justifican como fuera del contrato?
11. **Contrato Cloud → Edge Pi** (`/status`, `/health`, telemetría que el cloud consume): ¿cuál es el archivo fuente de verdad en el repo `embolsadora-edge` (ruta y rama)?

## Candidatos para el repositorio central

- `docs/EMAIL_SETUP.md` — DNS (Vercel), Resend y SMTP de Supabase: afecta a todo el sistema y al repo frontend.
- `PACTS_ANALYSIS.md` — contratos frontend ↔ cloud.
- `docs/superpowers/plans/2026-08-05-cloud-ingest-endpoint.md` (contrato congelado Edge ↔ Cloud) — contrato que cruza repos (no se mueve: es carpeta de herramienta; se referencia).
- `CLAUDE.md` § "Estado de producción" / incidente 2026-08-18 — orden de deploy entre frontend y backend.
- `migrations/README.md` § "Orden de deploy: migración 000011" — coordinación backend ↔ frontend.

## Plan de aplicación por lotes

**Lote A — agregar (sin tocar nada existente):**
`docs/README.md` (E02) · `docs/architecture.md` (E03) · `docs/adr/README.md` + `docs/adr/_template.md` (E04) · `docs/integrations.md` (E09) · `docs/development.md` (E10) · `docs/operations/deploy.md` + `docs/operations/runbooks/{mongo-no-disponible,jwks-no-disponible,redis-no-disponible}.md` (E11) · `.github/pull_request_template.md` (E15) · `.github/ISSUE_TEMPLATE/{bug,feature,tech-debt}.md` + `config.yml` (E16) · `.github/CODEOWNERS` (E17, tras respuesta 1) · release-please (E18, tras respuesta 2 y conflicto con constitution) · `.markdownlint.jsonc` + `.github/workflows/docs.yml` (E19) · job `spectral lint` para `docs/openapi.yaml` (E08) · front-matter E20 en todos los nuevos.

**Lote B — editar documentación existente:**
`README.md` (E01) · `.env.example` (E06) · `docs/openapi.yaml`: `PATCH /users/{id}/status`, `servers`, contrato congelado de ingesta (E08) · front-matter MADR en ADRs sin tocar el contenido (E04) · `CLAUDE.md`: enlaces a docs humanos, corregir Koyeb→Cloud Run (E05/E21) · `migrations/README.md`: tabla hasta 000015, sección deploy (E21) · avisos `⚠️ Desactualizado` donde falte decisión (E21).

**Lote C — mover / renombrar / borrar:**
`git rm --cached api.exe .claude/settings.local.json` + `*.exe` en `.gitignore` (E07) · renombrar ADRs a `CLOUD-ADR-NNN-*` (E04) · `QUICKSTART.md`, `SETUP_MIGRATIONS.md`, `TESTING_AUTH.md` → borrar (sin contenido vigente; QUICKSTART tiene un enlace entrante solo desde los otros dos) (E10) · `PACTS_ANALYSIS.md`, `docs/EMAIL_SETUP.md` → `docs/_process/` (E12) · borrar `specs/develop/` (duplicado de 013) (E12) · `DEUDA-TECNICA.md` → reemplazar por enlace a issues (E13) · `postman/EDGE-DEVICE-README.md` y guías de la colección vieja → `docs/_process/` o borrar (E12).

**Lote D — GitHub (requiere tu OK):**
Crear 7 labels (E14) · crear 2–4 issues de E13 · cerrar #33/#34 si se confirma · (opcional) protección de `main`: hoy no tiene (`Branch not protected`).

Nota sobre commits: por tu pedido, ningún lote se commitea ni se pushea. Los cambios quedan en el working tree.

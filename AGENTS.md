# Embolsadora Cloud API — Guía para agentes

Fuente única de reglas para agentes de código en este repo. `CLAUDE.md` es un adaptador
que incluye este archivo; no dupliques reglas allí.

## Empezar acá

- Go 1.24+ instalado de forma nativa: corré `go` directo, sin Docker.
- Antes de declarar un trabajo terminado corré `scripts/ci-check.sh` (build, vet,
  golangci-lint y tests, en el mismo orden que `.github/workflows/ci.yml`).
- Tests de integración: se saltean en silencio salvo que estén exportadas
  `DATABASE_URL`, `MONGO_URI` y `REDIS_URL`. Levantá las dependencias con
  `docker compose up -d db redis mongo`. Detalle en [`docs/development.md`](docs/development.md).
- Arquitectura interna, capas y patrones obligatorios: [`docs/architecture.md`](docs/architecture.md).
- Deploy, rollback, troubleshooting y estado de producción: [`docs/operations.md`](docs/operations.md).

## Comandos

```bash
go build ./...
go test ./...
go test ./internal/security/... -run TestJWKSVerifier -v   # un test puntual
go get github.com/some/package && go mod tidy               # agregar dependencia
migrate -path migrations/ -database "$DATABASE_URL" up      # aplicar migraciones
```

## Límites de arquitectura

- Capas hexagonales: `transport (handler) → app (usecase) → domain ← infra (repo/platform/security)`.
  `domain/` no importa nada de infraestructura.
- Los tipos de respuesta de `GET /api/v1/me` viven en `internal/api/usecases`, no en el
  handler: el handler importa el usecase y al revés habría un ciclo.
- La superficie `/api/v1/consumers/events` es un **contrato con el Edge Pi Service**:
  sin envelope `{"success":...}`, códigos `errors[].code` fijos. No cambies su forma sin
  un PR con label `contract-change` y sin coordinar con el repo del edge. Ver
  [`docs/architecture.md`](docs/architecture.md#superficies-http).
- El enforcement de permisos lee el catálogo `perm_*` desde `roles.permissions` en
  Postgres en cada request (`TenantFromHeader` → `security.Can()`). Un permiso nuevo es
  una migración, no un cambio en Go.
- Redis es opcional: rate limit y caché de API keys fallan abiertos si no está.
  Mongo caído al arrancar deja la ingesta degradada (500), no tumba el proceso.
  No conviertas ninguno de los dos en `log.Fatalf`.

## Cambios seguros

- Nunca leas, imprimas ni commitees valores de `.env*`; los nombres de variables están
  en `.env.example`.
- No despliegues a mano: el deploy a Cloud Run lo hace `deploy-cloud-run.yml` al pushear
  a `main`.
- Una migración que cambie firmas de funciones SQL o ids de permisos tiene orden de
  deploy: leé las secciones "⚠️ Orden de deploy" de [`migrations/README.md`](migrations/README.md)
  antes de mergear.
- `docs/openapi.yaml` se mantiene a mano: toda ruta nueva o modificada se refleja ahí en
  el mismo PR. `TestOpenAPIMatchesRouter` (`internal/routes`) falla si no.

## Specs

- `superpowers:brainstorming` escribe el diseño en `specs/NNN-slug/spec.md`,
  NO en `docs/superpowers/specs/`. Tomá el próximo NNN libre de [`specs/README.md`](specs/README.md).
- `superpowers:writing-plans` escribe en `specs/NNN-slug/plan.md`.
- Todo `spec.md` lleva front-matter completo y al menos un `RF-NNN`.
- Al cerrar una feature, actualizá `status` en el front-matter y la fila en `specs/README.md`.

## Documentación

- Un documento nuevo en `docs/` lleva front-matter `title`, `status`, `owner` y
  `last_reviewed`. Antes de terminar un cambio de documentación, corré
  `scripts/harness-check.sh` y `npx markdownlint-cli2@0.18.1`.
- En la raíz solo van `README.md`, `AGENTS.md`, `CLAUDE.md`, `CHANGELOG.md` y `LICENSE`.
  Material de proceso (planes, auditorías, análisis cerrados) va a `docs/_process/`.
- Las decisiones difíciles de revertir (contratos, persistencia, seguridad, despliegue)
  llevan ADR en `docs/adr/`. Un ADR aceptado no se edita: se crea otro que lo reemplaza.

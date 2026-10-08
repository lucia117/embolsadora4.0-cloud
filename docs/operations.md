---
title: Operación — embolsadora4.0-cloud
status: vigente
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Operación

Deploy, rollback, troubleshooting y observabilidad **de este componente**. Los runbooks
que cruzan componentes (orden de deploy frontend ↔ backend, DNS y mail) van al hub y
enlazan acá.

## Dónde corre

| Pieza | Dónde | Evidencia |
|---|---|---|
| API | Google Cloud Run, servicio `embolsadora-api`, región `us-east1`, proyecto `embolsadora` | `.github/workflows/deploy-cloud-run.yml` |
| Imagen | Artifact Registry `us-east1-docker.pkg.dev/embolsadora/cloud-run-source-deploy/embolsadora-api` | ídem |
| PostgreSQL | Supabase (Postgres administrado) | configuración del servicio (fuera del repo) |
| Auth | Supabase Auth (JWKS con clave EC, ES256; Admin API para invitaciones y recovery) | `internal/security/jwt.go`, `internal/platform/supabase` |
| MongoDB | `MONGO_URI` del servicio | > Pendiente: documentar proveedor y confirmar que la variable está seteada en Cloud Run |
| Redis | `REDIS_URL` del servicio (opcional) | > Pendiente: confirmar si producción tiene Redis |

> Documentos viejos (CLOUD-ADR-014, versiones previas de este repo) mencionan **Koyeb**. El
> deploy real es Cloud Run.

## Deploy

Automático: un push a `main` dispara `deploy-cloud-run.yml` (también se puede lanzar a
mano con `workflow_dispatch`). El job:

1. Se autentica sin claves con Workload Identity Federation (OIDC de GitHub; el provider
   solo acepta tokens de este repositorio).
2. Construye la imagen con el `Dockerfile` y la etiqueta con el SHA del commit y `latest`.
3. Publica en Artifact Registry.
4. `gcloud run deploy embolsadora-api --image <imagen>:<sha>`: **solo cambia la imagen**;
   las variables de entorno del servicio se conservan.
5. Smoke test: `GET <url>/ping` debe responder `pong`.

Las variables de entorno se administran en el servicio de Cloud Run, no en el workflow.
Lista y defaults en [`.env.example`](../.env.example).

### Migraciones en producción

El workflow **no aplica migraciones**. Hay dos caminos:

- A mano, antes del deploy: `migrate -path migrations/ -database "$DATABASE_URL" up`
  contra la base de producción.
- Al arrancar: `RUN_MIGRATIONS_ON_BOOT=true` en el servicio (la imagen trae
  `migrations/` en `/app/migrations`). Si la migración falla, el proceso sale con código 1.

> Pendiente: confirmar cuál de los dos está configurado en producción.

Antes de mergear a `main` una migración con cambios de firmas SQL o ids de permisos, leé
las secciones "⚠️ Orden de deploy" de [`migrations/README.md`](../migrations/README.md)
(`000010`: la migración va antes del binario; `000011`: el frontend tiene que salir
inmediatamente después).

### Orden de deploy con el frontend

El frontend (`embolsadora-frontend`, Vercel) consume los ids de permisos que devuelve
`GET /api/v1/me`, que se leen **en vivo** de `roles.permissions`. Un cambio de catálogo
de permisos obliga a coordinar:

1. Aplicar la migración en producción.
2. Deployar este backend.
3. Deployar el frontend.

**Incidente 2026-08-18 (resuelto).** El frontend (PR #69 `develop→main`) salió a
producción antes que el backend (PR #62). El frontend nuevo exigía los ids finos
(`perm_users_view`, `perm_tenants_manage`, … — migración `000011`) y la base seguía en la
versión 10 con los ids viejos (`perm_users`, `perm_tenants`): la sección de
administración quedó inaccesible para todos los roles, incluido `super_admin`. Se
resolvió corriendo `migrate ... up` directo contra producción hasta la versión 11, sin
redeploy del backend: `GetMe` arma `Permissions` desde la base, así que `/me` empezó a
devolver los ids nuevos con el binario viejo.

### Fase MVP

Al 2026-08-18 producción funcionaba como entorno de prueba, sin usuarios reales que
dependan de uptime o datos; aplicar una migración directo contra producción para
destrabar un incidente era aceptable. **Esto cambia cuando el MVP salga de esa fase:
confirmalo antes de asumirlo.**

## Rollback

> Pendiente: no hay procedimiento definido ni probado.

Propuesta **[R]**, sin validar: redirigir el tráfico a la revisión anterior de Cloud Run.

```bash
gcloud run revisions list --service embolsadora-api --region us-east1
gcloud run services update-traffic embolsadora-api --region us-east1 --to-revisions <REVISION>=100
```

Si la versión a revertir incluyó una migración incompatible con el binario anterior,
hay que revertir también la migración (`migrate ... down 1`), en el orden que indica
[`migrations/README.md`](../migrations/README.md) para `000010` y `000011`.

## Troubleshooting

| Síntoma | Causa probable | Qué hacer |
|---|---|---|
| `POST /api/v1/consumers/events` responde 500 en todos los requests; `/health` → `mongo: error`; `ingest_mongo_up` = 0 | MongoDB inalcanzable (al arrancar o después) | Verificar `MONGO_URI` y conectividad. El Edge reintenta con backoff sin perder datos; al volver Mongo, redeployar o reiniciar si la caída fue al arrancar (el repo queda en stub degradado hasta el próximo arranque). |
| El proceso termina al arrancar con "no se pudieron crear los indices de measurements" | Mongo conecta pero falla `EnsureIndexes` | Revisar permisos del usuario de Mongo y colisiones con índices existentes. Es fatal a propósito: sin el índice único `(tenantId, eventId)` no hay idempotencia. |
| Todas las rutas autenticadas responden 503 | JWKS de Supabase inalcanzable (`ErrJWKSUnavailable`) | Verificar `SUPABASE_JWKS_URL` y el estado de Supabase. |
| 401 en todas las rutas autenticadas | `SUPABASE_JWT_ISSUER` / `SUPABASE_JWT_AUDIENCE` no coinciden con el token | Comparar `iss` y `aud` del token con la configuración. |
| Rate limit no actúa; `/health` → `redis: error` o `no configurado` | Redis caído o sin `REDIS_URL` | Falla abierto a propósito: la API sigue. Restaurar Redis o setear `REDIS_URL`. |
| 400 `missing X-Tenant-ID header` / `X-Tenant-ID must be a valid UUID` | El cliente no manda el header o manda un valor inválido | Corregir el cliente. `/me` y `/auth/change-password` son las únicas rutas ABM que no lo piden. |
| 403 `tenant access denied` | El usuario no tiene membresía activa en el tenant ni es operador de plataforma | Revisar `user_tenant_roles`. |
| 403 `password_change_required` | El usuario tiene cambio de contraseña forzado | Esperado: el usuario tiene que pasar por `POST /api/v1/auth/change-password`. |
| El arranque falla con `missing required env vars` | Falta alguna obligatoria de `config.Load` | Ver la lista en [`development.md`](development.md#configuración). |
| `schema_migrations.dirty = true` | Una migración falló a medias | Ver "Estado dirty / recuperación" en [`migrations/README.md`](../migrations/README.md). |

## Observabilidad

| Endpoint | Qué expone |
|---|---|
| `GET /ping` | Liveness: `pong`. Lo usa el smoke test del deploy. |
| `GET /health` | `{"status": "ok"}` o `{"status": "degraded"}`, con `checks` para `postgres`, `mongo` y `redis`. 503 si Postgres o Mongo fallan; Redis degradado no cambia el estado. |
| `GET /metrics` | Métricas Prometheus (`internal/telemetry`): `auth_*` (incluido `auth_cross_tenant_grants_total{role}`, accesos de operadores de plataforma a tenants ajenos), `invitations_*`, `ingest_*` (incluido `ingest_mongo_up`), `log_*`, `notification_*`, `permissions_*`, `dashboard_*`. |

Logs: Zap en modo desarrollo en todos los ambientes, con `request_id` por request
(header `X-Request-ID`). `LOG_LEVEL` se lee pero hoy no tiene efecto.

## Pasos manuales pendientes

Seguimiento en el issue #95, que junta todos los `> Pendiente` de este documento.

- **Admin MRG.** Crear el usuario en Supabase Auth y asignarle `super_admin` en el tenant
  MRG (`11b36b85-033d-4bb3-9e31-4c92161887c0`). Procedimiento en
  [`migrations/README.md`](../migrations/README.md#activación-del-admin-mrg-post-deploy).
  > Pendiente: confirmar si ya se hizo en producción.
- **MongoDB en producción.** Sin `MONGO_URI` la variable cae a `mongodb://localhost:27017`
  y la ingesta arranca degradada. > Pendiente: confirmar que está seteada en Cloud Run.

---
id: 021
title: "Mails de autenticación: plantillas propias y URL por instancia"
tier: feature
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-07-29
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: []
prs: [53]
adrs: [CLOUD-ADR-005]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 021 — Mails de autenticación: plantillas propias y URL por instancia

> **Procedencia.** `origen: original` viene del diseño aprobado del 2026-07-29,
> conservado en [`design.md`](design.md) y citado por sección (`§N`). `origen: derivado`
> se reconstruyó del código y de los documentos de cierre de la misma carpeta:
> [`followups.md`](followups.md), [`research-tokenhash-migration.md`](research-tokenhash-migration.md),
> [`research-invited-user-password.md`](research-invited-user-password.md) y
> [`plan-pr53-review-fixes.md`](plan-pr53-review-fixes.md). **Verificado contra `develop`
> el 2026-09-29.** Esta spec cubre solo la parte del backend; el BFF y el callback viven en
> `embolsadora-frontend`.

## Contexto y problema

Los mails de autenticación tenían dos problemas:

1. **La URL estaba mal.** El link llegaba siempre con `http://localhost:3000`, aunque la
   invitación saliera de producción, porque el backend armaba el `redirect_to` con una
   sola `APP_BASE_URL` global. Además, todo link enviado hasta entonces daba 404: apuntaba
   a `/s/{uuid}/…` y el frontend resuelve tenants por slug.
2. **El diseño era el de Supabase por defecto:** en inglés, sin marca y enviado desde la
   infraestructura compartida de Supabase.

Además, **el reset de contraseña no enviaba nada**: usaba `admin/generate_link`, que
genera el link pero no manda el mail. Se confirmó contra el proyecto real (`followups.md`).

## Alcance

**Entra (backend):**

- Resolución del origin del frontend por request, con allow-list.
- Metadata del invite para personalizar la plantilla.
- Las cuatro plantillas versionadas y el script que las publica.
- El fix del reset de contraseña.
- Logs de error en el handler de invitaciones.

**No entra:**

- Branding por tenant (decisión de diseño) y mails en más de un idioma.
- Cambiar el vencimiento de 7 días de la invitación en la base.
- Reemplazar a Supabase como emisor.
- La configuración manual de Resend, DNS y SMTP, que está en el runbook
  `docs/_process/EMAIL_SETUP.md`.

## Requisitos

### URL por instancia

- **RF-001** `origen: original` (§Arquitectura, §1) — El backend DEBE tomar el origin del
  frontend que disparó el mail del header `X-App-Base-URL` (lo manda el BFF) y usarlo solo
  si está en la allow-list `APP_ALLOWED_ORIGINS`. Si no está, DEBE usar `APP_BASE_URL` y
  loguear un `warn` con el origin rechazado.
  → `apimw.AppBaseURLFromHeader`, `apporigin.AllowList.Resolve`.
- **RF-002** `origen: original` (§1, reglas 1 a 4) — La validación del origin DEBE:
  - normalizar el valor (espacios, minúsculas, barra final);
  - rechazar lo que no sea una URL absoluta `http` o `https`;
  - comparar el origin completo por **igualdad exacta**, nunca por prefijo, porque
    `https://embolsadora.site.atacante.com` sería un open redirect enviado por mail;
  - aceptar entradas `https://*.dominio` solo para hosts con al menos una etiqueta propia
    antes del dominio.
  → `apporigin.Parse`, `Normalize`, `Allows`.
- **RF-003** `origen: derivado` — El backend DEBE loguear al arrancar cuántas entradas
  exactas y cuántas con comodín cargó de la allow-list, para que una mala configuración se
  vea en el arranque y no en un `warn` por request.
  → `logger.Info("app origin allow-list", …)` en `internal/routes/url_mappings.go`.
- **RF-004** `origen: original` (§4) — El `redirect_to` de la invitación, del reenvío y
  del reset de contraseña DEBE ser `{base}/s/{tenantId}/auth/callback`, con `{base}`
  resuelto por RF-001.
  → `callbackURL` (`internal/api/usecases/invite_metadata.go`).

### Contenido del mail

- **RF-005** `origen: original` (§2) — El invite a Supabase DEBE llevar en `data` los
  campos `tenant_name`, `inviter_name` y `role_name`, que la plantilla usa.
  → `supabase.AdminClient.InviteUserByEmail`, `resolveInviteDisplayNames`.
- **RF-006** `origen: original` (§2, "Degradación") — Si falla la resolución del nombre
  del tenant o del rol, el campo DEBE ir vacío y el envío continuar. Se loguea el error
  pero no se propaga.
  → `invite_metadata.go`, tests `TestResolveInviteDisplayNames_*`.
- **RF-007** `origen: original` (§3) — Las cuatro plantillas (invite, recovery,
  confirmation, magic link) DEBEN versionarse en `emails/`, en español y con marca
  Embolsadora, y **toda variable DEBE tener fallback**, porque los usuarios invitados
  antes del cambio no tienen metadata.
  → `emails/*.html`.
- **RF-008** `origen: original` (§3, "Publicación"; §Verificación 4) — DEBE existir un
  render local de las plantillas con datos completos y vacíos, y un script que las publique
  en Supabase por la Management API.
  → `cmd/renderemails`, `scripts/publish-email-templates.sh`, `emails/README.md`.

### Reset y observabilidad

- **RF-009** `origen: original` (§4) — El reset de contraseña forzado DEBE enviar el mail
  de verdad, usando `POST /auth/v1/recover` con `redirect_to`, en lugar de
  `admin/generate_link`.
  → `AdminClient.SendPasswordResetEmail`, `PasswordUsecase.ForcePasswordChange`.
- **RF-010** `origen: original` (§5) — El handler de alta de invitaciones DEBE loguear el
  error subyacente antes de responder 500.
  → `usecases.Log.Error("create invitation failed", …)` en `create_invitation.go`.

### Configuración

- **RNF-001** `origen: original` (§Configuración) — `APP_ALLOWED_ORIGINS` es una variable
  nueva y `APP_BASE_URL` pasa a ser solo el fallback. Las dos están documentadas en
  `.env.example`.

## Criterios de éxito

Vienen de la sección "Verificación" del diseño, pasos 1 a 5.

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | El validador acepta un match exacto, rechaza `https://embolsadora.site.atacante.com`, normaliza barra final y mayúsculas, hace caer el vacío al fallback, trata el comodín como se especificó y rechaza lo que no es URL | `internal/platform/apporigin/*_test.go` (`TestResolve`, `TestParse_EntradasInvalidasSeIgnoran`, …) |
| **CE-002** | El payload del invite lleva los tres campos | `internal/platform/supabase/admin_client_test.go`, `invite_metadata_test.go` |
| **CE-003** | Si falla el repo de tenants o de roles, el invite se manda igual con el campo vacío | `TestResolveInviteDisplayNames_Falla*` |
| **CE-004** | Las cuatro plantillas se renderizan con datos completos y vacíos | `go run ./cmd/renderemails` (manual) |
| **CE-005** | E2E: un invite desde `localhost` y otro desde `embolsadora.site` traen cada uno su URL y salen del remitente propio | **Manual, sin registro de ejecución en el repo.** `followups.md` marca la tarea 11 (configuración y E2E) como no ejecutada al cierre del plan. `EMAIL_SETUP.md` documenta la configuración posterior, con las variables de Cloud Run como pendiente |

## Decisiones

| Decisión | Elegido | Descartado | Fuente |
|---|---|---|---|
| Quién envía | Supabase renderiza y envía; el backend agrega metadata | Envío propio desde Go con un proveedor de mail | design, tabla de decisiones |
| Cómo llega la URL | Header `X-App-Base-URL` más allow-list | Solo corregir variables de entorno; que el BFF mande el `redirect_to` completo | ídem |
| SMTP | Resend con dominio `embolsadora.site` | SMTP compartido de Supabase, DonWeb, Postmark | ídem |
| Diseño del mail | Sobrio, marca Embolsadora, sin branding por tenant | Tarjeta con tabla de datos; branding por tenant | ídem |
| Comparación de origins | Igualdad exacta | Prefijo | §1 (open redirect) |

## Riesgos y preguntas abiertas

- **Verificación E2E sin evidencia** (CE-005). Confirmar en producción que
  `APP_ALLOWED_ORIGINS` está seteada en Cloud Run y que los mails salen con la URL
  correcta. Sin esa variable, todos los mails vuelven a usar `APP_BASE_URL`.
- **`token_hash` y `verifyOtp`:** el cambio de flujo del callback está analizado en
  `research-tokenhash-migration.md`. Afecta sobre todo al frontend y a las plantillas.
- **Vencimiento:** el mail dice 24 horas (el tope real del token de GoTrue) y la
  invitación en la base vence a los 7 días. Es intencional (§3), pero puede confundir.
- Los diferidos restantes, con su criticidad, están en `followups.md`.

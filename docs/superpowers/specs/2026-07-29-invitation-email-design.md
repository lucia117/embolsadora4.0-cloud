# Mails de autenticación: plantillas propias y URL por instancia

**Fecha:** 2026-07-29
**Estado:** diseño aprobado, pendiente de plan de implementación
**Repos afectados:** `embolsadora4.0-cloud` (backend Go), `embolsadora-frontend` (BFF Next.js), proyecto Supabase `cdjehkbidqqsldaajbui`

---

## Problema

El mail de invitación que reciben los usuarios tiene dos defectos.

**Diseño.** Es la plantilla default de Supabase: un `<h2>`, un párrafo y un link azul, en inglés, sin marca, enviado desde `noreply@mail.app.supabase.io` por la infraestructura de correo compartida de Supabase. Esa infraestructura está documentada como solo para desarrollo y tiene un rate limit muy bajo, incompatible con un flujo real de invitaciones.

**URL.** El link llega siempre con `http://localhost:3000`, incluso cuando la invitación se dispara desde producción. Hay dos causas distintas:

1. El texto "…create a user on http://localhost:3000" viene de `{{ .SiteURL }}`, un valor global del proyecto Supabase.
2. El `redirect_to` del link se arma en `internal/api/usecases/invitation_usecase.go:93` como `{APP_BASE_URL}/s/{tenantId}/auth/callback`. `APP_BASE_URL` es una env var única, y en Cloud Run vale `http://localhost:3000`.

La causa raíz de (2) es estructural: **un solo backend en Cloud Run atiende tanto al frontend local como a `embolsadora.site`**, así que ninguna env var puede por sí sola distinguir desde qué instancia se originó la invitación. Y como `backendFetch` (`src/lib/backend-fetch.ts:78`) llama al backend server-to-server, el header `Origin` del navegador tampoco llega al Go.

---

## Decisiones tomadas

| Decisión | Elegido | Descartado |
|---|---|---|
| Quién envía | Supabase renderiza y envía; el backend enriquece con `user_metadata` | Envío propio desde Go con `generate_link` + ESP |
| Cómo llega la URL | Header `X-App-Base-URL` desde el BFF + allow-list en el backend | Solo arreglar env vars; que el BFF mande el `redirect_to` completo |
| SMTP | Resend, dominio `embolsadora.site` | Servicio compartido de Supabase; SMTP de DonWeb; Postmark |
| Diseño del mail | Sobrio, marca Embolsadora, sin color, datos en prosa | Tarjeta con tabla de datos; branding por tenant |
| Alcance de plantillas | Las cuatro: invite, recovery, confirmation, magic link | Solo invite |

---

## Arquitectura

### Flujo de la URL por instancia

```
Browser (embolsadora.site | localhost:3000 | preview de Vercel)
   │  POST /api/invitations          ← acá sí llega el header Origin
   ▼
BFF Next.js — src/app/api/invitations/route.ts
   │  appBaseUrl = Origin ?? (x-forwarded-proto + host)
   │  POST /api/v1/invitations  +  X-App-Base-URL: https://embolsadora.site
   ▼
Backend Go
   │  valida contra APP_ALLOWED_ORIGINS (match exacto de origin)
   │  si no matchea → APP_BASE_URL, con log warn
   │  redirect_to = {base}/s/{tenantId}/auth/callback
   ▼
GoTrue POST /auth/v1/invite?redirect_to=…  +  body { email, data }
   ▼
Resend SMTP → mail al invitado
```

### Componentes y responsabilidades

**`resolveAppBaseURL` (backend, nuevo)** — Recibe el valor crudo del header y la config; devuelve un base URL confiable. Es la única pieza con superficie de seguridad del diseño y por eso vive aislada y testeada aparte. No conoce nada de invitaciones.

**`OriginAllowList` (backend, nuevo, en config)** — Parsea `APP_ALLOWED_ORIGINS` y responde una pregunta: `¿este origin está permitido?`. Comparación por **match exacto**, nunca por prefijo.

**`AdminClient.InviteUserByEmail` (backend, modificado)** — Cambia la firma para aceptar un struct de payload en vez de solo `email`. Su contrato con el resto del sistema no cambia: sigue devolviendo `error` o `nil`.

**`InvitationUsecase` (backend, modificado)** — Suma dependencias de los repos de `tenants` y `roles` para resolver nombres. La resolución es best-effort: su falla no aborta el envío.

**`backendFetch` (frontend, modificado)** — Suma un campo opcional `appBaseUrl` en `BackendFetchOptions` que se inyecta como header, siguiendo exactamente el patrón de `X-Tenant-ID` (línea 74).

**Rutas BFF que deben mandar el header — son tres, no una:**

| Ruta | Mail que dispara |
|---|---|
| `POST /api/invitations` | Invitación nueva |
| `POST /api/invitations/[id]/resend` | Reenvío de invitación |
| `POST /api/users/[id]/force-password-change` | Reset de contraseña forzado por un admin |

No existe hoy un flujo público de "olvidé mi contraseña" en el frontend: el reset solo lo dispara un administrador sobre otro usuario.

**`emails/` (backend, nuevo)** — Las cuatro plantillas HTML como fuente de verdad versionada, más un script que las publica en Supabase.

---

## Especificación

### 1. Resolución del base URL

`APP_ALLOWED_ORIGINS` es una lista separada por comas de origins completos. Una entrada puede además tener la forma `https://*.dominio` para habilitar un conjunto de subdominios — el único caso previsto son los previews de Vercel, cuyas URLs no se pueden enumerar de antemano:

```
APP_ALLOWED_ORIGINS=https://embolsadora.site,http://localhost:3000,https://*.vercel.app
```

Reglas de validación, en orden:

1. Normalizar el valor recibido: recortar espacios, pasar a minúsculas, quitar la barra final.
2. Rechazar si no parsea como URL absoluta con esquema `http` o `https`.
3. Comparar el **origin completo** (esquema + host + puerto) contra cada entrada literal del allow-list por igualdad exacta.
4. Para las entradas con `*.`, aceptar solo si el esquema coincide y el host termina en `.dominio` con **al menos una etiqueta propia** antes del punto. `https://vercel.app` a secas no matchea `https://*.vercel.app`.
5. Si nada matchea → devolver `APP_BASE_URL` y loguear warn con el origin rechazado.

El wildcard es una entrada opcional: si `https://*.vercel.app` no está en la lista, los previews caen al fallback de producción y siguen funcionando, solo que con la URL de prod en el mail.

La comparación por prefijo queda **prohibida**: `https://embolsadora.site.atacante.com` la pasaría, y el resultado se usa para construir un link que se manda por mail. Es un open-redirect con entrega incluida.

### 2. Payload del invite

El body que el backend manda a `/auth/v1/invite`:

```json
{
  "email": "usuario@ejemplo.com",
  "data": {
    "tenant_name": "MRG SRL",
    "inviter_name": "Federico De Giovanni",
    "role_name": "Operador"
  }
}
```

`inviter_name` sale del `callerUser` que ya está en el contexto del request — sin costo. `tenant_name` y `role_name` requieren una query cada uno.

**Degradación:** si cualquiera de las dos queries falla, el campo va vacío y el envío continúa. Se loguea el error; no se propaga.

### 3. Plantillas

Cuatro archivos en `emails/`: `invite.html`, `recovery.html`, `confirmation.html`, `magic-link.html`.

Restricciones técnicas comunes:

- Layout con `<table>`, ancho máximo 600px. Sin flexbox ni grid en la estructura.
- CSS **inline** en cada elemento. Muchos clientes descartan el `<style>` del head.
- El wordmark "Embolsadora" es texto, no imagen: no depende de que el cliente cargue recursos remotos.
- Botón de acción sólido (`#111827`), y debajo el mismo link en texto plano para cuando el botón no renderiza.
- Español rioplatense, voseo, en línea con `locale: es-AR` de los tenants.
- Sin `{{ .SiteURL }}` en ninguna plantilla — es el valor global que causaba el bug original.

Solo `invite.html` usa datos de tenant. Las otras tres son genéricas con el mismo estilo.

**Toda variable lleva fallback.** Los usuarios invitados antes de este cambio no tienen metadata, y recovery/confirmation/magic-link no tienen contexto de tenant en absoluto:

```
{{ if .Data.tenant_name }}
  Te invitaron a sumarte a {{ .Data.tenant_name }} en Embolsadora.
{{ else }}
  Te invitaron a sumarte a Embolsadora.
{{ end }}
```

**Vencimiento:** el mail dice "el link vence en 24 horas", que es el tope real del token de GoTrue. No usa `expires_at` de la DB, que son 7 días (`migrations/000001_initial_schema.up.sql:340`) y corresponde a la vida del registro para permitir reenvíos. Son dos relojes distintos; el mail solo habla del que le importa al invitado.

**Publicación:** un script sube las plantillas y los subjects vía Management API de Supabase (`PATCH /v1/projects/{ref}/config/auth`, campos `mailer_templates_*_content` y `mailer_subjects_*`). Editar las plantillas en el dashboard queda prohibido por convención: se pisa en la próxima publicación.

**A verificar durante la implementación:** si el campo *subject* admite variables de template. Si las admite, el asunto del invite es `Te invitaron a {{ .Data.tenant_name }}`; si no, queda fijo en `Te invitaron a Embolsadora`.

### 4. Fix del reset de contraseña

`SendPasswordResetEmail` (`internal/platform/supabase/admin_client.go:60`) llama a `/auth/v1/admin/generate_link` con `type=recovery`. Ese endpoint genera y devuelve el link pero **no envía el mail** — está pensado para cuando el envío lo hace uno mismo. Además `doWithRetry` descarta el body de la respuesta, así que el link generado se pierde.

**Primer paso de la implementación: verificarlo con una prueba real.** Si se confirma, el reemplazo es `POST /auth/v1/recover`, que sí envía y también acepta `redirect_to` — o sea que arrastra el mismo problema de URL por instancia y se resuelve con el mismo `resolveAppBaseURL`.

Eso implica cambiar la firma de `SendPasswordResetEmail`, que hoy recibe solo el email y no tiene por dónde recibir un `redirect_to`. `ForcePasswordChange` (`internal/api/usecases/password_usecase.go:62`) es su único llamador.

El destino del `redirect_to` de recovery no es el mismo que el de invite: la invitación va a `/s/{tenantId}/auth/callback`, mientras que el reset tiene que llevar a la pantalla de cambio de contraseña. **A definir en el plan de implementación** cuál es esa ruta exacta en el frontend.

### 5. Observabilidad

El handler de invitación devuelve 500 sin loguear el error subyacente. Es un gap ya conocido, y sin él cualquier falla de SMTP o de allow-list va a ser invisible en producción. Se corrige en este trabajo porque la ruta se toca igual.

---

## Configuración

### Variables de entorno

| Variable | Dónde | Valor |
|---|---|---|
| `APP_ALLOWED_ORIGINS` | Cloud Run (nueva) | `https://embolsadora.site,http://localhost:3000` |
| `APP_BASE_URL` | Cloud Run (existente) | `https://embolsadora.site` — pasa a ser el fallback, ya no la única fuente |

### Supabase Auth (dashboard, manual)

- **SMTP Settings**: host, puerto, usuario y contraseña de Resend. Remitente `no-responder@embolsadora.site`, nombre "Embolsadora".
- **Redirect URLs**: deben incluir `https://embolsadora.site/**` y `http://localhost:3000/**`. Sin esto GoTrue descarta el `redirect_to` y manda al Site URL, y el fix del backend no sirve de nada.
- **Site URL**: `https://embolsadora.site`.

### DNS en DonWeb

Tres registros para `embolsadora.site`, con los valores que da el panel de Resend al dar de alta el dominio: SPF, DKIM, y DMARC en `p=none` para arrancar y poder endurecerlo después.

---

## Verificación

De barato a caro:

1. **Unit Go — validador de origin.** Match exacto pasa; `https://embolsadora.site.atacante.com` se rechaza; barra final y mayúsculas se normalizan; string vacío cae al fallback; `.vercel.app` pasa solo con previews habilitados; un valor que no es URL absoluta se rechaza.
2. **Unit Go — payload del invite.** Que `data` lleve los tres campos. El mock del `AdminClient` ya existe (uber/mock).
3. **Unit Go — degradación.** Repo de tenants devolviendo error, y el invite se manda igual con el campo vacío.
4. **Render local de plantillas.** Un script que escupa los cuatro HTML con datos completos y con datos vacíos. Sin esto la rama `{{ else }}` no la ve nadie hasta que un usuario real recibe un mail roto.
5. **E2E manual.** Invitar desde `localhost:3000` y desde `embolsadora.site`; confirmar que cada mail trae su propia URL, sale de `no-responder@embolsadora.site`, y que el link efectivamente lleva al callback correcto.

Los comandos de Go corren vía Docker: en esta máquina no hay Go instalado en el host (ver `CLAUDE.md`).

---

## Fuera de alcance

- **Branding por tenant en el mail.** El theme (colores, logo) vive en `tenants/tenants.json` del frontend; la tabla `tenants` del backend solo tiene name, company_name, subdomain, description e is_active. Hacerlo requeriría propagar color y logo por toda la cadena o replicar el branding en la DB.
- **Mails en más de un idioma.** Todo en español rioplatense.
- **Cambiar el vencimiento de 7 días** del registro de invitación en la DB.
- **Reemplazar Supabase como emisor.** El envío propio desde Go queda descartado en esta iteración.

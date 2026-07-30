# Migración a `token_hash` + `verifyOtp` en los mails de auth

**Fecha:** 2026-07-29
**Rama:** `feat/auth-emails` (en ambos repos)
**Repos:** `embolsadora4.0-cloud` (backend Go) · `embolsadora-frontend` (Next.js 16)

## Problema

La rama arregló los mails de invitación que siempre traían `http://localhost:3000`, pero el
flujo seguía roto de punta a punta.

Las plantillas apuntaban a `{{ .ConfirmationURL }}`, o sea al `/auth/v1/verify` de GoTrue.
Ese endpoint responde:

```
HTTP/2 303
location: http://localhost:3000/s/mrgsrl/auth/callback#access_token=…&expires_at=…&refresh_token=…&type=invite
```

El token llega en el **fragmento** de la URL. Un fragmento nunca sale del browser, así que el
Route Handler del lado del servidor jamás lo ve y `exchangeCodeForSession` no tiene nada que
intercambiar. Pasa porque estos links se emiten del lado del servidor sin `code_challenge`
PKCE, y GoTrue cae al flujo implícito.

## Solución

El patrón server-side documentado: linkear directo a nuestro propio callback llevando
`token_hash` y `type` como parámetros de **query**, y que el callback llame a `verifyOtp`.
Así se saltea `/auth/v1/verify` por completo y nunca se produce un fragmento.

### Backend — las cuatro plantillas de `emails/`

Cada `{{ .ConfirmationURL }}` pasó a ser:

```
{{ .RedirectTo }}?token_hash={{ .TokenHash }}&type=<tipo de esa plantilla>
```

`{{ .RedirectTo }}` es el `redirect_to` que mandó nuestro backend — es lo que preserva la URL
por instancia, que es el punto de toda esta rama. No se introdujo `{{ .SiteURL }}` en ningún
lado (es un valor global del proyecto, y era el bug original).

| plantilla | `type` | fuente del TokenHash en GoTrue |
|---|---|---|
| `emails/invite.html` | `invite` | `user.ConfirmationToken` |
| `emails/confirmation.html` | `signup` | `user.ConfirmationToken` |
| `emails/recovery.html` | `recovery` | `user.RecoveryToken` |
| `emails/magic-link.html` | `magiclink` | `user.RecoveryToken` |

Son **8 ocurrencias de link** (2 por plantilla × 4): el `href` del botón y el link de fallback
en texto plano. Como el fallback muestra la URL además de linkearla, son **12 sustituciones
textuales** de `{{ .ConfirmationURL }}` en total (3 por archivo).

El `git diff` confirma que sólo cambiaron líneas de URL — 4 líneas por archivo, 8 inserciones
y 8 borrados en total. La copy en castellano rioplatense quedó intacta.

### Backend — el renderer

`cmd/renderemails/main.go` sumó los campos `RedirectTo` y `TokenHash` al struct `templateData`,
poblados en los tres casos existentes (`completo`, `vacio`, `nil`) para que la salida renderizada
muestre la forma real del link. Se mantuvo `ConfirmationURL` en el struct: ya no lo usa ninguna
plantilla, pero sigue reflejando el contrato real de GoTrue.

Los tres casos siguen ahí, así que la guarda de `nil` en `.Data` no regresionó — verificado:
`nil-invite.html` y `vacio-invite.html` renderizan las 2 ocurrencias del link igual que
`completo-invite.html`.

`RedirectTo` usa la forma con UUID (`/s/11b36b85-…/auth/callback`) porque es lo que emite de
verdad el backend Go, que sólo conoce UUIDs de tenant. Eso ejercita justamente el caso que hace
falta que el frontend canonicalice.

## Links renderizados

Salida real de `go run ./cmd/renderemails` (caso `completo`), citada textual:

**`invite.html`**
```
http://localhost:3000/s/11b36b85-033d-4bb3-9e31-4c92161887c0/auth/callback?token_hash=pkce_2f8a1c9e7b4d6a03f5e1c8b2d947a6e0f3c15b8d2a7e94c6b1f0d385&type=invite
```

**`confirmation.html`**
```
http://localhost:3000/s/11b36b85-033d-4bb3-9e31-4c92161887c0/auth/callback?token_hash=pkce_2f8a1c9e7b4d6a03f5e1c8b2d947a6e0f3c15b8d2a7e94c6b1f0d385&type=signup
```

**`recovery.html`**
```
http://localhost:3000/s/11b36b85-033d-4bb3-9e31-4c92161887c0/auth/callback?token_hash=pkce_2f8a1c9e7b4d6a03f5e1c8b2d947a6e0f3c15b8d2a7e94c6b1f0d385&type=recovery
```

**`magic-link.html`**
```
http://localhost:3000/s/11b36b85-033d-4bb3-9e31-4c92161887c0/auth/callback?token_hash=pkce_2f8a1c9e7b4d6a03f5e1c8b2d947a6e0f3c15b8d2a7e94c6b1f0d385&type=magiclink
```

Cada uno aparece dos veces por archivo (botón + fallback), y el fallback además lo muestra como
texto visible. Ninguna URL apunta ya a `/auth/v1/verify`, y ninguna contiene `SiteURL`.

La copy en castellano quedó igual, por ejemplo en `invite.html`:

> Si el botón no funciona, copiá y pegá este link en tu navegador:

## Frontend — el Route Handler del callback

`src/app/s/[tenantId]/auth/callback/route.ts`.

### Cómo se estrechó el union de `type`

`type` sale de la query como `string | null`. En vez de castearlo a ciegas, se declaró la lista
de valores válidos como un array en runtime, chequeado contra el union del propio SDK:

```ts
const EMAIL_OTP_TYPES = [
  'invite', 'signup', 'recovery', 'magiclink', 'email_change', 'email',
] as const satisfies readonly EmailOtpType[];

function toEmailOtpType(value: string | null): EmailOtpType | null {
  return EMAIL_OTP_TYPES.includes(value as (typeof EMAIL_OTP_TYPES)[number])
    ? (value as EmailOtpType)
    : null;
}
```

El `satisfies readonly EmailOtpType[]` hace que el compilador rechace cualquier entrada que no
sea un `EmailOtpType` real, así que la lista no puede desincronizarse del SDK sin romper `tsc`.
La definición del union en `@supabase/auth-js@2.99.0` es
`'signup' | 'invite' | 'magiclink' | 'recovery' | 'email_change' | 'email'`.

Un `type` no reconocido resuelve a `null`. Si viene un `token_hash` que no se puede aparear con
un `type` válido, el handler redirige a login en vez de crashear:

```ts
if (tokenHash && !type) {
  return redirectTo(loginUrl);
}
```

### Cómo se preservó el fallback de OAuth

Las dos ramas son mutuamente excluyentes, con `token_hash` teniendo prioridad:

```ts
if (tokenHash && type) {
  // link de mail: verifyOtp
} else if (code) {
  // proveedor OAuth: exchangeCodeForSession
}
```

La rama de `code` no se tocó en su comportamiento. Los proveedores OAuth sí usan PKCE y siguen
volviendo con `?code=`, y este callback es compartido con ellos. Como los links de mail nunca
traen `code` y los callbacks de OAuth nunca traen `token_hash`, no hay ambigüedad.

### Lo que se preservó exactamente

- `error` en la query → redirect a login (antes de todo lo demás, sin cambios).
- Verify/exchange fallido → redirect a login, nunca a un dashboard sin sesión.
- `type === 'recovery'` tras un verify exitoso → `/s/{slug}/auth/change-password`; si no →
  `/s/{slug}/dashboard`. Sigue funcionando para ambas ramas: `'recovery'` está en el union, así
  que un flujo PKCE de recovery con `?code=&type=recovery` también sigue yendo a change-password.
- **Canonicalización del slug**: `getTenantById(tenantId)?.id ?? tenantId` se calcula una sola vez
  en `tenantSlug` y alimenta las tres URLs de destino — `loginUrl`, `change-password` y
  `dashboard`. Todos los `return` del handler pasan por `redirectTo(...)` con un path armado a
  partir de `tenantSlug`, incluidos los caminos de falla. Verificado: hay 6 `return redirectTo`
  en el handler; 4 usan `loginUrl` (que se arma con `tenantSlug`) y 2 interpolan `tenantSlug`
  directo. Ninguno usa el `tenantId` crudo.
- Origen del redirect derivado del request entrante (`x-forwarded-host` / `x-forwarded-proto`,
  con fallback a `request.nextUrl.origin`), sin hardcodear.
- `src/lib/supabase/server.ts` no se tocó.

## Verificación

### Backend

Todo vía Docker (`golang:1.24-alpine`), porque Go no está instalado en el host.

```
=== BUILD OK ===      go build ./...
=== VET OK ===        go vet ./...
=== TEST DONE ===     go test ./internal/...
```

Ningún paquete falló. Los que tienen tests dieron `ok`:

```
ok  github.com/tu-org/embolsadora-api/internal/core/errors            0.017s
ok  github.com/tu-org/embolsadora-api/internal/platform/apporigin     0.018s
ok  github.com/tu-org/embolsadora-api/internal/platform/dbmigrate     0.017s
ok  github.com/tu-org/embolsadora-api/internal/platform/supabase      0.042s
ok  github.com/tu-org/embolsadora-api/internal/repo/pg/tenants        0.008s
ok  github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles     0.008s
ok  github.com/tu-org/embolsadora-api/internal/repo/pg/users          0.008s
ok  github.com/tu-org/embolsadora-api/internal/security               0.692s
```

El resto son `[no test files]`.

Renderer — 12 archivos escritos, 4 plantillas × 3 casos:

```
=== RENDER OK ===
escrito tmp/emails/nil-confirmation.html
escrito tmp/emails/completo-confirmation.html
escrito tmp/emails/vacio-confirmation.html
escrito tmp/emails/nil-invite.html
escrito tmp/emails/completo-invite.html
escrito tmp/emails/vacio-invite.html
escrito tmp/emails/nil-magic-link.html
escrito tmp/emails/completo-magic-link.html
escrito tmp/emails/vacio-magic-link.html
escrito tmp/emails/nil-recovery.html
escrito tmp/emails/completo-recovery.html
escrito tmp/emails/vacio-recovery.html
```

### Frontend

Node 22.17.1 (`nvm use 22`). Los tres comandos en foreground:

```
=== TSC ===       pnpm tsc --noEmit          → TSC OK (sin salida, sin errores)
=== ESLINT ===    pnpm exec eslint route.ts  → ESLINT OK (sin salida, sin warnings)
=== BUILD ===     pnpm build                 → ✓ Compiled successfully in 24.9s
```

La ruta del callback quedó registrada como dinámica, que es lo correcto para un Route Handler
que lee la query y escribe cookies:

```
├ ƒ /s/[tenantId]/auth/callback
```

No se corrió `pnpm lint` a nivel repo — tiene ~165k problemas preexistentes por un directorio
`.worktrees/**/.next` que se escanea de más. Conocido y fuera de alcance.

## Autorevisión

- **¿Las 8 ocurrencias de link actualizadas (2 por plantilla × 4)?** Sí. `git diff --stat` da
  4 líneas cambiadas por archivo en los 4 archivos (8 inserciones / 8 borrados), y son
  exactamente la línea del `href` del botón y la del fallback. El fallback lleva la URL dos veces
  (atributo + texto visible), así que son 12 sustituciones textuales. `grep` confirma cero
  `{{ .ConfirmationURL }}` restantes en `emails/`.
- **¿La canonicalización del slug intacta en todos los destinos, incluidos los de falla?** Sí.
  Un solo `tenantSlug` alimenta los 6 `return redirectTo` del handler.
- **¿El fallback de OAuth con `code` sigue funcionando?** Sí, la rama `else if (code)` conserva
  `exchangeCodeForSession` y su manejo de error sin cambios de comportamiento.
- **¿Se coló `{{ .SiteURL }}`?** No. `grep` sobre `emails/` da 0 ocurrencias.

## Observaciones

1. **`&` sin escapar en el `href`.** `html/template` de Go emite `&type=invite` literal, no
   `&amp;type=invite`. Técnicamente un `&` suelto en un atributo no es HTML bien formado, pero
   todos los browsers y clientes de mail lo parsean bien porque `&type;` no es una entidad
   válida. Como GoTrue renderiza con el mismo `html/template`, la salida en producción va a ser
   idéntica a la del renderer. No lo cambié para no tocar lo que ya está verificado, pero queda
   anotado.

2. **El renderer no valida contra GoTrue real.** Reproduce el mapa de datos a mano a partir de
   la lectura de `internal/mailer/templatemailer/templatemailer.go`. Si GoTrue renombra un campo
   en una versión futura, el renderer va a seguir pasando en verde mientras los mails reales
   salen con el link vacío. Un mail de invitación real end-to-end sigue siendo el único chequeo
   que cierra esto del todo.

3. **Un `token_hash` con `type` inválido va a login sin señal.** Es el comportamiento pedido (no
   crashear), pero no queda rastro de por qué falló. Si aparecen reportes de links que "no hacen
   nada", no hay observabilidad en el handler para diagnosticar. Agregar logging estructurado en
   los caminos de falla sería un seguimiento razonable — lo dejé afuera por estar fuera de alcance.

4. **Falta el checkpoint empírico.** Todo lo verificado acá es estático: compila, lintea, buildea
   y el renderer produce la forma de link correcta. No mandé un mail de invitación real ni recorrí
   el redirect en un browser. Dado que el chequeo empírico anterior fue el que descubrió el
   problema del fragmento, recomiendo fuerte disparar una invitación real contra Supabase y
   seguir el link antes de dar la feature por cerrada.

5. **Las plantillas hay que republicarlas en Supabase.** Editar `emails/` no actualiza nada por
   sí solo: hay un script de publicación en la rama para eso. Los mails van a seguir saliendo con
   el `ConfirmationURL` viejo hasta que se corra.

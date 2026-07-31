# Mails de autenticación — pendientes y hallazgos diferidos

Cierre de la ejecución del plan `2026-07-29-invitation-email.md`. Las tareas 1 a 10
están implementadas y revisadas en las ramas `feat/auth-emails` de ambos repos. La
tarea 11 es checklist manual del operador y **no fue ejecutada**.

## Bloqueante para dar la feature por terminada

**La tarea 11 del plan.** Alta de Resend, registros DNS en DonWeb, SMTP y Redirect
URLs en Supabase, env vars en Cloud Run, deploy, y las pruebas end-to-end. Empezar
por el **Step 0**, que se agregó durante la ejecución: mandar una invitación de
prueba y mirar el link *antes* de tocar DNS o SMTP. Si el token llega en el
fragmento (`#access_token=`) en vez de `?code=`, hay que reescribir las plantillas
a `{{ .TokenHash }}` y el callback a `verifyOtp` — y es mucho mejor descubrirlo
antes de invertir horas en configuración.

## Cosas que estaban rotas desde antes y se arreglaron acá

- **El link del mail apuntaba a `/s/{uuid}/auth/callback`** mientras el frontend
  resuelve tenants por slug (`mrgsrl`, `cordoba`). Todo link de invitación enviado
  hasta hoy daba 404. `getTenantById` ahora resuelve también por `backendId`.
- **El callback era un Server Component**, y Next.js no permite escribir cookies
  desde ahí: el `catch` vacío de `src/lib/supabase/server.ts` se comía el error de
  `exchangeCodeForSession`, así que la sesión nunca persistía y el invitado
  terminaba en la pantalla de login. Convertido a Route Handler.
- **El reset de contraseña no enviaba nada**: usaba `admin/generate_link`, que acuña
  el link y lo devuelve en el body en vez de mandarlo. Confirmado contra el proyecto
  real. Ahora usa `/auth/v1/recover`.

## Diferidos, con su criticidad

**Vale la pena atacar:**

- `src/app/s/[tenantId]/auth/callback/route.ts` — el intercambio del código corre
  *antes* de validar que el tenant resuelva. Con un tenant inexistente, el código de
  un solo uso se consume y recién después el usuario ve el 404, quedándose con un
  link de invitación quemado y sin forma de reintentar. Hoy es inalcanzable: los
  cuatro tenants de `tenants.json` tienen `backendId`. Es una trampa para el próximo
  tenant que se dé de alta sin él.
- Un intercambio fallido redirige a login **sin dejar rastro**: en producción no se
  distingue "invitación vencida" de "Supabase caído". Un span de OTel es el lugar
  correcto (no `console.error`, por el presupuesto de warnings de ESLint).
- `exchangeCodeForSession` no está envuelto en try/catch — una falla de red daría un
  500 en vez de un redirect a login. Es el patrón preexistente del resto de la app,
  pero esta es la ruta crítica de invitación.
- **`pnpm lint` está roto en el frontend**: reporta ~165.000 problemas porque
  `eslint .` escanea `.worktrees/roles-permissions-fix/.next`, que son artefactos de
  build. Ignorar ese directorio en la config de ESLint.

**Menores, anotados y no urgentes:**

- `apporigin.Parse` acepta una entrada wildcard con path (`https://*.vercel.app/foo`)
  como entrada inerte en vez de descartarla — falla cerrada, pero contradice su doc.
- `Normalize` no quita el punto final de un FQDN (`https://embolsadora.site.`); solo
  produce falsos negativos.
- Ningún test cubre el guard `candidate != ""` del warn en `AppBaseURLFromHeader`.
- El test de recovery no assertea que el body tenga exactamente una key.
- `x-forwarded-proto` se lee sin `split(',')`: una cadena de proxies daría
  `"https,http://host"`. Mitigado por el allow-list del backend.
- `host.startsWith('localhost')` no cubre `127.0.0.1` ni IPs de LAN en dev sin proxy.
- `emails/README.md` no repite la advertencia dura del header del script: una edición
  hecha en el dashboard de Supabase se pisa en la próxima publicación.
- `getTenantById` no tiene test de regresión — el frontend no tiene runner.

## Decisión de merge

**La rama se mergea con squash.** Los commits de las tareas 4, 5 y 6 no compilan por
diseño (cambian firmas que la tarea 7 cablea), así que `main` no debe recibirlos
sueltos. El CI del backend dispara en push a `main` y buildea el estado mergeado, con
lo cual no se ve afectado; el costo era solo la bisectabilidad, y el squash lo elimina.

## Correcciones al plan durante la ejecución

El plan se equivocó tres veces, siempre sobre comandos o APIs de terceros que
afirmaba sin haber ejecutado. Las tres están corregidas en el propio documento y
marcadas como "Corrección aplicada durante la ejecución":

1. `{{ if index .Data "x" }}` como remedio para un `.Data` nulo — no funciona, tira
   `index of untyped nil`. El patrón correcto es un guard externo `{{ if .Data }}`.
2. El `curl` del script de publicación no detectaba fallas HTTP: ante un 401 imprimía
   el error y aun así declaraba éxito con exit 0.
3. El comando `gcloud` de la tarea 11 tenía el delimitador `^|^` embebido en el valor
   en vez de prefijando todo el argumento, con lo cual habría dejado la allow-list
   vacía y reinstalado el bug original en silencio.

El razonamiento de **diseño** del plan se sostuvo bajo revisión. Sus afirmaciones
sobre **líneas de comando y APIs ajenas** conviene tratarlas como no verificadas
hasta correrlas. La tarea 11 está compuesta enteramente de ese segundo tipo.

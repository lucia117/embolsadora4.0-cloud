# Configuración del envío de mails de autenticación

Runbook operativo para que los mails de invitación, reset de contraseña,
confirmación y magic link salgan desde `no-responder@embolsadora.site` en vez
del mailer compartido de Supabase.

**Estado al 2026-07-30: COMPLETADO.** Los Pasos 0 a 4 están ejecutados y
verificados. Los mails de auth salen por Resend desde
`no-responder@embolsadora.site`.

Resumen de lo aplicado:

- **Paso 0** — delegación DNS unificada en Vercel. El registrador quedó con
  `ns1/ns2.vercel-dns.com` únicamente; propagó en 3 minutos. El CNAME inválido
  del ápex desapareció y el sitio siguió respondiendo 200 durante todo el
  cambio.
- **Pasos 1 y 2** — dominio dado de alta en Resend (región `sa-east-1`). Con la
  delegación arreglada, el "Auto configure" (Domain Connect) funcionó y escribió
  los tres registros en Vercel. El DMARC se agregó aparte con `vercel dns add`.
- **Paso 3** — API key creada con permiso *Sending access* únicamente. Verificado
  que no puede listar dominios, solo enviar.
- **Paso 4** — SMTP configurado en Supabase por Management API. Verificado con
  read-back: host `smtp.resend.com`, puerto 465, remitente
  `no-responder@embolsadora.site`, `smtp_max_frequency` en 60 segundos.

El dominio se verificó funcionalmente antes de tocar el SMTP: se envió un mail
de prueba por la API de Resend, que habría sido rechazado si el dominio no
estuviese verificado. Ese orden importa — configurar el SMTP antes de que
Resend verifique deja todos los mails de auth fallando.

---

## Paso 0 — Arreglar la delegación DNS (bloqueante)

**Hay que hacer esto antes que nada.** No es una mejora opcional: sin esto, la
autenticación de los mails va a fallar de forma intermitente e imposible de
diagnosticar.

### El problema

El dominio está delegado a **dos proveedores distintos al mismo tiempo**.
Según los servidores autoritativos del TLD `.site`:

```
embolsadora.site.  900  IN  NS  ns1.donweb.com.
embolsadora.site.  900  IN  NS  ns2.donweb.com.
embolsadora.site.  900  IN  NS  ns1.vercel-dns.com.
embolsadora.site.  900  IN  NS  ns2.vercel-dns.com.
```

DonWeb y Vercel sirven zonas **diferentes**, y cada resolver del mundo elige
uno arbitrariamente. Para un sitio web da igual, porque los dos caminos llevan
a Vercel. Para mails es fatal: si el DKIM se carga en un solo proveedor, Gmail
lo va a encontrar únicamente cuando le toque consultar ese servidor. El resto
de las veces el mail llega sin firma válida y cae en spam — de forma
intermitente, que es la peor manera de fallar.

Esto también explica por qué el botón **"Auto configure"** de Resend devuelve
*"Domain Connect Failed — This domain is not using Vercel DNS"*: Vercel ve
nameservers ajenos en la delegación y se niega a escribir.

### Qué tiene cada zona hoy

Consultando cada servidor por separado:

| | `ns1.donweb.com` | `ns1.vercel-dns.com` |
|---|---|---|
| SOA | `ns3.hostmar.com` | `ns1.vercel-dns.com` |
| Contenido | **un solo CNAME en el ápex** → `cname.vercel-dns.com` | zona completa: NS propios, A del sitio (`216.198.79.65`, `64.29.17.1`) |
| MX / TXT / www | ninguno | ninguno |

La zona de DonWeb es un apaño para simular un ALIAS en el ápex: como todo
cuelga de ese CNAME, cualquier consulta (A, MX, TXT) devuelve lo mismo. **No
contiene nada que valga la pena conservar.**

Además, un CNAME en el ápex es inválido según RFC 1034 — impide que ese mismo
nombre tenga cualquier otro registro. Mientras exista, no se puede poner un SPF
en `embolsadora.site` a secas.

### La solución

En el registrador (**Dattatec / DonWeb**), dejar **únicamente** los
nameservers de Vercel:

```
ns1.vercel-dns.com
ns2.vercel-dns.com
```

Es decir: quitar `ns1.donweb.com` y `ns2.donweb.com`.

En el panel: *Mis servicios → Dominios → Gestionar → Nameservers y Zona DNS*.

**Por qué es seguro:** el sitio ya resuelve a Vercel por los dos caminos, así
que no cambia a dónde apunta. Y la zona de DonWeb no tiene ningún registro
propio que se pierda.

**Qué se gana:** Vercel pasa a ser la única autoridad, desaparece el CNAME
inválido en el ápex (Vercel resuelve el ápex correctamente con A/ALIAS),
el panel deja de decir "Third Party", y a partir de ahí los registros se
cargan en Vercel — por UI o por CLI.

### Verificar que se aplicó

```bash
whois embolsadora.site | grep -i "name server"
dig @8.8.8.8 NS embolsadora.site +short
```

Ambos deben mostrar solo los dos de Vercel. La propagación de un cambio de
nameservers puede tardar de minutos a 48 horas. **No seguir hasta que esté.**

---

## Paso 1 — Alta del dominio en Resend

1. Crear cuenta en https://resend.com/signup (gratis, sin tarjeta).
2. **Domains → Add Domain →** `embolsadora.site`.
3. Región de envío: **`sa-east-1` (São Paulo)**, la más cercana a Argentina.
4. En DNS Records, una vez hecho el Paso 0, **"Auto configure"** debería
   funcionar y escribir los registros en Vercel solo. Si igual falla, usar
   **"Manual setup"** y seguir el Paso 2.

Los registros que pide Resend son tres, todos sobre subdominios:

| Tipo | Nombre | Valor | Notas |
|------|--------|-------|-------|
| MX | `send` | `feedback-smtp.sa-east-1.amazonses.com` | prioridad 10 |
| TXT | `send` | `v=spf1 include:amazonses.com ~all` | SPF |
| TXT | `resend._domainkey` | `p=MIGfMA0GCSq...` (clave larga) | DKIM |

> Los valores exactos los da Resend en pantalla. Los de arriba son la forma
> esperada, no para copiar a ciegas.

---

## Paso 2 — Cargar los registros en Vercel (si el auto-configure falla)

Ya con la delegación unificada, los registros van **en Vercel**, no en DonWeb.

Por UI: *Vercel → Domains → embolsadora.site → DNS Records → Add*.

Por CLI, que es más rápido y deja constancia:

```bash
cd ~/Develop/UTN/embolsadora-frontend

vercel dns add embolsadora.site send MX feedback-smtp.sa-east-1.amazonses.com 10
vercel dns add embolsadora.site send TXT "v=spf1 include:amazonses.com ~all"
vercel dns add embolsadora.site resend._domainkey TXT "p=MIGfMA0GCSq..."
vercel dns add embolsadora.site _dmarc TXT "v=DMARC1; p=none; rua=mailto:federicoadegiovanni@gmail.com"

vercel dns ls embolsadora.site
```

El nombre va **relativo** (`send`, no `send.embolsadora.site`): si se pone el
nombre completo termina como `send.embolsadora.site.embolsadora.site`. Y la
clave DKIM es larga: va entera, sin espacios ni saltos agregados.

Sobre el DMARC: `p=none` significa "observá y reportá, no bloquees". Es el modo
correcto para arrancar — si algo está mal, los mails siguen llegando y los
reportes avisan. Recién con reportes limpios durante un tiempo tiene sentido
subirlo a `p=quarantine` y después a `p=reject`.

### Verificar la propagación

```bash
dig @8.8.8.8 TXT send.embolsadora.site +short
dig @8.8.8.8 MX  send.embolsadora.site +short
dig @8.8.8.8 TXT resend._domainkey.embolsadora.site +short
dig @8.8.8.8 TXT _dmarc.embolsadora.site +short
```

Después volver a Resend y esperar a que el dominio figure como **Verified**.
**No seguir hasta que verifique**: si se configura el SMTP antes, todos los
mails de auth del proyecto van a fallar.

---

## Paso 3 — Crear la API key en Resend

**API Keys → Create API Key**, permiso *Sending access*. Empieza con `re_`.

Esa key **es la contraseña SMTP**. Es un secreto: no pegarla en un chat, un
issue ni un commit. Para dejarla disponible localmente:

```bash
umask 077 && printf 're_TU_API_KEY' > ~/.resend-api-key
```

---

## Paso 4 — Configurar el SMTP en Supabase

Dashboard → **Project Settings → Authentication → SMTP Settings** → *Enable
Custom SMTP*:

| Campo | Valor |
|-------|-------|
| Host | `smtp.resend.com` |
| Port | `465` |
| Username | `resend` |
| Password | la API key del paso 3 |
| Sender email | `no-responder@embolsadora.site` |
| Sender name | `Embolsadora` |

Equivalente por Management API:

```bash
curl -X PATCH "https://api.supabase.com/v1/projects/cdjehkbidqqsldaajbui/config/auth" \
  -H "Authorization: Bearer $SUPABASE_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "smtp_host": "smtp.resend.com",
    "smtp_port": 465,
    "smtp_user": "resend",
    "smtp_pass": "re_...",
    "smtp_admin_email": "no-responder@embolsadora.site",
    "smtp_sender_name": "Embolsadora"
  }'
```

**Este paso tiene efecto inmediato y global.** Apenas se guarda, *todos* los
mails de auth del proyecto salen por Resend.

No hace falta crear ninguna casilla: Resend permite enviar *desde*
`no-responder@embolsadora.site` con solo tener el dominio verificado. Es solo
envío — si alguien responde, no llega a ningún lado, que es el comportamiento
esperado de un "no responder".

---

## Paso 5 — Probar

Invitar a una dirección propia desde la aplicación y confirmar en el mail:

- [ ] El remitente es `Embolsadora <no-responder@embolsadora.site>` y ya no
      `noreply@mail.app.supabase.io`.
- [ ] En Gmail: *Mostrar original* → `SPF: PASS`, `DKIM: PASS`, `DMARC: PASS`.
- [ ] El mail no cayó en spam.
- [ ] El link del botón tiene la forma
      `https://embolsadora.site/s/<tenant>/auth/callback?token_hash=...&type=invite`
      — sin `#`, y con el dominio de la instancia desde la que se invitó.

En Resend, **Logs** muestra cada envío con su estado de entrega. Es el primer
lugar donde mirar si algo no llega.

---

## Lo que queda después de esto

Fuera del alcance de este documento, pero pendiente para que la feature quede
completa en producción:

- **Env vars en Cloud Run:** `APP_BASE_URL=https://embolsadora.site` y
  `APP_ALLOWED_ORIGINS`. Ojo con la sintaxis de `gcloud` para un valor con
  comas — ver el Step 5 de
  `docs/superpowers/plans/2026-07-29-invitation-email.md`, que documenta la
  forma correcta del delimitador alternativo `^|^`.
- **Deploy de ambos repos.** La rama del backend se mergea con **squash**: tres
  commits intermedios no compilan por diseño.
- **Prueba final:** invitar desde la app ya desplegada y confirmar que al
  aceptar se cae en la pantalla de cambio de contraseña, no en el dashboard.

## Referencias

- Diseño y decisiones: `docs/superpowers/specs/2026-07-29-invitation-email-design.md`
- Plan de implementación: `docs/superpowers/plans/2026-07-29-invitation-email.md`
- Pendientes y hallazgos diferidos: `docs/superpowers/plans/2026-07-29-invitation-email-followups.md`
- Plantillas y cómo publicarlas: `emails/README.md`
- Documentación de DonWeb sobre zona DNS:
  https://soporte.donweb.com/hc/es/articles/18274192917012--C%C3%B3mo-crear-y-modificar-la-Zona-DNS

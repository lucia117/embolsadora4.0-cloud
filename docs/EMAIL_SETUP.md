# Configuración del envío de mails de autenticación

Runbook operativo para que los mails de invitación, reset de contraseña,
confirmación y magic link salgan desde `no-responder@embolsadora.site` en vez
del mailer compartido de Supabase.

**Estado al 2026-07-30:** el código está listo y las plantillas ya están
publicadas en Supabase. Falta únicamente lo de este documento: el proveedor de
envío. Hasta que se complete, los mails salen igual y con el diseño correcto,
pero desde `noreply@mail.app.supabase.io` y con el rate limit del servicio
compartido de Supabase, que está documentado como solo para desarrollo.

---

## Lo que hay que saber antes de empezar

### El DNS está en DonWeb, no en Vercel

Aunque el sitio se sirve desde Vercel, la zona DNS de `embolsadora.site` la
administra **DonWeb**. Verificado contra un resolver público:

```
$ dig @8.8.8.8 NS embolsadora.site
embolsadora.site.  14400  IN  CNAME  cname.vercel-dns.com.
```

DonWeb publica un CNAME en el ápex apuntando a Vercel. Los nameservers
`ns1..4.vercel-dns-3.com` que aparecen en una consulta NS son los de *ese
destino*, no los del dominio. Por eso el panel de Vercel muestra
**"Nameservers: Third Party"**, y por eso los registros van cargados **en el
panel de DonWeb**.

### Por qué falla "Auto configure" en Resend

El botón usa Domain Connect, que le pide al proveedor de DNS que escriba los
registros automáticamente. Resend detecta Vercel por el CNAME y le pide a
Vercel; Vercel responde *"This domain is not using Vercel DNS"* y aborta.

**Usar siempre "Manual setup".**

### El CNAME en el ápex limita qué se puede cargar

Un CNAME en el ápex impide cualquier otro registro en ese mismo nombre (RFC
1034). O sea: **no se puede poner un TXT de SPF en `embolsadora.site` a secas.**

No es un problema para esto, porque Resend usa subdominios para todos sus
registros (`send.embolsadora.site`, `resend._domainkey.embolsadora.site`) y los
subdominios no están afectados. Pero conviene tenerlo presente si alguna vez
hace falta un registro en el ápex — ahí habría que mover los nameservers a
Vercel o a otro DNS que soporte ALIAS/ANAME.

---

## Paso 1 — Alta del dominio en Resend

1. Crear cuenta en https://resend.com/signup (gratis, sin tarjeta).
2. **Domains → Add Domain →** `embolsadora.site`.
3. Región de envío: **`sa-east-1` (São Paulo)**, la más cercana a Argentina.
4. En la pantalla de DNS Records, elegir **"Manual setup"**.

Resend va a mostrar tres registros, todos sobre subdominios:

| Tipo | Nombre | Valor | Notas |
|------|--------|-------|-------|
| MX | `send` | `feedback-smtp.sa-east-1.amazonses.com` | prioridad 10 |
| TXT | `send` | `v=spf1 include:amazonses.com ~all` | SPF |
| TXT | `resend._domainkey` | `p=MIGfMA0GCSq...` (clave larga) | DKIM |

> Los valores exactos los da Resend en pantalla. Los de arriba son la forma
> esperada, no para copiar a ciegas.

---

## Paso 2 — Cargar los registros en DonWeb

Panel de DonWeb → zona DNS de `embolsadora.site` → agregar los tres registros
del paso anterior.

Ojo con dos cosas que rompen esto seguido:

- **El nombre es relativo.** Si el panel pide el nombre a secas, va `send` y
  `resend._domainkey`, no `send.embolsadora.site`. Si el panel completa el
  dominio solo y cargás el nombre completo, termina en
  `send.embolsadora.site.embolsadora.site` y no funciona nada.
- **La clave DKIM es larga y no debe cortarse.** Copiarla entera, sin espacios
  ni saltos de línea agregados.

Agregar además el DMARC en modo observación, que Resend no siempre incluye:

| Tipo | Nombre | Valor |
|------|--------|-------|
| TXT | `_dmarc` | `v=DMARC1; p=none; rua=mailto:federicoadegiovanni@gmail.com` |

`p=none` significa "observá y reportá, no bloquees". Es el modo correcto para
arrancar: si algo está mal configurado, los mails siguen llegando y los
reportes avisan. Recién cuando lleguen reportes limpios durante un tiempo tiene
sentido subirlo a `p=quarantine` y después a `p=reject`.

### Verificar la propagación

Desde la terminal, hasta que las tres respondan:

```bash
dig @8.8.8.8 TXT send.embolsadora.site +short
dig @8.8.8.8 MX  send.embolsadora.site +short
dig @8.8.8.8 TXT resend._domainkey.embolsadora.site +short
dig @8.8.8.8 TXT _dmarc.embolsadora.site +short
```

Después volver a Resend y esperar a que el dominio figure como **Verified**.
Puede tardar de minutos a algunas horas. **No seguir hasta que verifique**: si
se configura el SMTP antes, todos los mails de auth del proyecto van a fallar.

---

## Paso 3 — Crear la API key en Resend

**API Keys → Create API Key**, con permiso *Sending access*. Empieza con `re_`.

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

Equivalente por Management API, si se prefiere no usar el dashboard:

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

No hace falta crear ninguna casilla de correo: Resend permite enviar *desde*
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
  `APP_ALLOWED_ORIGINS`. Ojo con la sintaxis de `gcloud` para un valor que
  contiene comas — ver el Step 5 de
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

# Plantillas de mail de Supabase Auth

Estos cuatro archivos son la **fuente de verdad** de los mails que manda
Supabase Auth. Lo que esté cargado en el dashboard es una copia derivada.

## Editar

1. Modificar el `.html` acá.
2. Previsualizar las dos ramas de cada condicional:
   `go run ./cmd/renderemails` y abrir `tmp/emails/`.
3. Publicar: `SUPABASE_ACCESS_TOKEN=… SUPABASE_PROJECT_REF=cdjehkbidqqsldaajbui ./scripts/publish-email-templates.sh`

## Variables disponibles

- `{{ .ConfirmationURL }}` — el link de acción, ya con el `redirect_to` que armó el backend.
- `{{ .Email }}` — mail del destinatario.
- `{{ .Data.tenant_name }}`, `{{ .Data.inviter_name }}`, `{{ .Data.role_name }}` — solo en `invite.html`.
  Los puebla el backend en `internal/platform/supabase/admin_client.go`. Un usuario invitado
  antes de este cambio no tiene `user_metadata`, así que GoTrue le pasa `.Data` como un
  `interface{}` nil.

  Esto se verificó de forma empírica:
  - Un `{{ .Data.tenant_name }}` a secas **panickea** con
    `nil pointer evaluating interface {}.tenant_name`.
  - El guard que parece obvio, `{{ if index .Data "tenant_name" }}`, **tampoco sirve**:
    tira `error calling index: index of untyped nil`.
  - Lo único que funciona es un guard exterior sobre `.Data` mismo, y es el patrón que
    usa `invite.html`:

    ```gotemplate
    {{ if .Data }}
      ... {{ .Data.tenant_name }} ...
    {{ else }}
      ... valor por defecto ...
    {{ end }}
    ```

  No alcanza con envolver cada variable en su propio `{{ if }}` si no hay primero un
  `{{ if .Data }}` que lo contenga: sin ese guard exterior, cualquier acceso a `.Data.algo`
  vuelve a panickear apenas `.Data` es nil.

**No usar `{{ .SiteURL }}`.** Es un valor global del proyecto y era la causa de
que los mails mostraran `http://localhost:3000` desde producción.

#!/usr/bin/env bash
# Publica emails/*.html en Supabase Auth.
#
# El dashboard NO es la fuente de verdad: cualquier cosa editada ahi la pisa la
# proxima corrida de este script. Editar siempre los archivos del repo.
#
# Requiere:
#   SUPABASE_ACCESS_TOKEN  personal access token (Account -> Access Tokens)
#   SUPABASE_PROJECT_REF   ref del proyecto, p.ej. cdjehkbidqqsldaajbui
#   jq
set -euo pipefail

: "${SUPABASE_ACCESS_TOKEN:?falta SUPABASE_ACCESS_TOKEN}"
: "${SUPABASE_PROJECT_REF:?falta SUPABASE_PROJECT_REF}"

cd "$(dirname "$0")/.."

payload=$(jq -n \
  --rawfile invite       emails/invite.html \
  --rawfile recovery     emails/recovery.html \
  --rawfile confirmation emails/confirmation.html \
  --rawfile magiclink    emails/magic-link.html \
  '{
     mailer_templates_invite_content:       $invite,
     mailer_templates_recovery_content:     $recovery,
     mailer_templates_confirmation_content: $confirmation,
     mailer_templates_magic_link_content:   $magiclink,
     mailer_subjects_invite:       "Te invitaron a Embolsadora",
     mailer_subjects_recovery:     "Restablecé tu contraseña de Embolsadora",
     mailer_subjects_confirmation: "Confirmá tu cuenta de Embolsadora",
     mailer_subjects_magic_link:   "Tu link de acceso a Embolsadora"
   }')

curl -sS -X PATCH \
  "https://api.supabase.com/v1/projects/${SUPABASE_PROJECT_REF}/config/auth" \
  -H "Authorization: Bearer ${SUPABASE_ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$payload" \
  | jq '{invite_subject: .mailer_subjects_invite, invite_bytes: (.mailer_templates_invite_content | length)}'

echo "plantillas publicadas en el proyecto ${SUPABASE_PROJECT_REF}"

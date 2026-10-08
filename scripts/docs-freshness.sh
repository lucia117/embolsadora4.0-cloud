#!/usr/bin/env bash
# Lista los documentos curados cuyo `last_reviewed` tiene más de MAX_AGE_DAYS
# días (120 por defecto). Sale con 1 si encontró alguno.
# Lo corre el job semanal `docs-freshness` de .github/workflows/docs.yml, que
# abre o actualiza un issue con la salida. En local: scripts/docs-freshness.sh
set -uo pipefail

cd "$(dirname "$0")/.."

MAX_AGE_DAYS=${MAX_AGE_DAYS:-120}
today=$(date -u +%s)
stale=0

for f in README.md AGENTS.md docs/*.md docs/adr/*.md specs/README.md specs/*/spec.md; do
  [ -f "$f" ] || continue
  [ "$f" = "docs/adr/_template.md" ] && continue
  reviewed=$(awk 'NR==1 && !/^---/{exit} NR>1 && /^---/{exit} /^last_reviewed:/{print $2; exit}' "$f" | tr -d '\r')
  [ -n "$reviewed" ] || continue
  ts=$(date -u -d "$reviewed" +%s 2>/dev/null) || { echo "- \`$f\`: last_reviewed inválido ($reviewed)"; stale=$((stale + 1)); continue; }
  age=$(( (today - ts) / 86400 ))
  if [ "$age" -gt "$MAX_AGE_DAYS" ]; then
    echo "- \`$f\`: revisado por última vez el $reviewed (hace $age días)"
    stale=$((stale + 1))
  fi
done

[ "$stale" -eq 0 ]

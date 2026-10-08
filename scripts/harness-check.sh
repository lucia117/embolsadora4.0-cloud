#!/usr/bin/env bash
# Verifica el layout y las prohibiciones del harness documental
# (docs/_process/estandar/plan-harness-2026-09-29.md, fase 4).
# Lo corre el job `harness-layout` de .github/workflows/docs.yml y se puede
# correr en local: scripts/harness-check.sh
set -uo pipefail

cd "$(dirname "$0")/.."

errors=0
warnings=0
fail() { echo "ERROR: $*"; errors=$((errors + 1)); }
warn() { echo "WARN:  $*"; warnings=$((warnings + 1)); }

# ── 1. Archivos obligatorios ────────────────────────────────────────────────
for f in README.md AGENTS.md CLAUDE.md .env.example \
         docs/architecture.md docs/development.md docs/operations.md \
         docs/adr/index.md docs/openapi.yaml specs/README.md; do
  [ -f "$f" ] || fail "falta el archivo obligatorio $f"
done
# CHANGELOG.md lo genera release-please, en suspenso por el conflicto con la
# constitution (ver la auditoría). Hasta que se resuelva es advertencia.
[ -f CHANGELOG.md ] || warn "falta CHANGELOG.md (release-please pendiente de decisión)"

grep -q '^@AGENTS.md' CLAUDE.md 2>/dev/null || fail "CLAUDE.md debe incluir @AGENTS.md"
grep -q '^## Specs' AGENTS.md 2>/dev/null || fail "AGENTS.md debe tener la sección '## Specs'"

# ── 2. Prohibiciones sobre archivos versionados ─────────────────────────────
tracked=$(git ls-files)

while IFS= read -r f; do
  case "$f" in
    *.md) case "$f" in README.md|AGENTS.md|CLAUDE.md|CHANGELOG.md|LICENSE.md) ;; *) fail "markdown suelto en la raíz: $f (va a docs/ o docs/_process/)" ;; esac ;;
  esac
done < <(printf '%s\n' "$tracked" | grep -v '/')

printf '%s\n' "$tracked" | grep -E '(^|/)\.env(\.[^/]*)?$' | grep -v '\.env\.example$' \
  | while IFS= read -r f; do echo "ERROR: archivo .env versionado: $f"; done
n=$(printf '%s\n' "$tracked" | grep -E '(^|/)\.env(\.[^/]*)?$' | grep -vc '\.env\.example$')
errors=$((errors + n))

for pattern in '\.exe$' '(^|/)build\.log$' '^\.specify/specs/' '^docs/superpowers/'; do
  while IFS= read -r f; do
    [ -n "$f" ] && fail "archivo prohibido versionado: $f"
  done < <(printf '%s\n' "$tracked" | grep -E "$pattern")
done

# ── 3. Front-matter ─────────────────────────────────────────────────────────
# frontmatter_has <archivo> <campo>...: el archivo arranca con '---' y el
# bloque inicial define cada campo.
frontmatter_has() {
  local file=$1; shift
  if [ "$(head -n1 "$file" | tr -d '\r')" != "---" ]; then
    fail "$file: sin front-matter"
    return
  fi
  local block
  block=$(awk 'NR==1{next} /^---\r?$/{exit} {print}' "$file")
  for field in "$@"; do
    printf '%s\n' "$block" | grep -qE "^${field}:" || fail "$file: falta '${field}' en el front-matter"
  done
}

for f in docs/*.md specs/README.md docs/_process/README.md; do
  [ -f "$f" ] && frontmatter_has "$f" title status owner last_reviewed
done
for f in docs/adr/CLOUD-ADR-*.md; do
  frontmatter_has "$f" id title status date owner last_reviewed supersedes superseded_by
done
for f in specs/*/spec.md; do
  frontmatter_has "$f" id title tier status veredicto owner date repos origin last_reviewed
done

# ── 4. ADRs: nombre con prefijo y sin colisiones ────────────────────────────
for f in docs/adr/*.md; do
  base=$(basename "$f")
  case "$base" in
    index.md|_template.md) ;;
    CLOUD-ADR-[0-9][0-9][0-9]-*.md) ;;
    *) fail "ADR con nombre fuera de formato: $f (CLOUD-ADR-NNN-slug.md)" ;;
  esac
done
dups=$(ls docs/adr/CLOUD-ADR-*.md 2>/dev/null | sed -E 's#.*/CLOUD-ADR-([0-9]{3})-.*#\1#' | sort | uniq -d)
[ -z "$dups" ] || fail "números de ADR repetidos: $dups"

# ── 5. Specs: cada directorio figura en el índice ───────────────────────────
for d in specs/*/; do
  id=$(basename "$d" | cut -d- -f1)
  grep -qE "^\| ${id} \|" specs/README.md || fail "specs/README.md no lista la spec ${id} ($d)"
  [ -f "${d}spec.md" ] || fail "${d} no tiene spec.md"
done

echo
echo "harness-check: ${errors} error(es), ${warnings} advertencia(s)"
[ "$errors" -eq 0 ]

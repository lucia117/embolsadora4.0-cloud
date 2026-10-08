# Claude Code Adapter

@AGENTS.md

## Claude Code

- Las reglas para agentes viven en `AGENTS.md`; este archivo solo agrega lo específico
  de Claude Code.
- La arquitectura interna está en `docs/architecture.md` y el estado de producción
  (incluido el incidente de orden de deploy del 2026-08-18) en `docs/operations.md`:
  leelos antes de tocar middlewares, migraciones o la superficie de ingesta.
- Hooks, permisos y configuración de MCP van en `.claude/` (`settings.local.json` no se
  versiona).

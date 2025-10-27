# Colección Postman – embolsadora-api

## Importar
1. Postman → **Import**.
2. Seleccionar `api-embolsadora-api.postman_collection.json` y un entorno (`env-*.postman_environment.json`).
3. Elegir entorno y setear variables sensibles (no están en el repo).

## Variables
- `{{base_url}}`: URL base del ambiente.
- `{{auth_token}}`: JWT u otro esquema de auth.
- `{{api_key}}`: Si aplica.

## Actualizar colección
1. Exportar desde Postman (v2.1).
2. Reemplazar `postman/api-embolsadora-api.postman_collection.json`.
3. Commit: `chore(postman): update collection`.

## Buenas prácticas
- No subir tokens al repo.
- Mantener ejemplos de requests/headers.
- Versionar cambios junto con PRs de endpoints.

---
id: 025
title: "Endurecimiento de roles custom (UpdateRole y guarda de rol propio)"
tier: feature
status: draft
veredicto: D
owner: Lucia Scharff
date: 2026-10-07
repos: [embolsadora4.0-cloud]
origin: code-review
issues: [93, 107]
prs: [106, 108, 109]
adrs: []
last_reviewed: 2026-10-07
---

# 025 — Endurecimiento de roles custom

> **Procedencia.** Sale del code review del PR #109 (develop → main, 2026-10-07), que
> revisó el código ya mergeado de #106 y #108. Los dos puntos son huecos que esos fixes
> dejaron, no regresiones. Cada afirmación sobre el código se leyó en `develop`
> (`2e6e860`) el 2026-10-07.

## Contexto y problema

Dos PRs recientes cerraron fallas de seguridad en los roles custom:

- #106 (issue #93) valida los permisos de un rol custom contra el catálogo
  `permissions`, para que no se guarden ids inexistentes.
- #108 (issue #107) cierra la escalada de privilegios: `ensureGrantable` impide otorgar
  permisos de sistema que quien edita no tiene, y `update_user_role` rechaza cambiar el
  rol de la propia asignación (`ErrCannotChangeOwnRole` → 403).

Quedan dos huecos:

1. **`UpdateRole` valida todo el payload, no solo lo nuevo.** `validatePermissions`
   (`internal/app/roles/service.go`) recibe la lista completa de permisos, mientras que
   `ensureGrantable` ya recibe solo los agregados. Un rol custom que guarda un id que
   luego salió del catálogo (por ejemplo `perm_all_tenants`, que salió en la migración
   000008, o un permiso custom borrado) devuelve 400 `UNKNOWN_PERMISSIONS` en cualquier
   edición, aunque el cambio sea solo el nombre o la descripción. El frontend reenvía la
   lista actual, así que el rol queda bloqueado hasta quitar el id a mano. Ninguna
   migración limpia `roles.permissions`. No se verificó si existen roles así en
   producción.
2. **La guarda de rol propio falla abierta.** En
   `internal/api/usecases/user_roles/update_user_role/usecase.go`, la condición
   `caller != nil && *caller == utr.UserID` se salta cuando falta el `UserID` en el
   contexto. `JWTAuth` solo lo inyecta si `uuid.Parse(user.ID)` tiene éxito y descarta el
   error si no. Hoy `user.ID` viene de Postgres y debería ser siempre un UUID, así que el
   caso es improbable, pero la guarda de una frontera de seguridad no debería depender de
   eso. El comentario del código dice "uso fuera del flujo HTTP", lo cual no cubre este
   camino.

## Alcance

**Entra:**

- `UpdateRole` valida contra el catálogo solo los permisos agregados.
- `update_user_role` rechaza la operación cuando no puede identificar al caller.

**No entra:**

- Migración que limpie ids huérfanos de `roles.permissions`. No hay evidencia de roles
  afectados en producción y tocar datos de prod conlleva el riesgo de orden de deploy de
  `migrations/README.md`. Si se detectan roles afectados, va en una spec aparte.
- Que `permissions.Delete` limpie `roles.permissions` al borrar un permiso.
- La atomicidad entre validar el catálogo y escribir el rol (carrera con
  `permissions.Delete`), la auditoría cross-tenant, la doble consulta a `permissions`,
  el apagado ordenado y los `spec.md` sin `RF-NNN`. Son otros hallazgos del mismo review.
- Cambiar `JWTAuth`. Ver decisión D2.
- Cambios en `CreateRole`: todos sus permisos son nuevos, así que ya se validan todos.

## Requisitos

### Validación en `UpdateRole`

- **RF-001** (`origen: derivado`) — `UpdateRole` calcula los permisos agregados
  (presentes en el payload y ausentes de `role.Permissions`) y valida contra el catálogo
  solo esos. Un id que el rol ya tenía no se consulta.
- **RF-002** (`origen: derivado`) — Un payload que conserva un id huérfano ya guardado
  se acepta y el rol se actualiza. Un payload que lo quita también se acepta. En ambos
  casos el resultado refleja exactamente el payload deduplicado.
- **RF-003** (`origen: derivado`) — Un id agregado que no existe en el catálogo (global ni
  del tenant) se rechaza con `ErrRoleUnknownPermissions` → 400 `UNKNOWN_PERMISSIONS`, sin
  modificar el rol. El mensaje lista solo los ids agregados desconocidos.
- **RF-004** (`origen: derivado`) — `ensureGrantable` sigue recibiendo solo los agregados.
  No cambia su comportamiento.
- **RF-005** (`origen: derivado`) — Si no hay permisos agregados, `UpdateRole` no consulta
  el catálogo.

### Guarda de rol propio

- **RF-006** (`origen: derivado`) — `update_user_role` devuelve un error de dominio nuevo,
  `ErrCallerNotIdentified`, cuando `platform.UserID(ctx)` es nil. La comprobación ocurre
  después de resolver la asignación (conserva el 404 de `ErrAssignmentNotFound`) y antes
  de `EnsureAssignable` y de `Update`.
- **RF-007** (`origen: derivado`) — El handler mapea `ErrCallerNotIdentified` a
  401 `{"success": false, "error": "unauthenticated"}`, igual que el handler ya responde
  cuando falta el tenant en el contexto.
- **RF-008** (`origen: derivado`) — Cuando el caller se identifica, el comportamiento no
  cambia: misma asignación → 403 `ErrCannotChangeOwnRole`; otra asignación → se actualiza.
- **RF-009** (`origen: derivado`) — Se corrige el comentario de `usecase.go` que describe
  el caso `UserID` ausente como "uso fuera del flujo HTTP, se sigue".
- **RF-010** (`origen: derivado`) — `docs/openapi.yaml` documenta la respuesta 401 de
  `PUT /user-roles/{id}` si todavía no está. `TestOpenAPIMatchesRouter` debe seguir
  pasando.

## Criterios de éxito

- **CE-001** — Test de `UpdateRole`: un rol con un id huérfano en `role.Permissions` se
  edita (solo nombre) sin error y el catálogo no recibe consultas. Cubre RF-001, RF-002
  y RF-005.
- **CE-002** — Test de `UpdateRole`: agregar un id inexistente devuelve
  `ErrRoleUnknownPermissions` y el repositorio no recibe `Update`. Cubre RF-003.
- **CE-003** — Test de `UpdateRole`: el mensaje de error lista solo los ids agregados
  desconocidos, no los huérfanos preexistentes. Cubre RF-003.
- **CE-004** — Test de `update_user_role`: contexto sin `UserID` devuelve
  `ErrCallerNotIdentified` y el repositorio no recibe `Update`. Cubre RF-006.
- **CE-005** — Test de handler: `ErrCallerNotIdentified` produce 401. Cubre RF-007.
- **CE-006** — Los tests existentes de `self_change_test.go` y
  `service_permissions_test.go` pasan; los que hoy dependen de un contexto sin `UserID`
  se ajustan para inyectarlo. Cubre RF-008.
- **CE-007** — `scripts/ci-check.sh` pasa, incluido `TestOpenAPIMatchesRouter`.

## Decisiones

- **D1 — Tolerancia sin migración.** Se elige tolerar los huérfanos en `UpdateRole` y no
  limpiarlos. Alternativas descartadas por ahora: una migración que quite de
  `roles.permissions` los ids ausentes de `permissions` (toca datos de producción, sin
  evidencia de que haga falta) y limpiar al borrar un permiso (evita huérfanos nuevos,
  no arregla los existentes).
- **D2 — La guarda falla cerrada en el use case, no en `JWTAuth`.** La causa de fondo es
  que `JWTAuth` descarta el error de `uuid.Parse(user.ID)`. Arreglarlo ahí cambiaría el
  comportamiento de todas las rutas autenticadas, un alcance mayor que este endurecimiento.
  Se deja anotado como seguimiento posible.
- **D3 — 401 y no 403.** Un caller sin identidad resoluble es un fallo de autenticación,
  no de permisos. Es el mismo código que el handler ya usa cuando falta el tenant.

## Riesgos

- **Cambio de contrato.** `PUT /user-roles/{id}` gana una respuesta 401 que antes no
  existía en la práctica. Como la condición es improbable, el impacto esperado es nulo,
  pero se documenta en OpenAPI (RF-010). No toca la superficie
  `/api/v1/consumers/events`.
- **Ids huérfanos sin otorgar.** Un id huérfano que se conserva no concede nada:
  `ensureGrantable` solo cuenta permisos de sistema, y un id ausente del catálogo no lo
  es. Se asume que ningún código autoriza con ids ausentes del catálogo; conviene
  confirmarlo en el plan.

## Preguntas abiertas

Ninguna.

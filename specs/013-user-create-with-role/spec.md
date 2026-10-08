---
id: 013
title: "POST /users con asignación de rol inicial"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-11
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [29]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 013 — POST /users con asignación de rol inicial

> **Procedencia.** Esta feature tiene dos documentos de origen del mismo día
> (2026-04-11):
>
> - [`design-speckit.md`](design-speckit.md), la spec de speckit, que ya numeraba sus
>   requisitos como `RF-001` a `RF-007`; acá se conserva esa numeración.
> - [`design.md`](design.md), el diseño de superpowers, con sus decisiones D1 a D5.
>
> `origen: derivado` se reconstruyó del código, incluidos los fixes de seguridad
> posteriores. **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

`POST /api/v1/users` creaba el usuario pero no su asignación en `user_tenant_roles`: el
usuario quedaba sin acceso hasta una segunda llamada a `POST /user-roles`. Era además la
única interacción pendiente del pact `user-service-api-roles-extension` del frontend. La
feature hace que el alta cree el usuario **y** su rol activo en una sola operación
atómica.

## Alcance

**Entra:** la creación atómica de usuario y rol en `POST /api/v1/users`, la validación
del rol y el mapeo de errores.

**No entra:**

- Cambios de esquema.
- Cambios en la forma de la respuesta.
- Otros endpoints de usuarios, que están en [002a](../002a-user-management/spec.md).

## Requisitos

- **RF-001** `origen: original` — El sistema DEBE crear el usuario y su asignación de rol
  en una única transacción.
  → `PostgresRepository.CreateWithRole` (`db.Begin` y `defer tx.Rollback`).
- **RF-002** `origen: original` — La asignación DEBE crearse con `status = 'active'` y
  `assigned_at = NOW()`: el alta directa por un administrador no pasa por el flujo de
  invitaciones (D2).
- **RF-003** `origen: original`, **restringido después** — `role` DEBE aceptar el `id` de
  un rol de sistema o de un rol custom del tenant. **Derivado:** además, el rol tiene que
  ser *asignable* por quien crea. Existencia, pertenencia al tenant y visibilidad se
  validan con el mismo lookup "cloakeado" que usan `GET /roles` e invitaciones, de modo
  que un administrador del tenant plataforma no puede crear un `super_admin`.
  → `appRoles.EnsureAssignable` (`internal/app/roles/assignable.go`).
- **RF-004** `origen: original` — Si el rol no existe (o, por el cambio de RF-003, no es
  asignable), DEBE responder 400 `INVALID_ROLE`. La respuesta es la misma en los tres
  casos para no revelar la existencia de roles de plataforma.
  → `domain.ErrInvalidRoleID`, `internal/api/handler/users/errors.go`.
- **RF-005** `origen: original` — El `assigned_by` de la asignación DEBE ser el
  administrador autenticado, tomado del contexto del JWT (D4).
  → `Handler.CreateUser` (`AssignedBy: callerUUID`).
- **RF-006** `origen: original` — La respuesta DEBE ser el mismo `UserResponse` de
  antes, sin campo `roles` (D5). Los roles se consultan con `GET /users/:id?include=roles`.
- **RF-007** `origen: original` — Si falla cualquier parte de la transacción, NINGÚN
  registro DEBE persistir.
  → rollback en `CreateWithRole`, verificado por `TestCreateWithRole_RoleIDInexistente`.
- **RF-008** `origen: original`, **código cambiado** — Un email repetido en el tenant
  DEBE responder 409. La spec y el diseño pedían el código `EMAIL_TAKEN`; el código real
  es **`DUPLICATE_EMAIL`**.
  → `users.ErrEmailTaken` (violación `23505`), `errors.go`.
- **RF-009** `origen: derivado` — Crear el usuario exige `perm_users_manage`.
  → `internal/api/router.go`.

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-001** | Con un rol válido responde 201 y crea un usuario y una asignación activa | original (SC-001) | `internal/repo/pg/users/create_test.go` (`TestCreateWithRole_HappyPath`) |
| **CE-002** | Con un rol inválido responde 400 y no crea nada | original (SC-002) | `TestCreateWithRole_RoleIDInexistente` |
| **CE-003** | Un email duplicado en el mismo tenant se rechaza | original (escenario 3) | `TestCreateWithRole_EmailDuplicadoEnElMismoTenant` |
| **CE-004** | Un administrador del tenant plataforma no puede crear un usuario con rol global; `super_admin` sí | derivado | `internal/app/users/service_test.go` (`TestCreateUserConRolGlobal*`) |

- SC-003 (el pact del frontend pasa) **no se verifica en este repo**: los pacts no corren
  en su CI (ver `docs/_process/PACTS_ANALYSIS.md`).
- SC-004 (sin regresiones en los otros endpoints) no tiene un criterio propio; lo cubren
  los tests de [002a](../002a-user-management/spec.md).

## Decisiones

| Decisión | Alternativa descartada | Por qué | Fuente |
|---|---|---|---|
| Transacción en el repositorio | Dos operaciones desde el servicio | Si falla el segundo INSERT, el usuario queda sin rol y sin recuperación automática | D3 |
| Reutilizar el campo `role` como `role_id` | Un campo nuevo | El campo ya existía; es compatible hacia atrás | D1 |
| Validar asignabilidad en el backend | Confiar solo en la FK (D1 original) | La FK no impedía crear usuarios con roles de plataforma: era una escalada de privilegios | derivado (`assignable.go`) |

## Riesgos y preguntas abiertas

- **Código de error distinto al acordado** (`DUPLICATE_EMAIL` en vez de `EMAIL_TAKEN`).
  Hoy no rompe nada: `embolsadora-frontend` (`origin/develop`, 2026-09-29) no referencia
  ninguno de los dos códigos. Conviene alinear la spec o el código antes de que alguien
  empiece a depender de uno.
- **`ErrUserAlreadyHasActiveRole`** no está mapeado en el handler de usuarios y
  terminaría en 500. En la práctica no ocurre, porque el usuario es nuevo.

# Production readiness cleanup — diseño consolidado

**Fecha:** 2026-08-19
**Repos:** `embolsadora4.0-cloud` (Go/Gin) y `embolsadora-frontend` (Next.js) — este documento vive en el repo backend porque concentra la mayoría de los cambios, pero cubre ambos.
**Origen:** consolida los 15 pendientes listados en `~/Develop/UTN/handoff-2026-08-19-cleanup-completa-y-pendientes-consolidados.md` §3, más un finding adicional (extensión del fix A a `UpdateUser`/`UpdateUserStatus`) descubierto durante el brainstorming de este documento y confirmado por el usuario.

**Nota de estado ya resuelto durante el brainstorming**: el handoff decía que el PR frontend #76 (fix Dependabot) estaba mergeado a `main`, pero en realidad solo estaba en `develop` — el PR `develop→main` (#77) seguía abierto. Se mergeó durante esta sesión, antes de escribir este documento. Producción ya no tiene las 2 vulnerabilities critical expuestas.

Cada sub-proyecto abajo tiene una sección proporcional a su complejidad. El plan de implementación (fases, orden, tareas) se escribe aparte vía `writing-plans`.

---

## Sub-proyecto A — Cross-tenant lookup en mutaciones de usuario (Hallazgo C + item #3 + extensión)

**Root cause (compartido por los 4 puntos de abajo):** el fix de Hallazgo A (2026-08-18) agregó un parámetro `crossTenant bool`, calculado en el handler vía `security.IsCrossTenantRole(ctx)`, y lo threadeó handler→service→repo **solo para `GetUser`** (`GetByID`). El resto de las mutaciones de usuario nunca recibieron el mismo tratamiento — todas hardcodean `crossTenant=false` (o directamente no tienen el parámetro) en el precheck `GetByID`, y las queries de escritura en sí tampoco tienen la vía de escape `OR $N` que sí tiene la query de lectura de `GetByID`.

**Alcance de este sub-proyecto:**

1. **`DeleteUser`** (Hallazgo C) — `internal/api/handler/users/handler.go:358-379`, `internal/app/users/service.go:246-266`, `internal/repo/pg/users/postgres.go:289-309` (`Delete`).
2. **`GetUserWithRoles` / `?include=roles`** (item #3) — `internal/api/handler/users/handler.go:116-117`, `internal/app/users/service.go:74-91` (`GetUserWithRoles`), `internal/repo/pg/users/postgres.go:311-345` (`GetByIDWithRoles`).
3. **`UpdateUser`** (extensión acordada) — `internal/app/users/service.go:188` (precheck).
4. **`UpdateUserStatus`** (extensión acordada) — `internal/app/users/service.go:313,349` (dos call sites: precheck y el update en sí).

**Fix, mismo patrón en los 4 puntos, mirror exacto de `GetUser`/`GetByID`:**

- **Handler**: calcular `crossTenant := security.IsCrossTenantRole(c.Request.Context())` y pasarlo al service (ya se hace en `GetUser`; falta en `DeleteUser`, y en el branch `include=roles` de `GetUser`, y en `UpdateUser`/`UpdateUserStatus`).
- **Service**: agregar `crossTenant bool` a la firma de `DeleteUser`, `GetUserWithRoles`, `UpdateUser`, `UpdateUserStatus`; pasarlo al precheck `s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)` en vez del `false` hardcodeado.
- **Repo — lecturas** (`GetByIDWithRoles`): agregar `crossTenant bool` a la firma y sumar `OR $4` a la cláusula `WHERE u.id = $1 AND u.deleted_at IS NULL AND (u.tenant_id = $2 OR utr.id IS NOT NULL OR $4)`, mismo patrón que ya tiene `GetByID`.
- **Repo — escrituras** (`Delete`, y el `UPDATE` de `UpdateUserStatus`/`UpdateUser` si tienen el mismo scoping por tenant en su `WHERE`): agregar `crossTenant bool` a la firma y sumar `OR $N` al `WHERE ... AND (tenant_id = $2 OR EXISTS(...) OR $N)`.

**Testing**: `internal/repo/pg/users/cross_tenant_test.go` ya cubre `GetByID` con `crossTenant=true/false` — replicar los mismos casos (super_admin en tenant A borra/edita/lee-con-roles a un usuario del tenant B; falla sin `crossTenant`, funciona con) para `Delete`, `GetByIDWithRoles`, y el `UPDATE` de `UpdateUser`/`UpdateUserStatus`.

---

## Sub-proyecto B — Investigar 5xx pese a escritura exitosa (Hallazgo D)

No hay causa raíz identificada todavía — no hay diseño posible sin esa investigación primero. Plan de trabajo (a ejecutar con `systematic-debugging`, no como "fix" directo):

1. Ubicar en logs de Cloud Run los dos borrados de la tarde del 2026-08-19 (usuario → 502, tenant → 503) por timestamp.
2. Identificar qué ocurre **después** del `UPDATE`/`DELETE` SQL que ya committeó — candidatos: limpieza de Supabase Auth (borrar el usuario en GoTrue), algún paso async con timeout, un error en la serialización de la respuesta.
3. Una vez identificada la causa, decidir el fix concreto (puede ser tan simple como no fallar la request si el paso post-commit falla y loguearlo en cambio, o mover el paso a fire-and-forget).

Este sub-proyecto no tiene "diseño" cerrado — el resultado de la investigación determina el fix. Se documenta acá como placeholder; el plan de implementación lo trata como una tarea de investigación con salida abierta.

---

## Sub-proyecto C — RBAC para edge-devices (item #4)

Hoy `internal/api/handler/edge_devices/routes.go` registra sus 10 rutas sin ningún middleware de autorización (`grep -rn "RBACCheck" internal/api/handler/edge_devices/` no devuelve nada) — solo heredan `JWTAuth` + `ResolveTenantAndCheckMembership` del grupo padre (`internal/routes/url_mappings.go:192-200`). Cualquier miembro autenticado del tenant, incluido `operario`, puede crear/actualizar/habilitar/deshabilitar dispositivos.

Los permission IDs ya existen en el seed (`perm_edge_devices_view`, `perm_edge_devices_manage`, migración 000011) — no hace falta migración nueva, solo cablear.

**Fix**: aplicar `middleware.RBACCheck(...)` por ruta en `RegisterRoutes`, mismo patrón que `internal/api/router.go:68-90` usa para `users` (`RBACCheck("perm_users_view")` en GETs, `RBACCheck("perm_users_manage")` en mutaciones):

- `GET /edge-devices`, `GET /edge-devices/:deviceId`, `GET /edge-devices/:deviceId/telemetry`, `GET /edge-devices/:deviceId/events` → `RBACCheck("perm_edge_devices_view")`.
- `POST /edge-devices`, `PUT /edge-devices/:deviceId`, `POST .../enable`, `POST .../disable`, `POST .../status`, `POST .../health-check` → `RBACCheck("perm_edge_devices_manage")`. Confirmado revisando `status_check.go`/`health_check.go`: ambos llaman a `service.StatusCheck`/`service.HealthCheck` pasando `userID`/`userEmail` del actor (para audit trail), lo que indica que disparan una acción activa contra el device (no una lectura pasiva de estado ya almacenado) — mismo nivel que `enable`/`disable`.

---

## Sub-proyecto D — Batch de limpieza frontend RBAC/UI (items #5, #6, #10, #11, #12, #13)

Un solo PR, 6 cambios chicos y relacionados, todos en `embolsadora-frontend`:

- **#5 — Gating de botón `_manage`**: `useCanPerformAction` (`src/hooks/use-can-access.tsx:103-106`) hoy tiene un solo call site en todo el repo (`reports/generate/page.tsx`). Agregar el check a: borrar usuario (`src/components/users/columns.tsx:96-103` / `user-list.tsx`), crear/editar/borrar tenant (`tenants/page.tsx:17-19`, `tenants/columns.tsx:85-98`), editar/borrar rol (`roles/columns.tsx:120-137` — hoy solo chequea `!rep.isSystemRole`, ningún permiso).
- **#6 — Header `x-tenant-id` muerto**: `user-tenant-roles.tsx:33,40-48` recibe el prop `currentTenantId` pero lo descarta (`currentTenantId: _currentTenantId`) y arma el header con `profile?.tenant?.id` (el tenant del viewer, no el que se está mirando). Fix: usar el prop.
- **#10 — Tipado estricto de `PERMISSIONS`**: `src/lib/permissions.ts:74-76`, `Record<string, Permission>` → `Record<keyof typeof PERMISSION_IDS, Permission>`.
- **#13 — Tipado de `sidebar.tsx`**: línea 42, `permission?: string` → tipo derivado de `PERMISSION_IDS` (`typeof PERMISSION_IDS[keyof typeof PERMISSION_IDS]` o similar), relacionado con #10.
- **#12 — Borrar código muerto**: `SECTION_PERMISSIONS`, `useCanAccess`, `useAccessibleSections` en `use-can-access.tsx:27-41` y sus exports — confirmado 0 call sites reales en todo el repo (el propio código en `route-permissions.ts:85` ya lo documenta como muerto).
- **#11 — Comentario desactualizado**: `scripts/generate-system-permissions.mjs:4`, "17" → "18" (confirmado: 18 entradas reales en `PERMISSION_IDS` y en `system-permissions.json`).

---

## Sub-proyecto E — Edge cases de datos backend (items #7, #8)

**#7 — Tiebreak NULL en `assigned_at`**: la query `LEFT JOIN LATERAL` de `GetByID` (`internal/repo/pg/users/postgres.go:116-142`) ordena `ORDER BY (t.tenant_id = $2) DESC, t.assigned_at DESC` sin `NULLS LAST` explícito. El default de Postgres para `DESC` es `NULLS FIRST`, así que una membresía activa con `assigned_at IS NULL` gana el desempate aunque no sea la más reciente — el comentario en el código (líneas 111-115) documenta la intención ("la más reciente por `assigned_at` gana") pero el SQL no la cumple en el caso NULL. Fix: agregar `NULLS LAST` a ese `ORDER BY`. Si el fix del sub-proyecto A introduce queries `LATERAL` nuevas con el mismo patrón (en `Delete`, `GetByIDWithRoles`, etc.), aplicar `NULLS LAST` ahí también desde el inicio.

**#8 — Verificar que `platform_admin` no sea asignable fuera del tenant plataforma**: `tenant_can_use_role(tenant_id, role_id)` (migración 000010) y el trigger `enforce_platform_role_tenant` deberían rechazar asignar `platform_admin` (is_global=TRUE, no está en `('admin','operario')`) fuera del tenant plataforma — pero no hay test que lo confirme. El único test existente (`TestTriggerRechazaInsertRawDeAdminEnTenantNoPlataforma`, `internal/repo/pg/user_roles/repository_test.go:141-165`) solo cubre `role_id='admin'`. Fix: agregar un test parametrizado (o dos casos nuevos) para `platform_admin` y `super_admin` (mismo eje `is_global`, mismo riesgo). Es verificación pura — si el trigger ya rechaza correctamente, el test cierra el pendiente sin tocar código de producción; si no, ahí aparece un bug real a arreglar antes de cerrar este punto.

---

## Sub-proyecto F — Bug UX formulario "Nuevo Tenant" (item #14)

Revisado `src/components/tenants/tenant-form.tsx` a fondo por análisis estático: en el flujo de **creación** no hay `form.reset()`, remount vía `key`, ni refetch que debería borrar valores tras un error de validación — el único `form.reset()` del archivo está atado a `initialData` (líneas 110-132), que solo aplica al flujo de **edición**. `onSubmit` en modo creación (líneas 134-224) solo llama `form.setError(...)` en fallos puntuales (`adminEmail` faltante, `subdomain` en conflicto) y no toca los demás campos.

No se pudo confirmar la causa raíz por análisis estático — candidatos no descartados: interacción con autofill del browser, un comportamiento de React 19/Zod no evidente en el código fuente, o que el bug ya no reproduzca (posible que se haya corregido indirectamente en un cambio posterior no relacionado). Plan: reproducir interactivamente (dev server + browser automation) llenando el form y forzando un error de validación real, antes de proponer cualquier fix — esto es un caso para `systematic-debugging`, no un diseño cerrado.

---

## Sub-proyecto G — Decisiones diferidas documentadas (items #9, #15)

No se implementan ahora. Se cierra el pendiente con esta decisión documentada en vez de código especulativo (YAGNI — ninguno de los dos tiene caso de uso concreto hoy, ambos están explícitamente marcados así en el handoff origen).

- **#9 — Split de `_manage` en `_create`/`_update`/`_delete`**: implicaría migración de DB + cambios de backend (enforcement) + frontend (gating), para una granularidad de permisos que hoy nadie pidió. **Señal para retomarlo**: aparición de un caso de uso concreto — ej. un rol que necesite poder editar pero no borrar, o crear pero no editar.
- **#15 — Rate limiting real en el endpoint público**: `consumers/middleware.RateLimit()` es un stub no-op; el endpoint público `GET /api/v1/public/tenants/:idOrSubdomain` no tiene ningún middleware aplicado (ni siquiera `CORS`/`Logger`). El propio `CLAUDE.md` del backend documenta que producción "funciona como entorno de prueba" sin usuarios reales todavía. **Señal para retomarlo**: evidencia real de abuso/scraping sobre ese endpoint, o la salida de esta fase MVP (momento en que ese mismo `CLAUDE.md` dice que hay que reconfirmar el nivel de cautela).

---

## Orden de implementación propuesto

Los que no dependen de investigación interactiva primero, agrupados por archivo tocado para minimizar PRs:

1. **C** (RBAC edge-devices) — aislado, rápido, cierra un gap de seguridad real.
2. **A** (cross-tenant en las 4 mutaciones) — mismo archivo que E7 en parte (`postgres.go`), agruparlos si el diff no queda confuso.
3. **E** (#7 tiebreak NULL + #8 test platform_admin) — rápidos, bajo riesgo.
4. **D** (batch frontend) — independiente del backend, en paralelo si se quiere.
5. **G** (documento de decisión diferida) — sin código, se puede hacer en cualquier momento, incluso en paralelo con lo anterior.
6. **B** (investigación Hallazgo D) — necesita logs de Cloud Run.
7. **F** (bug formulario Nuevo Tenant) — necesita repro interactivo.

B y F van al final porque su resultado (causa raíz) no se conoce de antemano y pueden requerir ida y vuelta con el usuario (acceso a logs, confirmación de una reproducción en browser).

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

### 🆕 Investigación completada (2026-08-19, sesión de cierre de producción) — causa raíz identificada, NO es un bug del backend Go

**Metodología**: se ubicó el timestamp exacto del soft-delete del usuario de smoketest vía SQL directo (`deleted_at = 2026-08-19 12:25:50.036909+00`), y se cruzó contra los logs de Cloud Run del backend y los logs de runtime de Vercel del frontend para esa misma ventana.

**Hallazgo — logs de Cloud Run (backend)**: el `DELETE /api/v1/users/{id}` que produjo ese `deleted_at` **devolvió 204** (`users/service.go:268 "user soft-deleted"` a las `12:25:50.109Z`, log HTTP `status: 204`). El `DELETE` del tenant en la misma sesión también muestra `status: 200`. **El backend Go nunca devolvió un 5xx para estos dos borrados** — el paso post-commit que se sospechaba (limpieza de Supabase Auth, serialización, etc.) no existe como tal; no hay ningún log de error entre el `UPDATE` y la respuesta.

**Hallazgo — logs de runtime de Vercel (frontend BFF)**: `get_runtime_errors` sobre el proyecto `v0-embolsadora` muestra un error agrupado:
```
[BFF DELETE /api/users] backend status: 503 body: Backend unreachable
routes: /api/users/[id]
last=2026-08-19T12:25:48.000Z   (2 ocurrencias totales, la otra el 2026-08-04)
```
Ese timestamp es **~1.7s antes** de que el backend registre haber recibido el request (`12:25:49.725Z`, `delete user request`). El mensaje "Backend unreachable" sale de `src/lib/backend-fetch.ts:82-105`: el `fetch()` de la función serverless de Vercel hacia Cloud Run está envuelto en un `try/catch` desnudo — si el `fetch` en sí lanza (timeout, conexión rechazada, TLS), se mapea incondicionalmente a `{ status: 503, error: 'Backend unreachable' }`, sin ninguna forma de saber si el backend llegó a recibir el request.

**Causa raíz real**: Cloud Run (`embolsadora-api`) no tiene `minScale` configurado (confirmado vía `gcloud run services describe`, el campo viene vacío = default 0) — el servicio puede hacer *cold start* completo ante tráfico esporádico. El handler Go de `DeleteUser` **no chequea `r.Context().Done()`**, así que aunque el `fetch()` de Vercel aborte/timeout del lado del cliente, el request ya aceptado por el runtime de Go sigue procesándose hasta el final y committea el `UPDATE`. El resultado observable es exactamente el síntoma reportado: el cliente ve un 5xx, pero el dato ya quedó escrito — porque son dos partes independientes de la misma llamada HTTP que se desincronizaron por la latencia del cold start, no un bug de lógica de aplicación en ningún lado.

**Por qué no se propone un fix acotado en este plan**: la causa es de infraestructura/configuración (cold start de Cloud Run + timeout de `fetch` de Vercel), no un bug de código con un fix chico y aislado. Las dos palancas reales disponibles tienen trade-offs que le corresponden decidir al usuario, no a una corrección silenciosa:
- **`gcloud run services update embolsadora-api --min-instances=1`**: elimina el cold start en este servicio a costa de un instance corriendo 24/7 (costo continuo en vez de escalar a cero).
- **Subir el timeout del `fetch` en `backend-fetch.ts`** (hoy usa el default, sin `AbortSignal`/timeout explícito) le da más margen a un cold start antes de que Vercel se dé por vencido — pero no lo elimina, solo lo hace menos frecuente, y alarga la latencia percibida en el peor caso.
- Ya existe una mitigación parcial en el frontend (Fix 1 de la sesión del 2026-08-19): un DELETE reintentado sobre un recurso que ya se borró da 404, y el toast lo trata como éxito — así que el impacto real hoy es "mensaje de error confuso en el primer intento", no pérdida de datos ni un estado inconsistente.

**Cierre**: pendiente documentado, no bloqueante — no se identificó ningún bug de aplicación para arreglar en `embolsadora4.0-cloud` ni en `embolsadora-frontend`. Recomendación para una decisión futura del usuario: evaluar `min-instances=1` en Cloud Run si estos cold starts se vuelven más frecuentes al salir de la fase MVP (ver la nota de `CLAUDE.md` del backend sobre "producción funciona como entorno de prueba").

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

### 🆕 Investigación completada (2026-08-19, sesión de cierre de producción) — no reproduce

Se reprodujo interactivamente contra `mrgsrl` (dev server local apuntando al backend/DB reales de producción, login como Super Admin real):

1. **Intento 1** — se llenaron todos los campos de "Información Básica" y "Dirección" (Calle, Ciudad, Provincia, Código Postal, País) con valores de prueba, se dejó vacío el email del admin (el único campo que dispara `form.setError('adminEmail', ...)` explícitamente en `tenant-form.tsx`), y se hizo submit. Apareció el error "Email del admin es requerido" — **todos los valores de dirección tipeados permanecieron intactos**, sin excepción.
2. **Intento 2** — con el email ya completo, se vació el campo Ciudad para forzar el error de validación de Zod directamente sobre un campo de dirección (`react-hook-form` revalida en `onChange` tras el primer submit, así que el error "La ciudad es requerida" apareció sin necesidad de un segundo submit). **El resto de los campos de dirección (Calle, Provincia, Código Postal, País) permanecieron intactos.**

**Conclusión**: el bug no reproduce con ninguno de los dos triggers de validación disponibles en el formulario actual. Esto es consistente con el análisis estático original: no existe ningún `form.reset()`, remount, ni refetch en el flujo de creación que pudiera limpiar valores — react-hook-form simplemente no toca campos que no fallaron su propia validación. No se identificó ningún cambio de código posterior al reporte original que explique una corrección indirecta; lo más probable es que el reporte original haya sido un caso puntual de autofill del browser interfiriendo, o una observación imprecisa del momento.

**Cierre**: pendiente cerrado sin fix — no hay nada que reproducir ni arreglar en el estado actual del código. Si el problema reaparece, sería valioso capturar la secuencia exacta de acciones (incluyendo si el browser autocompletó algún campo) la próxima vez que ocurra.

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

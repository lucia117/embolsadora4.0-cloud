---
id: CLOUD-ADR-020
title: "La suspensión de un cliente se aplica en la resolución de tenant"
status: aceptada
date: 2026-10-16
owner: Federico Degiovanni
last_reviewed: 2026-10-16
supersedes: []
superseded_by: []
---

# CLOUD-ADR-020 — La suspensión de un cliente se aplica en la resolución de tenant

## Contexto

`tenants.is_active` existe desde la migración `000001` y está cableado de punta a punta como
**dato**: el repositorio lo lee y lo escribe, `PATCH /api/v1/tenants/{tenantId}` lo acepta, las
respuestas lo serializan como `isActive` y el frontend lo muestra con un `StatusBadge`
«Activo / Inactivo» y lo deja editar desde el formulario del cliente.

Lo que no existía era **que sirviera para algo**. Una búsqueda de `is_active` sobre `internal/`
devolvía únicamente el modelo de dominio y las consultas SQL del repositorio de tenants: ningún
middleware, ninguna verificación de autorización y ninguna consulta de negocio lo leían.

La consecuencia es que suspender un cliente no suspendía nada. Sus usuarios seguían
autenticándose y operando con normalidad, mientras la interfaz los mostraba como «Inactivo». El
único chequeo de estado que sí existe —en `JWTAuth`, que rechaza con 403 a un usuario `revoked`
o `disabled`— es **por usuario**, no por cliente, y no cubre este caso.

El objetivo específico 3 del anteproyecto y la historia `US-003` («Crear, editar, **suspender** o
eliminar empresas») comprometen la suspensión como capacidad real. La alternativa que sí existía
era `DELETE /api/v1/tenants/{tenantId}`, que ejecuta un `DELETE FROM tenants` sin recuperación:
la única forma efectiva de cortarle el acceso a un cliente era borrarlo.

## Decisión

El estado del cliente se verifica en `TenantFromHeader`, que es el punto por el que pasa toda
petición con `X-Tenant-ID` y donde ya se resuelve la membresía. La consulta de membresía ya hacía
`JOIN tenants`, así que agregar `COALESCE(t.is_active, TRUE)` al `SELECT` no suma una consulta.

Un miembro común de un cliente suspendido recibe **403 `tenant suspended`**.

Con dos excepciones, que no son concesiones por comodidad:

- **Los operadores de plataforma pasan.** Si no lo hicieran, la suspensión sería irreversible:
  nadie podría volver a poner `is_active` en `true`, y una medida pensada como reversible se
  convertiría en una baja de hecho —peor que el `DELETE` que viene a reemplazar, porque además
  sería silenciosa. Se reutiliza `resolvePlatformOperator`, que ya es el camino cross-tenant de
  [`CLOUD-ADR-015`](CLOUD-ADR-015-plataforma-cross-tenant.md) y que no filtra por `is_active`. El
  acceso queda auditado por `auditCrossTenantGrant`, igual que cualquier otro acceso cruzado.
- **El tenant plataforma no se bloquea a sí mismo.** Poner su `is_active` en `false` dejaría a
  todo el mundo afuera del sistema, sin camino de vuelta.

`COALESCE` protege contra la columna nula: `is_active` es `boolean DEFAULT true` pero admite
`NULL`, y un `NULL` tiene que leerse como «activo», no como «suspendido». Interpretarlo al revés
convertiría cualquier fila vieja o insertada a mano en un lock-out.

## Consecuencias

- Suspender un cliente pasa a ser una alternativa real a borrarlo, que era el objetivo.
- **Es un cambio de control de acceso con efecto inmediato.** Cualquier cliente que hoy tenga
  `is_active = false` —sea a propósito o por un dato cargado sin intención— deja de funcionar
  para sus usuarios en cuanto esto se despliegue. Conviene revisar esa columna en producción
  antes de desplegar.
- El `DELETE` destructivo sigue existiendo. Esta ADR no lo toca: ofrece la alternativa, no
  retira la operación.
- La verificación es por petición y no agrega costo: usa el `JOIN` que ya estaba.
- No cambia ninguna ruta ni ningún contrato, así que `docs/openapi.yaml` no se modifica. Lo que
  cambia es el conjunto de peticiones que reciben 403.

## Alternativas consideradas

**Verificar en cada caso de uso.** Habría que acordarse en cada uno nuevo, y el primer olvido es
un agujero silencioso. El middleware es el único lugar por el que pasa todo.

**Filtrar por `is_active` en las consultas de los repositorios.** Resuelve la lectura de datos y
no la autenticación: el usuario seguiría entrando y vería pantallas vacías en vez de un rechazo
explicable.

**Bloquear también a los operadores de plataforma.** Es la opción más simple y hace la suspensión
irreversible. Descartada por eso.

## Verificación

`internal/api/middleware/tenant_suspendido_test.go`, cuatro casos de integración: miembro común
de un cliente suspendido (403), miembro de uno activo (200), operador de plataforma sobre un
cliente suspendido (200) y tenant plataforma con `is_active = false` (200).

El primero es una regresión real: ejecutado contra el código anterior devuelve 200.

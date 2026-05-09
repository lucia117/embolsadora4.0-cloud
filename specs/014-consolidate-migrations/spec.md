# Feature Specification: Consolidación de migraciones para deploy en Koyeb

**Feature Branch**: `014-consolidate-migrations`
**Created**: 2026-05-08
**Status**: Draft
**Input**: User description: "necesito pasar de tener multiples migraciones a tener una sola, mas los seed necesarios para que funcione el software para poder llevar los cambios a la base de datos de produccion en koyeb"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Aplicar el esquema completo en una base de datos vacía (Priority: P1)

Como operador del sistema, necesito poder aplicar el esquema final de la base de datos sobre una instancia vacía (la de producción en Koyeb) ejecutando una sola operación de migración, sin tener que conocer ni encadenar el historial completo de cambios incrementales que se acumuló durante el desarrollo.

**Why this priority**: Es el bloqueante para llevar el software a producción. Sin un esquema reproducible y consolidado, cada despliegue requiere ejecutar 20+ pasos con riesgo de inconsistencias, conflictos de numeración (ej: dos migraciones con el mismo prefijo `000019`) y fallos de orden.

**Independent Test**: Apuntar la herramienta de migraciones contra una base de datos PostgreSQL recién creada y verificar que, al finalizar, todas las tablas, índices, llaves foráneas y restricciones del modelo final están presentes y la operación termina sin errores.

**Acceptance Scenarios**:

1. **Given** una base de datos PostgreSQL vacía, **When** se ejecuta la migración inicial consolidada, **Then** el esquema resultante es funcionalmente equivalente al que se obtenía aplicando todas las migraciones históricas en orden.
2. **Given** la migración inicial aplicada, **When** se ejecuta la operación de reversión, **Then** la base queda nuevamente vacía sin objetos residuales ni errores.
3. **Given** el repositorio en su estado final, **When** un nuevo desarrollador clona el proyecto y aplica las migraciones, **Then** obtiene el mismo esquema que producción sin necesidad de pasos manuales adicionales.

---

### User Story 2 - Sembrar los datos esenciales para que la aplicación arranque (Priority: P1)

Como operador del sistema, después de aplicar el esquema necesito cargar los datos mínimos imprescindibles —catálogo de roles y permisos del sistema, tenant administrativo de la plataforma y al menos un usuario administrador— para que la aplicación pueda autenticar, autorizar y operar en producción.

**Why this priority**: Sin estos datos la aplicación arranca pero no es usable: no hay roles para asignar, no hay permisos para validar, no hay tenant administrativo donde gestionar al resto, y nadie puede iniciar sesión. Este seed es parte del "deploy mínimo viable".

**Independent Test**: Tras aplicar el esquema y los seeds esenciales sobre una base vacía, levantar la aplicación e iniciar sesión con el usuario administrador de la plataforma; el endpoint de "datos del usuario actual" debe responder correctamente y reportar los permisos esperados.

**Acceptance Scenarios**:

1. **Given** el esquema ya aplicado, **When** se ejecuta el seed esencial, **Then** existen los roles globales y de tenant, el catálogo completo de permisos del sistema y el tenant administrativo de la plataforma con su usuario admin.
2. **Given** los seeds esenciales aplicados, **When** el usuario admin de plataforma se autentica, **Then** la aplicación lo reconoce, le devuelve sus permisos y puede ejecutar operaciones administrativas.
3. **Given** los seeds esenciales aplicados, **When** se intenta volver a aplicarlos, **Then** la operación es idempotente o produce un error claro y no deja datos a medias.

---

### User Story 3 - Cargar tenants de prueba en entornos no productivos (Priority: P2)

Como desarrollador o tester, necesito poder cargar opcionalmente un conjunto de tenants de demostración (ciudades) con sus usuarios para hacer QA, demos y desarrollo local, sin que esos datos se propaguen automáticamente a producción.

**Why this priority**: Útil para flujos de prueba pero no requerido para que producción funcione. Mantenerlo separado evita ensuciar la base de Koyeb con datos ficticios.

**Independent Test**: En un entorno de desarrollo, ejecutar manualmente el script de tenants de prueba y verificar que aparecen los tenants de ciudades y sus usuarios; en producción, omitirlo y confirmar que solo está el tenant administrativo.

**Acceptance Scenarios**:

1. **Given** una base con esquema y seeds esenciales, **When** se ejecuta el seed opcional de tenants de prueba, **Then** se crean los tenants de ciudades y sus usuarios asociados sin afectar los datos esenciales.
2. **Given** un despliegue a producción, **When** no se ejecuta el seed opcional, **Then** la base contiene únicamente el tenant administrativo y los usuarios reales que se invitaron posteriormente.

---

### Edge Cases

- ¿Qué pasa si alguien intenta aplicar la migración consolidada sobre una base que ya tiene tablas de versiones anteriores? Se espera que la operación falle de forma clara antes de modificar nada, no que intente "fusionarse".
- ¿Qué pasa si el seed esencial se ejecuta dos veces? Debe ser idempotente o fallar de forma explícita sin dejar datos parciales.
- ¿Qué pasa con el conflicto histórico de dos archivos con el mismo prefijo `000019`? Debe quedar resuelto: en el repo final no puede haber dos migraciones con el mismo número.
- ¿Cómo se entrega la credencial inicial del admin de plataforma sin exponerla en el repositorio? Mediante invitación con flujo de set/reset de contraseña, o un valor que se rota inmediatamente después del primer login.
- ¿Qué pasa si el operador olvida ejecutar el seed esencial después del esquema? La aplicación debe permitir detectarlo (por ejemplo, no hay roles ni tenant admin) sin corromperse.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE proveer una única migración inicial que, aplicada sobre una base de datos vacía, deje el esquema funcionalmente equivalente al estado final actual del modelo (todas las tablas, índices, llaves foráneas, restricciones y triggers que el código de la aplicación necesita).
- **FR-002**: La migración inicial DEBE tener su contraparte de reversión que devuelve la base de datos a un estado vacío sin objetos residuales.
- **FR-003**: El sistema DEBE proveer un mecanismo de seeds **esenciales** separado del esquema, que cargue: el catálogo de roles globales y de tenant, el catálogo completo de permisos del sistema, y el tenant administrativo de la plataforma. El usuario administrador inicial NO se siembra en SQL: se crea en Supabase Auth y se auto-provisiona en `users` vía `internal/api/usecases/auth_usecase.go::ProvisionUser` en el primer login; la asignación al rol `super-admin` dentro del tenant MRG se realiza como paso documentado post-deploy (ver `quickstart.md` Paso 5). Esto preserva FR-007 (cero credenciales en el repo) sin romper FR-008 (cero código nuevo).
- **FR-004**: El sistema DEBE proveer un mecanismo de seeds **opcionales** (tenants de ciudades de prueba y sus usuarios) que pueda ejecutarse explícitamente en entornos no productivos y que NO se aplique automáticamente al desplegar.
- **FR-005**: El repositorio NO DEBE contener migraciones con prefijos numéricos duplicados ni archivos huérfanos del historial anterior una vez completada la consolidación.
- **FR-006**: La documentación de migraciones DEBE describir el flujo simplificado para producción, incluyendo el comando o procedimiento para aplicar el esquema y los seeds esenciales contra Koyeb.
- **FR-007**: El proceso DEBE permitir entregar al operador credenciales iniciales del admin de plataforma sin filtrarlas al control de versiones (vía invitación, variable de entorno, o rotación post-primer-login).
- **FR-008**: La consolidación NO DEBE requerir cambios en el código de la aplicación (el modelo de dominio, repositorios y handlers existentes deben seguir funcionando contra el esquema consolidado sin modificaciones).

### Key Entities

- **Esquema base**: Conjunto de objetos relacionales que sostiene la aplicación: usuarios, tenants, asignaciones usuario-tenant-rol, roles, permisos, dispositivos edge, layouts de dashboard, reglas de alarma, log entries, notificaciones, invitaciones. Refleja el estado final tras todas las migraciones históricas.
- **Seeds esenciales**: Datos mínimos sin los cuales la aplicación no puede operar — roles del sistema, permisos, tenant administrativo de plataforma, usuario admin inicial.
- **Seeds opcionales**: Datos de demostración o QA — tenants de ciudades de prueba y sus usuarios; útiles en dev/staging, excluidos de producción.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Aplicar el esquema consolidado más los seeds esenciales sobre una base de datos vacía termina en menos de 30 segundos sin errores ni intervención manual.
- **SC-002**: Tras el deploy completo en Koyeb, el usuario administrador de la plataforma puede autenticarse y consultar sus datos en menos de 1 minuto desde el momento en que se le entrega la credencial inicial.
- **SC-003**: La suite de pruebas existente del proyecto sigue ejecutándose sin regresiones tras la consolidación, sin requerir cambios en el código de la aplicación.
- **SC-004**: La cantidad de archivos de migración (contando up + down por separado) se reduce al menos en un 80% respecto del estado actual (de 40 archivos / 20 pares a 4 archivos / 2 pares = 90%), eliminando los conflictos de numeración.
- **SC-005**: Un operador que recibe únicamente la documentación actualizada y acceso a Koyeb puede llevar la base a `schema_migrations.version=2, dirty=false` en menos de 15 minutos. La activación del admin MRG (login + `GET /api/v1/me` 200) está cubierta por SC-002.
- **SC-006**: En un entorno de producción, ejecutar solamente los pasos documentados como "esenciales" no introduce ningún tenant ni usuario de prueba.

## Assumptions

- La base de datos de producción en Koyeb se puede recrear desde cero (no hay datos productivos preexistentes que preservar). *Confirmado por el usuario.*
- Los seeds esenciales para producción incluyen RBAC (roles y permisos) y el tenant de plataforma MRG con su usuario admin; los tenants de ciudades quedan fuera de producción. *Confirmado por el usuario.*
- El equipo continúa usando una herramienta de migraciones versionadas (no se reemplaza por scripts manuales sueltos), preservando la trazabilidad del esquema en el repositorio.
- La credencial inicial del admin de plataforma se entrega por un canal fuera del repositorio y se rota tras el primer login.

## Out of Scope

- Migrar o transformar datos productivos preexistentes (no aplica: la base se recrea).
- Cambios en el código de la aplicación, modelos de dominio, endpoints o lógica de negocio.
- Pruebas de carga, performance o resiliencia más allá del tiempo de aplicación de la migración.
- Provisión de infraestructura en Koyeb (la base de datos ya existe; solo se le aplica el esquema).
- Automatización de CI/CD para ejecutar migraciones en cada deploy (puede ser un follow-up).

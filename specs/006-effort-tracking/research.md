# Investigación (Fase 0): Registro de Esfuerzo

**Feature**: `specs/006-effort-tracking` | **Fecha**: 2026-10-24 | **Plan**: [plan.md](./plan.md)

Este documento resuelve las incógnitas técnicas del plan. Las decisiones de producto ya están
cerradas en la spec (sesiones de clarificación del 2026-10-02 y del 2026-10-04); acá solo se decide
**cómo** se construyen. Cada sección sigue el formato Decisión / Rationale / Alternativas
consideradas.

---

## 0. Contexto: sobre qué se construye esta feature

Esta feature consume datos de cuatro features ya integradas y cierra dos puertos que dejaron
anotados para ella:

| Feature | Qué aporta | Cómo lo consume la 006 |
| --- | --- | --- |
| 001 | `users`, sesión JWT, `RequireSession`, `CurrentUserID` | Autoría del registro y nombre del autor (`users.full_name`) |
| 002 | `projects` (estado, `start_date`, `hours_per_story_point_hundredths`), `project_members`, `RequireMembership`, `APP_TIMEZONE` | Acceso, fecha mínima, factor vigente, solo lectura, "hoy" |
| 003 | `user_stories` (Story Points, `NULL` = sin estimar), `RequireStoryAccess`, puerto `EffortRecords` | Historia del registro, comparación, bloqueo de eliminación |
| 004 | `sprints` (`status`, `closed_at`), `sprint_stories` (`outcome` congelado al cierre), puerto `SprintEffort` | Anclaje al sprint, plazo de gracia, bloqueo de eliminación del sprint |
| 005 | — | Ninguna relación. La 006 no toca el módulo `poker` |

Los dos puertos que esta feature reemplaza:

- `backlog/repository/effort_records.go` (`EffortRecords.HasEffort(storyID)`), que hoy responde
  `false` y bloquea la eliminación de una historia con esfuerzo (003, FR-041 y FR-043; 006, RC-05).
- `sprints/repository/sprint_effort.go` (`SprintEffort.HasEffort(sprintID)`), que hoy responde
  `false` y bloquea la eliminación de un sprint Planificado con esfuerzo (004, FR-048, RN-20 y RC-10;
  006, RC-07). Su comentario además anticipa la clave foránea con `RESTRICT` y la traducción de
  `gorm.ErrForeignKeyViolated` a `ErrSprintHasEffort` en `Delete`.

---

## 1. Versiones del stack: sin cambios

### Decisión

Se mantienen exactamente las versiones vigentes en `main`: Go 1.27.3, Gin v1.12.0, GORM v1.31.2 con
`gorm.io/driver/postgres` v1.6.3, `google/uuid` v1.6.0, `swaggo/*`, `testify` v1.12.1; Next.js
16.4.2, React 19.3.0, Tailwind CSS 4.3.3, Vitest 5.0.3. **Ninguna dependencia nueva** de Go ni de
npm.

### Rationale

Todo lo que pide la spec se resuelve con lo existente: aritmética entera de la biblioteca estándar
para las horas y los porcentajes, `clause.Locking` de GORM para los bloqueos, `json.RawMessage` para leer
decimales sin pasar por `float64`, `fetch` y componentes propios en el frontend.

### Alternativas consideradas

- **Una librería de decimales** (`shopspring/decimal` o similar): innecesaria, porque todas las
  magnitudes son enteros de centésimas (sección 5). Requeriría autorización (Principio IV).
- **Una librería de tablas o de formularios en el frontend**: la vista es un formulario de tres
  campos y dos tablas simples; no justifica una dependencia.

---

## 2. Un módulo `effort` y la forma de las rutas (FR-001, FR-030, FR-036, FR-053)

### Decisión

Un único módulo vertical `backend/internal/effort/` con sus cuatro capas. **Siete endpoints**:

| # | Método y ruta | Historia | Middleware de acceso |
| --- | --- | --- | --- |
| 1 | `POST /api/v1/stories/{storyId}/effort-entries` | US1 | `RequireStoryAccess` de la 003 |
| 2 | `GET /api/v1/stories/{storyId}/effort` | US2 | `RequireStoryAccess` de la 003 |
| 3 | `GET /api/v1/stories/{storyId}/effort-comparison` | US5 | `RequireStoryAccess` de la 003 |
| 4 | `PATCH /api/v1/effort-entries/{entryId}` | US3 | `RequireEffortEntryAccess` (nuevo) |
| 5 | `DELETE /api/v1/effort-entries/{entryId}` | US3 | `RequireEffortEntryAccess` (nuevo) |
| 6 | `GET /api/v1/projects/{projectId}/effort-entries` | US4 | `RequireMembership` de la 002 (subgrupo) |
| 7 | `GET /api/v1/projects/{projectId}/effort-comparison` | US5 | `RequireMembership` de la 002 (subgrupo) |

Las rutas sobre una historia cuelgan del prefijo `/api/v1/stories/:storyId` y reciben el
`RequireStoryAccess` de la 003 como `gin.HandlerFunc` desde `cmd/app/`, igual que hicieron la 004 y la
005. Las de proyecto cuelgan del subgrupo `/api/v1/projects/:projectId` que la 002 ya protege con
sesión y membresía. Las de ítem van en un grupo plano `/api/v1/effort-entries/:entryId`.

### Rationale

- Reutilizar `RequireStoryAccess` y `RequireMembership` hace que el `404` de historia y de proyecto
  sea **byte a byte el mismo** que el de la 003 y la 002 (FR-059, SC-018), sin duplicar consultas.
- La forma `colección bajo el padre, ítem plano` es la convención que siguen la 003 (historias), la
  004 (sprints) y la 005 (sesiones).
- La comparación de una historia y la del proyecto son endpoints separados porque FR-053 pide ambas
  y porque la de una historia la usa el detalle de la historia sin traer todo el proyecto.

### Alternativas consideradas

- **`GET /api/v1/stories/{storyId}/effort` con la comparación incluida**: mezcla dos lecturas que la
  spec trata como historias distintas (US2 y US5) y obliga a leer el factor del proyecto en cada
  consulta de esfuerzo. Descartada por responsabilidad única.
- **Rutas de ítem anidadas** (`/api/v1/stories/{storyId}/effort-entries/{entryId}`): exige validar
  que el registro pertenece a la historia de la ruta, sin aportar nada; la 003 y la 004 ya usan ítems
  planos.
- **Un módulo por historia de usuario** (registro, consulta, comparación): fragmenta una sola
  entidad en tres módulos. Descartado por el Principio I.

---

## 3. Acceso a un registro: una consulta y un `404` (FR-057, FR-059, US3-18, SC-018)

### Decisión

`RequireEffortEntryAccess` resuelve en **una sola consulta** la existencia del registro y la
membresía vigente del titular de la sesión en el proyecto de su historia:

```text
effort_entries → user_stories (project_id) → project_members (project_id, user_id = sesión)
```

Devuelve un `EntryAccess` con `EntryID`, `StoryID`, `ProjectID`, `AuthorID` y `ProjectFinished`. El
identificador mal formado, el inexistente y el de un proyecto del que quien pide no es integrante
comparten **una única salida 404** con el mismo cuerpo (`"El registro de esfuerzo no existe."`).

La **autoría** no la resuelve el middleware: la decide el servicio (FR-022), para que la respuesta
`403` solo la reciba un integrante vigente que intenta tocar un registro ajeno.

### Rationale

- Es el patrón de `RequireStoryAccess`, `RequireSprintAccess` y `RequireEstimationAccess`.
- US3-18 queda resuelto sin código especial: una persona quitada del proyecto no tiene fila en
  `project_members`, así que recibe el `404` de registro inexistente, aunque sea la autora. Si la
  reincorporan, la fila vuelve y recupera el acceso (Casos Límite, "quitado y reincorporado").
- El orden `404 → 403` no filtra nada: el `403` solo lo ve quien ya puede leer ese registro por
  FR-058.

### Alternativas consideradas

- **Responder `404` también al registro ajeno**: ocultaría un registro que el integrante ve en la
  lista (FR-058). La spec pide "rechazo por falta de permiso".
- **Verificar la autoría en el middleware**: obligaría a un segundo middleware o a uno con dos
  modos, solo para escrituras. Descartado por simplicidad.

---

## 4. Modelo: una tabla `effort_entries` y una marca en `user_stories` (FR-011, FR-012, FR-014, RC-05)

### Decisión

- **Una tabla** `effort_entries` con: `id` (UUID), `story_id`, `user_id`, `sprint_id` anulable,
  `entry_date` (`date`), `activity` (`varchar(300)`), `hours_hundredths` (`integer`), `created_at` y
  `updated_at`. Claves foráneas hacia `user_stories`, `users` y `sprints` **sin `ON DELETE
  CASCADE`** (la acción por defecto, `NO ACTION`, rechaza el borrado del padre).
- **Una columna nueva** `user_stories.ever_had_effort` (`boolean NOT NULL DEFAULT false`), que el
  alta de un registro pone en `true` y que nadie vuelve a `false`. El puerto `EffortRecords` de la
  003 la lee.
- **Sin columna `project_id`** en `effort_entries`: el proyecto se deriva de la historia, que nunca
  cambia de proyecto.

El detalle de columnas, índices y reglas está en [data-model.md](./data-model.md).

### Rationale

- La spec define una sola entidad persistible nueva (Registro de esfuerzo). La asociación al sprint
  es una columna, no una entidad: es una referencia anulable e inmutable (FR-015, RN-08).
- **RC-05 exige una marca permanente**: "aunque después se eliminen todos los registros, la historia
  no vuelve a ser eliminable". El existir de filas no alcanza, porque las filas se pueden borrar
  (FR-026). Es el mismo problema que la 004 resolvió con `ever_committed` (su research, sección 7) y
  se resuelve igual, con el mismo costo: una columna escrita por otro módulo con `UpdateColumn`, sin
  tocar `updated_at`.
- Las claves foráneas sin cascada cumplen FR-029 y RN-21 en la base: ni borrar una historia, ni un
  sprint, ni un usuario pueden arrastrar registros de esfuerzo.
- `project_id` sería una columna derivable (Principio I). Las consultas por proyecto la obtienen con
  un `JOIN` a `user_stories`, que ya tiene un índice por `project_id`.

### Alternativas consideradas

- **Borrado lógico** (`deleted_at`) para que "tuvo esfuerzo" se derive de filas borradas: la spec
  dice que eliminar es definitivo y sin papelera (Supuestos); además cada consulta tendría que
  filtrar las borradas. Descartado.
- **Una tabla de "historias con esfuerzo"**: una tabla para un booleano. Descartada.
- **`project_id` desnormalizado**: ahorra un `JOIN` en dos consultas a cambio de una columna que hay
  que mantener coherente. Descartado.

---

## 5. Aritmética exacta: centésimas de hora enteras (FR-004, FR-046 a FR-049, SC-015, SC-022)

### Decisión

- **Todas las horas viajan dentro del backend como enteros de centésimas de hora**: las de un
  registro (`hours_hundredths`, de 25 a 2400 y múltiplo de 25), las estimadas, las reales, las
  diferencias y los totales. Es la misma unidad que el factor del proyecto
  (`hours_per_story_point_hundredths`) y que el factor congelado del sprint (004).
- **Horas estimadas** = `story_points × hours_per_story_point_hundredths`: un entero exacto en
  centésimas.
- **Diferencia absoluta** = `real − estimada`, en centésimas, con signo.
- **Diferencia porcentual** en centésimas de punto porcentual:
  `round(diferencia × 10000 / estimada)`, con un único redondeo entero **mitad lejos de cero** sobre
  el cociente exacto (se calcula con cociente y resto enteros, nunca con `float64`). Si la estimada
  es 0, es "no calculable".
- **En la API**, todas las magnitudes de horas y de porcentaje se serializan como **números JSON con
  exactamente 2 decimales** (`6.50`, `-20.00`, `13.10`), con un tipo de `delivery` que implementa
  `json.Marshaler` a partir del entero (`"%d.%02d"` con signo). El valor "no calculable" es `null`.
- **En la entrada**, las horas llegan como número JSON y se leen como `json.RawMessage` sobre su
  texto literal: un texto entre comillas, `null` o un booleano se rechazan, porque `json.Number`
  aceptaría `"6.5"`. Como máximo 2 decimales y sin exponente; se convierten a centésimas sin pasar
  por `float64`, con la misma lectura de dígitos que `ParseHoursPerStoryPoint` de la 002.

### Rationale

- SC-015 exige que 100 registros de 0,25 sumen **exactamente** 25. Con enteros es trivial; con
  `float64`, `0.25` es exacto pero los factores como `6.1` no lo son, y la comparación los
  multiplica.
- SC-022 exige "exactamente 2 decimales" y "ningún resultado difiere del redondeo solo al final". Un
  único redondeo entero sobre el cociente exacto cumple las dos cosas, y el formato `%d.%02d`
  garantiza los dos decimales también para `20.00` (un `float64` se serializaría como `20`).
- **Mitad lejos de cero** es simétrico para desviaciones positivas y negativas (`+0,125 %` y
  `−0,125 %` redondean a `±0,13 %`), y es lo que hace `math.Round`, que es lo que un lector espera.
  La spec 008 (FR-033) no fija el criterio de empate; RC-13 obliga a que ambas features usen el
  mismo, así que la función queda en `effort/domain` con un comentario para que la 008 la reutilice
  (punto señalado en el plan).
- Ejemplo de 008/US1-1 reproducido con esta aritmética: diferencia 1650, estimada 12600 →
  `1650 × 10000 / 12600 = 1309,52…` → `1310` → `13.10`.

### Alternativas consideradas

- **Cuartos de hora enteros** (`quarter_hours`): expresa de forma natural la regla del múltiplo de
  0,25, pero introduce una segunda unidad en el producto, distinta de la del factor; cada
  comparación tendría que convertir. Descartada.
- **`numeric(5,2)` en PostgreSQL y `float64` en Go**: pierde exactitud al pasar por `float64` y
  obliga a redondear en Go. Descartada.
- **Porcentaje como texto** (`"13.10"`): garantiza los decimales pero deja de ser un número en el
  contrato. Descartada en favor del número JSON con formato fijo.

---

## 6. Validación de entrada (FR-003, FR-004, FR-006, FR-013, FR-019, FR-023, FR-025)

### Decisión

La validación de forma vive en `effort/delivery/validation.go` y devuelve **todos** los campos
inválidos a la vez como `400` con `ValidationErrorResponse` de la 001, antes de llegar al servicio
(Principio VII):

| Campo | Regla | Mensaje (resumen) |
| --- | --- | --- |
| `entryDate` | Obligatoria, `AAAA-MM-DD`, fecha real | "La fecha es obligatoria." / "…formato AAAA-MM-DD." |
| `activity` | Obligatoria; 1 a 300 caracteres (runas) tras recortar espacios extremos | "La actividad es obligatoria." / "…como máximo 300 caracteres." |
| `hours` | Obligatoria, numérica, `> 0`, `≤ 24`, múltiplo de 0,25, a lo sumo 2 decimales | "Las horas deben ser mayores que 0 y como máximo 24." / "…múltiplos de 0,25." |
| `storyId`, `userId`, `sprintId` en el `PATCH` | Prohibidos | "La historia, el integrante y el sprint de un registro no se modifican; para mover horas, eliminá el registro y creá uno nuevo." |

Las reglas que dependen de datos (fecha no futura según `APP_TIMEZONE`, fecha no anterior al inicio
del proyecto, tope diario, sprint definitivo, proyecto Finalizado, autoría) son **reglas de
dominio** y las evalúa el servicio o el repositorio con los datos releídos (secciones 7 a 10). Las
dos de fecha se informan también como `400` con `field: "entryDate"`, para que el formulario las
muestre junto al campo.

El `PATCH` es parcial: cada campo es opcional, pero debe venir al menos uno. Los campos inmutables se
decodifican como `json.RawMessage` **solo para rechazarlos** (patrón de `StoryPatch` de la 003). En el
`POST`, el DTO **no declara** `userId` ni ningún campo de autoría: Gin los ignora, y el servicio toma
siempre el autor de la sesión (FR-002, US1-25, SC-004).

### Rationale

- La asimetría `POST` ignora / `PATCH` rechaza es la de la spec: US1-25 pide **ignorar** el autor
  indicado en el alta y US3-16 pide **rechazar** el cambio de historia o integrante en la
  modificación. En el `POST`, la historia viene en la ruta y el sprint lo calcula el sistema, así que
  no hay campo que rechazar.
- Las validaciones de fecha relativas a "hoy" y al inicio del proyecto necesitan el reloj y el
  proyecto, por eso no pueden estar en `delivery`; se reportan con la misma forma para no cambiar la
  experiencia.
- La atomicidad (FR-013, FR-025) es consecuencia directa: nada se escribe si la validación falla, y
  las reglas de dominio se evalúan dentro de la transacción antes de la escritura.

### Alternativas consideradas

- **Rechazar también `userId` en el `POST`**: contradice US1-25 ("el sistema ignora ese dato").
- **`DisallowUnknownFields`** para todo el cuerpo: ninguna feature lo usa; cambiaría la convención y
  rechazaría campos inofensivos.

---

## 7. "Hoy" y la fecha de calendario (FR-006, FR-007, RC-08)

### Decisión

- `entry_date` es una columna `date` que en Go se maneja **a medianoche UTC**, igual que las fechas
  de proyecto y de sprint.
- "Hoy" se calcula en el servicio con el reloj inyectado y la zona `APP_TIMEZONE`
  (`now().In(location).Date()`), igual que `StatusService.today` de la 002 y `schedule.go` de la 004.
- La regla pura `ValidateEntryDate(date, projectStart, today)` devuelve `ErrEntryDateInFuture` si
  `date > today` y `EntryDateBeforeProjectStartError{MinDate}` si `date < projectStart`; ambos
  extremos son válidos.

### Rationale

Reutiliza la decisión de la 002, que la spec pide explícitamente (RC-08). El reloj inyectado hace
reproducibles US1-14 (hoy en la zona del sistema) y las fronteras de medianoche.

### Alternativas consideradas

- **Recibir "hoy" del cliente**: viola FR-007.

---

## 8. Anclaje al sprint y plazo de gracia (FR-014 a FR-017, FR-020, FR-021, RN-07 a RN-10, RN-23 a RN-25)

### Decisión

El sprint del registro se decide **una sola vez, al crearlo**, con una función pura del dominio:

```text
ResolveAnchor(open *OpenCommitment, lastClosed *ClosedCommitment, now, grace) (sprintID *uuid.UUID, err error)
```

- `open` es el compromiso **abierto** de la historia (`sprint_stories.outcome IS NULL`), cuyo sprint
  está Planificado o Activo. La 004 garantiza que hay como máximo uno (índice único parcial).
- `lastClosed` es el compromiso **congelado** de la historia cuyo sprint tiene el `closed_at` más
  reciente, con su `outcome` (`completed` o `not_completed`).

Reglas, en orden:

1. Si hay `open` → el sprint de `open` (FR-014, regla general). Cubre US1-1, US1-3, US1-4 y el caso
   "devuelta al backlog y recomprometida dentro del plazo del anterior": manda el sprint actual.
2. Si no, y hay `lastClosed` con `now < closed_at + grace` → el sprint de `lastClosed`, sea
   `completed` o `not_completed` (FR-014 excepción, FR-016, RN-24). Cubre US1-7, US1-8 y US1-9 a las
   17:59. "El de cierre más reciente" (FR-014) es exactamente `lastClosed`: como el plazo es el
   mismo para todos los sprints, si el más reciente venció, los anteriores también.
3. Si no, y `lastClosed.outcome = completed` → `ErrSprintFinalized` ("el sprint ya es definitivo";
   FR-016, RN-25, US1-22). Una historia Completada nunca vuelve a comprometerse (la 004 la rechaza en
   `Admission`), así que sigue "perteneciendo" a ese sprint para siempre.
4. Si no → sin sprint (historia del backlog: US1-2, o devuelta al backlog con el plazo vencido:
   RN-25).

La **vigencia del plazo** es otra función pura, compartida por el alta y por las escrituras sobre un
registro existente (FR-021: "se habilitan y se cierran a la vez"):

```text
GraceOpen(closedAt, now, grace) bool   // now < closedAt + grace
EnsureEntryWritable(sprint *AnchoredSprint, now, grace) error
```

`EnsureEntryWritable` devuelve `nil` si el registro no tiene sprint, si el sprint está Planificado o
Activo, o si está Cerrado con el plazo vigente; y `ErrEffortFinalized` si está Cerrado con el plazo
vencido (FR-020, RN-09, RN-10). El plazo **se cuenta cumplido al alcanzarse**: `now == closedAt +
grace` ya está fuera (US1-9 a las 18:01 con cierre a las 18:00 y 48 h → domingo 18:00 vence;
US3-10).

**La configuración del plazo** es la variable nueva `EFFORT_GRACE_PERIOD_HOURS`, entero de horas
`≥ 1`, con valor por defecto **48** cuando falta o está vacía; un valor no entero o menor que 1 impide
el arranque, igual que un `APP_TIMEZONE` inválido. Se evalúa siempre como `closed_at + valor actual`,
sin congelar vencimientos (RC-11).

**La asociación nunca se recalcula**: `sprint_id` se escribe en el alta y ninguna operación lo
modifica (FR-015, FR-019). Ni la modificación del registro (US3-3, US3-4, US3-5), ni el cierre, ni la
quita de la historia, ni su recompromiso tocan la columna.

### Rationale

- Separar el cálculo en funciones puras hace que las pruebas unitarias obligatorias (Principio II)
  cubran todas las combinaciones (abierto, cerrado en plazo, cerrado vencido completado, vencido no
  completado, dos cerrados, recomprometida) sin base de datos, y que las de integración solo
  verifiquen el cableado.
- Los datos que necesita ya existen en la 004: `sprint_stories.outcome` distingue las Completadas de
  las devueltas, y `sprints.closed_at` es el cierre real (RC-06). No hace falta tocar la 004 para
  leerlos.
- Un valor por defecto es lo que pide la spec ("por defecto 48 horas"); un valor inválido detiene el
  arranque para no operar con un plazo distinto del que se cree configurado.

### Alternativas consideradas

- **Recalcular el sprint al consultar**: viola FR-015 y es justo lo que la spec quiere evitar
  (Objetivo, segundo párrafo).
- **Congelar `grace_ends_at` por sprint al cerrarlo**: RC-11 lo descarta explícitamente y tocaría la
  004.
- **Plazo como duración de Go** (`48h`): más flexible, pero la spec habla en horas y un entero es
  más simple de validar y documentar.

---

## 9. El tope diario de 24 horas bajo concurrencia (FR-005, FR-024, RN-04, RC-09, SC-005)

### Decisión

Toda escritura que cambia las horas de una persona en una fecha (alta, modificación) toma, dentro de
su transacción, un **bloqueo `FOR NO KEY UPDATE` sobre la fila de `users` del autor** antes de sumar
sus horas de esa fecha. Con el bloqueo tomado, suma `hours_hundredths` de **todos** sus registros de
la fecha, en cualquier proyecto (incluidos los Finalizados y aquellos de los que fue quitado; RC-09),
excluye el registro que se está modificando (FR-024) y evalúa la regla pura:

```text
CheckDailyCap(alreadyLogged, requested int) error   // DailyCapExceededError{AvailableHundredths}
```

El rechazo informa las horas disponibles (`24 − ya cargadas`, nunca negativas) y **no nombra ningún
proyecto** (RC-09, SC-018). La eliminación no toma este bloqueo: solo libera capacidad.

### Rationale

- La spec exige que el tope sea una **invariante** aun ante pedidos simultáneos (Clarifications del
  2026-10-02; FR-005 "no alcanza con validar cada pedido por separado"). Un bloqueo que serializa las
  escrituras de esfuerzo **de esa persona** es la pieza mínima que lo garantiza: el segundo pedido
  espera, relee el total ya confirmado del primero y rechaza si excede.
- **`FOR NO KEY UPDATE` y no `FOR UPDATE`**: las inserciones de otras tablas que referencian a
  `users.id` (sprints, historias, sesiones) toman `FOR KEY SHARE` sobre la fila del usuario, que
  `FOR NO KEY UPDATE` no bloquea. Así el bloqueo solo serializa lo que tiene que serializar.
- La fila del usuario es una clave natural del tope ("es de la persona, no del proyecto") y cruza
  proyectos sin ningún artificio. Las escrituras simultáneas de una misma persona son raras, así que
  la contención es despreciable.
- Cubre también la combinación de un alta y una modificación concurrentes (FR-024), y la
  modificación que cambia la fecha: el bloqueo es por persona, no por fecha.

### Alternativas consideradas

- **`pg_advisory_xact_lock(hash(user_id), fecha)`**: más fino (por fecha), pero exige SQL crudo, un
  hash con colisiones posibles y dos claves ordenadas cuando una modificación cambia la fecha. Más
  piezas móviles para un beneficio inexistente a esta escala.
- **Tabla de totales diarios con `CHECK (total <= 2400)`**: la base garantiza la invariante, pero es
  un dato derivado que hay que mantener en cada alta, modificación y baja. Descartada por el
  Principio I.
- **Aislamiento `SERIALIZABLE`**: correcto, pero obliga a reintentar las transacciones abortadas por
  conflicto y no lo usa ninguna otra feature.

---

## 10. Transacciones y orden de bloqueos (FR-005, FR-008, FR-027, RC-07)

### Decisión

**Alta** (`EffortRepository.Create`), en una transacción:

1. Leer la historia unida a su proyecto con **`FOR SHARE OF projects`** (patrón
   `lockWritableStoryProject` de la 003): estado, `start_date`. Proyecto Finalizado →
   `ErrEffortReadOnly` (FR-008, US1-23), **antes** de evaluar el plazo de gracia.
2. Validar la fecha contra "hoy" y el inicio del proyecto (sección 7).
3. Leer el compromiso abierto y el último congelado de la historia (`sprint_stories` unido a
   `sprints`), sin bloqueo, y resolver el anclaje (sección 8).
4. Bloquear la fila del autor en `users` con `FOR NO KEY UPDATE`, sumar sus horas de la fecha y
   evaluar el tope (sección 9).
5. Insertar el registro con `sprint_id` resuelto. La clave foránea toma implícitamente
   `FOR KEY SHARE` sobre la historia y el sprint.
6. `UPDATE user_stories SET ever_had_effort = true WHERE id = ? AND ever_had_effort = false` con
   `UpdateColumn`, sin tocar `updated_at`.

**Modificación** y **eliminación**, en una transacción: proyecto `FOR SHARE` (Finalizado →
`ErrEffortReadOnly`, FR-027), autoría (`ErrNotEntryAuthor`, FR-022), registro `FOR UPDATE` releído con
el estado y el `closed_at` de su sprint (`EnsureEntryWritable`, FR-020); y, solo en la modificación,
la fecha nueva (sección 7) y el bloqueo del autor con el tope sobre la fecha resultante excluyendo el
propio registro (FR-024).

**Por qué no hay interbloqueos con la 004.** Los cierres, compromisos y quitas de la 004 bloquean
`sprints → user_stories` (`FOR UPDATE`); `Delete` de un sprint bloquea `projects → sprints`. El alta
de esfuerzo toma `projects (SHARE) → users → [sprints (KEY SHARE) y user_stories (KEY SHARE) al
insertar] → user_stories (NO KEY UPDATE)`: **nunca bloquea una historia antes que su sprint**, y toma
el proyecto antes que todo lo demás, igual que `Delete`. No se forma ningún ciclo.

**Semántica ante una operación de sprint simultánea.** El compromiso se lee con la instantánea de una
sola sentencia. Si un cierre, una quita o un compromiso de la misma historia ocurre en paralelo, el
registro queda anclado al estado que leyó, como si el alta hubiese ocurrido inmediatamente antes de
esa operación: el sprint resultante es siempre uno en el que la historia estaba comprometida durante
el pedido, del mismo proyecto (FR-017) y existente (la clave foránea lo garantiza). En particular, un
alta que corre con el cierre del sprint queda anclada a ese sprint, que es exactamente lo que el
plazo de gracia habría decidido.

**Eliminación de un sprint Planificado con esfuerzo (RC-07).** El servicio de la 004 consulta
`SprintEffort.HasEffort` antes de borrar. Si un registro se ancla entre esa consulta y el borrado, el
`DELETE` del sprint viola la clave foránea de `effort_entries.sprint_id`; `sprints/repository` traduce
`gorm.ErrForeignKeyViolated` a `ErrSprintHasEffort`, que es lo que su comentario ya anticipa, y
responde el mismo `409`.

### Rationale

- Tomar el proyecto con `FOR SHARE` es el mecanismo que la 003 ya usa para que un proyecto no pase a
  Finalizado entre la verificación y la escritura; el cambio de estado de la 002 actualiza esa fila y
  queda serializado.
- El orden de bloqueos es la única garantía contra interbloqueos con las escrituras de la 004, que
  tocan las mismas historias y sprints. Insertar antes de marcar la historia evita el orden
  `historia → sprint` que choca con `Close`.

### Alternativas consideradas

- **Bloquear la historia con `FOR SHARE` para serializar con los cambios de sprint**: forma un ciclo
  con `Close` (`sprint → historia` frente a `historia → sprint` al insertar) y PostgreSQL abortaría
  una de las dos con un `500`. Descartada.
- **Bloquear el sprint con `FOR KEY SHARE` en la lectura del compromiso**: tras esperar a un cierre,
  PostgreSQL relee la fila del sprint pero no la de `sprint_stories`, así que el resultado podría
  seguir desactualizado; agrega complejidad sin cambiar la semántica.

---

## 11. Consultas: esfuerzo de una historia y del proyecto (FR-030 a FR-045)

### Decisión

- **Esfuerzo de una historia** (`GET /stories/{id}/effort`): total, desglose por integrante y lista,
  en **tres consultas** dentro de una lectura: la lista (`effort_entries` unido a `users` para el
  nombre y a `sprints` para el nombre y el estado del sprint), y el total y el desglose agregados en
  Go a partir de la lista, que ya está en memoria. Orden de la lista: `entry_date DESC, created_at ASC,
  id ASC` (FR-035; `id` desempata en el mismo instante). Orden del desglose: `full_name ASC, user_id
  ASC`.
- **Esfuerzo del proyecto** (`GET /projects/{id}/effort-entries`): una consulta con filtros
  conjuntivos (FR-037) sobre `effort_entries` unido a `user_stories` (filtrado por `project_id`), a
  `users` y a `sprints`; el total se agrega en Go sobre el resultado. Mismo orden que el de la
  historia (FR-045). **Sin paginación**. Una lista vacía lleva `emptyReason`: `no_effort` si el
  proyecto no tiene ningún registro (US4-8) o `no_matches` si los filtros no encuentran ninguno
  (US4-7), igual que el `emptyReason` del listado de sprints de la 004.
- **Filtros** (query string, todos opcionales):

  | Parámetro | Valor | Regla |
  | --- | --- | --- |
  | `userId` | UUID | Debe ser integrante vigente **o** autor de algún registro del proyecto (FR-040, FR-044) |
  | `storyId` | UUID | Debe ser una historia del proyecto (FR-044) |
  | `sprintId` | UUID o `none` | Un sprint del proyecto; `none` = sin sprint. Opera sobre `effort_entries.sprint_id` (FR-039) |
  | `from`, `to` | `AAAA-MM-DD` | Inclusivos; uno, otro o ambos; `to < from` se rechaza (FR-038, FR-043) |

  Un filtro mal formado, de otro proyecto o inexistente se rechaza con `400` y
  `field: "<parámetro>"` y el **mismo mensaje** (`"El filtro no corresponde al proyecto."`), sin
  distinguir "no existe" de "es de otro proyecto" (FR-044, SC-018).
- Las personas quitadas del proyecto aparecen con su nombre porque el nombre sale de `users`, que
  nunca se borra (FR-032, RN-14). Ningún `SELECT` incluye `password_hash`.
- Cada registro de las respuestas lleva `definitive: true` si su sprint está Cerrado con el plazo
  vencido, calculado con `EnsureEntryWritable`, para que la interfaz no ofrezca editar lo que se va a
  rechazar.

### Rationale

- Agregar en Go sobre la lista ya leída evita una segunda consulta que podría ver otra instantánea y
  dar un total que no cuadra con la lista.
- Sin paginación: la spec no la pide, ninguna otra feature la usa en sus listados y el volumen de un
  proyecto académico es de cientos de registros. Si hiciera falta, se agregaría sin romper el
  contrato.
- La regla de `userId` resuelve FR-040 y FR-044 a la vez sin historial de membresías: la persona que
  dejó registros se reconoce por sus registros. Una persona quitada que nunca cargó horas se rechaza
  como ajena, lo que no revela nada y no oculta ningún dato (no tiene registros que mostrar).
- `definitive` es la única concesión de la lectura a la interfaz; se deriva, no se almacena.

### Alternativas consideradas

- **Totales con `SUM` en SQL**: correcto, pero es una consulta más y una instantánea distinta.
- **Paginación por cursor**: YAGNI.
- **`sprintId=null` en lugar de `none`**: el texto `null` en una query es ambiguo; `none` es explícito
  y no colisiona con un UUID.

---

## 12. Comparación entre estimado y real (FR-046 a FR-055, RN-16 a RN-20, RN-26)

### Decisión

Una función pura del dominio:

```text
Compare(storyPoints *int, factorHundredths int, actualHundredths int) Comparison
```

| Caso | `estimatedHours` | `actualHours` | `deviationHours` | `deviationPercent` |
| --- | --- | --- | --- | --- |
| Sin estimar (`storyPoints = nil`) | `null` | real | `null` | `null` |
| 0 Story Points | `0.00` | real | real | `null` |
| Estimada `> 0` | `SP × factor` | real | `real − estimada` | `round(dif × 10000 / estimada)` |

- El factor es el **vigente** del proyecto en cada consulta (FR-054, RN-20); se lee de
  `projects.hours_per_story_point_hundredths`, no del congelado del sprint.
- La respuesta incluye `storyPoints` (`null` o entero), de modo que la interfaz distingue "sin
  estimar" (`null`) de 0 (FR-055).
- **Comparación del proyecto**: una consulta trae todas las historias del proyecto con sus Story
  Points y la suma de sus horas (`LEFT JOIN` agregado sobre `effort_entries`, 0 si no tiene; FR-052),
  en el **mismo orden que el listado del backlog de la 003** (prioridad, `created_at`, `id`). Incluye
  historias de cualquier estado.
- **Comparación de una historia**: la misma consulta acotada a una historia.

### Rationale

- La tabla de casos es la de la spec (US5-1 a US5-9, FR-050, FR-051). Una función pura con tabla de
  casos es la prueba unitaria obligatoria de "cálculo de métricas" del Principio II.
- `null` como "no calculable" es la representación de la ausencia de valor que la 003 ya usa para
  "sin estimar"; nunca se confunde con `0.00` (Supuestos: "no calculable es un estado").

### Alternativas consideradas

- **Un campo de estado explícito** (`"deviationStatus": "not_calculable"`): redundante con `null`, y
  obliga a mantener dos campos coherentes.
- **Usar el factor congelado del sprint**: es el de las métricas históricas de la 008; la spec dice
  que esta comparación usa siempre el vigente (Supuestos).

---

## 13. Integración con la 003 y la 004: puertos reemplazados

### Decisión

- `backlog/repository/effort_records.go`: `NewEffortRecords(db)` pasa a recibir la conexión y
  `HasEffort` lee `user_stories.ever_had_effort` (RC-05). `backlog/service` no cambia.
- `backlog/domain/user_story.go`: se agrega el campo `EverHadEffort` (`ever_had_effort boolean NOT
  NULL DEFAULT false`); la 003 nunca lo escribe, igual que `EverCommitted`.
- `sprints/repository/sprint_effort.go`: `NewSprintEffort(db)` pasa a recibir la conexión y
  `HasEffort` responde `EXISTS (SELECT 1 FROM effort_entries WHERE sprint_id = ?)`.
- `sprints/repository/sprint_repository.go` (`Delete`): traduce `gorm.ErrForeignKeyViolated` del
  `DELETE` del sprint a `domain.ErrSprintHasEffort` (sección 10).
- `cmd/app/main.go`: compone el módulo y pasa `db` a los dos puertos.

### Rationale

Es exactamente lo que los dos puertos dejaron escrito para esta feature. Ni `backlog/service` ni
`sprints/service` cambian, y sus pruebas con dobles siguen valiendo.

### Alternativas consideradas

- **Que `HasEffort` de la 003 lea `EXISTS` sobre `effort_entries`**: no cumple RC-05 cuando se borran
  todos los registros.

---

## 14. Frontend (SC-001 a SC-003)

### Decisión

- **Detalle de la historia** (`app/projects/[projectId]/backlog/[storyId]/page.tsx`, de la 003): se
  agrega la sección "Esfuerzo" (`StoryEffortSection`) con el formulario de registro de tres campos
  (fecha con hoy por defecto, actividad, horas con paso 0,25) y botón; el total; el desglose por
  integrante; la lista de registros con edición en línea y eliminación con confirmación **solo** en
  los propios y no definitivos; y la comparación de la historia. Registrar es: escribir la actividad,
  escribir las horas, confirmar (SC-001: 3 pasos, la fecha ya viene cargada).
- **Página nueva** `app/projects/[projectId]/effort/page.tsx`: filtros (integrante, historia,
  sprint con opción "Sin sprint", rango de fechas), lista con total, y la tabla de comparación de
  todas las historias. El selector de integrantes se arma con los integrantes vigentes (endpoint de
  la 002) más los autores que aparecen en el listado sin filtros (SC-003).
- **Resumen del proyecto** (`project-overview`, de la 002): un enlace a "Esfuerzo".
- Validación en el cliente con las mismas reglas de forma (`features/effort/validation.ts`), como
  ayuda; el backend sigue siendo la autoridad. Formato en `utils/effort-format.ts`: horas con coma
  decimal (`6,5 h`), porcentajes con signo (`+20,00 %`), y "No calculable" para `null`.
- Antes de escribir código de Next.js se lee la documentación de la versión instalada en
  `frontend/node_modules/next/dist/docs/` (CLAUDE.md).

### Rationale

La sección en el detalle de la historia es donde el integrante ya está cuando trabaja sobre una
historia (SC-002: total y desglose en una sola pantalla). La página del proyecto concentra la lectura
a escala de proyecto (US4, US5).

### Alternativas consideradas

- **Una pantalla de "mis horas"**: no la pide la spec; la vista del proyecto filtrada por integrante
  la cubre.
- **Un modal para editar**: más piezas que la edición en línea de tres campos.

---

## 15. Códigos HTTP

| Situación | Código | Cuerpo |
| --- | --- | --- |
| Alta | `201` | El registro creado |
| Modificación | `200` | El registro actualizado |
| Eliminación | `204` | — |
| Consultas | `200` | Ver contrato |
| Sin sesión | `401` | `ErrorResponse` de la 001 |
| Campo inválido, fecha futura o anterior al inicio, campo inmutable, filtro inválido | `400` | `ValidationErrorResponse` |
| Registro ajeno | `403` | `ErrorResponse` |
| Historia, proyecto o registro inexistente o ajeno | `404` | `ErrorResponse` idéntico al de la 003, la 002 o el propio |
| Proyecto Finalizado | `409` | `ErrorResponse` (`code: read_only`) |
| Sprint o esfuerzo definitivo | `409` | `ErrorResponse` (`code: effort_finalized`) |
| Tope diario superado | `409` | `DailyCapExceededResponse` con `availableHours` |

Los tres `409` llevan un campo `code` para que la interfaz los distinga sin interpretar el texto,
igual que el `code` de los motivos de bloqueo de la eliminación de una historia en la 003
(`DeletionBlockedReason`).

---

## Puntos abiertos que no bloquean el diseño

1. **Criterio de empate del redondeo** (sección 5): se adopta mitad lejos de cero. La 008 debe usar
   la misma función (RC-13). Se señala al desarrollador.
2. **Carrera entre `HasEffort` y el borrado de una historia en la 003**: el `DELETE` de la historia
   violaría la clave foránea y respondería `500` en lugar de `409`. Es la misma ventana que la 005
   dejó señalada para `/revisar-seguridad`; no se toca la 003 más allá del puerto.
3. **`EFFORT_GRACE_PERIOD_HOURS` en los `.env` locales**: se agrega a `backend/.env.example`, a
   `backend/.env` y a `backend/.env.test`, porque el Principio VI exige las mismas claves en cada
   `.env` y su plantilla. El valor por defecto solo evita que un `.env` desactualizado impida el
   arranque; no reemplaza la sincronización.

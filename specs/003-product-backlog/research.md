# Investigación (Fase 0): Product Backlog

**Feature**: `specs/003-product-backlog` | **Fecha**: 2026-10-20

Este documento resuelve las incógnitas técnicas del plan. La spec no tiene preguntas abiertas: las
cinco decisiones de producto se cerraron en las sesiones de clarificación del 2026-09-25 y del
2026-09-26 (sección Clarifications de [spec.md](./spec.md)). Cada sección registra la decisión, su
fundamento y las alternativas descartadas.

---

## 0. Contexto: esta feature se construye sobre la 001 y la 002

Las features `specs/001-user-auth` y `specs/002-project-members` están implementadas e integradas en
`main` (último merge: PR #287, fase 10 de la 002). Esta feature reutiliza sin cambios:

- El módulo de Go `github.com/juanku21/seguimiento-y-medicion/backend`, Gin, GORM y los contenedores
  de PostgreSQL de desarrollo (`smye_dev`, puerto 5432) y de test (`smye_test`, puerto 5433).
- El middleware de sesión de la 001, inyectado como `gin.HandlerFunc` desde `cmd/app/main.go`, y el
  accesor `authdelivery.CurrentUserID` para leer al titular de la sesión.
- El middleware `RequireMembership` de la 002, a través del subgrupo `/api/v1/projects/:projectId`
  que `projectsdelivery.RegisterRoutes` ya devuelve.
- Los esquemas de error `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de `auth/delivery`.
- El middleware global `httpserver.LimitRequestBody` (1 MiB) de la fase 10 de la 002, que ya protege
  los endpoints nuevos sin hacer nada más.
- El ayudante de pruebas `testsupport.OpenDatabase`, con su lock de la base de test.

Esta feature **no tiene fase de arranque**: no agrega dependencias, servicios ni variables de
entorno.

---

## 1. Versiones del stack: sin cambios

**Decisión**: se mantienen las versiones fijadas por la 001 y la 002 (Go 1.27.2, Gin v1.12.0, GORM
v1.31.2 con `gorm.io/driver/postgres` v1.6.3, `google/uuid` v1.6.0, `testify` v1.12.1, Next.js
16.3.8, React 19.3.0, Tailwind CSS 4.3.3, Vitest 5.0.3). **Esta feature no agrega ninguna dependencia
de Go ni de npm.**

**Rationale**: todo lo que pide la spec se resuelve con lo que ya está: validación con la biblioteca
estándar (`unicode/utf8`, `strings`, `encoding/json`), consultas y transacciones con GORM, bloqueo de
fila con `gorm.io/gorm/clause` (parte de GORM) y formularios con React sin librerías de formularios.

**Alternativas consideradas**: una librería de validación declarativa (`go-playground/validator`
directo, o `zod` en el frontend) — descartada, no está autorizada (Principio IV) y las dos features
anteriores ya validan con funciones propias que devuelven mensajes por campo.

---

## 2. Un módulo `backlog` y la forma de las rutas (FR-031, FR-039, FR-047)

### Decisión

Se agrega **un** módulo vertical, `backend/internal/backlog/`, con sus cuatro capas. Expone seis
endpoints, uno por historia de usuario de la spec, con dos formas de ruta:

| Endpoint | Historia | Recurso de la ruta |
| --- | --- | --- |
| `POST /api/v1/projects/{projectId}/stories` | US1 | el proyecto |
| `GET /api/v1/projects/{projectId}/stories` | US2 | el proyecto |
| `GET /api/v1/stories/{storyId}` | US3 | la historia |
| `PUT /api/v1/stories/{storyId}/story-points` | US4 | la historia |
| `PATCH /api/v1/stories/{storyId}` | US5 | la historia |
| `DELETE /api/v1/stories/{storyId}` | US6 | la historia |

Las dos rutas de **colección** cuelgan del subgrupo `/api/v1/projects/:projectId` de la 002, de modo
que heredan `RequireMembership` y su `404` de proyecto inexistente. Las cuatro rutas de **ítem** son
planas y usan un middleware propio, `RequireStoryAccess` (sección 3), con un único `404` de historia
inexistente.

### Rationale

- **La spec distingue dos respuestas de "no acceso"**. En la creación y el listado la tabla de
  Entradas y Salidas dice "respuesta de **proyecto** inexistente"; en el detalle, la modificación, la
  estimación y la eliminación dice "respuesta de **historia** inexistente, idéntica a la de un
  identificador que no existe" (US3-5, US3-6, FR-047, SC-006). Con rutas de ítem anidadas bajo el
  proyecto, quien no es integrante recibiría el `404` de proyecto, cuyo cuerpo no es el de historia
  inexistente: el escenario US3-6 pide que ambas respuestas sean idénticas.
- El identificador de la historia es un UUID global (FR-009), así que no necesita el proyecto en la
  ruta para ser único. La ruta plana evita además el caso "historia de B pedida con el proyecto A en
  la ruta", que con rutas anidadas habría que detectar y responder igual que una inexistente.
- Reutilizar el subgrupo de la 002 para la colección no exige que `backlog/delivery` importe
  `projects/delivery`: `cmd/app/main.go` le pasa el `*gin.RouterGroup` que
  `projectsdelivery.RegisterRoutes` ya devuelve. Es composición, que es el trabajo de `cmd/app/`
  (Principio III).

### Alternativas consideradas

- **Todo anidado bajo `/api/v1/projects/{projectId}/stories/{storyId}`**: descartada por lo anterior.
- **Todo plano, incluida la colección (`/api/v1/stories?projectId=`)**: descartada. La creación y el
  listado sí operan sobre el proyecto, la spec pide para ellos la respuesta de proyecto inexistente,
  y reimplementar la verificación de membresía duplicaría `RequireMembership` (DRY).
- **Dos módulos (`stories` y `acceptance_criteria`)**: descartada. Un criterio no existe fuera de su
  historia y ninguna funcionalidad pedida opera sobre un criterio suelto.

---

## 3. Acceso a una historia: una sola consulta y un solo `404` (FR-046, FR-047, SC-006)

**Decisión**: `RequireStoryAccess` resuelve con **una consulta** la existencia de la historia, la
membresía vigente del titular de la sesión en su proyecto y el estado de ese proyecto:

```text
user_stories  JOIN project_members ON project_id = user_stories.project_id AND user_id = <titular>
              JOIN projects        ON projects.id  = user_stories.project_id
WHERE user_stories.id = <storyId>
```

Si no hay fila, o si el identificador no es un UUID, responde siempre el mismo `404` con el mismo
cuerpo (`La historia no existe.`). Si hay fila, deja en el contexto un `StoryAccess` con el
identificador de la historia, el de su proyecto y si el proyecto está Finalizado.

**Rationale**:

- Una única salida `404` hace que la historia inexistente, la ajena y la eliminada (FR-044, US6-10)
  sean indistinguibles por construcción, sin depender de que cada manejador elija el mismo mensaje.
  Es la misma técnica que `RequireMembership` usa en la 002 (research de la 002, sección 5).
- Traer el estado del proyecto en la misma consulta evita una segunda lectura para la regla de solo
  lectura (FR-048) en las tres acciones de escritura sobre una historia.
- Un integrante quitado del proyecto pierde el acceso de inmediato (caso límite de la spec), porque
  la consulta exige una fila vigente en `project_members`.

**Alternativas consideradas**: leer la historia y después verificar la membresía con
`RequireMembership` — descartada, son dos consultas y dos posibles salidas `404` con cuerpos
distintos.

---

## 4. Escala de Story Points: almacenamiento y validación (FR-014, FR-015, FR-019, SC-007)

### Decisión

- Se persisten en `user_stories.story_points` como `smallint` **anulable**: `NULL` es "sin estimar" y
  `0` es una estimación válida (FR-015, RN-07). En Go el campo es `*int`.
- La escala (`0, 1, 2, 3, 5, 8, 13, 20, 40, 100`) se declara **una sola vez** en `backlog/domain`
  como el conjunto cerrado de valores admitidos, con una función `IsValidStoryPoints(int) bool`.
- En la API el valor llega como `json.Number` y se valida sobre su texto literal: debe ser un entero
  sin signo, sin parte decimal ni exponente, y pertenecer a la escala. `6.5`, `4`, `7`, `50`, `101` y
  `-1` se rechazan con `400` y el mensaje que enumera la escala completa.
- En la API "sin estimar" es `null`. En la estimación el campo es obligatorio: `null` significa
  "devolver a sin estimar" y el campo ausente es un error de validación (sección 8).

### Rationale

- Un `*int` con `NULL` en la base es la representación más simple que mantiene distinguibles el 0 y
  la ausencia de estimación en todas las capas. Un centinela (por ejemplo `-1`) obligaría a
  traducirlo en cada lectura y es exactamente el tipo de error que SC-009 y US3-4 miden.
- Deserializar a `int` haría fallar `6.5` con un error de tipo de Go cuyo mensaje no indica la
  escala, y deserializar a `float64` aceptaría `5.0` como si fuera `5`. Con `json.Number` se rechaza
  todo valor no entero con el mismo mensaje que un entero fuera de la escala (FR-019). Es la misma
  técnica que la 002 usa para el factor (research de la 002, sección 3).
- La escala no es configurable: la spec la define cerrada y no pide administrarla (Principio I).

**Alternativas consideradas**:

- **Tabla o enumerado de PostgreSQL para la escala**: descartada, agrega una pieza que migrar para
  diez constantes que la spec fija.
- **`CHECK` en la base con la escala**: descartada como mecanismo principal. La validación ocurre en
  `delivery` antes del servicio (Principio VII), y un `CHECK` exige SQL en crudo fuera de
  `AutoMigrate`.

---

## 5. Criterios de aceptación: tabla propia y reemplazo completo (FR-006, FR-007, FR-022)

**Decisión**: los criterios se persisten en una tabla `acceptance_criteria`, una fila por criterio,
con `story_id`, `position` (entero desde 0) y `text` (`varchar(500)`), e índice único
`(story_id, position)`. La API los expone como **una lista ordenada de textos** (`["...", "..."]`),
sin identificadores propios. Toda modificación de la lista la **reemplaza completa** dentro de una
transacción: se borran las filas de la historia y se insertan las nuevas con sus posiciones.

**Rationale**:

- La constitución exige que todo modelo persistido tenga clave primaria UUID y marcas de creación y
  actualización. Una columna JSON o un `text[]` dentro de `user_stories` esquivaría esa regla para
  un dato que tiene su propio límite de longitud por elemento y su propio orden.
- FR-022 define una sola vía para agregar, quitar y reordenar: enviar la lista entera. Exponer
  identificadores de criterio invitaría a operaciones por criterio que la spec no pide y que
  chocarían con FR-025 (la lista se trata como un único campo).
- `position` explícita conserva el orden de carga (US3-1) y permite reordenar (US5-3) sin depender
  del orden físico de las filas.
- El listado necesita la **cantidad** de criterios (FR-033): se obtiene con una subconsulta de
  conteo, sin cargar los textos.

**Alternativas consideradas**:

- **`jsonb` en `user_stories`**: descartada por la regla de modelos persistidos y porque la
  validación de 1 a 500 caracteres por criterio quedaría fuera del alcance del tipo de columna.
- **Actualizar los criterios fila por fila (diff)**: descartada; más código para el mismo resultado
  y riesgo de fusionar listas bajo concurrencia, que FR-025 prohíbe.

---

## 6. Modificación parcial: `PATCH` con detección de presencia (FR-020, FR-021, FR-023, FR-024)

### Decisión

`PATCH /api/v1/stories/{storyId}` recibe un objeto cuyos campos editables son todos opcionales:
`title`, `description`, `priority` y `acceptanceCriteria`. La regla de FR-020 se aplica al pie de la
letra:

| Campo en el cuerpo | Efecto |
| --- | --- |
| Ausente | El valor vigente no cambia |
| `title` presente | Se valida igual que en la creación; vacío o solo espacios se rechaza |
| `description` presente como `null`, `""` o solo espacios | Se borra (la descripción admite estar vacía) |
| `priority` presente | Se valida igual que en la creación; `null` se rechaza |
| `acceptanceCriteria` presente como `[]` | Se borran todos los criterios (salvo la regla de RC-09, sección 11) |
| `acceptanceCriteria` presente como `null` | Se rechaza: para vaciar la lista se envía `[]` |

Para distinguir "ausente" de "presente con `null`", el cuerpo se deserializa en un tipo cuyos campos
son `json.RawMessage`: un campo ausente queda `nil` y un `null` llega como el literal `null`.

Además:

- Un cuerpo sin ningún campo editable se rechaza con `400` (`La solicitud no incluye ningún campo a
  modificar.`), para que una petición vacía no actualice `updated_at` sin haber cambiado nada.
- Los campos `status` y `storyPoints` se **rechazan** con `400` y un mensaje por campo: el estado no
  se cambia desde el backlog (FR-027, US5-13) y la estimación tiene su propio endpoint (sección 8).
- Los campos de solo lectura (identificador, proyecto, autor, momento de creación) no son parte del
  esquema de entrada y no se pueden alterar por construcción (FR-024).
- La validación de todos los campos presentes ocurre antes de escribir; si uno falla no se aplica
  ninguno (FR-023).

### Rationale

- La 002 eligió `PUT` con la representación completa porque su spec pedía exactamente las mismas
  validaciones que en la creación y no definía qué significa un campo ausente. Esta spec sí lo
  define (FR-020: "un campo ausente deja su valor vigente sin cambios, y un campo enviado vacío
  borra el valor cuando ese campo admite estar vacío"), y además exige que dos ediciones simultáneas
  de campos distintos no se pisen (FR-025). Con `PUT` y la representación completa, la segunda
  petición reescribiría el campo que cambió la primera con el valor viejo que tenía el formulario:
  es el conflicto que US5-6 prohíbe. `PATCH` con solo los campos tocados es lo que lo hace posible.
- Rechazar `status` de forma explícita, en lugar de ignorarlo, hace verificable US5-13 con una
  respuesta concreta y evita que un cliente crea que el cambio se aplicó.

**Alternativas consideradas**:

- **`PUT` con todos los campos**: descartada por FR-020 y FR-025.
- **JSON Merge Patch (RFC 7396) completo con `application/merge-patch+json`**: descartada; el
  comportamiento es casi el mismo pero exige otro tipo de contenido y reglas generales para `null`
  que acá no aplican igual a todos los campos (la prioridad no admite `null`).
- **Un endpoint por campo**: descartada, multiplica las rutas sin requisito que lo pida y vuelve no
  atómica una modificación de varios campos (FR-023).

---

## 7. Edición simultánea campo por campo (FR-025, SC-013, US5-6, US5-7)

**Decisión**: la modificación se aplica en una transacción que:

1. Toma un bloqueo de fila sobre la historia (`SELECT ... FOR UPDATE`, con `clause.Locking` de GORM).
2. Actualiza **solo las columnas presentes** en el `PATCH`, más `updated_at`.
3. Si `acceptanceCriteria` está presente, reemplaza la lista completa (sección 5).

**Rationale**:

- Un `UPDATE` que escribe solo las columnas tocadas es lo que deja aplicados los dos cambios cuando
  dos integrantes modifican campos distintos a la vez (US5-6, SC-013): ninguna de las dos sentencias
  escribe la columna de la otra.
- Cuando tocan el mismo campo escalar, gana el último `UPDATE` en confirmarse (US5-7, FR-025), sin
  aviso, que es lo que la spec pide.
- El bloqueo de fila es necesario para la **lista** de criterios. Sin él, dos reemplazos simultáneos
  podrían intercalarse (borrar A, borrar B, insertar A, insertar B) y dejar las dos listas
  fusionadas, que FR-025 prohíbe expresamente. Con el bloqueo, el segundo reemplazo espera al
  primero y lo sustituye entero.
- No hace falta un control optimista con versión: la spec descarta avisar del conflicto (supuesto
  "Sin aviso de conflicto de edición"), así que una versión solo serviría para rechazar, que es lo
  que no se quiere.

**Alternativas consideradas**:

- **Leer la historia, aplicar los cambios en memoria y guardar el registro entero (`Save`)**:
  descartada; es la forma clásica de perder el cambio del otro campo.
- **Bloqueo optimista con `version`**: descartada por lo anterior.

La prueba de SC-013 envía las dos modificaciones a la vez desde dos goroutines contra la base de
test, igual que las pruebas de concurrencia de la 002 (`project_concurrency_test.go`).

---

## 8. Estimación: endpoint propio (FR-016 a FR-019, RC-06)

**Decisión**: `PUT /api/v1/stories/{storyId}/story-points` recibe `{"storyPoints": <valor de la
escala>}` o `{"storyPoints": null}`. El campo es obligatorio; ausente se rechaza con `400`. Responde
`200` con el detalle actualizado. La lógica vive en un método del servicio, `Estimate`, que valida la
escala, el estado Completada, la solo lectura y la regla de RC-09 antes de guardar.

**Rationale**:

- US4 es una historia propia con su prueba independiente, y la estimación es la materia prima de
  todas las métricas (sección "Por qué esta prioridad" de US4). Un endpoint propio la deja
  verificable y documentada por separado, igual que la 002 separó el cambio de estado.
- RC-06 dice que Planning Poker (`specs/005-planning-poker`) actualiza **el mismo valor** con la
  misma escala y las mismas restricciones de estado. Tener la regla en un único método del servicio
  es el punto de integración natural para esa feature: reutilizará `Estimate` en lugar de reescribir
  las reglas (DRY).
- `PUT` es idempotente: estimar dos veces en 8 deja la historia igual.
- FR-021 menciona los Story Points entre las validaciones de la modificación. Se cumple porque el
  endpoint de estimación usa **la misma función de validación** que la creación; lo que no se hace
  es admitir el campo en el `PATCH`, para que exista una sola vía de cambio de la estimación.

**Alternativas consideradas**: aceptar `storyPoints` en el `PATCH` — descartada, duplica la vía de
cambio y mezcla en una sola petición reglas distintas (la de RC-09 sobre la estimación y la de FR-022
sobre los criterios).

---

## 9. Listado: orden, filtros y los dos vacíos (FR-031 a FR-038, SC-003, SC-004)

### Decisión

- **Una sola consulta** sobre `user_stories` del proyecto con una subconsulta de conteo de
  `acceptance_criteria` por historia. Sin N+1.
- **Orden**: `CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, created_at ASC, id
  ASC`. El desempate por `id` hace el orden reproducible entre consultas (FR-032, US2-6).
- **Filtros**: parámetros de consulta repetibles, `?status=pending&status=in_progress` y
  `?priority=high`. Cada filtro admite varios valores (`IN`); los dos se combinan con `AND`
  (FR-034). Un valor fuera de los admitidos se rechaza con `400`, con el campo y los valores válidos
  (FR-037).
- **Los dos vacíos**: la respuesta incluye `appliedFilters` y un campo `emptyReason` con tres
  valores posibles: `null` (hay resultados), `backlog_empty` (el proyecto no tiene historias) o
  `no_matches` (hay historias pero ninguna cumple los filtros). Solo cuando el resultado filtrado
  viene vacío se hace una segunda consulta de existencia (`SELECT 1 ... LIMIT 1`) para elegir entre
  los dos.
- Sin paginación ni búsqueda por texto: están en "Fuera de Alcance".

### Rationale

- El listado es la pantalla de trabajo del backlog y SC-003 pide encontrar las historias Alta sin
  estimar en menos de 15 segundos; resolverlo en una consulta indexada por `project_id` lo garantiza
  en el orden de magnitud de un backlog real.
- Guardar la prioridad como texto (`high`, `medium`, `low`) y ordenar con `CASE` mantiene la columna
  legible y coherente con `projects.status`. La alternativa de un rango numérico obliga a traducir en
  cada lectura y escritura.
- `emptyReason` es lo que hace **distinguibles** el backlog vacío y los filtros sin resultados, como
  exige FR-036, sin que el cliente tenga que adivinarlo por la presencia de filtros: un backlog vacío
  consultado con filtros sigue siendo `backlog_empty`.
- Los parámetros repetidos son la forma nativa de Gin (`QueryArray`) y de `URLSearchParams` en el
  frontend; no hace falta parsear listas separadas por comas.

**Alternativas consideradas**:

- **Columna `priority_rank` además de `priority`**: descartada, duplica un dato derivable.
- **Contar con `Preload` de los criterios**: descartada, trae todos los textos a memoria para
  devolver un número.
- **Devolver siempre el total del backlog**: descartada; es una consulta de conteo en cada listado
  para resolver un caso que solo aparece cuando el resultado viene vacío.

---

## 10. "Lista para planificar" se deriva, no se persiste (FR-028, FR-029, SC-009)

**Decisión**: la condición se calcula al leer: `story_points IS NOT NULL` **y** al menos un criterio
de aceptación. En el listado sale de la misma consulta (columna derivada con el conteo). En el
detalle se informa además qué condición falta, como lista de motivos: `story_points`,
`acceptance_criteria` o ambos (FR-029, US3-2). Una función de dominio,
`Readiness(storyPoints *int, criteriaCount int)`, es la única fuente de la regla.

**Rationale**: SC-009 exige que el 100 % de las historias marcadas cumplan las dos condiciones y que
ninguna que las cumpla quede sin marcar. Con una columna persistida habría que mantenerla en sincronía
en cada creación, estimación y modificación; derivarla hace imposible la desincronización. La
función de dominio es además la que `specs/004-sprint-management` necesita para hacer cumplir RN-13
al armar un sprint (RC-03).

**Alternativas consideradas**: columna booleana `ready_to_plan` — descartada por lo anterior.

---

## 11. Puertos hacia sprints, esfuerzo, defectos y Planning Poker (FR-018, FR-022, FR-039, FR-041, FR-043, RC-05, RC-09, RC-10)

### El problema

Tres decisiones de esta feature dependen de datos que producen features que todavía no existen:

- Si una historia **está comprometida en un sprint abierto** (RC-09: no se le pueden quitar los Story
  Points ni el último criterio) y si **estuvo alguna vez en un sprint** (FR-039: se muestra en el
  detalle; FR-041: impide eliminarla). Los produce `specs/004-sprint-management`.
- Si tiene **esfuerzo registrado** (`specs/006-effort-tracking`), **defectos asociados**
  (`specs/007-defect-tracking`) o **sesiones de Planning Poker** (`specs/005-planning-poker`).
  Las tres impiden eliminarla (FR-041, FR-043).

RC-05 y RC-10 dicen que esta feature **consulta** esa existencia y no la genera.

### Decisión

`internal/backlog/domain/ports.go` declara **cuatro puertos**, uno por feature productora, con
exactamente las operaciones que la spec necesita:

| Puerto | Operación | Feature que lo implementará | Requisitos |
| --- | --- | --- | --- |
| `SprintAssignments` | `Find(ctx, storyID) (SprintAssignment, error)` | 004 | FR-018, FR-022, FR-039, FR-041, RN-16, RN-18 |
| `EffortRecords` | `HasEffort(ctx, storyID) (bool, error)` | 006 | FR-041, FR-043 |
| `DefectRecords` | `HasDefects(ctx, storyID) (bool, error)` | 007 | FR-041, FR-043 |
| `EstimationSessions` | `HasSessions(ctx, storyID) (bool, error)` | 005 | FR-041, FR-043, RC-10 |

`SprintAssignment` tiene dos campos: `CommittedToOpenSprint` y `EverAssigned`.

Mientras esas features no existan, las implementaciones viven en `internal/backlog/repository/` e
informan lo que hoy es cierto: **ninguna historia está ni estuvo en un sprint, ni tiene esfuerzo,
defectos o sesiones de estimación**. Cuando cada feature llegue, reemplaza el cuerpo de su archivo
sin tocar `backlog/service`.

### Rationale

- Es la misma decisión que la 002 tomó con sus puertos hacia el backlog y los sprints (research de
  la 002, sección 9), y por las mismas razones: los requisitos que los puertos sirven están en esta
  spec, hay escenarios que los prueban (US4-6, US5-8, US3-3, US6-4, US6-6, US6-7) y sin los puertos
  esas reglas no se pueden implementar ni probar. Con dobles de `testify` los escenarios se escriben
  hoy con valores distintos de cero.
- **Un puerto por feature productora** y no un único puerto de "historial": cada uno lo implementará
  una feature distinta, en un momento distinto, y FR-043 exige decir **cuál** de las cuatro
  condiciones bloquea. Un puerto único obligaría a la primera de esas features a implementar
  también las consultas de las otras tres.
- Lo que informan las implementaciones actuales no es un atajo ni un dato inventado: en el estado
  actual del producto no existen sprints, registros de esfuerzo, defectos ni sesiones de estimación.

### Alternativas consideradas

- **Devolver `false` directamente en el servicio**: descartada; deja FR-018, FR-022 y FR-041 sin
  implementar y sus escenarios sin forma de ejercitarse.
- **Implementar en esta feature una parte de sprints o de esfuerzo**: prohibido por "Fuera de
  Alcance" y por el Principio I.
- **Diferir la eliminación hasta que existan las cuatro features**: descartada, US6 es una historia
  de esta feature con su propia prueba independiente.

---

## 12. Eliminación definitiva y las cuatro condiciones (FR-041 a FR-044, SC-011)

**Decisión**: el servicio consulta los cuatro puertos (sección 11). Si alguno informa historial,
responde `409` con un cuerpo que, además del mensaje, lista **todos** los motivos que aplican, con
los códigos `sprint_history`, `effort`, `defects` y `estimation_sessions`. Si ninguno aplica, borra
en una transacción los criterios de la historia y la historia, y responde `204`. Una historia ya
eliminada responde el `404` de `RequireStoryAccess` (US6-10).

**Rationale**:

- FR-043 pide indicar "cuál de las cuatro condiciones se lo impide". Una historia puede cumplir
  varias a la vez (estuvo en un sprint **y** tiene esfuerzo); listarlas todas evita que la persona
  resuelva una y se tope con la siguiente.
- El borrado es definitivo (supuesto "Eliminación definitiva" y RC-08): no hay columna `deleted_at`.
  Con borrado lógico, toda consulta de la feature tendría que filtrar las eliminadas y FR-044 se
  rompería con el primer olvido.
- Los criterios se borran explícitamente en la misma transacción, sin `ON DELETE CASCADE`, para que
  el efecto de la eliminación sea visible en el código del repositorio.

**Señalado para las features 004 a 007**: cuando esas features creen tablas que referencien
`user_stories` (asignación a sprint, registros de esfuerzo, defectos, sesiones), sus claves foráneas
con la restricción por defecto (`RESTRICT`) serán un **respaldo** de la regla: aunque una asignación
llegara entre la verificación y el borrado, la base rechazaría el borrado. El servicio debe traducir
esa violación a la misma respuesta `409`. Hoy no hay ninguna tabla que referencie a `user_stories`
salvo `acceptance_criteria`.

**Alternativas consideradas**: borrado lógico — descartada por lo anterior y porque la spec no pide
restaurar.

---

## 13. Integración con la 002: resumen del backlog y etiquetas de estado (RC-07)

**Decisión**: esta feature **reemplaza el cuerpo** de
`internal/projects/repository/backlog_summary.go`, tal como dejó anotado el plan de la 002 (puntos
señalados, punto 4). La implementación nueva hace una consulta agrupada sobre `user_stories`:

```text
SELECT status, COUNT(*), COALESCE(SUM(story_points), 0)
FROM user_stories WHERE project_id = ? GROUP BY status
```

`CountsByState` queda con las claves `pending`, `in_progress` y `completed` (solo las que tienen
historias) y `EstimatedStoryPoints` con la suma de las estimadas: las "sin estimar" se cuentan pero
no suman, y las estimadas en 0 suman 0 (FR-045 de la 002). `NewBacklogSummaryProvider` pasa a
recibir la conexión de GORM, así que cambian `cmd/app/main.go` y los ayudantes de prueba de
`projects/delivery` que lo construyen. `projects/service` no cambia.

En el frontend, `ProjectStatusCard` muestra hoy la clave cruda de cada estado ("pending"), porque la
002 no conocía el conjunto de estados. Esta feature agrega las etiquetas en español
(`utils/story-labels.ts`) y la tarjeta pasa a usarlas.

**Rationale**:

- El plan de la 002 fijó este punto de integración para que la vista de estado empiece a informar
  datos reales sin tocar su servicio. Cumplirlo acá es lo que hace verdadera RC-07.
- La implementación usa el modelo `backlogdomain.UserStory` en lugar del nombre de la tabla en
  texto, para que un cambio en el modelo rompa la compilación y no una consulta en silencio. No hay
  ciclo de importación: `projects/repository` importa `backlog/domain`, y `backlog/domain` importa
  solo `projects/domain` y `auth/domain`.

**Alternativas consideradas**: mover la implementación a `backlog/repository` y borrar el archivo de
la 002 — descartada. Contradice la decisión registrada en la 002 y obliga a que las pruebas de
`projects` importen el repositorio del backlog, sin ninguna ventaja funcional.

---

## 14. Solo lectura del proyecto Finalizado y existencia del proyecto (FR-048, RN-15, SC-012)

**Decisión**:

- En las rutas de ítem, el estado del proyecto llega con `StoryAccess` (sección 3).
- En la creación, el servicio lo lee a través de un puerto propio, `ProjectDirectory.IsFinished(ctx,
  projectID)`, implementado en `backlog/repository` sobre el modelo `projectsdomain.Project`
  seleccionando solo la columna `status`.
- Toda escritura sobre un backlog de proyecto Finalizado responde `409` (`El proyecto está
  finalizado y su backlog es de solo lectura.`). El listado y el detalle no lo consultan.

**Rationale**: la membresía ya la verificó `RequireMembership`; solo falta el estado, y un puerto
de una operación es menos acoplamiento que importar el `ProjectRepository` entero de la 002. Se usa
`409` y no `403` por la misma razón que en la 002 (research de la 002, sección 12): no es una
cuestión de permisos, la acción está prohibida para cualquiera.

**Alternativas consideradas**: reutilizar `projectsdomain.ProjectRepository.FindByID` — descartada,
trae el proyecto completo para leer un campo y acopla el backlog a una interfaz que la 002 declaró
para sus propios casos de uso.

---

## 15. Autoría y atribución (FR-011, FR-049, FR-050)

**Decisión**:

- `user_stories.created_by_id` guarda al autor, con clave foránea a `users.id`. El detalle devuelve
  `createdBy` con el identificador y el nombre completo, leídos con un `JOIN` que selecciona solo
  `users.id` y `users.full_name` (nunca `password_hash`, igual que la 002).
- La identidad del autor y de quien ejecuta cada acción **sale siempre de la sesión**
  (`authdelivery.CurrentUserID`), nunca del cuerpo de la petición. Ningún esquema de entrada tiene un
  campo de autor.
- No se persisten columnas de "último modificador" ni de "eliminado por": el historial de auditoría
  consultable está en "Fuera de Alcance".
- Como la baja de un integrante no borra su cuenta (002, FR-027), las historias que creó siguen
  mostrando su nombre (FR-050) sin hacer nada especial.

**Rationale**: es la misma lectura de "atribuida al titular de la sesión" que hizo la 002 con su
FR-051. Persistir quién modificó o eliminó cada historia sería empezar el historial de auditoría que
la spec deja fuera.

**Señalado al desarrollador**: si el equipo entiende que FR-049 pide **persistir** también quién
hizo la última modificación o la estimación, la forma más simple es una columna `updated_by_id` en
`user_stories`. No se incluye porque ningún escenario ni criterio de éxito la verifica.

---

## 16. Códigos de estado y forma de los errores

**Decisión**: se reutilizan los esquemas de error de la 001 y se fija esta correspondencia:

| Situación | Código | Requisito |
| --- | --- | --- |
| Entrada inválida (título, descripción, prioridad, Story Points, criterios, filtros, `PATCH` vacío, `status` o `storyPoints` en el `PATCH`) | `400` | FR-013, FR-019, FR-021, FR-027, FR-037 |
| Sin sesión válida | `401` | FR-045 |
| Proyecto inexistente **o** quien pide no es integrante (creación y listado) | `404` | FR-047 |
| Historia inexistente, eliminada **o** de un proyecto del que quien pide no es integrante | `404` | FR-044, FR-047 |
| Escritura sobre el backlog de un proyecto Finalizado | `409` | FR-048, RN-15 |
| Modificación o reestimación de una historia Completada | `409` | FR-030, RN-14 |
| Quitar los Story Points o el último criterio a una historia comprometida en un sprint abierto | `409` | FR-018, FR-022, RN-18 |
| Eliminación de una historia con historial | `409` | FR-041, FR-043, RN-16 |

**Precedencia**: `401` → `404` → `400` → `409`. El middleware resuelve la sesión y el acceso antes de
leer el cuerpo; la validación de entrada ocurre en `delivery` antes del servicio (Principio VII), y
las reglas de estado en el servicio. Entre los `409`, la solo lectura del proyecto se evalúa
primero, porque vuelve irrelevante cualquier otra regla.

**Rationale**: el `409` agrupa, como en la 002, los conflictos con el estado actual del recurso: la
petición está bien formada pero el estado del sistema no la admite.

---

## Resumen: incógnitas resueltas

| Incógnita | Resolución | Sección |
| --- | --- | --- |
| Dependencias nuevas | Ninguna | 1 |
| Organización y forma de las rutas | Un módulo `backlog`; colección anidada en el proyecto, ítem plano | 2 |
| `404` indistinguible para historias | `RequireStoryAccess` con una consulta y una salida | 3 |
| 0 frente a "sin estimar" | `smallint` anulable y `json.Number` validado contra la escala | 4 |
| Persistencia de los criterios | Tabla propia con posición; reemplazo completo | 5 |
| Ausente frente a vacío en la modificación | `PATCH` con `json.RawMessage` | 6 |
| Edición simultánea sin fusionar listas | `UPDATE` de columnas presentes y bloqueo de fila | 7 |
| Vía de estimación | Endpoint propio reutilizable por Planning Poker | 8 |
| Orden estable, filtros y los dos vacíos | `CASE` + `created_at` + `id`; `emptyReason` | 9 |
| "Lista para planificar" | Derivada al leer, con una única función de dominio | 10 |
| Datos de features que no existen | Cuatro puertos con implementaciones que informan lo cierto hoy | 11 |
| Eliminación con historial | `409` con todos los motivos; borrado definitivo | 12 |
| Vista de estado de la 002 | Se reemplaza su proveedor del resumen del backlog | 13 |
| Proyecto Finalizado | Estado leído con el acceso o con un puerto de una operación | 14 |
| Atribución | Autor persistido; identidad siempre desde la sesión | 15 |
| Códigos HTTP | Tabla y precedencia | 16 |

### Puntos abiertos para el desarrollador (no bloquean el diseño)

1. **Lectura de FR-049** (sección 15): se persiste solo el autor; confirmar que no se pide guardar el
   último modificador.
2. **Forma del `PATCH`** (sección 6): se rechazan `status` y `storyPoints` en lugar de ignorarlos.
   Afecta al contrato.
3. **Puntos de integración que anotar al planificar 004, 005, 006 y 007** (secciones 8, 11 y 12):
   reemplazar las implementaciones de los cuatro puertos, reutilizar `Estimate` desde Planning Poker
   y traducir la violación de clave foránea al borrar una historia en el mismo `409`.

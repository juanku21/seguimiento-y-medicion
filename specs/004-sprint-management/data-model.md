# Modelo de Datos (Fase 1): Gestión de Sprints

**Feature**: `specs/004-sprint-management` | **Fecha**: 2026-10-21

Entidades derivadas de la sección "Entidades Clave" de [spec.md](./spec.md). Las decisiones de
diseño que las justifican están en [research.md](./research.md).

Reglas transversales obligatorias (restricciones técnicas de la constitución):

- La clave primaria de los modelos persistidos es **UUID**; prohibidos los enteros secuenciales.
- Todo modelo registra **fecha y hora de creación y de última actualización** (`created_at`,
  `updated_at`, gestionadas por GORM).
- El esquema se crea con `AutoMigrate` de GORM.
- Los identificadores del código están en inglés (Principio VIII).

**Glosario**: "caracteres" de la spec son **runas** de Go, igual que en la 002 y la 003; coinciden
con lo que cuenta `varchar(n)` de PostgreSQL en los dos límites de esta feature (100 para el nombre y
500 para el Sprint Goal).

Esta feature **no modifica** las tablas `users`, `projects` ni `project_members`: las lee. De
`user_stories` (003) escribe **solo** `status` y la columna nueva `ever_committed` (sección 3).

---

## 1. `Sprint` — tabla `sprints`

Período acotado de trabajo dentro de un proyecto, con un objetivo y un conjunto de historias
comprometidas. Al cerrarse, sus columnas de cierre forman parte de la instantánea.

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK; lo genera la aplicación con `google/uuid` | FR-007 |
| `project_id` | `uuid` | `NOT NULL`; FK a `projects.id`; índice | FR-008 |
| `name` | `varchar(100)` | `NOT NULL`; recortado; no único | FR-002 |
| `goal` | `varchar(500)` | `NOT NULL`; recortado | FR-003 |
| `start_date` | `date` | `NOT NULL`; fecha prevista de inicio | FR-004 |
| `end_date` | `date` | `NOT NULL`; `end_date > start_date` | FR-004 |
| `status` | `varchar(16)` | `NOT NULL`; `planned`, `active` o `closed`; nace `planned` | FR-006, RN-02 |
| `committed_story_points` | `integer` | anulable; se fija al iniciar y no cambia más | FR-028 |
| `started_at` | `timestamptz` | anulable; momento real de inicio | FR-028 |
| `closed_at` | `timestamptz` | anulable; momento real de cierre | FR-040 |
| `closed_by_id` | `uuid` | anulable; FK a `users.id` | FR-040 |
| `hours_per_story_point_hundredths` | `integer` | anulable; factor del proyecto copiado al cerrar | FR-043 |
| `created_by_id` | `uuid` | `NOT NULL`; FK a `users.id` | FR-009 |
| `created_at` | `timestamptz` | `NOT NULL` | FR-009 |
| `updated_at` | `timestamptz` | `NOT NULL` | FR-009 |

**Índices**:

- `idx_sprints_project_start (project_id, start_date, id)`: sirve al listado, a su orden y a la
  verificación de superposición.
- `idx_sprints_one_active_per_project` **único parcial** sobre `(project_id) WHERE status =
  'active'`: respaldo de RN-06 (research, sección 5).

### Invariantes por estado

| Estado | `committed_story_points` | `started_at` | `closed_at`, `closed_by_id`, `hours_per_story_point_hundredths` |
| --- | --- | --- | --- |
| `planned` | `NULL` | `NULL` | `NULL` |
| `active` | no nulo | no nulo | `NULL` |
| `closed` | no nulo | no nulo | no nulos |

Las columnas que no son de cierre (`name`, `goal`, `start_date`, `end_date`) solo cambian en
`planned` (FR-012, FR-016). Desde que el sprint deja de estar Planificado la fila solo cambia una vez
más, al cerrarse, y después nunca (FR-044, FR-046).

### Reglas de validación

| Campo | Regla | Mensaje (resumen) |
| --- | --- | --- |
| `name` | Obligatorio; recortado; 1 a 100 runas | "El nombre es obligatorio" / "máximo 100 caracteres" |
| `goal` | Obligatorio; recortado; 1 a 500 runas | "El Sprint Goal es obligatorio" / "máximo 500 caracteres" |
| `startDate` | Obligatoria; `YYYY-MM-DD` | "La fecha de inicio es obligatoria" / "formato AAAA-MM-DD" |
| `endDate` | Obligatoria; `YYYY-MM-DD`; estrictamente posterior a `startDate` | "La fecha de fin debe ser posterior a la de inicio" |

La validación ocurre en `delivery` antes del servicio y devuelve **todos** los campos inválidos a la
vez. La superposición (FR-005) es una regla de estado del proyecto, no de formato: la verifica el
servicio y responde `409` (sección 6).

---

## 2. `SprintStory` — tabla `sprint_stories`

**Compromiso de historia en el sprint**. Mientras el sprint está abierto es el plan vivo; al cerrarlo
se completan sus campos congelados y pasa a ser parte de la instantánea (research, sección 6).

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK | Constitución |
| `sprint_id` | `uuid` | `NOT NULL`; FK a `sprints.id` | FR-017 |
| `story_id` | `uuid` | `NOT NULL`; FK a `user_stories.id` | FR-017 |
| `committed_at` | `timestamptz` | `NOT NULL` | Entidad Compromiso |
| `completed_at` | `timestamptz` | anulable; fecha y hora del último completado dentro de este sprint | FR-034, FR-035 |
| `title_at_close` | `varchar(150)` | anulable; se fija al cerrar | FR-043 |
| `priority_at_close` | `varchar(8)` | anulable; se fija al cerrar | FR-043 |
| `story_points_at_close` | `smallint` | anulable; se fija al cerrar | FR-043 |
| `outcome` | `varchar(16)` | anulable; `completed` o `not_completed`; `NULL` mientras el sprint está abierto | FR-042, FR-043 |
| `created_at` | `timestamptz` | `NOT NULL` | Constitución |
| `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

**Índices**:

- `idx_sprint_stories_sprint_story` **único** sobre `(sprint_id, story_id)`: una historia figura una
  sola vez en cada sprint.
- `idx_sprint_stories_one_open_per_story` **único parcial** sobre `(story_id) WHERE outcome IS
  NULL`: como máximo un compromiso abierto por historia (FR-020, RN-08).
- `idx_sprint_stories_story (story_id, committed_at)`: historial de una historia (FR-055).

### Ciclo de vida de una fila

| Evento | Efecto sobre la fila |
| --- | --- |
| Asignar la historia (sprint `planned` o `active`) | Se crea con `committed_at = now` y los campos congelados en `NULL` |
| Completar la historia (sprint `active`) | `completed_at = now` |
| Devolverla a En progreso (sprint `active`) | `completed_at = NULL` |
| Quitar la historia (sprint abierto) | Se **borra** la fila |
| Eliminar el sprint (`planned`) | Se **borran** todas sus filas |
| Cerrar el sprint | Se completan `title_at_close`, `priority_at_close`, `story_points_at_close` y `outcome`; desde ahí es inmutable |

`outcome` es `completed` si la historia estaba Completada al cerrar y `not_completed` en cualquier
otro caso. `completed_at` se conserva en las filas con `outcome = completed` y queda `NULL` en las
demás.

---

## 3. Cambios en `UserStory` (003) — tabla `user_stories`

| Columna | Tipo | Restricciones | Quién la escribe |
| --- | --- | --- | --- |
| `ever_committed` | `boolean` | `NOT NULL DEFAULT false`; pasa a `true` al primer compromiso y nunca vuelve | Solo esta feature (research, sección 7) |
| `status` | (existente) | `pending`, `in_progress`, `completed` | Solo esta feature, según la tabla de la sección 5 |

El campo `EverCommitted bool` se agrega al modelo `backlogdomain.UserStory` para que `AutoMigrate`
cree la columna. Ninguna otra columna de `user_stories` se escribe desde esta feature.

---

## 4. Estado del sprint — tipo `SprintStatus`

| Valor en la API y la base | Etiqueta en la interfaz |
| --- | --- |
| `planned` | Planificado |
| `active` | Activo |
| `closed` | Cerrado |

### Transiciones (FR-047, RN-02)

```text
planned ──start──▶ active ──close──▶ closed
```

`ValidateTransition(to)` devuelve `nil` solo para `planned → active` y `active → closed`; cualquier
otro par devuelve `ErrInvalidSprintTransition`. No hay transición de vuelta ni permanencia.

### Operaciones admitidas por estado

| Operación | `planned` | `active` | `closed` |
| --- | --- | --- | --- |
| Modificar nombre, fechas y Sprint Goal | Sí | `ErrSprintNotPlanned` | `ErrSprintClosed` |
| Eliminar | Sí, si no tiene esfuerzo | `ErrSprintAlreadyStarted` | `ErrSprintAlreadyStarted` |
| Asignar historias | Sí | Sí | `ErrSprintClosed` |
| Quitar historias | Sí | Sí | `ErrSprintClosed` |
| Iniciar | Sí | `ErrInvalidSprintTransition` | `ErrSprintClosed` |
| Registrar avance de sus historias | `ErrProgressOutsideActiveSprint` | Sí | `ErrSprintClosed` |
| Cerrar | `ErrSprintNotActive` | Sí | `ErrSprintClosed` |

Para un sprint Cerrado, el mensaje de cada rechazo dice que el estado Cerrado es definitivo o que un
sprint cerrado no admite cambios, según la operación (Condiciones de Error de la spec).

---

## 5. Avance de la historia dentro del sprint Activo

Valores de `StoryStatus` definidos por la 003: `pending`, `in_progress`, `completed`.

### Transiciones admitidas (FR-033, FR-038)

| Desde | Hacia | Efecto adicional |
| --- | --- | --- |
| `pending` | `in_progress` | — |
| `in_progress` | `completed` | `sprint_stories.completed_at = now` (FR-034) |
| `completed` | `in_progress` | `sprint_stories.completed_at = NULL` (FR-035) |

Cualquier otro par se rechaza con `ErrInvalidStoryTransition`, que enumera las tres admitidas.

### Escrituras de `user_stories.status` que hace esta feature

| Evento | Historias afectadas | Nuevo `status` |
| --- | --- | --- |
| Avance en el sprint Activo | La historia indicada | Según la tabla anterior |
| Quitar del sprint | Las historias del lote (ninguna puede estar `completed`, FR-025) | `pending` |
| Cerrar el sprint | Las que no están `completed` | `pending` |
| Eliminar el sprint | Sus historias comprometidas | `pending` |

---

## 6. Reglas de dominio puras (con prueba unitaria obligatoria)

| Función | Regla | Requisitos |
| --- | --- | --- |
| `PeriodsOverlap(a, b Period) bool` | `a.Start < b.End && b.Start < a.End`; compartir el día de borde no superpone | FR-005, RN-05 |
| `SprintStatus.ValidateTransition(to)` | Solo `planned → active` y `active → closed` | FR-047, RN-02 |
| `ValidateStoryProgress(from, to)` | Las tres transiciones de la sección 5 | FR-033, FR-038 |
| `Admission(story, openCommitment)` | Motivos `not_ready` (con `Readiness` de la 003), `completed`, `committed_elsewhere` | FR-018 a FR-020, RN-08 |
| `Schedule(endDate, now, location)` | `remainingDays` o `overdueDays`, nunca negativo (research, sección 15) | FR-053, RN-23 |
| `Outcome(status)` | `completed` si la historia está Completada; `not_completed` si no | FR-043 |

### Tabla de `Schedule`

| Fecha de fin | Hoy (en `APP_TIMEZONE`) | `remainingDays` | `overdueDays` |
| --- | --- | --- | --- |
| 2026-10-14 | 2026-10-04 | 10 | `null` |
| 2026-10-14 | 2026-10-14 | 0 | `null` |
| 2026-10-14 | 2026-10-17 | `null` | 3 |

Solo se calcula para sprints `active`; para `planned` y `closed` el detalle devuelve `schedule: null`.

---

## 7. Instantánea de cierre (vista lógica, no tabla propia)

La **Instantánea de cierre** de la spec es la unión de las columnas de cierre de `sprints` y las filas
congeladas de `sprint_stories` (research, sección 11).

| Dato | Fuente |
| --- | --- |
| Story Points comprometidos al inicio | `sprints.committed_story_points` |
| Story Points planificados al cierre | `COALESCE(SUM(sprint_stories.story_points_at_close), 0)` |
| Story Points completados | `COALESCE(SUM(story_points_at_close) FILTER (WHERE outcome = 'completed'), 0)` |
| Historias planificadas | `COUNT(sprint_stories)` |
| Historias completadas | `COUNT(*) FILTER (WHERE outcome = 'completed')` |
| Historias involucradas | Filas de `sprint_stories` con `title_at_close`, `priority_at_close`, `story_points_at_close`, `outcome` y `completed_at` |
| Cuatro fechas | `start_date`, `end_date`, `started_at`, `closed_at` |
| Factor de horas por Story Point | `sprints.hours_per_story_point_hundredths` |

Los cinco totales se calculan en un único fragmento de consulta del repositorio, compartido por el
listado y el detalle; no hay una segunda implementación en Go.

Como todas las fuentes son inmutables desde el cierre, la instantánea no cambia ante
reestimaciones, renombres ni cambios del factor del proyecto (FR-044, SC-008). Un sprint sin
historias cierra con todos los totales en 0 (FR-045).

---

## 8. Tipos de lectura (no persistidos)

| Tipo | Lo arma | Campos | Para qué |
| --- | --- | --- | --- |
| `SprintAccess` | `SprintRepository.FindAccessible` | `SprintID`, `ProjectID`, `ProjectFinished` | `RequireSprintAccess` |
| `SprintInput` | `delivery` | `Name`, `Goal`, `StartDate`, `EndDate` ya validados | Creación y modificación |
| `SprintListItem` | `SprintRepository.List` | Datos del sprint, `StoryCount` y `Results *SprintResults` | Listado e historial (FR-050, FR-051) |
| `SprintResults` | Agregados de `sprint_stories` | `CommittedStoryPoints`, `PlannedStoryPoints`, `CompletedStoryPoints`, `PlannedStoryCount`, `CompletedStoryCount` | Totales de la instantánea |
| `SprintDetail` | `SprintRepository.FindDetail` | El sprint, `CreatedBy` y `ClosedBy` (`ID`, `FullName`), sus historias y, si está Cerrado, `Results` y el factor | Detalle (FR-053) |
| `SprintStoryView` | `FindDetail` | `StoryID`, `Title`, `Priority`, `StoryPoints`, `Status`, `CompletedAt`, `Outcome` | Historias del detalle: vivas en un sprint abierto, congeladas en uno Cerrado (US6-8) |
| `StorySprintHistoryItem` | `SprintRepository.ListForStory` | `SprintID`, `SprintName`, `SprintStatus`, `StartDate`, `EndDate`, `Outcome`, `CompletedAt` | Historial de una historia (FR-055) |
| `ConflictingSprint` | `FindOverlapping` / `FindActive` | `ID`, `Name` | Mensajes de superposición y de sprint activo |
| `AdmissionProblem` | `AssignStories`, dentro de `StoriesNotAdmittedError` | `StoryID`, `Reason`, `Missing []string`, `Sprint *ConflictingSprint` | Cuerpo del `409` de asignación |

En el detalle de un sprint **abierto**, las historias muestran los datos vigentes de `user_stories`;
en uno **Cerrado**, los congelados de `sprint_stories` (FR-043, US6-8). `CreatedBy` y `ClosedBy` se
leen con un `JOIN` a `users` que selecciona solo `id` y `full_name`.

En el historial de una historia, `Outcome` es `completed` o `not_completed` en los sprints Cerrados
y `null` en el sprint abierto en el que está. El orden es `start_date ASC, sprint id ASC`.

---

## 9. Interfaces de repositorio y puertos (capa `domain`)

Declaran solo las operaciones que se usan (Principio III). Ninguna devuelve
`gorm.ErrRecordNotFound` ni `(nil, nil)` para un recurso obligatorio.

```text
SprintRepository
  FindAccessible(ctx, sprintID, userID) (*SprintAccess, error)           // RequireSprintAccess; ErrSprintNotFound si no hay fila
  Create(ctx, sprint *Sprint) error                                     // bloquea el proyecto; ErrSprintsReadOnly o SprintOverlapError
  Update(ctx, sprintID, input SprintInput) error                        // bloquea proyecto y sprint; SprintOverlapError excluyéndose
  Delete(ctx, sprintID) error                                           // bloquea; estado planned; historias a pending; borra filas y sprint
  Start(ctx, sprintID, now) error                                       // bloquea el proyecto; ActiveSprintExistsError; congela committed_story_points
  Close(ctx, sprintID, closedBy, now) error                             // bloquea sprint e historias; congela filas; historias a pending; copia el factor
  AssignStories(ctx, sprintID, storyIDs, now) error                     // bloquea sprint e historias; StoriesNotAdmittedError, todo o nada
  RemoveStories(ctx, sprintID, storyIDs) error                          // bloquea; rechaza completed; borra filas; historias a pending
  ChangeStoryProgress(ctx, storyID, to, now) error                      // bloquea historia, FOR SHARE del sprint; transición y completed_at
  List(ctx, projectID, statuses []SprintStatus) ([]SprintListItem, error)
  HasSprints(ctx, projectID) (bool, error)                              // solo cuando el listado filtrado viene vacío
  FindDetail(ctx, sprintID) (*SprintDetail, error)
  ListForStory(ctx, storyID) ([]StorySprintHistoryItem, error)
```

Los conflictos con datos (sprint superpuesto, sprint activo, historias no admitidas) viajan **solo**
en los errores tipados de la tabla de errores de esta sección; el servicio y `delivery` los extraen con `errors.As`.

Las operaciones de escritura reciben el instante desde el servicio, que lo toma del reloj inyectado,
para que las pruebas fijen `started_at`, `completed_at` y `closed_at`. Las reglas puras de la
sección 6 se evalúan dentro de esas transacciones, después de tomar los bloqueos, con los datos
releídos.

### Puerto hacia la 006

```text
SprintEffort
  HasEffort(ctx, sprintID) (bool, error)   // implementación actual: false para todo sprint
```

### Errores de dominio (`sprints/domain/errors.go`)

| Error | Situación | Código HTTP |
| --- | --- | --- |
| `ErrSprintNotFound` | Sprint inexistente, eliminado o ajeno | `404` |
| `ErrStoryNotFound` (de la 003) | Historia inexistente o de otro proyecto en un lote | `404` |
| `ErrSprintsReadOnly` | Escritura sobre los sprints de un proyecto Finalizado | `409` |
| `SprintOverlapError` (con `ConflictingSprint`) | Período superpuesto | `409` |
| `ActiveSprintExistsError` (con `ConflictingSprint`) | Ya hay un sprint Activo | `409` |
| `ErrInvalidSprintTransition` | Iniciar un sprint Activo | `409` |
| `ErrSprintNotPlanned` | Modificar un sprint Activo | `409` |
| `ErrSprintNotActive` | Cerrar un sprint Planificado | `409` |
| `ErrSprintClosed` | Cualquier escritura sobre un sprint Cerrado | `409` |
| `ErrSprintAlreadyStarted` | Eliminar un sprint Activo o Cerrado | `409` |
| `ErrSprintHasEffort` | Eliminar un sprint con esfuerzo registrado | `409` |
| `StoriesNotAdmittedError` (con `[]AdmissionProblem`) | Lote con historias no admisibles | `409` |
| `ErrCompletedStoryRemoval` | Quitar una historia Completada | `409` |
| `ErrProgressOutsideActiveSprint` | Avance de una historia sin sprint o de un sprint Planificado | `409` |
| `ErrInvalidStoryTransition` | Transición de historia no admitida | `409` |

### Motivos de no admisión (cuerpo del `409` de asignación)

| Código | Condición | Mensaje |
| --- | --- | --- |
| `not_ready` | `Readiness` informa faltantes; `missing` lista `story_points` y/o `acceptance_criteria` | "No está lista para planificar: le faltan Story Points o criterios de aceptación." |
| `completed` | `status = completed` | "La historia ya está terminada." |
| `committed_elsewhere` | Tiene un compromiso abierto; `sprint` indica en cuál | "Ya está comprometida en el sprint «…»." |

---

## 10. Relaciones

```text
projects (002)     1 ──── N sprints          (project_id; un sprint no cambia de proyecto)
users (001)        1 ──── N sprints          (created_by_id, closed_by_id)
sprints            1 ──── N sprint_stories   (sprint_id)
user_stories (003) 1 ──── N sprint_stories   (story_id; historial de la historia)
project_members (002)                        (solo se lee: decide el acceso)
```

- `project_id` no es parte de ningún esquema de entrada de modificación (FR-008).
- Las claves foráneas usan la restricción por defecto. La de `sprint_stories.story_id` es además el
  respaldo de RC-06: una historia con filas en `sprint_stories` no se puede borrar aunque la
  verificación de la 003 fallara (research de la 003, sección 12).
- La 006 agregará una clave foránea desde sus registros de esfuerzo hacia `sprints.id` (research,
  sección 16).

### Efecto sobre la 002

`internal/projects/repository/sprint_directory.go` pasa a leer `sprints`:

| Operación del puerto (002) | Implementación |
| --- | --- |
| `FindActiveSprint(projectID)` | `SELECT id, name FROM sprints WHERE project_id = ? AND status = 'active'`; `(nil, nil)` sin fila |

### Efecto sobre la 003

`internal/backlog/repository/sprint_assignments.go` pasa a leer `sprint_stories` y `user_stories`:

| Campo de `SprintAssignment` (003) | Implementación |
| --- | --- |
| `CommittedToOpenSprint` | `EXISTS (SELECT 1 FROM sprint_stories WHERE story_id = ? AND outcome IS NULL)` |
| `EverAssigned` | `user_stories.ever_committed` |

---

## 11. Trazabilidad: requisitos cubiertos por el modelo

| Requisitos | Dónde se resuelven |
| --- | --- |
| FR-001 a FR-011 | `Sprint`, reglas de validación, `PeriodsOverlap`, `Create` (secciones 1, 6 y 9) |
| FR-012 a FR-016 | `Update` y la tabla de operaciones por estado (secciones 4 y 9) |
| FR-017 a FR-025 | `SprintStory`, `Admission`, `AssignStories`, `RemoveStories`, `ever_committed` (secciones 2, 3, 6 y 9) |
| FR-026 a FR-032 | `ValidateTransition`, `Start`, `committed_story_points`, índice parcial de activo (secciones 1, 4 y 9) |
| FR-033 a FR-038 | `ValidateStoryProgress`, `ChangeStoryProgress`, `completed_at` (secciones 2, 5 y 9) |
| FR-039 a FR-047 | `Close`, instantánea, inmutabilidad (secciones 2, 4 y 7) |
| FR-048 | `Delete` y el puerto `SprintEffort` (sección 9) |
| FR-049 | Ninguna operación escribe `projects` (sección 10) |
| FR-050 a FR-055 | `SprintListItem`, `SprintResults`, `SprintDetail`, `Schedule`, `ListForStory` (secciones 6 y 8) |
| FR-056 a FR-058 | `SprintAccess` y `RequireSprintAccess`; `RequireMembership` de la 002; `RequireStoryAccess` de la 003 |
| FR-059 | `SprintAccess.ProjectFinished` y la lectura del proyecto en cada transacción |
| FR-060 | `created_by_id`, `closed_by_id` y la identidad leída de la sesión |
| RN-01 a RN-23 | Distribuidas en las secciones 1 a 7; ver la tabla de pruebas unitarias en [plan.md](./plan.md) |
| RC-04, RC-06 | Puertos de la 002 y la 003 (sección 10) y `ever_committed` (sección 3) |

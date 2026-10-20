# Modelo de Datos (Fase 1): Product Backlog

**Feature**: `specs/003-product-backlog` | **Fecha**: 2026-10-20

Entidades derivadas de la sección "Entidades Clave" de [spec.md](./spec.md). Las decisiones de
diseño que las justifican están en [research.md](./research.md).

Reglas transversales obligatorias (restricciones técnicas de la constitución):

- La clave primaria de los modelos persistidos es **UUID**; prohibidos los enteros secuenciales.
- Todo modelo registra **fecha y hora de creación y de última actualización** (`created_at`,
  `updated_at`, gestionadas por GORM).
- El esquema se crea con `AutoMigrate` de GORM.
- Los identificadores del código están en inglés (Principio VIII).

**Glosario**: donde la spec dice "caracteres", este documento dice **runas** (`rune` de Go, un punto
de código Unicode), que es la unidad en la que se cuenta al validar y la que también cuenta
`varchar(n)` de PostgreSQL. Las dos coinciden en todos los límites de esta feature —150 para el
título, 2000 para la descripción y 500 para cada criterio—, igual que en la 002.

Esta feature **no modifica** las tablas `users`, `projects` ni `project_members`: las lee, y solo
para atribuir la autoría, verificar el acceso y conocer el estado del proyecto.

---

## 1. `UserStory` — tabla `user_stories`

Representa una unidad de trabajo que el equipo quiere construir y medir. Es la unidad a la que se
referirán la planificación de sprints, el esfuerzo registrado y los defectos.

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK; lo genera la aplicación con `google/uuid` | FR-009 |
| `project_id` | `uuid` | `NOT NULL`; FK a `projects.id`; índice | FR-010, FR-038 |
| `title` | `varchar(150)` | `NOT NULL`; recortado; sin índice único | FR-002, FR-012, RN-03 |
| `description` | `varchar(2000)` | anulable; recortada; vacía se guarda como `NULL` | FR-003, RN-04 |
| `priority` | `varchar(8)` | `NOT NULL`; `high`, `medium` o `low` | FR-004, RN-05 |
| `status` | `varchar(16)` | `NOT NULL`; `pending`, `in_progress` o `completed`; nace `pending` | FR-008, FR-026, RN-09 |
| `story_points` | `smallint` | anulable; `NULL` es "sin estimar"; si no es nulo, pertenece a la escala | FR-005, FR-014, FR-015, RN-06, RN-07 |
| `created_by_id` | `uuid` | `NOT NULL`; FK a `users.id` | FR-011, FR-049, FR-050 |
| `created_at` | `timestamptz` | `NOT NULL`; inmutable | FR-011, FR-024 |
| `updated_at` | `timestamptz` | `NOT NULL`; cambia con cada modificación y estimación | FR-011, FR-017 |

**Índice compuesto** `idx_user_stories_project_order (project_id, priority, created_at, id)`: sirve
al filtro por proyecto del listado y, en parte, a su orden. El orden por prioridad se resuelve con un
`CASE` (sección 3), así que el índice no lo cubre entero; con backlogs de decenas o cientos de
historias por proyecto no hace falta más.

### Reglas de validación

| Campo | Regla | Mensaje (resumen) |
| --- | --- | --- |
| `title` | Obligatorio; se recortan los espacios extremos; entre 1 y 150 runas | "El título es obligatorio" / "máximo 150 caracteres" |
| `description` | Opcional; se recorta; hasta 2000 runas; vacía o solo espacios equivale a ausente | "máximo 2000 caracteres" |
| `priority` | Obligatoria; `high`, `medium` o `low` | "Los valores admitidos son Alta, Media y Baja" |
| `storyPoints` | Opcional en la creación; `null` o un entero de la escala | "Los valores admitidos son 0, 1, 2, 3, 5, 8, 13, 20, 40 y 100" |
| `acceptanceCriteria` | Opcional; lista de textos; cada uno recortado con entre 1 y 500 runas | "Cada criterio debe tener contenido" / "máximo 500 caracteres" |

La validación ocurre en `delivery` antes de llegar al servicio (Principio VII) y devuelve **todos**
los campos inválidos a la vez, con el índice del criterio cuando el error es de uno de ellos
(`acceptanceCriteria[2]`).

### Notas de diseño

- **Sin columna "lista para planificar"**: se deriva al leer (sección 5; research, sección 10).
- **Sin borrado lógico**: la eliminación es definitiva (research, sección 12).
- **Sin columnas de último modificador ni de sprint**: la autoría de los cambios sale de la sesión y
  la asignación a sprint la persistirá `specs/004-sprint-management` (research, secciones 11 y 15).
- Los campos `Project` y `CreatedBy` del modelo de Go existen solo para que `AutoMigrate` cree las
  claves foráneas: no son columnas y nunca se precargan, igual que en los modelos de la 002.
- El estado lo modifica **solo** la feature de sprints (FR-027, RC-04). En esta feature ninguna
  operación escribe la columna `status` después de la creación.

---

## 2. `AcceptanceCriterion` — tabla `acceptance_criteria`

Enunciado que define cuándo una historia se considera terminada. Pertenece a una historia y ocupa
una posición en su lista ordenada.

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK; lo genera la aplicación | Constitución |
| `story_id` | `uuid` | `NOT NULL`; FK a `user_stories.id` | FR-006 |
| `position` | `integer` | `NOT NULL`; desde 0, consecutivo dentro de la historia | FR-006, FR-022 |
| `text` | `varchar(500)` | `NOT NULL`; recortado; entre 1 y 500 runas | FR-007, RN-08 |
| `created_at` | `timestamptz` | `NOT NULL` | Constitución |
| `updated_at` | `timestamptz` | `NOT NULL` | Constitución |

**Índice único** `idx_acceptance_criteria_story_position (story_id, position)`: garantiza un único
criterio por posición y sirve a la lectura ordenada del detalle y al conteo del listado.

### Ciclo de vida

- Se crean junto con la historia, en la misma transacción (FR-013).
- Toda modificación de la lista **la reemplaza completa**: se borran las filas de la historia y se
  insertan las nuevas con posiciones `0..n-1`, en la misma transacción que bloquea la fila de la
  historia (FR-022, FR-025; research, secciones 5 y 7).
- Se borran junto con la historia, en la misma transacción (FR-044).
- La API no expone sus identificadores: la lista viaja como textos ordenados.

---

## 3. Prioridad — tipo `StoryPriority`

| Valor en la API y la base | Etiqueta en la interfaz | Rango de orden |
| --- | --- | --- |
| `high` | Alta | 0 |
| `medium` | Media | 1 |
| `low` | Baja | 2 |

El orden del listado es `rango ASC, created_at ASC, id ASC` (FR-032, RN-17). El rango no se
persiste: lo calcula un `CASE` en la consulta.

---

## 4. Estado de la historia — tipo `StoryStatus`

| Valor en la API y la base | Etiqueta en la interfaz |
| --- | --- |
| `pending` | Pendiente |
| `in_progress` | En progreso |
| `completed` | Completada |

### Transiciones

Esta feature **no produce ninguna transición** (FR-027, RN-10, RC-04). Toda historia nace `pending`
y queda así hasta que la feature de sprints la cambie. El backlog solo **lee** el estado para dos
reglas:

| Estado | Modificar (`PATCH`) | Estimar (`PUT story-points`) | Eliminar |
| --- | --- | --- | --- |
| `pending` | Sí | Sí | Sí, si no tiene historial |
| `in_progress` | Sí (US5-5) | Sí | No: una historia En progreso estuvo en un sprint |
| `completed` | No: `409` (FR-030, RN-14) | No: `409` (FR-030, RN-14) | No: estuvo en un sprint |

La columna "Eliminar" no es una regla adicional: se deriva de RN-16, porque una historia solo deja
de estar `pending` dentro de un sprint.

---

## 5. Escala de Story Points y "lista para planificar"

### Escala

```text
STORY_POINTS_SCALE = {0, 1, 2, 3, 5, 8, 13, 20, 40, 100}
```

Se declara una sola vez en `backlog/domain` (y su espejo en `frontend/src/utils/story-labels.ts` para
el selector). `NULL` en la base y `null` en la API significan "sin estimar". El 0 es una estimación
válida (FR-015, RN-07).

### Lista para planificar (FR-028, FR-029, RN-12)

```text
Readiness(storyPoints, criteriaCount):
  missing = []
  si storyPoints es nulo  → agregar "story_points"
  si criteriaCount == 0   → agregar "acceptance_criteria"
  ready = (missing está vacío)
```

| Story Points | Criterios | `ready` | `missing` |
| --- | --- | --- | --- |
| 5 | 2 | `true` | `[]` |
| 0 | 1 | `true` | `[]` |
| `null` | 2 | `false` | `["story_points"]` |
| 3 | 0 | `false` | `["acceptance_criteria"]` |
| `null` | 0 | `false` | `["story_points", "acceptance_criteria"]` |

Es la única fuente de la regla: la usan el listado, el detalle y, más adelante, la feature de sprints
para hacer cumplir RN-13.

### Restricción de una historia comprometida (FR-018, FR-022, RN-18, RC-09)

Si el puerto `SprintAssignments` informa `CommittedToOpenSprint = true`:

| Operación | Resultado |
| --- | --- |
| Estimar con otro valor de la escala | Se acepta: la historia sigue lista |
| Estimar con `null` | `409`: primero hay que quitarla del sprint |
| `PATCH` con `acceptanceCriteria: []` | `409`: primero hay que quitarla del sprint |
| `PATCH` con una lista no vacía | Se acepta |

---

## 6. Tipos de lectura (no persistidos)

| Tipo | Lo arma | Campos | Para qué |
| --- | --- | --- | --- |
| `StoryAccess` | `StoryRepository.FindAccessible` | `StoryID`, `ProjectID`, `ProjectFinished` | `RequireStoryAccess` (research, sección 3) |
| `StoryListItem` | `StoryRepository.List` | `ID`, `Title`, `Priority`, `Status`, `StoryPoints *int`, `CriteriaCount`, `CreatedAt` | Listado (FR-033); `ReadyToPlan` se calcula con `Readiness` |
| `StoryDetail` | `StoryRepository.FindDetail` | La historia, sus criterios en orden y `CreatedBy` (`ID`, `FullName`) | Detalle y respuesta de las escrituras (FR-039) |
| `StoryFilter` | `delivery` | `Statuses []StoryStatus`, `Priorities []StoryPriority` | Filtros del listado (FR-034) |
| `StoryChanges` | `delivery` → `service` | Campos opcionales: `Title *string`, `Description` (presente/nulo), `Priority *StoryPriority`, `AcceptanceCriteria *[]string` | `PATCH` (FR-020) |

`StoryDetail.CreatedBy` se lee con un `JOIN` a `users` que selecciona solo `id` y `full_name`. El
hash de la contraseña nunca entra al módulo, igual que en la 002.

---

## 7. Puertos hacia otras features (no persistidos)

Declarados en `internal/backlog/domain/ports.go` (research, secciones 11 y 14).

### `SprintAssignments` — implementará `specs/004-sprint-management`

```text
Find(ctx, storyID) (SprintAssignment, error)
```

| Campo de `SprintAssignment` | Tipo | Para qué |
| --- | --- | --- |
| `CommittedToOpenSprint` | booleano | FR-018, FR-022, RN-18 |
| `EverAssigned` | booleano | FR-039 (detalle), FR-041 (eliminación), RN-16 |

### `EffortRecords` — implementará `specs/006-effort-tracking`

```text
HasEffort(ctx, storyID) (bool, error)
```

### `DefectRecords` — implementará `specs/007-defect-tracking`

```text
HasDefects(ctx, storyID) (bool, error)
```

### `EstimationSessions` — implementará `specs/005-planning-poker`

```text
HasSessions(ctx, storyID) (bool, error)
```

Cuenta las sesiones Finalizadas y las Canceladas (RC-10).

### `ProjectDirectory` — lectura de la tabla `projects` de la 002

```text
IsFinished(ctx, projectID) (bool, error)
```

Selecciona solo `projects.status`. Lo usa la creación de historias, porque las rutas de ítem ya
reciben el estado con `StoryAccess`.

### Estado actual de las implementaciones

Las implementaciones de los cuatro primeros puertos viven en `internal/backlog/repository/` e
informan lo que hoy es cierto: **ninguna historia está ni estuvo en un sprint ni tiene esfuerzo,
defectos o sesiones de estimación**. Cuando llegue cada feature, reemplaza el cuerpo de su archivo
sin tocar `backlog/service`. `ProjectDirectory` sí lee la base desde el primer día.

### Motivos de bloqueo de la eliminación (FR-043)

| Código | Puerto | Mensaje |
| --- | --- | --- |
| `sprint_history` | `SprintAssignments.EverAssigned` | "Estuvo asignada a un sprint." |
| `effort` | `EffortRecords` | "Tiene esfuerzo registrado." |
| `defects` | `DefectRecords` | "Tiene defectos asociados." |
| `estimation_sessions` | `EstimationSessions` | "Tiene sesiones de estimación por Planning Poker." |

El rechazo lista **todos** los motivos que aplican, en este orden.

---

## 8. Relaciones

```text
users (001) 1 ──── N user_stories            (created_by_id; la autoría se conserva)
projects (002) 1 ── N user_stories            (project_id; una historia no cambia de proyecto)
user_stories 1 ──── N acceptance_criteria     (story_id; posición única por historia)
project_members (002)                         (solo se lee: decide el acceso)
```

- Una historia pertenece a exactamente un proyecto y no puede moverse (FR-010): `project_id` no es
  parte de ningún esquema de entrada de modificación.
- Ninguna tabla de esta feature referencia a `project_members`: la membresía decide el acceso pero no
  forma parte de la historia. Por eso quitar a un integrante no toca sus historias (FR-050).
- Hoy ninguna tabla referencia a `user_stories` salvo `acceptance_criteria`. Las features 004 a 007
  agregarán las suyas (research, sección 12).

### Efecto sobre la 002

`internal/projects/repository/backlog_summary.go` pasa a leer `user_stories` con una consulta
agrupada por estado (research, sección 13):

| Campo de `BacklogSummary` (002) | Cálculo |
| --- | --- |
| `CountsByState` | `COUNT(*)` por `status`, con las claves `pending`, `in_progress`, `completed` presentes solo si tienen historias |
| `EstimatedStoryPoints` | `COALESCE(SUM(story_points), 0)`: las "sin estimar" no suman, las de 0 suman 0 |

---

## 9. Interfaces de repositorio (capa `domain`)

Declaran solo las operaciones que se usan (Principio III). Cada operación devuelve el error de
negocio y nunca `gorm.ErrRecordNotFound` ni `(nil, nil)`.

```text
StoryRepository
  FindAccessible(ctx, storyID, userID) (*StoryAccess, error)       // RequireStoryAccess; ErrStoryNotFound si no hay fila
  Create(ctx, story *UserStory, criteria []string) error           // historia y criterios en una transacción
  FindDetail(ctx, storyID) (*StoryDetail, error)                   // historia, criterios ordenados y autor
  List(ctx, projectID, filter StoryFilter) ([]StoryListItem, error) // una consulta con conteo de criterios
  HasStories(ctx, projectID) (bool, error)                         // solo cuando el listado filtrado viene vacío
  Update(ctx, storyID, changes StoryChanges) error                 // bloqueo de fila, columnas presentes y criterios
  UpdateStoryPoints(ctx, storyID, storyPoints *int) error          // estimación
  Delete(ctx, storyID) error                                       // criterios e historia en una transacción
```

No se declaran: búsqueda por texto, paginación, cambio de estado, cambio de proyecto, ni listados
fuera de un proyecto, porque ninguna funcionalidad pedida los usa.

### Errores de dominio (`backlog/domain/errors.go`)

| Error | Situación | Código HTTP |
| --- | --- | --- |
| `ErrStoryNotFound` | Historia inexistente, eliminada o ajena | `404` |
| `ErrBacklogReadOnly` | Escritura sobre el backlog de un proyecto Finalizado | `409` |
| `ErrStoryCompleted` | Modificar o reestimar una historia Completada | `409` |
| `ErrCommittedStoryMustStayReady` | Quitar Story Points o el último criterio a una historia comprometida | `409` |
| `DeletionBlockedError` (tipo con `Reasons`) | Eliminar una historia con historial | `409` |

---

## 10. Trazabilidad: requisitos cubiertos por el modelo

| Requisitos | Dónde se resuelven |
| --- | --- |
| FR-001 a FR-013 | `UserStory`, `AcceptanceCriterion`, reglas de validación (secciones 1 y 2) |
| FR-014 a FR-019 | `story_points` anulable y escala (secciones 1 y 5) |
| FR-020 a FR-025 | `StoryChanges`, `StoryRepository.Update` (secciones 6 y 9; research, secciones 6 y 7) |
| FR-026 a FR-030 | `StoryStatus` y `Readiness` (secciones 4 y 5) |
| FR-031 a FR-038 | `StoryListItem`, `StoryFilter`, `List` y `HasStories` (secciones 6 y 9) |
| FR-039 y FR-040 | `StoryDetail` y `SprintAssignments.EverAssigned` (secciones 6 y 7) |
| FR-041 a FR-044 | Cuatro puertos y `DeletionBlockedError` (secciones 7 y 9) |
| FR-045 a FR-047 | `StoryAccess` y `RequireStoryAccess`; `RequireMembership` de la 002 |
| FR-048 | `StoryAccess.ProjectFinished` y `ProjectDirectory` |
| FR-049 y FR-050 | `created_by_id` y la identidad leída de la sesión |
| RN-01 a RN-18 | Distribuidas en las secciones 1 a 7; ver la tabla de pruebas unitarias en [plan.md](./plan.md) |
| RC-07 | Proveedor del resumen del backlog de la 002 (sección 8) |

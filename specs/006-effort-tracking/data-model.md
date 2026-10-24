# Modelo de Datos (Fase 1): Registro de Esfuerzo

**Feature**: `specs/006-effort-tracking` | **Fecha**: 2026-10-24 | **Plan**: [plan.md](./plan.md)

Las decisiones que justifican este modelo están en [research.md](./research.md). Los nombres de
tablas, columnas, tipos y valores de dominio están en inglés (Principio VIII); los comentarios y los
mensajes, en español.

---

## 1. `EffortEntry` → tabla `effort_entries` (NUEVA)

Representa las horas que una persona trabajó sobre una historia en un día concreto, y qué hizo
(spec, Entidades Clave).

| Campo Go | Columna | Tipo PostgreSQL | Nulo | Notas |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | no | PK, generado con `google/uuid` (FR-011) |
| `StoryID` | `story_id` | `uuid` | no | FK → `user_stories.id`, sin cascada. **Inmutable** (FR-019) |
| `UserID` | `user_id` | `uuid` | no | FK → `users.id`, sin cascada. Siempre el titular de la sesión que lo creó (FR-002). **Inmutable** |
| `SprintID` | `sprint_id` | `uuid` | **sí** | FK → `sprints.id`, sin cascada. `NULL` = sin sprint. Se fija en el alta y **nunca cambia** (FR-014, FR-015) |
| `EntryDate` | `entry_date` | `date` | no | Fecha de calendario, a medianoche UTC en Go (RC-08) |
| `Activity` | `activity` | `varchar(300)` | no | Recortada; 1 a 300 caracteres (FR-003) |
| `HoursHundredths` | `hours_hundredths` | `integer` | no | Centésimas de hora: 25 a 2400, múltiplo de 25 (FR-004) |
| `CreatedAt` | `created_at` | `timestamptz` | no | Lo completa GORM (FR-012) |
| `UpdatedAt` | `updated_at` | `timestamptz` | no | Lo completa GORM (FR-012) |
| `Story` | — | — | — | Solo para que `AutoMigrate` cree la FK; nunca se precarga |
| `User` | — | — | — | Ídem |
| `Sprint` | — | — | — | Ídem |

**Índices**:

| Nombre | Columnas | Para qué |
| --- | --- | --- |
| `idx_effort_entries_story_order` | `(story_id, entry_date, created_at)` | Esfuerzo de una historia (FR-030, FR-035) y suma de la comparación |
| `idx_effort_entries_user_date` | `(user_id, entry_date)` | Suma del tope diario (FR-005) y filtro por integrante |
| `idx_effort_entries_sprint` | `(sprint_id)` | `SprintEffort.HasEffort` de la 004 y filtro por sprint (FR-039) |

**Claves foráneas sin `ON DELETE CASCADE`**: borrar una historia, un sprint o un usuario con registros
falla en la base. Es la garantía de FR-029 y RN-21; la 003 y la 004 ya lo impiden antes con sus
puertos (sección 3).

**No hay** columna `project_id` (se deriva de la historia), ni de autor de la última modificación
(siempre es el autor; FR-061), ni de borrado lógico, ni de "definitivo" (se deriva del sprint; sección
5).

---

## 2. `UserStory` → tabla `user_stories` (TOCA, de la 003)

| Campo Go | Columna | Tipo | Nulo | Notas |
| --- | --- | --- | --- | --- |
| `EverHadEffort` | `ever_had_effort` | `boolean` | no | `DEFAULT false`. Pasa a `true` con el primer registro de esfuerzo y **nunca vuelve a `false`**, aunque se eliminen todos los registros (RC-05, RN-22) |

Solo la escribe esta feature, con `UpdateColumn` (sin tocar `updated_at`), dentro de la transacción
del alta. La lee el puerto `EffortRecords` de la 003. Es el mismo patrón que `ever_committed` de la
004.

---

## 3. Puertos de otras features que esta feature implementa

| Puerto | Archivo | Implementación |
| --- | --- | --- |
| `backlogdomain.EffortRecords.HasEffort(storyID)` | `backlog/repository/effort_records.go` | Lee `user_stories.ever_had_effort` |
| `sprintsdomain.SprintEffort.HasEffort(sprintID)` | `sprints/repository/sprint_effort.go` | `EXISTS` sobre `effort_entries.sprint_id` |

Además, `sprints/repository.SprintRepository.Delete` traduce `gorm.ErrForeignKeyViolated` a
`ErrSprintHasEffort` (research, sección 10).

---

## 4. Datos que esta feature lee y no escribe

| Tabla (feature) | Columnas leídas | Para qué |
| --- | --- | --- |
| `projects` (002) | `status`, `start_date`, `hours_per_story_point_hundredths` | Solo lectura, fecha mínima, factor vigente |
| `project_members` (002) | `project_id`, `user_id` | Acceso y validez del filtro por integrante |
| `users` (001) | `id`, `full_name` (y bloqueo de la fila) | Nombre del autor; tope diario. **Nunca** `password_hash` |
| `user_stories` (003) | `id`, `project_id`, `title`, `story_points`, `priority`, `created_at` | Historia del registro, comparación y su orden |
| `sprint_stories` (004) | `story_id`, `sprint_id`, `outcome` | Compromiso abierto y último congelado |
| `sprints` (004) | `id`, `project_id`, `name`, `status`, `closed_at` | Anclaje, plazo de gracia, nombre en las listas |

---

## 5. Valores derivados (no almacenados)

| Valor | Fórmula | Requisitos |
| --- | --- | --- |
| Plazo vigente de un sprint Cerrado | `now < closed_at + EFFORT_GRACE_PERIOD_HOURS` | FR-016, RC-11 |
| Registro definitivo (`definitive`) | Sprint asociado Cerrado y plazo no vigente | FR-020, RN-09 |
| Horas reales de una historia | `Σ hours_hundredths` de sus registros | FR-031, FR-047 |
| Desglose por integrante | `Σ hours_hundredths` agrupado por `user_id` | FR-032 |
| Total de un listado filtrado | `Σ hours_hundredths` del resultado | FR-041 |
| Horas estimadas | `story_points × hours_per_story_point_hundredths` | FR-046 |
| Diferencia absoluta | `real − estimada` | FR-048 |
| Diferencia porcentual | `round(dif × 10000 / estimada)` en centésimas de punto, mitad lejos de cero | FR-049, RN-26 |

---

## 6. Reglas de dominio (funciones puras de `effort/domain`)

Cada una tiene pruebas unitarias obligatorias (Principio II).

### 6.1 `ResolveAnchor(open, lastClosed, now, grace)` — sprint del alta (FR-014, FR-016)

| `open` | `lastClosed` | Plazo de `lastClosed` | `outcome` | Resultado |
| --- | --- | --- | --- | --- |
| Sí (Planificado o Activo) | cualquiera | cualquiera | cualquiera | Sprint de `open` |
| No | Sí | vigente | `completed` | Sprint de `lastClosed` (US1-7) |
| No | Sí | vigente | `not_completed` | Sprint de `lastClosed` (US1-8) |
| No | Sí | vencido | `completed` | `ErrSprintFinalized` (US1-22) |
| No | Sí | vencido | `not_completed` | Sin sprint (RN-25) |
| No | No | — | — | Sin sprint (US1-2) |

`lastClosed` es el compromiso congelado con el `closed_at` más reciente (FR-014, "el de cierre más
reciente").

### 6.2 `GraceOpen(closedAt, now, grace)` y `EnsureEntryWritable(sprint, now, grace)` (FR-020, FR-021)

| Sprint asociado | Resultado |
| --- | --- |
| Ninguno | Escribible (RN-10) |
| Planificado o Activo | Escribible |
| Cerrado, `now < closed_at + grace` | Escribible (US3-9) |
| Cerrado, `now ≥ closed_at + grace` | `ErrEffortFinalized` (US3-10, US3-12, US3-13) |

### 6.3 `ValidateEntryDate(date, projectStart, today)` (FR-006)

`date > today` → `ErrEntryDateInFuture`; `date < projectStart` → `EntryDateBeforeProjectStartError`
con la fecha mínima. Ambos extremos son válidos (US1-13, US1-14).

### 6.4 `CheckDailyCap(alreadyLogged, requested)` (FR-005, FR-024)

`alreadyLogged + requested > 2400` → `DailyCapExceededError{AvailableHundredths: max(0, 2400 −
alreadyLogged)}`. En una modificación, `alreadyLogged` excluye el propio registro y se calcula sobre
la fecha resultante.

### 6.5 `Compare(storyPoints, factorHundredths, actualHundredths)` (FR-046 a FR-051)

Ver la tabla de casos en research.md, sección 12. `RoundPercentHundredths(numerator, denominator)` es
el redondeo único compartido (mitad lejos de cero, aritmética entera).

### 6.6 Autoría (FR-022, RN-11)

`EnsureAuthor(entry.UserID, actorID)` → `ErrNotEntryAuthor` si difieren; el propietario del proyecto
no es excepción.

---

## 7. Errores de dominio (`effort/domain/errors.go`)

| Error | HTTP | `code` | Mensaje para la persona |
| --- | --- | --- | --- |
| `ErrEntryNotFound` | 404 | — | "El registro de esfuerzo no existe." |
| `ErrEffortReadOnly` | 409 | `read_only` | "El proyecto está finalizado y su esfuerzo es de solo lectura." |
| `ErrSprintFinalized` | 409 | `effort_finalized` | "El sprint de la historia ya está cerrado y su esfuerzo es definitivo." |
| `ErrEffortFinalized` | 409 | `effort_finalized` | "Este esfuerzo pertenece a un sprint cerrado y ya es definitivo." |
| `DailyCapExceededError` | 409 | `daily_cap_exceeded` | "Superarías las 24 horas registradas para ese día. Te quedan N horas disponibles." |
| `ErrNotEntryAuthor` | 403 | — | "Solo quien registró este esfuerzo puede modificarlo o eliminarlo." |
| `ErrEntryDateInFuture` | 400 | — (`field: entryDate`) | "No se registran horas en una fecha futura." |
| `EntryDateBeforeProjectStartError` | 400 | — (`field: entryDate`) | "La fecha no puede ser anterior al inicio del proyecto (AAAA-MM-DD)." |
| `ErrInvalidFilter` | 400 | — (`field: <filtro>`) | "El filtro no corresponde al proyecto." |

---

## 8. Tipos de lectura (no persistidos; `effort/domain/repository.go`)

- `EntryAccess`: `EntryID`, `StoryID`, `ProjectID`, `AuthorID`, `ProjectFinished`
  (`RequireEffortEntryAccess`).
- `OpenCommitment`: `SprintID`. `ClosedCommitment`: `SprintID`, `ClosedAt`, `Outcome`.
- `AnchoredSprint`: `ID`, `Name`, `Status`, `ClosedAt` (para `EnsureEntryWritable` y las listas).
- `EntryView`: el registro con `Author{ID, FullName}`, `Story{ID, Title}` y `Sprint *AnchoredSprint`.
- `MemberEffort`: `UserID`, `FullName`, `HoursHundredths`.
- `StoryEffort`: `TotalHundredths`, `ByMember []MemberEffort`, `Entries []EntryView`.
- `EntryFilter`: `UserID *uuid`, `StoryID *uuid`, `SprintID *uuid`, `WithoutSprint bool`,
  `From *time.Time`, `To *time.Time`.
- `ComparisonRow`: `StoryID`, `Title`, `StoryPoints *int`, `ActualHundredths`.
- `EntryInput`: `EntryDate`, `Activity`, `HoursHundredths` (alta). `EntryPatch`: los mismos como
  punteros (modificación parcial).

---

## 9. `EffortRepository` (nueve operaciones, solo las que se usan; Principio III)

| Operación | Usada por |
| --- | --- |
| `FindAccessible(ctx, entryID, userID) (*EntryAccess, error)` | `RequireEffortEntryAccess` |
| `Create(ctx, storyID, authorID, input, now, location, grace) (*EntryView, error)` | US1 |
| `Update(ctx, entryID, actorID, patch, now, location, grace) (*EntryView, error)` | US3 |
| `Delete(ctx, entryID, actorID, now, grace) error` | US3 |
| `ListForStory(ctx, storyID) ([]EntryView, error)` | US2 |
| `ListForProject(ctx, projectID, filter) ([]EntryView, error)` | US4 |
| `ValidateFilter(ctx, projectID, filter) error` | US4 (FR-044) |
| `HasProjectEffort(ctx, projectID) (bool, error)` | US4 (`emptyReason`: `no_effort` frente a `no_matches`) |
| `ComparisonRows(ctx, projectID, storyID *uuid) (factorHundredths int, rows []ComparisonRow, err error)` | US5 |

Las escrituras evalúan las reglas puras de la sección 6 dentro de su transacción, con el orden de
bloqueos de research.md, sección 10. Ninguna operación devuelve `gorm.ErrRecordNotFound` ni `(nil,
nil)`.

---

## 10. Configuración

| Variable | Tipo | Defecto | Validación al arrancar |
| --- | --- | --- | --- |
| `EFFORT_GRACE_PERIOD_HOURS` | entero de horas | `48` si falta o está vacía | Entero `≥ 1`; si no, la aplicación no arranca |
| `APP_TIMEZONE` (002) | zona IANA | — | Sin cambios |

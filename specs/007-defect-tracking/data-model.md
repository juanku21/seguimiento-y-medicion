# Modelo de Datos (Fase 1): Gestión de Defectos

**Feature**: `specs/007-defect-tracking` | **Fecha**: 2026-10-25 | **Plan**: [plan.md](./plan.md)

Las decisiones que justifican este modelo están en [research.md](./research.md). Los nombres de
tablas, columnas, tipos y valores de dominio están en inglés (Principio VIII); los comentarios y los
mensajes, en español.

---

## 1. `Defect` → tabla `defects` (NUEVA)

Representa un problema detectado en el producto, con sus dos anclas de sprint (spec, Entidades
Clave).

| Campo Go | Columna | Tipo PostgreSQL | Nulo | Notas |
| --- | --- | --- | --- | --- |
| `ID` | `id` | `uuid` | no | PK, generado con `google/uuid` (FR-012) |
| `StoryID` | `story_id` | `uuid` | no | FK → `user_stories.id`, sin cascada. Corregible solo con el defecto `open`, y siempre dentro del mismo proyecto (FR-004, FR-038) |
| `Description` | `description` | `varchar(2000)` | no | Recortada; 1 a 2000 caracteres (FR-002) |
| `Severity` | `severity` | `varchar(8)` | no | `critical`, `high`, `medium` o `low` (FR-003) |
| `Status` | `status` | `varchar(16)` | no | `open`, `in_progress`, `resolved` o `discarded` (FR-017). Nace `open` (FR-011) |
| `DetectedSprintID` | `detected_sprint_id` | `uuid` | no | FK → `sprints.id`, sin cascada. Sprint `active` o `closed` del mismo proyecto (FR-006). Corregible solo con el defecto `open`; ninguna transición lo cambia (FR-027, FR-038) |
| `ResolvedSprintID` | `resolved_sprint_id` | `uuid` | **sí** | FK → `sprints.id`, sin cascada. No nulo **si y solo si** `status = resolved` (FR-023, FR-029, RN-23) |
| `CreatedByID` | `created_by_id` | `uuid` | no | FK → `users.id`, sin cascada. Titular de la sesión que lo registró (FR-014, FR-055). **Inmutable** |
| `UpdatedByID` | `updated_by_id` | `uuid` | no | FK → `users.id`, sin cascada. Titular de la última escritura de cualquier tipo (FR-056) |
| `StatusChangedAt` | `status_changed_at` | `timestamptz` | no | Momento del último cambio de estado; en el alta, el de la creación (FR-022) |
| `StatusChangedByID` | `status_changed_by_id` | `uuid` | no | FK → `users.id`, sin cascada. Autor del último cambio de estado; en el alta, quien lo registró (FR-022) |
| `CreatedAt` | `created_at` | `timestamptz` | no | Lo completa GORM (FR-014) |
| `UpdatedAt` | `updated_at` | `timestamptz` | no | Lo completa GORM (FR-014) |
| `Story` | — | — | — | Solo para que `AutoMigrate` cree la FK; nunca se precarga |
| `DetectedSprint`, `ResolvedSprint` | — | — | — | Ídem |
| `CreatedBy`, `UpdatedBy`, `StatusChangedBy` | — | — | — | Ídem |

**Índices**:

| Nombre | Columnas | Para qué |
| --- | --- | --- |
| `idx_defects_story_order` | `(story_id, created_at)` | Defectos de una historia (FR-043), pertenencia del listado del proyecto vía `user_stories` y filtro `storyId` |
| `idx_defects_detected_sprint` | `(detected_sprint_id)` | Filtro por sprint (FR-042) y conteo de detectados por sprint de la 008 |
| `idx_defects_resolved_sprint` | `(resolved_sprint_id)` | Filtro por sprint (FR-042) y conteo de resueltos por sprint de la 008 |

El listado del proyecto filtra por `user_stories.project_id`, que ya tiene el índice
`idx_user_stories_project_order` de la 003; el orden por severidad se resuelve en memoria del motor
sobre el subconjunto del proyecto, que no se pagina (research, sección 12).

**Claves foráneas sin `ON DELETE CASCADE`**: borrar una historia, un sprint o un usuario referenciados
falla en la base. La 003 ya lo impide antes con su puerto (sección 3) y traduce la violación de
`fk_defects_story` a su `409`; la 004 nunca puede intentarlo (RC-08).

**No hay** columna `project_id` (se deriva de la historia), ni de borrado lógico (no hay
eliminación), ni historial de estados o severidades (RC-10, Fuera de Alcance).

---

## 2. `UserStory` → tabla `user_stories` (TOCA, de la 003)

| Campo Go | Columna | Tipo | Nulo | Notas |
| --- | --- | --- | --- | --- |
| `EverHadDefects` | `ever_had_defects` | `boolean` | no | `DEFAULT false`. Pasa a `true` cuando la historia recibe su primer defecto —por alta o por revinculación— y **nunca vuelve a `false`**, aunque el defecto se revincule a otra historia o se descarte (RC-05, RN-19) |

Solo la escribe esta feature, con `UpdateColumn` (sin tocar `updated_at`), dentro de la transacción
del alta o de la modificación que cambia la historia. La lee el puerto `DefectRecords` de la 003. Es
el mismo patrón que `ever_committed` de la 004 y `ever_had_effort` de la 006.

---

## 3. Puertos y código de otras features que esta feature toca

| Puerto o método | Archivo | Cambio |
| --- | --- | --- |
| `backlogdomain.DefectRecords.HasDefects(storyID)` | `backlog/repository/defect_records.go` | Lee `user_stories.ever_had_defects`; el constructor recibe `db` |
| `StoryRepository.Delete` | `backlog/repository/story_repository.go` | Traduce la violación de `fk_defects_story` a `DeletionBlockedError{defects}` (research, sección 10) |

---

## 4. Datos que esta feature lee y no escribe

| Tabla (feature) | Columnas leídas | Para qué |
| --- | --- | --- |
| `projects` (002) | `id`, `status` (y bloqueo `FOR SHARE`) | Solo lectura del proyecto Finalizado |
| `project_members` (002) | `project_id`, `user_id` | Acceso |
| `users` (001) | `id`, `full_name` | Nombres de autor, de último modificador y de último cambio de estado. **Nunca** `password_hash` |
| `user_stories` (003) | `id`, `project_id`, `title` | Historia relacionada y su pertenencia al proyecto |
| `sprints` (004) | `id`, `project_id`, `name`, `status`, `start_date` (y bloqueo `FOR KEY SHARE`) | Anclas, estado admitido, orden entre detección y resolución |

---

## 5. Valores de dominio

| Tipo Go | Valores | Etiqueta en la interfaz |
| --- | --- | --- |
| `DefectSeverity` | `critical`, `high`, `medium`, `low` | Crítica, Alta, Media, Baja |
| `DefectStatus` | `open`, `in_progress`, `resolved`, `discarded` | Abierto, En progreso, Resuelto, Descartado |

Rango de severidad para el orden: `critical` 0, `high` 1, `medium` 2, `low` 3 (FR-048). Orden
completo de todo listado: rango ascendente, `created_at DESC`, `id DESC`.

---

## 6. Máquina de estados (FR-017 a FR-020, FR-028 a FR-032, FR-039)

```text
            ┌──────────────── reabrir ────────────────┐
            ▼                                          │
        ┌────────┐  En progreso  ┌─────────────┐  resolver   ┌──────────┐
        │  open  │ ────────────▶ │ in_progress │ ──────────▶ │ resolved │
        └────────┘               └─────────────┘  (sprint)   └──────────┘
            │                          │
            │ descartar                │ descartar
            ▼                          ▼
        ┌──────────────────────────────────┐
        │            discarded             │   terminal: sin salidas
        └──────────────────────────────────┘
```

| Origen \ Destino | `open` | `in_progress` | `resolved` | `discarded` |
| --- | --- | --- | --- | --- |
| `open` | ✗ | ✓ | ✗ (FR-019) | ✓ |
| `in_progress` | ✗ (FR-020) | ✗ | ✓ exige sprint | ✓ |
| `resolved` | ✓ reabre | ✗ | ✗ | ✗ (FR-039) |
| `discarded` | ✗ | ✗ | ✗ | ✗ (RN-22) |

5 combinaciones válidas y 11 inválidas (SC-005).

**Efecto sobre los campos** (`PlanTransition`, research, sección 8):

| Transición | `resolved_sprint_id` | `detected_sprint_id`, `story_id` | `status_changed_*`, `updated_by_id` |
| --- | --- | --- | --- |
| `open → in_progress` | sigue `NULL` | sin cambio | quien actúa, ahora |
| `in_progress → resolved` | el sprint indicado | sin cambio | quien actúa, ahora |
| `resolved → open` | `NULL` (FR-029) | sin cambio | quien actúa, ahora |
| `open → discarded` | sigue `NULL` (RN-23) | sin cambio | quien actúa, ahora |
| `in_progress → discarded` | sigue `NULL` (RN-23) | sin cambio | quien actúa, ahora |

---

## 7. Reglas de dominio (funciones puras de `defects/domain`)

Cada una tiene pruebas unitarias obligatorias (Principio II).

### 7.1 `PlanTransition(current, to, detected, resolution)` (FR-018 a FR-020, FR-023 a FR-025, FR-029, FR-039)

| Condición | Resultado |
| --- | --- |
| `to` no admitido desde `current` | `InvalidTransitionError{Current, Requested, Allowed}` |
| `to = resolved`, `resolution = nil` | `ErrResolutionSprintRequired` |
| `to = resolved`, `resolution.Status = planned` | `ErrResolutionSprintPlanned` |
| `to = resolved`, `resolution.StartDate < detected.StartDate` | `ErrResolutionBeforeDetection` |
| `to = resolved`, todo válido | `&resolution.ID` (mismo sprint que detección incluido; FR-025) |
| `to ∈ {open, in_progress, discarded}`, admitido | `nil` |

`AllowedTransitions(current)` devuelve la lista de destinos admitidos, usada en el `409`.

### 7.2 `EnsureSprintUsableForDetection(status)` (FR-006, FR-007)

`planned` → `ErrDetectionSprintPlanned`; `active` y `closed` → `nil`.

### 7.3 `EnsureDefectEditable(status)` (FR-036, RN-22)

`discarded` → `ErrDefectDiscarded`; cualquier otro → `nil`.

### 7.4 `EnsureLinksCorrectable(status)` (FR-038, RN-16)

Distinto de `open` → `ErrLinksFrozen`; `open` → `nil`. Se evalúa solo para los vínculos cuyo valor
cambia (research, sección 11).

### 7.5 `SeverityRank(severity)` y validadores de valor

`SeverityRank` da el rango de la sección 5. `DefectSeverity.Valid()` y `DefectStatus.Valid()` sirven a
la validación del cuerpo y de los filtros (FR-003, FR-046).

---

## 8. Errores de dominio (`defects/domain/errors.go`)

| Error | HTTP | `code` / `field` | Mensaje para la persona |
| --- | --- | --- | --- |
| `ErrDefectNotFound` | 404 | — | "El defecto no existe." |
| `backlogdomain.ErrStoryNotFound` (cuerpo) | 404 | — | "La historia no existe." (cuerpo de la 003) |
| `sprintsdomain.ErrSprintNotFound` (cuerpo) | 404 | — | "El sprint no existe." (cuerpo de la 004) |
| `ErrDefectsReadOnly` | 409 | `read_only` | "El proyecto está finalizado y sus defectos son de solo lectura." |
| `ErrNoDetectionSprint` | 409 | `no_detection_sprint` | "Todavía no hay ningún sprint Activo ni Cerrado que pueda ser sprint de detección." |
| `InvalidTransitionError` | 409 | `invalid_transition` | Según el caso: "Un defecto Abierto primero debe pasar a En progreso." / "Para volver a Abierto, resolvé el defecto y después reabrilo." / "Solo se reabre un defecto Resuelto." / "Solo se descarta un defecto Abierto o En progreso; para descartarlo, primero reabrilo." / "El defecto está Descartado, que es un estado definitivo." / genérico con las transiciones permitidas |
| `ErrDefectDiscarded` | 409 | `defect_discarded` | "El defecto está Descartado y es de solo lectura." |
| `ErrLinksFrozen` | 409 | `links_frozen` | "La historia relacionada y el sprint de detección solo se corrigen mientras el defecto está Abierto." |
| `ErrDetectionSprintPlanned` | 400 | `field: detectedSprintId` | "Un defecto solo se detecta en un sprint Activo o Cerrado." |
| `ErrResolutionSprintRequired` | 400 | `field: resolvedSprintId` | "El sprint de resolución es obligatorio para resolver." |
| `ErrResolutionSprintPlanned` | 400 | `field: resolvedSprintId` | "Un defecto solo se resuelve en un sprint Activo o Cerrado." |
| `ErrResolutionBeforeDetection` | 400 | `field: resolvedSprintId` | "El sprint de resolución no puede empezar antes que el de detección." |
| `InvalidFilterError{Field}` | 400 | `field: storyId` o `sprintId` | "El filtro no corresponde al proyecto." |

`InvalidTransitionError` lleva `Current DefectStatus`, `Requested DefectStatus` (para que delivery elija el mensaje del caso) y `Allowed []DefectStatus`; el `409` expone `Current` y `Allowed`
como `currentStatus` y `allowedStatuses`. `ErrLinksFrozen` se informa con el `currentStatus` del
defecto releído.

---

## 9. Tipos de lectura (no persistidos; `defects/domain/repository.go`)

- `DefectAccess`: `DefectID`, `ProjectID`, `ProjectFinished` (`RequireDefectAccess`).
- `SprintAnchor`: `ID`, `Status`, `StartDate` (para `PlanTransition`).
- `UserRef`: `ID`, `FullName`. `StoryRef`: `ID`, `Title`. `SprintRef`: `ID`, `Name`, `Status`,
  `StartDate`.
- `DefectView`: el defecto con `Story StoryRef`, `DetectedSprint SprintRef`,
  `ResolvedSprint *SprintRef`, `CreatedBy UserRef`, `UpdatedBy UserRef`,
  `StatusChangedBy UserRef`. Los campos de relación de `Defect` nunca se precargan.
- `DefectInput` (alta): `StoryID`, `DetectedSprintID`, `Description`, `Severity`.
- `DefectPatch` (modificación parcial): los mismos como punteros; `nil` = no cambia.
- `StatusChange`: `To DefectStatus`, `ResolvedSprintID *uuid.UUID`.
- `DefectFilter`: `Statuses []DefectStatus`, `Severities []DefectSeverity`, `StoryID *uuid.UUID`,
  `SprintID *uuid.UUID`. Listas vacías y punteros `nil` no filtran.

---

## 10. `DefectRepository` (ocho operaciones, solo las que se usan; Principio III)

| Operación | Usada por |
| --- | --- |
| `FindAccessible(ctx, defectID, userID) (*DefectAccess, error)` | `RequireDefectAccess` |
| `Create(ctx, projectID, actorID, input, now) (*DefectView, error)` | US1 |
| `ChangeStatus(ctx, defectID, actorID, change, now) (*DefectView, error)` | US2, US5, US6 |
| `Update(ctx, defectID, actorID, patch) (*DefectView, error)` | US4 |
| `FindView(ctx, defectID) (*DefectView, error)` | Detalle (FR-049) |
| `ListForProject(ctx, projectID, filter) ([]DefectView, error)` | US3, US6 |
| `ValidateFilter(ctx, projectID, filter) error` | US3 (FR-047) |
| `HasProjectDefects(ctx, projectID) (bool, error)` | US3 (`emptyReason`: `no_defects` frente a `no_matches`) |

Los defectos de una historia (US3-2, US3-7) usan `ListForProject` con `DefectFilter{StoryID: &id}` y
el proyecto del `StoryAccess` de la 003; con una historia de ese proyecto no hace falta
`ValidateFilter`, y la lista vacía es siempre `no_defects`.

Las escrituras evalúan las reglas puras de la sección 7 dentro de su transacción, con el orden de
bloqueos de research.md, sección 10. Ninguna operación devuelve `gorm.ErrRecordNotFound` ni
`(nil, nil)`: si el defecto no existe o es ajeno, devuelve `ErrDefectNotFound`.

---

## 11. Configuración

Sin variables de entorno nuevas.

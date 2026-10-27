# Modelo de Datos (Fase 1): Cálculo de Métricas

**Feature**: `specs/008-metrics-calculation` | **Fecha**: 2026-10-26 | **Plan**: [plan.md](./plan.md)

Las decisiones que justifican este modelo están en [research.md](./research.md). Los nombres de
tipos, campos y valores de dominio están en inglés (Principio VIII); los comentarios y los mensajes,
en español.

**Esta feature no crea tablas ni columnas** (RC-11). Todo lo que sigue son tipos de lectura que viven
en memoria durante una consulta: los datos crudos que entrega el repositorio (sección 3), los valores
que produce el dominio (secciones 4 a 8) y las reglas que los relacionan.

---

## 1. Datos que esta feature lee y no escribe

Todas las lecturas de una consulta ocurren dentro de una transacción `REPEATABLE READ` de solo
lectura (research, sección 4).

| Tabla (feature) | Columnas leídas | Para qué |
| --- | --- | --- |
| `projects` (002) | `id`, `hours_per_story_point_hundredths` | Factor vigente del Activo y del Planificado |
| `project_members` (002) | — (lo lee `RequireMembership`) | Acceso |
| `user_stories` (003) | `id`, `project_id`, `title`, `story_points`, `status` | Puntos vigentes y completadas del Activo; contador sin estimar; pertenencia al proyecto del esfuerzo y de los defectos |
| `sprints` (004) | `id`, `project_id`, `name`, `status`, `start_date`, `end_date`, `closed_at`, `hours_per_story_point_hundredths` | Estado, orden, ventana de velocidad, plazo de gracia y factor congelado |
| `sprint_stories` (004) | `sprint_id`, `story_id`, `story_points_at_close`, `outcome` | Instantánea del Cerrado y compromisos abiertos (`outcome IS NULL`) del Activo y del Planificado |
| `effort_entries` (006) | `story_id`, `sprint_id`, `hours_hundredths` | Horas reales por sprint, fuera de sprint y en Planificados |
| `defects` (007) | `story_id`, `status`, `severity`, `detected_sprint_id`, `resolved_sprint_id` | Conteos por sprint y del proyecto |
| `users` (001) | — | Ninguna: las métricas no informan personas (Fuera de Alcance) |

Índices que ya existen y sirven a estas lecturas: `idx_sprints_project_start`,
`idx_sprint_stories_sprint_story`, `idx_user_stories_project_order`, `idx_effort_entries_sprint`,
`idx_defects_detected_sprint` e `idx_defects_resolved_sprint`. No hace falta ninguno nuevo.

---

## 2. Valores de dominio

| Tipo Go | Valores | Significado |
| --- | --- | --- |
| `ValueStatus` | `available`, `not_calculable`, `not_applicable` | Hay valor; denominador 0 (FR-030); no tiene sentido para el estado del sprint (FR-015) |
| `Finality` | `definitive`, `provisional`, `partial` | Estado de valor de una métrica de sprint o del acumulado (FR-014, FR-019, FR-023) |
| `WarningKind` | `planning`, `data_inconsistency` | Las dos clases de FR-038 |
| `WarningCode` | `unestimated_backlog_stories`, `unestimated_story_in_sprint`, `missing_frozen_factor`, `missing_close_time` | Research, sección 13 |
| `MetricKind` | `planned_story_points`, `completed_story_points`, `estimated_hours`, `actual_hours`, `deviation_hours`, `deviation_percent`, `story_completion_percent`, `defects_detected`, `defects_resolved` | Las nueve métricas de un sprint, para decidir su `Finality` |

Los estados del sprint (`sprintsdomain.SprintStatus`), de la historia (`backlogdomain.StoryStatus`),
el resultado congelado (`sprintsdomain.StoryOutcome`) y la severidad y el estado del defecto
(`defectsdomain.DefectSeverity`, `defectsdomain.DefectStatus`) se importan de sus features; no se
repiten.

---

## 3. Datos crudos que entrega el repositorio

### `SprintData`

| Campo | Tipo | Notas |
| --- | --- | --- |
| `ID`, `Name` | `uuid.UUID`, `string` | Identifican el sprint en las respuestas y en las series |
| `Status` | `SprintStatus` | `planned`, `active` o `closed` |
| `StartDate`, `EndDate` | `time.Time` | Medianoche UTC. `StartDate` ordena las series |
| `ClosedAt` | `*time.Time` | Solo en Cerrados. Ordena la ventana de velocidad y corre el plazo de gracia |
| `FrozenFactorHundredths` | `*int` | Solo en Cerrados (factor copiado al cerrar) |
| `Stories` | `[]SprintStoryData` | Instantánea en el Cerrado; compromisos abiertos en el resto (research, sección 8) |
| `ActualHundredths` | `int` | Suma de `effort_entries.hours_hundredths` anclados al sprint |
| `DefectsDetected`, `DefectsResolved` | `int` | Conteos de la sección 8 del research |

### `SprintStoryData`

| Campo | Tipo | Notas |
| --- | --- | --- |
| `StoryID`, `Title` | `uuid.UUID`, `string` | Identifican la historia en una advertencia de inconsistencia |
| `StoryPoints` | `*int` | Congelados en el Cerrado, vigentes en el resto. `nil` es dato roto (FR-037) |
| `Completed` | `bool` | `outcome = completed` en el Cerrado; `status = completed` en el Activo; siempre `false` en el Planificado |

### `ProjectData`

| Campo | Tipo | Notas |
| --- | --- | --- |
| `ProjectID` | `uuid.UUID` | — |
| `FactorHundredths` | `int` | Factor vigente del proyecto |
| `Sprints` | `[]SprintData` | **Todos** los sprints del proyecto, en orden `start_date ASC, id ASC` |
| `OutsideSprintsHundredths` | `int` | Esfuerzo con `sprint_id IS NULL` (FR-029) |
| `UnestimatedBacklogStories` | `int` | Historias del proyecto con `story_points IS NULL` (FR-036) |
| `DefectCounts` | `[]DefectCount` | Una fila por `(status, severity)` presente, excluido `discarded` |

`DefectCount` = `{Status DefectStatus; Severity DefectSeverity; Count int}`.

El esfuerzo en sprints Planificados no es un campo: es la suma de `ActualHundredths` de los sprints
`planned` de `Sprints`, y lo calcula el dominio (research, sección 11).

---

## 4. `Metric`: un valor con su estado

| Campo | Tipo | Notas |
| --- | --- | --- |
| `Status` | `ValueStatus` | Siempre presente |
| `Value` | `int` | Solo significativo con `available`. Unidad según el campo: Story Points, o centésimas de hora o de punto porcentual |
| `Finality` | `*Finality` | Solo en métricas de sprint y en puntos de serie; `nil` en el Planificado |

Constructores puros: `Available(v)`, `NotCalculable()`, `NotApplicable()`, y
`Percent(numerator, denominator)`, que devuelve `NotCalculable()` si el denominador es 0 y
`Available(RoundPercentHundredths(numerator, denominator))` si no.

---

## 5. `SprintMetrics`: el resultado de un sprint (FR-004 a FR-015, FR-019)

| Campo | Fórmula | Cerrado | Activo | Planificado |
| --- | --- | --- | --- | --- |
| `PlannedStoryPoints` | Σ `StoryPoints` (nil = 0) | `definitive` | `partial` | `available`, `nil` |
| `CompletedStoryPoints` | Σ `StoryPoints` con `Completed` | `definitive` | `partial` | `not_applicable` |
| `EstimatedHours` | `PlannedStoryPoints × factor` | `definitive`, factor congelado | `partial`, factor vigente | `available`, factor vigente |
| `ActualHours` | `ActualHundredths` | gracia vigente: `provisional`; vencida: `definitive` | `partial` | `not_applicable` |
| `DeviationHours` | `ActualHours − EstimatedHours` | ídem `ActualHours` | `partial` | `not_applicable` |
| `DeviationPercent` | `Percent(DeviationHours, EstimatedHours)` | ídem `ActualHours`; `not_calculable` si estimadas = 0 | `partial` | `not_applicable` |
| `StoryCompletionPercent` | `Percent(completadas, planificadas)` por cantidad de historias | `definitive`; `not_calculable` sin historias | `partial` | `not_applicable` |
| `DefectsDetected` | `DefectsDetected` | `provisional` siempre | `partial` | `not_applicable` |
| `DefectsResolved` | `DefectsResolved` | `provisional` siempre | `partial` | `not_applicable` |

Junto a las métricas viajan el sprint (`ID`, `Name`, `Status`, fechas, `ClosedAt`) y la lista de
advertencias de inconsistencia del sprint.

Funciones puras:

- `CalculateSprint(sprint SprintData, currentFactorHundredths int, now time.Time, grace time.Duration) (SprintMetrics, []Warning)`.
- `SprintFinality(status SprintStatus, kind MetricKind, graceOpen bool) *Finality`: la tabla de arriba.
- La gracia se decide con `effortdomain.GraceOpen(*ClosedAt, now, grace)`; con `ClosedAt == nil` se
  toma como vencida y se emite `missing_close_time`.

**Invariantes** (FR-003, SC-019): `CompletedStoryPoints ≤ PlannedStoryPoints`; `DeviationHours =
ActualHours − EstimatedHours` en la misma respuesta; historias completadas ≤ planificadas.

---

## 6. Métricas del proyecto (FR-020 a FR-029, FR-036)

### `Velocity`

| Campo | Notas |
| --- | --- |
| `Value` | `Metric` sin `Finality`: `AverageHundredths(Σ completados de la ventana, tamaño de la ventana)`; `not_calculable` sin Cerrados |
| `SprintsConsidered` | Tamaño de la ventana: 0 a 3 |
| `Series` | Un punto por sprint Cerrado, en orden `start_date ASC, id ASC`: `{SprintID, SprintName, ClosedAt, CompletedStoryPoints int}` |

Ventana: Cerrados ordenados por `ClosedAt DESC NULLS LAST, ID DESC`, los 3 primeros (FR-021).

`AverageHundredths(sum, count) = RoundPercentHundredths(sum, 100 × count)` = `sum × 100 / count`
redondeado una sola vez.

### `Cumulative`

| Campo | Fórmula |
| --- | --- |
| `Finality` | `partial` si algún sprint incluido está Activo; `definitive` si no (RN-27) |
| `IncludesActiveSprint` | `bool` |
| `PlannedStoryPoints`, `CompletedStoryPoints` | Σ sobre Cerrados + Activo |
| `EstimatedHours`, `ActualHours` | Σ sobre Cerrados + Activo (cada sprint con su factor) |
| `DeviationHours` | `ActualHours − EstimatedHours` sobre los totales |
| `DeviationPercent` | `Percent(DeviationHours, EstimatedHours)` sobre los totales (FR-024) |
| `StoryCompletionPercent` | `Percent(Σ completadas, Σ planificadas)` sobre los totales |

Las cifras son `Metric` sin `Finality`: la marca es del bloque (FR-023).

### `ProjectMetrics`

| Campo | Notas |
| --- | --- |
| `Velocity` | Arriba |
| `Cumulative` | Arriba |
| `EffortOutsideSprintsHundredths` | `ProjectData.OutsideSprintsHundredths`, siempre presente |
| `EffortInPlannedSprintsHundredths` | Σ `ActualHundredths` de los Planificados, siempre presente (research, sección 11) |
| `UnestimatedBacklogStories` | Siempre presente, 0 incluido |
| `Defects` | Sección 7 |
| `Warnings` | Planificación + inconsistencias de los sprints Cerrados y del Activo |

Función pura: `CalculateProject(data ProjectData, now time.Time, grace time.Duration) ProjectMetrics`.

**Invariante** (SC-016, ampliada por la decisión del 2026-10-26): `Cumulative.ActualHours +
EffortOutsideSprints + EffortInPlannedSprints` = total de `hours_hundredths` de las historias del
proyecto.

---

## 7. Defectos del proyecto (FR-026 a FR-028)

`ProjectDefects` = `{Detected, Resolved, Open DefectTally}`, con
`DefectTally` = `{Total int; BySeverity map[DefectSeverity]int}` con las cuatro severidades siempre
presentes.

| Conteo | Estados que suma |
| --- | --- |
| `Detected` | `open`, `in_progress`, `resolved` |
| `Resolved` | `resolved` |
| `Open` | `open`, `in_progress` |

`discarded` no llega desde el repositorio y, por defensa, el dominio lo ignora si llegara (FR-018).

**Invariantes** (SC-010): en cada conteo, Σ `BySeverity` = `Total`; `Detected.Total = Resolved.Total +
Open.Total`.

Función pura: `TallyDefects(counts []DefectCount) ProjectDefects`.

---

## 8. Series del proyecto (FR-022)

`ProjectSeries` tiene seis listas de `SeriesPoint` con los mismos sprints en el mismo orden
(`start_date ASC, id ASC`), solo Cerrados y Activo:

| Serie | Valor del punto |
| --- | --- |
| `PlannedStoryPoints` | `SprintMetrics.PlannedStoryPoints` |
| `CompletedStoryPoints` | `SprintMetrics.CompletedStoryPoints` (velocidad por sprint) |
| `EstimatedHours` | `SprintMetrics.EstimatedHours` |
| `ActualHours` | `SprintMetrics.ActualHours` |
| `DefectsDetected` | `SprintMetrics.DefectsDetected` |
| `DefectsResolved` | `SprintMetrics.DefectsResolved` |

`SeriesPoint` = `{SprintID, SprintName, SprintStatus, Value Metric}`; el `Metric` conserva la
`Finality` de su sprint. Junto a las series viajan las advertencias de inconsistencia de los sprints
incluidos.

Función pura: `BuildSeries(data ProjectData, now time.Time, grace time.Duration) (ProjectSeries, []Warning)`,
que reutiliza `CalculateSprint` para cada sprint, de modo que una serie y el detalle de su sprint
nunca difieren.

---

## 9. `Warning` (FR-038)

| Campo | Tipo | Notas |
| --- | --- | --- |
| `Kind` | `WarningKind` | `planning` o `data_inconsistency` |
| `Code` | `WarningCode` | Research, sección 13 |
| `SprintID` | `*uuid.UUID` | En las tres de inconsistencia |
| `StoryID` | `*uuid.UUID` | Solo en `unestimated_story_in_sprint` |
| `Count` | `*int` | Solo en `unestimated_backlog_stories` |

El mensaje en español lo arma `delivery` a partir del código; el dominio no lleva textos de interfaz.

---

## 10. Interfaces del dominio

### `MetricsRepository`

Declara solo las tres lecturas que usa el servicio (Principio III):

| Operación | Devuelve | Notas |
| --- | --- | --- |
| `LoadProject(ctx, projectID)` | `*ProjectData` | Todas las lecturas en una transacción `REPEATABLE READ` de solo lectura |
| `LoadSprint(ctx, projectID, sprintID)` | `*SprintData`, factor vigente | `ErrSprintNotFound` si el sprint no existe o es de otro proyecto |
| `LoadActiveSprint(ctx, projectID)` | `*SprintData` o `nil`, factor vigente | `nil` sin error si no hay Activo |

Ninguna operación devuelve `gorm.ErrRecordNotFound`.

### Errores de dominio

| Error | Traducción HTTP |
| --- | --- |
| `ErrSprintNotFound` | `404` `{"error":"El sprint no existe."}` |

El proyecto inexistente o ajeno no llega al servicio: lo resuelve `RequireMembership`.

---

## 11. Código de otras features que esta feature toca

| Archivo | Cambio |
| --- | --- |
| `platform/httpserver/hundredths.go` | **NUEVO**: el tipo `Hundredths` y su `MarshalJSON`, movidos sin cambios desde `effort/delivery/dto.go` |
| `effort/delivery/dto.go` | El tipo pasa a ser `type Hundredths = httpserver.Hundredths`; ningún uso cambia |
| `cmd/app/main.go` | Compone el módulo `metrics` sobre `projectGroup` con el plazo de gracia y el reloj |

`effortdomain.RoundPercentHundredths` y `effortdomain.GraceOpen` se **importan** desde
`metrics/domain`; no se modifican.

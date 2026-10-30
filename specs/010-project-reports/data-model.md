# Modelo de Datos (Fase 1): Reportes de Proyecto y de Sprint

**Feature**: `specs/010-project-reports` | **Fecha**: 2026-10-30 | **Plan**: [plan.md](./plan.md)

La feature persiste **una sola entidad**: el registro de generación. El reporte es un documento
derivado que se arma en cada pedido y no se guarda (FR-034). Las secciones 2 a 4 describen los tipos
no persistidos del dominio y del frontend. La forma JSON exacta está en
[contracts/openapi.yaml](./contracts/openapi.yaml).

---

## 1. `ReportGeneration` (tabla `report_generations`)

Rastro de auditoría de cada reporte entregado: quién, cuándo y con qué alcance (FR-034, RN-24,
SC-018). Cumple el modelo obligatorio de la constitución: clave UUID y fechas de creación y de
actualización.

| Campo Go | Columna | Tipo | Restricciones |
| --- | --- | --- | --- |
| `ID` | `id` | `uuid` | PK; la genera la aplicación con `google/uuid` |
| `ProjectID` | `project_id` | `uuid` | `NOT NULL`; FK a `projects.id`, sin cascada; índice `idx_report_generations_project_generated (project_id, generated_at)` |
| `SprintID` | `sprint_id` | `uuid` | Nulo en el alcance `project`; FK a `sprints.id`, sin cascada |
| `Scope` | `scope` | `varchar(8)` | `NOT NULL`; `sprint` o `project` (tipo `ReportScope`, como texto y no como enum de PostgreSQL) |
| `GeneratedByID` | `generated_by_id` | `uuid` | `NOT NULL`; FK a `users.id`, sin cascada |
| `GeneratedAt` | `generated_at` | `timestamptz` | `NOT NULL`; el mismo instante que el `generatedAt` del encabezado |
| `CreatedAt` | `created_at` | `timestamptz` | `NOT NULL`; lo completa GORM |
| `UpdatedAt` | `updated_at` | `timestamptz` | `NOT NULL`; lo completa GORM. El registro nunca se modifica, pero la constitución exige la columna |

Los campos `Project`, `Sprint` y `GeneratedBy` existen solo para que `AutoMigrate` cree las claves
foráneas, como en el resto de los modelos: no son columnas y nunca se precargan.

**Invariantes**:

- `scope = 'sprint'` ⇔ `sprint_id IS NOT NULL`. La aplica el servicio, que es el único que inserta. Se
  verifica en las pruebas de integración de los dos alcances.
- Solo se inserta después de armar un reporte que se va a responder con `200`. Un `404` o un `409` no
  dejan registro.
- Nunca se actualiza ni se elimina. No hay operación de lectura expuesta (fuera de alcance).
- Las claves foráneas son seguras sin cascada: la 002 no elimina proyectos, la 001 no elimina
  usuarios, y la 004 solo elimina sprints Planificados, que no se pueden reportar (RN-04).

**Cambios en la plataforma** (**TOCA**):

- `platform/database/postgres.go`: `&reportsdomain.ReportGeneration{}` al final de `AutoMigrate`.
- `platform/testsupport/database.go`: `report_generations` en el `TRUNCATE` de la base de test.

**Interfaz de persistencia** (Principio III, solo las operaciones que se usan):

```text
GenerationRepository
  Save(ctx, ReportGeneration) error
```

---

## 2. Tipos del dominio `reports` (no persistidos)

### 2.1 Alcance, estado de sección y errores

| Tipo | Valores | Uso |
| --- | --- | --- |
| `ReportScope` | `sprint`, `project` | Alcance del reporte y columna `scope` |
| `SectionState` | `complete`, `empty`, `unavailable` | Estado de cada sección y de cada grupo (FR-023, FR-024). Un grupo solo usa `complete` y `empty` |
| `StoryResult` | `pending`, `in_progress`, `completed`, `not_completed` | Estado de una historia en un grupo. Los tres primeros son el estado vigente; `completed` y `not_completed` son también el resultado congelado del Cerrado |
| `PokerState` | `agreed`, `no_agreement`, `none` | Estado de la estimación colaborativa (research, sección 5) |

| Error | Cuándo | Traducción HTTP |
| --- | --- | --- |
| `ErrSprintNotFound` | Sprint inexistente o de otro proyecto | `404` con el cuerpo de la 004 |
| `ErrSprintNotStarted` | Sprint Planificado (RN-04, FR-004) | `409` |

### 2.2 Reporte

```text
Report
  Scope        ReportScope
  Partial      bool                    // IsPartial (RN-03, RN-06)
  Header       Header
  Stories      StoriesSection
  Estimations  EstimationsSection
  Effort       EffortSection
  Metrics      MetricsSection
  Defects      DefectsSection

Header
  Project      { ID, Name, Status }    // estado: planned, in_progress, finished (002)
  Sprint       *SprintRef              // nil en el alcance project
  Period       *Period                 // ReportPeriod; nil si no hay sprints reportables
  GeneratedAt  time.Time               // instante único de la generación
  GeneratedBy  { ID, FullName }        // titular de la sesión (FR-041)

SprintRef      { ID, Name, Status, StartDate, EndDate, ClosedAt* }
Period         { StartDate, EndDate }  // fechas date a medianoche UTC
```

### 2.3 Secciones

```text
StoriesSection      { State, Groups []StoryGroup }
StoryGroup          { Sprint *SprintRef /* nil = no planificadas */, State, Stories []StoryLine }
StoryLine           { StoryID, Title, Priority, StoryPoints *int, Status StoryResult }

EstimationsSection  { State, Groups []EstimationGroup }
EstimationGroup     { Sprint *SprintRef, State, Items []EstimationLine }
EstimationLine      { StoryID, Title, StoryPoints *int, Poker PokerState, Session *PokerSummary }
PokerSummary        { Rounds int, AgreedStoryPoints int, FinishedAt time.Time }   // solo con agreed

EffortSection       { State, Groups []EffortGroup, ByMember []MemberEffort, TotalHundredths int }
EffortGroup         { Sprint SprintRef, State, Stories []StoryEffort, TotalHundredths int }
StoryEffort         { StoryID, Title, Hundredths int }
MemberEffort        { UserID, FullName, Hundredths int }

MetricsSection      { State, Sprint *metricsdomain.SprintMetrics + warnings   // alcance sprint
                             Project *metricsdomain.ProjectMetrics }          // alcance project

DefectsSection      { State, Defects []DefectLine, Totals *DefectTotals }
DefectLine          { DefectID, Description, Severity, Status, Story { ID, Title },
                      DetectedSprint { ID, Name }, ResolvedSprint *{ ID, Name } }
DefectTotals        { Detected int, Resolved int }   // de la 008 (research, sección 7)
```

Con `State = unavailable`, la sección no lleva datos: listas vacías y punteros nulos. En la API se
serializan como `null` (contrato).

### 2.4 Reglas de negocio (funciones puras del dominio, con prueba unitaria obligatoria)

| Función | Regla | Referencias |
| --- | --- | --- |
| `EnsureReportable(status)` | `planned` → `ErrSprintNotStarted`; `active` y `closed` → nil | FR-004, RN-04, SC-007 |
| `IsPartial(scope, sprints)` | `true` si alguno de los sprints del alcance está `active` | FR-005, FR-006, RN-03, RN-06, SC-006 |
| `ReportableSprints(sprints)` | Filtra `active` y `closed` y conserva el orden `start_date ASC, id ASC` | FR-006, FR-007, FR-011, RN-05, RN-07 |
| `ReportPeriod(sprints)` | Menor inicio y mayor fin; nil sin sprints | Supuestos de la spec |
| `SortStories(lines)` | Prioridad `high` > `medium` > `low`, después título y después `story_id` | SC-017 |
| `SelectEstimation(sessions, cutoff)` | Clasifica en `agreed`, `no_agreement` o `none` y elige la última `finished` dentro del corte | FR-013 a FR-015, RN-09, RN-10 |
| `StoryTitle(snapshot, current)` | Título de la instantánea del sprint si la historia está en ella; si no, el vigente | FR-021, SC-017 |
| `StoriesState(groups)`, `EffortState(...)`, `DefectsState(...)` | `empty` según la tabla de research, sección 8 | FR-023, RN-11, SC-008 |

### 2.5 Puertos del dominio

```text
ReportRepository                                      // lecturas; cada una independiente (research, sección 8)
  LoadHeader(ctx, projectID, userID) (ProjectInfo, UserInfo, error)
  LoadSprint(ctx, projectID, sprintID) (SprintRef, error)        // ErrSprintNotFound
  ListSprints(ctx, projectID) ([]SprintRef, error)               // todos, en orden start_date, id
  LoadSprintStories(ctx, sprint SprintRef) ([]StoryLine, error)  // instantánea o compromisos abiertos
  LoadUnplannedStories(ctx, projectID) ([]StoryLine, error)     // sin compromisos en sprints active o closed (FR-036)
  LoadEstimationSessions(ctx, storyIDs) (map[StoryID][]SessionRecord, error)
  LoadEffort(ctx, sprintIDs) ([]EffortRow, error)                // GROUP BY sprint, historia y usuario
  LoadDefects(ctx, projectID, sprintID *uuid) ([]DefectLine, error)

GenerationRepository
  Save(ctx, ReportGeneration) error

MetricsSource                                         // lo cumple *metricsservice.MetricsService
  SprintMetrics(ctx, projectID, sprintID) (SprintMetrics, []Warning, error)
  ProjectMetrics(ctx, projectID) (ProjectMetrics, error)
```

`SessionRecord` es `{ ID, Status, CreatedAt, ClosedAt*, AgreedStoryPoints*, Rounds }`, con `Rounds`
contado en la misma consulta. `EffortRow` es `{ SprintID, StoryID, CurrentTitle, UserID, FullName,
Hundredths }`. El título de la instantánea lo resuelve el servicio con `StoryTitle`, cruzando con las
historias de cada sprint.

---

## 3. Tipos del frontend

### 3.1 Respuesta (`frontend/src/types/report.ts`)

Espejo del contrato, con los nombres de la API. La sección de métricas reutiliza `SprintMetrics` y
`ProjectMetrics` de `types/metrics.ts` de la 009. Cada respuesta pasa por el reconocedor `isReport`
de `features/reports/api.ts` antes de mostrarse (Principio VII). Un cuerpo inesperado se trata como
falla de la generación, no como error de ejecución.

### 3.2 Modelo de presentación (`features/reports/view-model.ts`)

```text
ReportView
  title        string                 // "Reporte del sprint Sprint 3" o "Reporte del proyecto"
  partialNote  string | null          // "Reporte parcial: incluye un sprint en curso."
  header       { label, value }[]     // proyecto, alcance, período, generado el, generado por
  sections     SectionView[]          // siempre seis, en orden fijo
  fileName     string                 // reportFileName(...)

SectionView
  key          "header" | "stories" | "estimations" | "effort" | "metrics" | "defects"
  title        string
  state        "complete" | "empty" | "unavailable"
  message      string | null          // "Sin datos." o "No se pudo obtener esta sección."
  blocks       BlockView[]            // tablas y subtítulos de grupo; vacío con unavailable

BlockView
  heading      string | null          // p. ej. "Sprint 2 (01/09/2026 al 14/09/2026)" o "Historias no planificadas"
  message      string | null          // "Sin datos." en un grupo vacío
  table        { columns string[], rows string[][], footer string[] | null } | null
```

Todas las cadenas salen ya formateadas: fechas con `formatDate`, instantes con la zona del navegador,
horas y porcentajes con `formatDecimal` y `formatShare` de la 009, y estados con las etiquetas de la
003, la 004 y la 007. `ReportDocument` y `buildPdfDocument` solo recorren este objeto (research,
sección 13).

### 3.3 Estado de la pantalla (`ReportsView`)

```text
idle → generating → shown(report, view) ⇄ exporting → shown
                  ↘ error(mensaje)        exporting ↘ shown + exportError
```

`exporting` y `exportError` no modifican `report` ni `view` (FR-033). Generar de nuevo pasa por
`generating` y reemplaza el reporte.

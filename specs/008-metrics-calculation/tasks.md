---

description: "Lista de tareas de la feature 008 — Cálculo de Métricas"
---

# Tareas: Cálculo de Métricas

**Entrada**: documentos de diseño de `specs/008-metrics-calculation/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) a
[`specs/006-effort-tracking`](../006-effort-tracking/spec.md) integradas en `main`. Las fases 1 a 5
no dependen de [`specs/007-defect-tracking`](../007-defect-tracking/spec.md): solo la Fase 6 (US5)
necesita la 007 completa, con su tabla `defects`, sus tipos de dominio y sus endpoints. Esta feature usa del módulo
`internal/sprints/` el modelo `Sprint`, `SprintStory`, `SprintStatus`, `StoryOutcome` y los
endpoints de crear, comprometer, iniciar, avanzar y cerrar sprints; del módulo `internal/backlog/`
`StoryStatus` y los endpoints de historias; del módulo `internal/effort/` `GraceOpen`,
`RoundPercentHundredths`, el tipo `Hundredths` y el endpoint de alta de esfuerzo; del módulo
`internal/projects/` el subgrupo `/api/v1/projects/:projectId` con `RequireMembership` y el cambio de
factor; de la 001 el middleware de sesión y `ErrorResponse`; además `cfg.EffortGracePeriod`, el patrón
de reloj inyectado y `testsupport.OpenDatabase`. **No hay fase de Setup**: no se agregan
dependencias, servicios, variables de entorno ni tablas.

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para todas
las fórmulas, el redondeo, los estados de valor, la ventana de velocidad, las series, los acumulados,
los conteos de defectos, las advertencias y las reglas de negocio RN-01 a RN-31.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/metrics/{domain,repository,service,delivery}/`; se tocan
  de forma puntual `backend/cmd/app/main.go` y `backend/internal/effort/delivery/dto.go`, y se crea
  `backend/internal/platform/httpserver/hundredths.go`
- **Frontend**: sin cambios (RC-10)
- **Pruebas**: junto al código que prueban (`*_test.go` en el mismo paquete)

## Ciclo obligatorio en cada tarea

Según el Principio II y la skill `/speckit-implement`:

1. **Tarea de pruebas** → se escribe la prueba, se verifica que **falla** y se **detiene**. El
   desarrollador registra el commit con `/redactar-commit`, pie `TDD: red`.
2. **Tarea de implementación** → código mínimo que hace pasar la prueba (`TDD: green`), pausa, y
   refactor si corresponde (`TDD: refactor`).
3. **Tarea técnica** → sin prueba asociada, sin pie TDD.

Ningún agente avanza de RED a GREEN sin ese alto.

## `CalculateSprint` crece por historia

Para que cada historia tenga un RED real, la función pura `CalculateSprint` de
`backend/internal/metrics/domain/sprint.go` y la tabla de `SprintFinality` se amplían en la historia
que introduce cada estado de sprint:

| Historia | Estado que agrega | Por qué ahí |
| --- | --- | --- |
| US1 | `closed`: instantánea, factor congelado, `definitive`/`provisional` según la gracia | Es el núcleo de la feature |
| US2 | `active`: datos y factor vigentes, las nueve `partial` | Las series de US2-7 incluyen el sprint Activo con sus puntos parciales |
| US4 | `planned`: dos cifras `available` sin `finality` y siete `not_applicable` | Es la única historia que consulta un Planificado |

Los conteos de defectos de un sprint valen 0 en `SprintData` hasta US5, que agrega su lectura. Las
métricas `defectsDetected` y `defectsResolved` existen desde US1 con su estado de valor, así que
US1-5 y US1-6 verifican su `finality` con conteos en 0.

## Cómo se preparan los escenarios que dependen de otras features

- Las pruebas de integración usan un entorno `newMetricsTestEnv` (T011) que compone el router como
  `cmd/app` (sesión de la 001, rutas de la 002, la 003, la 004, la 006 y, desde US5, la 007, más las
  de métricas) sobre `httptest`, con **un reloj falso** compartido por los servicios de sprints, de
  esfuerzo y de métricas, y un plazo de gracia de 48 horas.
- Los proyectos y su factor se crean y se cambian con los endpoints de la 002; las historias y sus
  Story Points, con los de la 003; los sprints se crean, se comprometen, se inician, avanzan y se
  cierran con los de la 004; las horas se cargan con los de la 006, y los defectos se registran,
  resuelven, reabren y descartan con los de la 007.
- El reloj falso se adelanta 10 horas y 49 horas después del cierre para US1-5 y US1-6.
- Los datos rotos que ninguna API permite crear (una historia sin Story Points dentro de un sprint,
  un sprint Cerrado sin factor congelado o sin `closed_at`) se fuerzan con el ayudante
  `forceRow(t, sql, args...)` del entorno, que escribe directamente en `smye_test` como **dato de
  preparación**, solo en las pruebas de SC-020.
- Las pruebas sin sesión, la comparación de los `404`, la de coherencia concurrente, la de horas no
  invisibles, la de datos rotos, la de repetibilidad y la del proyecto Finalizado **no** llevan la
  etiqueta `008/USn`, porque no derivan de un escenario de la spec y no deben alterar el conteo de 61.

---

## Fase 1: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el tipo compartido de centésimas, los tipos de valor y de datos del dominio, los
errores, las advertencias y el contrato del repositorio. Todas las tareas son **técnicas**: declaran
estructuras, constantes y contratos, sin lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/008-foundational`

- [X] T001 [P] Mover sin cambios el tipo `Hundredths int` y su `MarshalJSON` desde
      `backend/internal/effort/delivery/dto.go` a `backend/internal/platform/httpserver/hundredths.go`
      (comentario: "magnitud en centésimas (de hora o de punto porcentual) que la API serializa como
      número JSON con exactamente 2 decimales; un *Hundredths nulo se serializa null"), y dejar en
      `backend/internal/effort/delivery/dto.go` el alias `type Hundredths = httpserver.Hundredths`
      para que ninguna línea de uso cambie. Si `effort/delivery` tiene pruebas propias de
      `MarshalJSON`, moverlas a `backend/internal/platform/httpserver/hundredths_test.go`. Verificar
      con `go test -p 1 ./internal/effort/... ./internal/platform/...` que la salida de la 006 no
      cambió (research.md sección 5). Refactor sin cambio de comportamiento
- [X] T002 [P] Declarar en `backend/internal/metrics/domain/values.go` el comentario de paquete
      ("declara los tipos de valor, las fórmulas puras y las interfaces del módulo de métricas; no
      depende de ninguna otra capa"), el tipo `ValueStatus string` con `ValueAvailable = "available"`,
      `ValueNotCalculable = "not_calculable"` y `ValueNotApplicable = "not_applicable"`; el tipo
      `Finality string` con `FinalityDefinitive = "definitive"`, `FinalityProvisional = "provisional"`
      y `FinalityPartial = "partial"`; y el struct `Metric{Status ValueStatus; Value int; Finality
      *Finality}`, con el comentario de data-model.md sección 4 ("`Value` solo es significativo con
      `available`"; "`Finality` solo en métricas de sprint y en puntos de serie; `nil` en el
      Planificado"). Los constructores se implementan en US1
- [X] T003 [P] Declarar en `backend/internal/metrics/domain/warnings.go` el tipo `WarningKind string`
      con `WarningPlanning = "planning"` y `WarningDataInconsistency = "data_inconsistency"`; el tipo
      `WarningCode string` con `CodeUnestimatedBacklogStories = "unestimated_backlog_stories"`,
      `CodeUnestimatedStoryInSprint = "unestimated_story_in_sprint"`,
      `CodeMissingFrozenFactor = "missing_frozen_factor"` y `CodeMissingCloseTime =
      "missing_close_time"`; y el struct `Warning{Kind; Code; SprintID *uuid.UUID; StoryID
      *uuid.UUID; Count *int}` (data-model.md sección 9). Sin textos de interfaz: los mensajes viven
      en `delivery`
- [X] T004 [P] Declarar en `backend/internal/metrics/domain/errors.go` el error
      `ErrSprintNotFound` ("el sprint no existe o es de otro proyecto"), con el comentario de que
      `delivery` lo traduce a `404` con el mismo cuerpo que la 004
- [X] T005 Declarar los tipos de datos y de resultado del dominio, sin funciones, con los campos de
      data-model.md secciones 3 y 5 a 8: en `backend/internal/metrics/domain/sprint.go`
      `SprintData` (`ID`, `Name`, `Status sprintsdomain.SprintStatus`, `StartDate`, `EndDate`,
      `ClosedAt *time.Time`, `FrozenFactorHundredths *int`, `Stories []SprintStoryData`,
      `ActualHundredths int`, `DefectsDetected int`, `DefectsResolved int`), `SprintStoryData`
      (`StoryID`, `Title`, `StoryPoints *int` —"`nil` es dato roto (FR-037)"—, `Completed bool`), el
      tipo `MetricKind string` con las nueve constantes de data-model.md sección 2 y
      `SprintMetrics` (el sprint más las nueve `Metric`); en
      `backend/internal/metrics/domain/project.go` `ProjectData` (`ProjectID`, `FactorHundredths`,
      `Sprints` "en orden `start_date ASC, id ASC`", `OutsideSprintsHundredths`,
      `UnestimatedBacklogStories`), `VelocityPoint`, `Velocity`
      (`Value Metric`, `SprintsConsidered int`, `Series []VelocityPoint`), `Cumulative` (`Finality`,
      `IncludesActiveSprint`, las siete `Metric`) y `ProjectMetrics`; en
      `backend/internal/metrics/domain/series.go` `SeriesPoint` (`SprintID`, `SprintName`,
      `SprintStatus`, `Value Metric`) y `ProjectSeries` con sus seis listas. `ProjectMetrics` no
      lleva todavía el bloque de defectos: lo agrega US5 (T039), para que las fases 1 a 5 no importen
      `defectsdomain` — depende de T002, T003
- [X] T006 Declarar en `backend/internal/metrics/domain/repository.go` la interfaz
      `MetricsRepository` con **exactamente** las tres operaciones de data-model.md sección 10:
      `LoadProject(ctx, projectID uuid.UUID) (*ProjectData, error)`,
      `LoadSprint(ctx, projectID, sprintID uuid.UUID) (*SprintData, int, error)` (el `int` es el
      factor vigente; `ErrSprintNotFound` si el sprint no existe o es de otro proyecto) y
      `LoadActiveSprint(ctx, projectID uuid.UUID) (*SprintData, int, error)` (`nil` sin error si no
      hay Activo). Comentario: todas las lecturas de una operación ocurren en una transacción
      `REPEATABLE READ` de solo lectura (research.md sección 4) y ninguna devuelve
      `gorm.ErrRecordNotFound` — depende de T004, T005

**Checkpoint**: `go build ./...` compila, `go test -p 1 ./internal/effort/...` pasa con el tipo
movido, y el dominio y el contrato del repositorio están declarados. Las historias pueden empezar.

---

## Fase 2: Historia de Usuario 1 — Métricas de un sprint cerrado (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante consulta un sprint Cerrado y obtiene los Story Points planificados y
completados, las horas estimadas y reales, las dos desviaciones y el porcentaje de historias
completadas, tomados de la instantánea y con su estado definitivo o provisorio.

**Rama**: `hu/008-us1-metricas-de-un-sprint-cerrado`

**Prueba independiente**: `GET /api/v1/projects/{projectId}/metrics/sprints/{sprintId}` sobre un
sprint Cerrado de 21 puntos planificados, 13 completados, factor 6 y 142,5 horas devuelve `21`, `13`,
`126.00`, `142.50`, `16.50`, `13.10` y `50.00`, con las horas reales y las desviaciones
`provisional` dentro del plazo de gracia; un sprint de otro proyecto devuelve
`404 {"error":"El sprint no existe."}`.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T007 [P] [US1] Pruebas unitarias **obligatorias** en
      `backend/internal/metrics/domain/values_test.go` y
      `backend/internal/metrics/domain/arithmetic_test.go`, cubriendo RN-17, RN-19 y RN-20:
      `Available(0)` tiene `status: available` y es distinto de `NotCalculable()` y de
      `NotApplicable()`; `Percent(n, 0)` → `NotCalculable()` para `n` = 0, positivo y negativo;
      `Percent(1650, 12600)` → `1310`; `Percent(-1600, 12600)` → `-1270`; `Percent(2, 4)` → `5000`;
      `Percent(1, 3)` → `3333`; `Percent(2, 3)` → `6667`; `Percent(7, 11)` → `6364`;
      `Percent(-6550, 37800)` → `-1733`; y empates en ,005 que redondean lejos de cero en ambos
      signos, con el mismo resultado que `effortdomain.RoundPercentHundredths` — depende de T006
- [X] T008 [P] [US1] Pruebas unitarias **obligatorias** de `CalculateSprint` y `SprintFinality` para
      sprints `closed` en `backend/internal/metrics/domain/sprint_test.go`, con datos armados en
      memoria y un `now` fijo, cubriendo RN-03, RN-07 a RN-09, RN-12, RN-17, RN-18, RN-23, RN-24 y
      RN-30: los números exactos de US1-1, US1-2, US1-8 (todas en 0 puntos: desviación porcentual
      `not_calculable`, absoluta = reales), US1-9 (sin historias: porcentaje y desviación porcentual
      `not_calculable`), US1-10 (`10000`) y US1-11 (`3333`); el factor y los puntos salen de
      `FrozenFactorHundredths` y de `StoryPoints` congelados aunque `currentFactorHundredths` sea
      otro; factores con dos decimales (625 × 21 = 13125); porcentaje por cantidad de historias y no
      por puntos; `SprintFinality(closed, kind, graceOpen)` para las **9** métricas × gracia vigente y
      vencida según la tabla de research.md sección 7, con ningún `partial`; el borde exacto
      `now = closedAt + grace` cuenta como vencido (`GraceOpen`); una historia con `StoryPoints` nil
      cuenta como 0 y produce `unestimated_story_in_sprint` con `SprintID` y `StoryID`; sin
      `FrozenFactorHundredths` se usa el factor vigente y se produce `missing_frozen_factor`; sin
      `ClosedAt` el plazo se toma como vencido y se produce `missing_close_time`; en los tres casos
      las nueve métricas se devuelven igual — depende de T006
- [X] T009 [P] [US1] Pruebas de integración del repositorio contra `smye_test` en
      `backend/internal/metrics/repository/metrics_repository_test.go`, preparando filas con GORM:
      `LoadSprint` de un sprint Cerrado devuelve las filas de `sprint_stories` con
      `story_points_at_close` y `outcome = completed` como `Completed`, `closed_at`, el factor
      congelado y la suma de `hours_hundredths` de `effort_entries` con ese `sprint_id`; un sprint de
      otro proyecto y uno inexistente → `ErrSprintNotFound`; devuelve también el factor vigente del
      proyecto; y, cubriendo RN-02, la función privada
      `readOnly(ctx context.Context, fn func(tx *gorm.DB) error) error` del paquete, que abre la
      transacción `REPEATABLE READ` de solo lectura que usan las tres operaciones, hace fallar un
      `UPDATE projects SET name = name` ejecutado dentro de `fn` con un error de transacción de solo
      lectura (SQLSTATE `25006`), y la fila no cambia (prueba interna del paquete `repository`) —
      depende de T006
- [X] T010 [US1] Crear en `backend/internal/metrics/delivery/testenv_test.go` el entorno
      `newMetricsTestEnv` descripto arriba (router, reloj falso compartido, plazo de 48 horas,
      ayudantes para crear cuentas, proyectos con factor, integrantes, historias con puntos, sprints
      con sus historias, iniciarlos, completar historias, cerrarlos, cargar horas, cambiar el factor,
      reestimar, y `forceRow`), y las pruebas de integración de la historia en
      `backend/internal/metrics/delivery/handler_test.go`, una función por escenario de spec.md
      precedida por `// Escenario: <nombre del escenario en spec.md> (008/US1)`:
      `TestUS1_SprintCerradoCasoNormal` (los siete valores exactos en **una sola** consulta; SC-011),
      `TestUS1_Sobrestimacion` (`-16.00` y `-12.70`), `TestUS1_CambioDeFactorPosterior` (factor 6 → 8:
      estimadas siguen `126.00`; SC-005), `TestUS1_ReestimacionPosterior` (5 → 13: planificados siguen
      `21`; SC-005), `TestUS1_EstadosDentroDelPlazoDeGracia` (reloj +10 h: cuatro `definitive`, tres
      `provisional` y los dos conteos `provisional`; SC-015), `TestUS1_EstadosConPlazoVencido` (reloj
      +49 h: horas y desviaciones `definitive`, conteos siguen `provisional`; SC-015),
      `TestUS1_EsfuerzoDentroDelPlazo` (3 horas más con la 006: reales `145.50`, desviación
      recalculada, puntos y estimadas sin cambio), `TestUS1_HistoriasEnCeroPuntos`,
      `TestUS1_SprintSinHistorias`, `TestUS1_TodasCompletadas` (`100.00`), `TestUS1_DecimalPeriodico`
      (`33.33`; SC-004), `TestUS1_HistoriaReplanificada` (planificada en los dos sprints, completada
      solo en el segundo), `TestUS1_SprintInexistente` (subtests con un UUID inexistente y con un
      identificador que no es UUID, por ejemplo `abc`: los dos `404 {"error":"El sprint no existe."}`
      con el mismo cuerpo byte a byte),
      `TestUS1_SprintDeOtroProyecto` (el mismo cuerpo byte a byte) y `TestUS1_NoIntegrante` (`404
      {"error":"El proyecto no existe."}`). En cada respuesta se verifica que horas y porcentajes se
      serializan con exactamente 2 decimales y los Story Points como enteros — depende de T006

### Implementación de la Historia 1

- [X] T011 [P] [US1] Implementar en `backend/internal/metrics/domain/values.go` los constructores
      `Available(value int) Metric`, `NotCalculable() Metric` y `NotApplicable() Metric`, y en
      `backend/internal/metrics/domain/arithmetic.go` `Percent(numerator, denominator int) Metric`,
      que devuelve `NotCalculable()` con denominador 0 y, si no,
      `Available(effortdomain.RoundPercentHundredths(numerator, denominator))`, sin ningún otro
      redondeo (FR-033, RC-13 de la 006) — depende de T007
- [X] T012 [US1] Implementar en `backend/internal/metrics/domain/sprint.go`
      `SprintFinality(status sprintsdomain.SprintStatus, kind MetricKind, graceOpen bool) *Finality`
      con las filas `closed` de research.md sección 7, y
      `CalculateSprint(sprint SprintData, currentFactorHundredths int, now time.Time, grace
      time.Duration) (SprintMetrics, []Warning)` para `closed`: planificados = Σ puntos (nil = 0 y
      advertencia), completados = Σ puntos de las `Completed`, estimadas = planificados × factor
      congelado (o vigente con `missing_frozen_factor`), reales = `ActualHundredths`, desviación =
      reales − estimadas, desviación porcentual = `Percent(desviación, estimadas)`, porcentaje =
      `Percent(completadas, planificadas)` por cantidad, y los dos conteos; la gracia con
      `effortdomain.GraceOpen(*ClosedAt, now, grace)`, o vencida con `missing_close_time`. Los estados
      `active` y `planned` todavía no se implementan — depende de T008, T011
- [X] T013 [US1] Implementar `LoadSprint` en
      `backend/internal/metrics/repository/metrics_repository.go` con `NewMetricsRepository(db
      *gorm.DB)`: la función privada `readOnly`, que abre la transacción con
      `&sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}` y que reutilizan
      `LoadProject` y `LoadActiveSprint`; (1) factor vigente de `projects`; (2) sprint `WHERE id = ? AND project_id = ?`, sin fila
      → `ErrSprintNotFound`; (3) historias: en un Cerrado, todas sus filas de `sprint_stories` con
      `story_points_at_close` y `outcome = 'completed'`; en un sprint abierto, las filas con
      `outcome IS NULL` unidas a `user_stories` con `story_points` vigentes y `status = 'completed'`
      (research.md sección 8); (4) `COALESCE(SUM(hours_hundredths), 0)` de `effort_entries` con ese
      `sprint_id`. `DefectsDetected` y `DefectsResolved` quedan en 0 hasta US5. Los errores de GORM se
      envuelven con `fmt.Errorf` y contexto en español (Principio VII) — depende de T009
- [X] T014 [US1] Implementar en `backend/internal/metrics/service/metrics_service.go` el constructor
      `NewMetricsService(repo domain.MetricsRepository, grace time.Duration, now func() time.Time)`,
      con la firma definitiva para que no cambie en las historias siguientes, y
      `SprintMetrics(ctx, projectID, sprintID)`, que toma `now` **una sola vez** por consulta, llama a
      `LoadSprint` y a `CalculateSprint`. El servicio es también el punto de entrada que reutilizará
      la 010 dentro del proceso — depende de T012, T013
- [X] T015 [US1] Declarar en `backend/internal/metrics/delivery/dto.go` los esquemas de
      contracts/openapi.yaml `Metric` y `SprintMetric` (con `value` como número JSON: Story Points y
      conteos enteros, horas y porcentajes con `httpserver.Hundredths`; `value: null` si y solo si
      `status` no es `available`; `finality` anulable), `SprintRef` (`closedAt` anulable),
      `SprintMetrics` y `Warning`, con los mensajes en español de cada `code` como constantes
      nombradas en un único lugar (por ejemplo, "La historia «{título}» del sprint no tiene Story
      Points; se contó como 0."); implementar el manejador `getSprintMetrics` en
      `backend/internal/metrics/delivery/handler.go` con anotaciones `swag` en español (etiqueta
      `Métricas`, `200`, `401`, `404`, `500`): `sprintId` que no es UUID → `404`
      `{"error":"El sprint no existe."}` sin consultar; `ErrSprintNotFound` → el mismo `404`; cualquier
      otro error → `500` genérico registrado con `slog`; crear en
      `backend/internal/metrics/delivery/routes.go` `RegisterRoutes(projectGroup *gin.RouterGroup,
      handler *Handler)` con `GET /metrics/sprints/:sprintId`, sin importar `projects/delivery`; y
      componer el módulo en `backend/cmd/app/main.go` (repositorio, servicio con
      `cfg.EffortGracePeriod` y `time.Now`, registro sobre `projectGroup`), ampliando el
      `@description` general de la API para que mencione el cálculo de métricas — depende de T010,
      T014

**Checkpoint**: US1 funciona sola: las métricas de un sprint Cerrado salen de la instantánea, con sus
estados de valor y sus advertencias.

---

## Fase 3: Historia de Usuario 2 — Velocidad del equipo y su evolución (Prioridad: P1)

**Objetivo**: un integrante consulta la velocidad del equipo, su serie por sprint Cerrado y las seis
series del proyecto, en orden cronológico y con el estado de valor de cada punto.

**Rama**: `hu/008-us2-velocidad-del-equipo-y-su-evolucion`

**Prueba independiente**: con 4 sprints Cerrados que completaron 13, 21, 8 y 20 puntos,
`GET /api/v1/projects/{projectId}/metrics` devuelve `velocity.value` `16.33`,
`sprintsConsidered: 3` y una serie de cuatro puntos; `GET .../metrics/series` devuelve seis listas
con los mismos sprints Cerrados y el Activo, sin los Planificados.

### Pruebas de la Historia 2 ⚠️

- [X] T016 [P] [US2] Pruebas unitarias **obligatorias** de `AverageHundredths(sum, count int) int` en
      `backend/internal/metrics/domain/arithmetic_test.go`, cubriendo RN-10 y RN-19: (13, 1) → `1300`;
      (34, 2) → `1700`; (42, 3) → `1400`; (49, 3) → `1633`; (0, 3) → `0`; y que coincide con
      `RoundPercentHundredths(sum, 100 × count)` — depende de T015
- [X] T017 [P] [US2] Pruebas unitarias **obligatorias** en
      `backend/internal/metrics/domain/sprint_test.go` de `CalculateSprint` y `SprintFinality` para
      sprints `active`, cubriendo RN-04 y RN-24: puntos y factor vigentes (no `FrozenFactorHundredths`),
      completadas por `Completed`, las **nueve** métricas `partial` y ningún `provisional`; los números
      de US4-1 (`10800`, `4000`, `-6800`, `-6296`) — depende de T015
- [X] T018 [P] [US2] Pruebas unitarias **obligatorias** en
      `backend/internal/metrics/domain/project_test.go` y
      `backend/internal/metrics/domain/series_test.go`, cubriendo RN-06, RN-10 y RN-28: la ventana de
      velocidad con **0** (`not_calculable`, `sprintsConsidered` 0), **1**, **2**, **3** y **4**
      Cerrados (SC-006), con `ClosedAt` en un orden distinto de `StartDate` para verificar que la
      ventana usa el cierre y la serie el inicio; un Cerrado sin `ClosedAt` queda último en la ventana;
      la velocidad de tres sprints que no completaron nada es `Available(0)`; con un Activo y sin
      Cerrados es `not_calculable` y la serie vacía; `Velocity` no tiene `Finality` (campo inexistente
      en el tipo); `BuildSeries(data, now, grace)` devuelve seis listas con **los mismos** sprints en
      el mismo orden `start_date ASC, id ASC`, solo Cerrados y Activo, cada punto igual al
      `SprintMetrics` de su sprint, puntos del Activo `partial`, un sprint sin datos con
      `Available(0)` en las seis, y seis listas vacías sin sprints Cerrados ni Activo — depende de T015
- [X] T019 [P] [US2] Pruebas de integración del repositorio en
      `backend/internal/metrics/repository/metrics_repository_test.go`: `LoadProject` devuelve
      **todos** los sprints del proyecto (`planned`, `active`, `closed`) en orden `start_date ASC, id
      ASC`, cada uno con sus historias según su estado y sus horas, el factor vigente, y ningún sprint
      de otro proyecto — depende de T015
- [X] T020 [US2] Pruebas de integración en `backend/internal/metrics/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (008/US2)`: `TestUS2_VelocidadTresCerrados`
      (`14.00`), `TestUS2_VentanaDeTres` (`16.33`; SC-004), `TestUS2_UnSprintCerrado` (`13.00`),
      `TestUS2_DosSprintsCerrados` (`17.00`), `TestUS2_SerieDeVelocidad` (13, 21, 8 y 20 en orden),
      `TestUS2_SeisSeries` (las seis listas en `GET .../metrics/series`, cada punto con `sprintId`,
      `sprintName` y `sprintStatus`), `TestUS2_SeriesConSprintActivo` (cuatro sprints; el Activo
      `partial`), `TestUS2_SeriesSinPlanificados`, `TestUS2_ValorCeroEnSeries` (`status: available`,
      `value: 0`), `TestUS2_SeriesVacias`, `TestUS2_VelocidadCero` (`0.00`, `available`),
      `TestUS2_SoloSprintActivo` (`not_calculable` y serie vacía), `TestUS2_ProyectoSinSprints` (`200`
      con `not_calculable`), `TestUS2_VentanaQueSeDesplaza` (al cerrar un cuarto cambia el valor y el
      primero conserva sus métricas en `.../metrics/sprints/{id}`) y `TestUS2_NoIntegrante` (`404`
      con el cuerpo de la 002, en las dos rutas) — depende de T015

### Implementación de la Historia 2

- [X] T021 [P] [US2] Implementar `AverageHundredths` en `backend/internal/metrics/domain/arithmetic.go`
      como `effortdomain.RoundPercentHundredths(sum, 100 × count)`, con el comentario de que es
      `sum × 100 / count` redondeado una sola vez (research.md sección 5) — depende de T016
- [X] T022 [US2] Ampliar `CalculateSprint` y `SprintFinality` en
      `backend/internal/metrics/domain/sprint.go` con el estado `active`: factor vigente, las nueve
      `partial` — depende de T017, T021
- [X] T023 [US2] Implementar en `backend/internal/metrics/domain/project.go` la ventana y la serie
      de velocidad (Cerrados por `ClosedAt DESC NULLS LAST, ID DESC`, los 3 primeros;
      `AverageHundredths`; serie de todos los Cerrados en orden de `Sprints`) y en
      `backend/internal/metrics/domain/series.go` `BuildSeries(data ProjectData, now time.Time, grace
      time.Duration) (ProjectSeries, []Warning)`, que reutiliza `CalculateSprint` para cada Cerrado y
      el Activo, de modo que un punto y el detalle de su sprint nunca difieren — depende de T018, T022
- [X] T024 [US2] Implementar `LoadProject` en
      `backend/internal/metrics/repository/metrics_repository.go` en una sola transacción
      `REPEATABLE READ` de solo lectura: factor vigente, todos los sprints del proyecto en orden
      `start_date ASC, id ASC`, sus historias con la misma regla que `LoadSprint` y las horas por
      sprint agrupadas por `sprint_id`, extrayendo a funciones privadas las lecturas que comparte con
      `LoadSprint` (DRY). Esfuerzo sin sprint, backlog sin estimar y defectos se agregan en US3 y US5 —
      depende de T019, T013
- [X] T025 [US2] Agregar en `backend/internal/metrics/service/metrics_service.go` `ProjectMetrics`
      (por ahora la velocidad y sus advertencias) y `ProjectSeries`, cada uno con un único `now`; en
      `backend/internal/metrics/delivery/dto.go` los esquemas `VelocityPoint`, `Velocity`,
      `SeriesPoint` (plano: `sprintId`, `sprintName`, `sprintStatus`, `status`, `value`, `finality`),
      `ProjectSeriesResponse` y la parte de `ProjectMetricsResponse` con `projectId`, `velocity` y
      `warnings`; en `backend/internal/metrics/delivery/handler.go` los manejadores
      `getProjectMetrics` y `getProjectSeries` con anotaciones `swag`; y en
      `backend/internal/metrics/delivery/routes.go` `GET /metrics` y `GET /metrics/series` — depende de
      T020, T023, T024

**Checkpoint**: US2 funciona sola: velocidad con su ventana de 3 y las seis series listas para
graficar.

---

## Fase 4: Historia de Usuario 3 — Métricas acumuladas del proyecto (Prioridad: P2)

**Objetivo**: un integrante ve los acumulados de los sprints Cerrados y del Activo, marcados como
parciales o definitivos, más el esfuerzo fuera de sprint, el esfuerzo en sprints Planificados y la
cantidad de historias del backlog sin estimar con su advertencia.

**Rama**: `hu/008-us3-metricas-acumuladas-del-proyecto`

**Prueba independiente**: con dos sprints Cerrados y uno Activo, `GET .../metrics` devuelve
`cumulative` con `63`, `39`, `378.00`, `312.50`, `-65.50`, `-17.33` y `finality: partial`;
`effortOutsideSprintsHours` `18.00` y `effortInPlannedSprintsHours` presentes aunque valgan 0.

### Pruebas de la Historia 3 ⚠️

- [X] T026 [P] [US3] Pruebas unitarias **obligatorias** de `CalculateProject` en
      `backend/internal/metrics/domain/project_test.go`, cubriendo RN-06, RN-11, RN-12, RN-25 a RN-27 y
      RN-31: los totales exactos de US3-1 y el `6364` de US3-2; los cocientes salen de los totales y
      no del promedio de los porcentajes por sprint (dos sprints con porcentajes distintos); una
      historia en dos sprints aporta dos veces a planificados y una a completados; un Planificado no
      entra; `Cumulative.Finality` `partial` con Activo y `definitive` sin él; solo Planificados o sin
      sprints → totales `Available(0)` y cocientes `not_calculable`; estimadas 0 con reales →
      desviación absoluta = reales y porcentual `not_calculable`; `EffortOutsideSprintsHundredths` y
      `EffortInPlannedSprintsHundredths` (Σ `ActualHundredths` de los `planned`) fuera de
      `Cumulative.ActualHours` y presentes con 0; la advertencia `planning`
      `unestimated_backlog_stories` con `Count` solo si `UnestimatedBacklogStories > 0`, nunca
      producida por `CalculateSprint`; y las advertencias `data_inconsistency` de los sprints
      incluidos viajan junto a las de planificación sin quitar ninguna métrica — depende de T025
- [X] T027 [P] [US3] Pruebas de integración del repositorio en
      `backend/internal/metrics/repository/metrics_repository_test.go`: `LoadProject` devuelve
      `OutsideSprintsHundredths` con la suma de los registros con `sprint_id IS NULL` de historias del
      proyecto (no de otro) y `UnestimatedBacklogStories` con las historias del proyecto con
      `story_points IS NULL` — depende de T025
- [X] T028 [US3] Pruebas de integración en `backend/internal/metrics/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (008/US3)`: `TestUS3_AcumuladosCasoNormal`,
      `TestUS3_PorcentajeAcumulado` (`63.64`), `TestUS3_SinPlanificados`, `TestUS3_AcumuladoParcial`
      (`cumulative.finality: partial` y `velocity` sin `finality`), `TestUS3_AcumuladoDefinitivo`
      (cerrar el Activo → `definitive`; SC-018), `TestUS3_HistoriaEnDosSprints` (10 y 5),
      `TestUS3_SoloSprintsPlanificados`, `TestUS3_ProyectoSinSprints`, `TestUS3_EstimadasEnCero`,
      `TestUS3_EsfuerzoFueraDeSprint` (`18.00` aparte, reales siguen `312.50`),
      `TestUS3_EsfuerzoFueraDeSprintEnCero` (`0.00`, no omitido),
      `TestUS3_EsfuerzoEnSprintsPlanificados` (9 horas cargadas con la 006 sobre una historia de un
      sprint Planificado: `effortInPlannedSprintsHours` `9.00`, reales siguen `312.50`, las tres
      cifras suman el total del proyecto; sin horas en Planificados → `0.00`; SC-016),
      `TestUS3_BacklogSinEstimar` (`6` y la advertencia `planning`), `TestUS3_BacklogEstimado` (`0` y
      `warnings` sin `unestimated_backlog_stories`; SC-017) y `TestUS3_ProyectoInexistente` (`404`
      con el cuerpo de la 002) — depende de T025
- [X] T029 [US3] Pruebas de integración en `backend/internal/metrics/delivery/consistency_test.go`,
      **sin** etiqueta `008/USn`: `TestMetrics_NoInvisibleEffort` (SC-016, ampliada: con horas sin
      sprint, en Cerrados, en el Activo y en un Planificado, `cumulative.actualHours +
      effortOutsideSprintsHours + effortInPlannedSprintsHours` = Σ `hours_hundredths` del proyecto
      leída de la base) y `TestMetrics_Consistency` (SC-019, RN-29): 20 consultas a `GET .../metrics`
      y a `GET .../metrics/sprints/{activo}` mientras otras goroutines cargan horas con la 006 y
      completan historias con la 004; en las 20 respuestas, completados ≤ planificados y
      `deviationHours` = `actualHours − estimatedHours` en la misma respuesta, sin ningún `500` —
      depende de T025

### Implementación de la Historia 3

- [X] T030 [US3] Implementar en `backend/internal/metrics/domain/project.go`
      `CalculateProject(data ProjectData, now time.Time, grace time.Duration) ProjectMetrics`:
      calcula cada Cerrado y el Activo con `CalculateSprint`, suma los totales, calcula las
      desviaciones y el porcentaje sobre los totales con `Percent`, fija `Cumulative.Finality`, arma
      la velocidad de T023, las dos cifras de esfuerzo separado y el contador sin estimar con su
      advertencia, y reúne las advertencias — depende de T026, T023
- [X] T031 [US3] Ampliar `LoadProject` en
      `backend/internal/metrics/repository/metrics_repository.go`, dentro de la misma transacción, con
      `OutsideSprintsHundredths` (`effort_entries` con `sprint_id IS NULL` unido a `user_stories` del
      proyecto) y `UnestimatedBacklogStories` (`COUNT(*)` de `user_stories` del proyecto con
      `story_points IS NULL`) — depende de T027
- [X] T032 [US3] Hacer que `ProjectMetrics` de `backend/internal/metrics/service/metrics_service.go`
      use `CalculateProject`, y agregar en `backend/internal/metrics/delivery/dto.go` el esquema
      `Cumulative` y los campos `cumulative`, `effortOutsideSprintsHours`,
      `effortInPlannedSprintsHours` y `unestimatedBacklogStories` de `ProjectMetricsResponse`, con el
      mensaje de la advertencia `planning` ("Hay {n} historias del backlog sin estimar; las cifras de
      planificación pueden quedar cortas."), actualizando las anotaciones `swag` de
      `getProjectMetrics` — depende de T028, T029, T030, T031

**Checkpoint**: US3 funciona sola: los acumulados cuadran con la suma de los sprints, ninguna hora
queda invisible y cada respuesta es internamente coherente bajo escritura concurrente.

---

## Fase 5: Historia de Usuario 4 — Métricas de un sprint activo o planificado (Prioridad: P2)

**Objetivo**: un integrante ve cómo viene el sprint Activo sin esperar a que cierre, qué se espera de
uno Planificado, y una indicación explícita cuando no hay sprint Activo.

**Rama**: `hu/008-us4-metricas-de-un-sprint-activo-o-planificado`

**Prueba independiente**: `GET .../metrics/active-sprint` con un Activo de 18 puntos y 40 horas
devuelve `108.00`, `-68.00`, `-62.96` y las nueve métricas `partial`; sin Activo devuelve `200`
`{"activeSprint": false, "metrics": null}`; un Planificado de 15 puntos devuelve `15` y `90.00` con
`finality: null` y siete `not_applicable`.

### Pruebas de la Historia 4 ⚠️

- [X] T033 [P] [US4] Pruebas unitarias **obligatorias** en
      `backend/internal/metrics/domain/sprint_test.go` de `CalculateSprint` y `SprintFinality` para
      sprints `planned`, cubriendo RN-05 y FR-015: planificados y estimadas `available` con el factor
      vigente y `Finality` nil; las otras siete `NotApplicable()` con `Finality` nil, distintas de
      `Available(0)`; un compromiso sin puntos en un Planificado cuenta como 0 y produce
      `unestimated_story_in_sprint`; y, para el Activo, los casos límite US4-5 (sin
      historias: totales 0, cocientes `not_calculable`, `partial`) y US4-6 (sin horas: reales 0,
      desviación = −estimadas) — depende de T032
- [X] T034 [P] [US4] Pruebas de integración del repositorio en
      `backend/internal/metrics/repository/metrics_repository_test.go`: `LoadActiveSprint` devuelve el
      sprint `active` del proyecto con sus compromisos abiertos y sus horas, y `nil` sin error cuando
      no hay ninguno — depende de T032
- [X] T035 [US4] Pruebas de integración en `backend/internal/metrics/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (008/US4)`: `TestUS4_SprintActivoCasoNormal` (las
      nueve `partial`; SC-007), `TestUS4_HistoriaAgregada` (26 puntos y `156.00`),
      `TestUS4_FactorVigente` (factor 6 → 8: `144.00`), `TestUS4_SprintPlanificado` (por
      `.../metrics/sprints/{id}`: `15`, `90.00`, `finality: null` y siete `not_applicable` con
      `value: null`), `TestUS4_ActivoSinHistorias`, `TestUS4_ActivoSinHoras` y
      `TestUS4_SinSprintActivo` (`200 {"activeSprint": false, "metrics": null}`, no un error ni
      ceros) — depende de T032

### Implementación de la Historia 4

- [X] T036 [US4] Ampliar `CalculateSprint` y `SprintFinality` en
      `backend/internal/metrics/domain/sprint.go` con el estado `planned` (research.md sección 7 y
      aclaración del 2026-10-26: dos cifras sin estado de valor) — depende de T033
- [X] T037 [US4] Implementar `LoadActiveSprint` en
      `backend/internal/metrics/repository/metrics_repository.go` reutilizando las lecturas privadas de
      `LoadSprint`, en una transacción `REPEATABLE READ` de solo lectura — depende de T034
- [X] T038 [US4] Agregar `ActiveSprintMetrics` en `backend/internal/metrics/service/metrics_service.go`
      (devuelve `nil` sin error si no hay Activo), el esquema `ActiveSprintMetricsResponse` en
      `backend/internal/metrics/delivery/dto.go`, el manejador `getActiveSprintMetrics` con
      anotaciones `swag` en `backend/internal/metrics/delivery/handler.go` y la ruta
      `GET /metrics/active-sprint` en `backend/internal/metrics/delivery/routes.go` — depende de T035,
      T036, T037

**Checkpoint**: US4 funciona sola: el sprint Activo se sigue en vivo con su marca de parcial y el
Planificado informa solo lo que tiene sentido.

---

## Fase 6: Historia de Usuario 5 — Métricas de defectos (Prioridad: P3)

**Objetivo**: un integrante ve cuántos defectos se detectaron y se resolvieron en cada sprint, y
cuántos se detectaron, resolvieron y siguen abiertos en el proyecto, con su desglose por severidad.

**Rama**: `hu/008-us5-metricas-de-defectos`

**Prerrequisito**: la 007 completa en `main` (sus endpoints preparan los escenarios).

**Prueba independiente**: con 12 defectos (3 Críticos, 4 Altos, 3 Medios, 2 Bajos), 5 Resueltos y 3
Descartados más, `GET .../metrics` devuelve `defects.detected.total` `12`, `resolved` `5`, `open`
`7`, y cada desglose suma su total.

### Pruebas de la Historia 5 ⚠️

- [X] T039 [P] [US5] Declarar en `backend/internal/metrics/domain/defects.go` `DefectCount`
      (`Status defectsdomain.DefectStatus`, `Severity defectsdomain.DefectSeverity`, `Count int`),
      `DefectTally` (`Total`, `BySeverity map[defectsdomain.DefectSeverity]int`) y `ProjectDefects`
      (`Detected`, `Resolved`, `Open`), y agregar `DefectCounts []DefectCount` a `ProjectData` y
      `Defects ProjectDefects` a `ProjectMetrics`, como firmas mínimas para que compilen. Luego,
      pruebas unitarias **obligatorias** de `TallyDefects(counts []DefectCount) ProjectDefects` en
      `backend/internal/metrics/domain/defects_test.go`, cubriendo RN-13 a RN-16 y
      SC-010: detectados = `open` + `in_progress` + `resolved`, resueltos = `resolved`, abiertos =
      `open` + `in_progress`; las cuatro severidades siempre presentes en los tres desgloses, con 0
      si faltan; una fila `discarded` se ignora; sin filas → los tres en 0 y no `not_calculable`; en
      cada conteo Σ `BySeverity` = `Total`, y `Detected.Total` = `Resolved.Total + Open.Total` —
      depende de T038
- [X] T040 [P] [US5] Pruebas de integración del repositorio en
      `backend/internal/metrics/repository/metrics_repository_test.go`: en `LoadSprint`, `LoadActiveSprint`
      y `LoadProject`, `DefectsDetected` cuenta los `defects` con ese `detected_sprint_id` y
      `status <> 'discarded'`, y `DefectsResolved` los de ese `resolved_sprint_id` con
      `status = 'resolved'`; `LoadProject.DefectCounts` agrupa por `(status, severity)` los defectos de
      historias del proyecto sin `discarded` y sin defectos de otro proyecto — depende de T038
- [ ] T041 [US5] Pruebas de integración en `backend/internal/metrics/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (008/US5)`, preparando los defectos con los endpoints
      de la 007 (el entorno de T010 agrega sus rutas): `TestUS5_DefectosDelSprint` (7 y 4),
      `TestUS5_DefectosDelProyecto` (12, 5, 7 y sus desgloses), `TestUS5_DeteccionYResolucionEnDistintosSprints`,
      `TestUS5_MismoSprint`, `TestUS5_DefectoReabierto` (sale de resueltos del sprint 2, sigue en
      detectados; SC-009), `TestUS5_DefectosDescartados` (subtests antes y después de descartar: los
      totales no cambian; SC-008), `TestUS5_ProyectoSinDefectos` (`0` en todo y no `not_calculable`),
      `TestUS5_DeteccionEnSprintCerrado` (cuenta en el Cerrado, con `finality: provisional`) y
      `TestUS5_NoIntegrante` (`404` con el cuerpo de la 002) — depende de T038

### Implementación de la Historia 5

- [ ] T042 [P] [US5] Implementar `TallyDefects` en `backend/internal/metrics/domain/defects.go` con las
      constantes de `defectsdomain`, sin repetir los valores como cadenas — depende de T039
- [ ] T043 [US5] Ampliar las lecturas de `backend/internal/metrics/repository/metrics_repository.go`
      con los dos conteos por sprint (en la función privada que comparten las tres operaciones) y con
      `DefectCounts` en `LoadProject`, dentro de la misma transacción — depende de T040
- [ ] T044 [US5] Agregar el bloque `Defects` a `CalculateProject` en
      `backend/internal/metrics/domain/project.go`, y en `backend/internal/metrics/delivery/dto.go` los
      esquemas `SeverityBreakdown`, `DefectTally` y `ProjectDefects` y el campo `defects` de
      `ProjectMetricsResponse`, actualizando las anotaciones `swag` — depende de T041, T042, T043

**Checkpoint**: las cinco historias funcionan; los Descartados no cuentan y un reabierto sale de los
resueltos.

---

## Fase 7: Polish (Cierre y asuntos transversales)

**Propósito**: pruebas transversales de seguridad, de datos rotos y de invariantes, documentación
viva, cobertura, vulnerabilidades y verificación de las puertas de calidad.

**Rama**: `fase/008-polish`

- [ ] T045 Pruebas de integración de caracterización en
      `backend/internal/metrics/delivery/handler_test.go`, **sin** etiqueta `008/USn`:
      `TestMetricsEndpoints_SinSesion`, con un subtest por cada uno de los **cuatro** endpoints sin
      cabecera y con token adulterado → `401 {"error":"Se requiere iniciar sesión."}` (SC-013);
      `TestMetricsEndpoints_NotFoundIndistinguible`, que compara byte a byte el `404` de proyecto
      inexistente, ajeno y mal formado en las cuatro rutas, y el de sprint inexistente, de otro
      proyecto y mal formado en `.../metrics/sprints/{sprintId}` (SC-014);
      `TestMetrics_DataInconsistency` (SC-020, RN-30), que con `forceRow` deja una historia sin Story
      Points dentro de un sprint Cerrado y otra dentro del Activo, un Cerrado sin factor congelado y
      otro sin `closed_at`, y verifica en las cuatro rutas que la respuesta es `200`, completa, con la
      historia contada como 0 puntos y la advertencia `data_inconsistency` correspondiente con su
      `sprintId` y `storyId`; `TestMetrics_Repeatable` (SC-012: dos consultas seguidas sin escrituras
      devuelven cuerpos idénticos); `TestMetrics_FinishedProject` (FR-043, RN-22: las cuatro rutas
      responden `200` sobre un proyecto Finalizado con la 002); y `TestMetrics_DoesNotWrite` (RN-02:
      el `updated_at` y la cantidad de filas de las tablas leídas no cambian después de consultar las
      cuatro rutas); y `TestMetrics_SingleSourceOfFormulas` en
      `backend/internal/metrics/delivery/architecture_test.go` (RN-01, SC-021), que recorre con
      `go/build` de la librería estándar todos los paquetes de `backend/` y falla si algún paquete
      fuera de `internal/metrics/` y `cmd/app/` importa `internal/metrics/...`, o si
      `internal/metrics/delivery` importa la capa `delivery` de otro módulo, sin dependencias nuevas.
      Caracterización: las reglas ya existen, así que no tiene RED; el commit lo
      justifica — depende de T044
- [ ] T046 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las **cuatro**
      operaciones, sus códigos de respuesta (todas declaran `401`, `404` y `500`), sus esquemas, los
      enums `ValueStatus` y `Finality`, los cuatro `code` de `Warning` y la anulabilidad de `value`,
      `finality`, `closedAt` y `metrics`; corregir las anotaciones si difieren (Principio VIII) —
      depende de T045
- [ ] T047 [P] Actualizar `README.md` de la raíz en español: la sección introductoria menciona el
      cálculo de métricas, y se agregan los cuatro endpoints y una explicación breve de los estados
      `available`/`not_calculable`/`not_applicable` y `definitive`/`provisional`/`partial`, manteniendo
      el criterio de que alguien que no conoce el proyecto pueda ejecutarlo siguiendo el documento —
      depende de T045
- [ ] T048 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T049 [P] Ejecutar `go test -p 1 ./... -coverprofile=coverage.out` y
      `go tool cover -func=coverage.out` en `backend/`, y revisar que `internal/metrics/domain/` tenga
      cubiertas todas sus ramas y que `internal/metrics/` en conjunto esté cubierto; correr además
      `go test -race -p 1 ./internal/metrics/...` — depende de T045
- [ ] T050 Verificar la trazabilidad y las puertas de calidad recorriendo `backend/internal/metrics/`:
      que buscar `008/US1` a `008/US5` en el código de pruebas devuelva los **61** escenarios; que las
      **31** reglas RN-01 a RN-31 tengan prueba asociada según la tabla de plan.md (SC-021); que los
      **cuatro** endpoints tengan su prueba de `401` (SC-013) y los `404` sean indistinguibles
      (SC-014); que `TestMetrics_SingleSourceOfFormulas` pase (RN-01, Principio III); que no haya dependencias nuevas en `go.mod`; que
      ningún bloque nuevo quede sin comentario en español; y que esta feature no escriba en ninguna
      tabla — depende de T045
- [ ] T051 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluida la
      validación manual de la sección 4, la prueba de coherencia con `-count=5` y la verificación de
      que las pruebas corrieron contra `smye_test` en el puerto 5433 y no contra desarrollo — depende
      de T046, T047, T050

---

## Phase 8: Convergence and Security

Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.

- [ ] T052 Eliminar en backend/internal/metrics/domain/project.go el recálculo de CalculateSprint
      en calculateVelocity, reutilizando las métricas de cada Cerrado ya calculadas en
      CalculateProject per Constitución I (partial) — origen: converge, detectado en
      hu/008-us3-metricas-acumuladas-del-proyecto

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Foundational**: parte de `main` con la 001 a la 006 integradas. **BLOQUEA** todas las
  historias.
- **Fases 2 a 6 (historias)**: todas dependen de la Fase 1, en orden de prioridad
  (P1 → P1 → P2 → P2 → P3). La Fase 6 necesita además la 007 completa en `main`.
- **Fase 7 Polish**: depende de que estén completas las cinco historias.

### Dependencias entre historias

Las historias **no son todas independientes en su implementación**, aunque cada una se prueba sola:

- **US1 Sprint cerrado (P1)**: puede empezar después de la Fase 1. **Aporta** `Percent`,
  `CalculateSprint` para el Cerrado, `LoadSprint`, el servicio con su firma definitiva, los DTO base,
  el entorno de pruebas (T010) y la composición en `cmd/app/main.go`, que usan todas las demás.
- **US2 Velocidad y series (P1)**: necesita US1. **Aporta** `CalculateSprint` para el Activo,
  `AverageHundredths`, `LoadProject`, `BuildSeries` y los endpoints del proyecto y de las series.
- **US3 Acumulados (P2)**: necesita `LoadProject` y la velocidad de US2; completa la respuesta del
  proyecto.
- **US4 Activo o Planificado (P2)**: necesita el Activo de US2; no depende de US3.
- **US5 Defectos (P3)**: amplía las tres lecturas y la respuesta del proyecto; necesita la 007
  completa.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T001 a T004 en paralelo; luego T005 y T006 en secuencia.
- Dentro de cada historia: las pruebas unitarias de dominio y las de integración del repositorio
  marcadas `[P]` en paralelo entre sí. Las de integración de `delivery` **no** son paralelizables
  entre historias porque las cinco comparten `handler_test.go`.
- En las implementaciones, las funciones puras de dominio marcadas `[P]` (cada una en su archivo) en
  paralelo; repositorio, servicio y `delivery` en secuencia, porque cada historia toca
  `metrics_repository.go`, `metrics_service.go`, `handler.go` y `dto.go`.
- Fase 7: T047 y T049 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las tareas de pruebas marcadas [P] pueden escribirse a la vez:
T007  backend/internal/metrics/domain/values_test.go + arithmetic_test.go
T008  backend/internal/metrics/domain/sprint_test.go
T009  backend/internal/metrics/repository/metrics_repository_test.go

# T010 (handler_test.go + testenv_test.go) va aparte: lo comparten las cinco historias.

# En la implementación, T011 (values.go + arithmetic.go) no se pisa con ninguna otra.
```

## Ejemplo de paralelismo: Historia de Usuario 2

```text
# Pruebas en archivos distintos:
T016  backend/internal/metrics/domain/arithmetic_test.go
T017  backend/internal/metrics/domain/sprint_test.go
T018  backend/internal/metrics/domain/project_test.go + series_test.go
T019  backend/internal/metrics/repository/metrics_repository_test.go
```

## Ejemplo de paralelismo: Historia de Usuario 5

```text
# Pruebas en archivos distintos:
T039  backend/internal/metrics/domain/defects_test.go
T040  backend/internal/metrics/repository/metrics_repository_test.go

# Implementación de dominio independiente del repositorio:
T042  backend/internal/metrics/domain/defects.go
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar la Fase 1 (Foundational). **CRÍTICO**: bloquea todas las historias.
2. Completar la Fase 2 (US1 Métricas de un sprint cerrado).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con los pasos 1 a 5 y 13 a 15 de la
   sección 4 de la guía de validación.
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Foundational → base lista (una fase técnica, un pull request).
2. US1 Sprint cerrado → probar sola → entregar (**MVP**: cada sprint cerrado tiene sus cifras
   exactas y reproducibles).
3. US2 Velocidad y series → probar sola → entregar (la 009 ya puede dibujar sus tres gráficos).
4. US3 Acumulados → probar sola → entregar (la foto completa del proyecto, sin horas invisibles).
5. US4 Activo o Planificado → probar sola → entregar (el sprint en curso se sigue en vivo).
6. US5 Defectos → probar sola → entregar (el eje de calidad).
7. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Foundational en conjunto.
2. US1 y US2 en secuencia: aportan el entorno de pruebas, el servicio, las lecturas y el cálculo del
   Cerrado y del Activo que usan todas.
3. Terminada US2, US3 y US4 pueden avanzar en paralelo coordinando `metrics_repository.go`,
   `metrics_service.go`, `handler.go`, `dto.go` y `handler_test.go`.
4. US5 al final, cuando la 007 esté completa.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `008/US1` a `008/US5` en el código de
  pruebas devuelve la trazabilidad completa de los 61 escenarios exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 008/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b), con el ámbito `metricas`. Los commits los registra **una
  persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni variables de entorno ni tablas. Si alguna
  tarea parece necesitar una, detenerse y pedir autorización (Principio IV).
- **Código de otras features que esta feature modifica**, todo señalado en el plan: el tipo
  `Hundredths` de la 006, movido a `platform/httpserver` con un alias (T001), y la composición en
  `cmd/app/main.go` (T015).
- **Ayudante `forceRow`**: solo prepara los datos rotos de SC-020 en T045, que ninguna API permite
  crear. Ninguna prueba lo usa para verificar una fórmula con datos válidos.
- **Fórmulas existentes fuera de esta feature** (002, 004 y 006; punto 6 del plan): no tienen tarea.
  Si el equipo decide que deleguen en `metrics`, es un cambio aparte.
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: Swagger se actualiza en la misma tarea que cada endpoint, pero la descripción de la
  feature y sus endpoints en el `README.md` se agrega una sola vez en la Fase 7 (T047). Entre el
  merge de una historia y la Fase 7, el README no lista los endpoints de métricas ya integrados. Se
  acepta porque Swagger es la referencia actualizada de la API en cada merge. Es el mismo criterio
  que aplicaron la 001 a la 007.

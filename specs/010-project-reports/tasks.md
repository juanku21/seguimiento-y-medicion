---

description: "Lista de tareas de la feature 010 — Reportes de Proyecto y de Sprint"
---

# Tareas: Reportes de Proyecto y de Sprint

**Entrada**: documentos de diseño de `specs/010-project-reports/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[contracts/ui-contract.md](./contracts/ui-contract.md), [quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) a
[`specs/008-metrics-calculation`](../008-metrics-calculation/spec.md) integradas en `main` para el
backend, y además [`specs/009-project-dashboard`](../009-project-dashboard/spec.md) para el frontend
(plan, punto 3). Del backend se usan `authdelivery.CurrentUserID` y `authdelivery.ErrorResponse`
(001), el subgrupo `/api/v1/projects/:projectId` con `RequireMembership` (002),
`metricsservice.MetricsService` (008) y `platform/testsupport.OpenDatabase`. Del frontend se usan
`postAuthorizedJson` e `isRecord` (`lib/api-client.ts`), `useAuthorizedQuery`, `SessionGuard`,
`SessionProvider`, `components/alert.tsx` y `components/button.tsx`; `listSprints` de la 004;
`formatDate` (`utils/format-date.ts`); `STORY_PRIORITY_LABELS`, `STORY_STATUS_LABELS` y
`UNESTIMATED_LABEL` (`utils/story-labels.ts`); `STORY_OUTCOME_LABELS` (`utils/sprint-labels.ts`);
`DEFECT_SEVERITY_LABELS` y `DEFECT_STATUS_LABELS` (`utils/defect-labels.ts`); y de la 009
`types/metrics.ts`, `formatDecimal`, `formatShare` y `explainNotCalculable`.

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas de integración en Go son el mecanismo
por defecto del backend. Las unitarias son obligatorias para las reglas de negocio RN-01 a RN-25, el
nombre del archivo (validación), el modelo de presentación, el documento PDF y el reconocedor de la
respuesta.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/reports/` (`domain/`, `repository/`, `service/`,
  `delivery/`); se tocan de forma puntual `backend/cmd/app/main.go`,
  `backend/internal/metrics/delivery/dto.go` y `handler.go`,
  `backend/internal/platform/database/postgres.go`, `backend/internal/platform/testsupport/database.go`
  y `backend/docs/`
- **Frontend**: página nueva `frontend/src/app/projects/[projectId]/reports/page.tsx`; módulo nuevo
  `frontend/src/features/reports/` (`api.ts`, `view-model.ts`, `pdf/`, `components/`); archivos
  nuevos `frontend/src/types/report.ts`, `frontend/src/types/pdfmake.d.ts` y
  `frontend/src/utils/report-file-name.ts`; se tocan `frontend/src/types/metrics.ts` (009),
  `frontend/src/features/projects/components/project-overview.tsx`, `frontend/package.json` y
  `frontend/package-lock.json`
- **Pruebas**: en Go, `*_test.go` en el paquete del módulo; en el frontend, junto al código que
  prueban (`*.test.ts` y `*.test.tsx`)

## Ciclo obligatorio en cada tarea

Según el Principio II y la skill `/speckit-implement`:

1. **Tarea de pruebas**: se escribe la prueba, se verifica que **falla** y se **detiene**. El
   desarrollador registra el commit con `/redactar-commit`, pie `TDD: red`.
2. **Tarea de implementación**: código mínimo que hace pasar la prueba (`TDD: green`), pausa y, si
   corresponde, refactor (`TDD: refactor`).
3. **Tarea técnica**: sin prueba asociada, sin pie TDD.

Ningún agente avanza de RED a GREEN sin ese alto.

## El reporte crece por historia

Las pruebas de escenario del backend llaman a las rutas reales con el router completo, y las del
frontend montan la vista real, `ReportsView`. Cada historia le suma al módulo lo suyo:

| Historia | Backend | Frontend |
| --- | --- | --- |
| US1 | `POST …/reports/sprints/{sprintId}`: las seis secciones del sprint con estado `complete`, y la de historias y estimaciones también `empty`; registro de generación | Selector con los sprints reportables, "Generar reporte", `buildReportView`, `ReportDocument` |
| US2 | Sin cambios | `reportFileName`, `buildPdfDocument`, `exportReportPdf` y "Exportar a PDF" |
| US3 | `POST …/reports/project`: grupos por sprint, no planificadas, período y métricas del proyecto | Opción "Proyecto completo" y el modelo de presentación del alcance de proyecto |
| US4 | Estados `empty` de esfuerzo, métricas y defectos; `unavailable` con lecturas independientes | Textos y roles de `empty` y `unavailable` en pantalla y en el PDF |

## Cómo se preparan los escenarios

- **Backend** (`reports/delivery/testenv_test.go`, T006): la base de test, el router completo de la
  aplicación (auth, proyectos, backlog, sprints, poker, esfuerzo, defectos, métricas y reportes) y un
  reloj inyectable compartido por `MetricsService` y `ReportService`. Se siembra por la API de cada
  feature y, para lo que no se puede crear por la API en el instante elegido (sesiones de Planning
  Poker con su `closed_at`, rondas, cierres en una fecha), con `forceRow`. Los datos son los de la
  spec: 4 historias y 3 defectos en US1-1, una sesión Finalizada de 3 rondas con valor 8 en US1-3,
  etcétera.
- **Comparación de dos reportes** (US3-9, SC-017): se comparan los dos cuerpos JSON después de
  borrar `header.generatedAt` y `header.generatedBy`.
- **Fallas de sección** (US4-6): `ReportService` se compone con un `MetricsSource` o un
  `ReportRepository` de prueba que envuelve al real y devuelve error en **una** operación.
- **Frontend** (`reports-test-backend.ts`, T009): sustituye `fetch` y responde
  `GET /api/v1/projects/{id}/sprints` y las dos operaciones `POST` del contrato con cuerpos
  completos. `export-pdf` se sustituye con `vi.mock` en cada archivo de prueba que lo necesite.
- Las pruebas sin etiqueta `010/USn` (sin sesión, identificador mal formado, igualdad con la 008,
  registro, `500` al guardar, exportar sin consultas) no alteran el conteo de 35.

---

## Fase 1: Setup (Inicialización)

**Propósito**: instalar la dependencia de PDF autorizada el 2026-10-30.

**Rama**: `fase/010-foundational`, la misma que la Fase 2: las dos forman un único pull request.

- [X] T001 Instalar en `frontend/` la dependencia autorizada con versión exacta:
      `npm install --save-exact pdfmake@0.3.13`. Verificar que `frontend/package.json` la lista en
      `dependencies` **sin** `^` ni `~`, que `frontend/package-lock.json` quedó actualizado, que no se
      agregó ninguna otra dependencia directa (en particular **no** `@types/pdfmake`, plan punto 2) y
      que `npm run build`, `npm run lint` y `npm test` siguen pasando. Si `npm` pide algo más,
      detenerse y avisar al desarrollador (Principio IV; research.md sección 14)

---

## Fase 2: Foundational (Prerrequisitos bloqueantes)

**Propósito**: la tabla del registro, los tipos del dominio y del frontend, el esqueleto de las dos
capas con sus rutas y los ayudantes de prueba compartidos. Todas las tareas son **técnicas**:
declaran tipos, esquema, composición y ayudantes, sin reglas de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/010-foundational`

- [X] T002 [P] Declarar los tipos del dominio en `backend/internal/reports/domain/`, con comentario
      de paquete ("Package domain declara las entidades, los tipos de valor, las reglas y las
      interfaces del módulo de reportes; no depende de ninguna otra capa"):
      `scope.go` con `ReportScope` (`ScopeSprint = "sprint"`, `ScopeProject = "project"`),
      `SectionState` (`complete`, `empty`, `unavailable`), `StoryResult` (`pending`, `in_progress`,
      `completed`, `not_completed`) y `PokerState` (`agreed`, `no_agreement`, `none`), todos `string`;
      `report.go` con `Report`, `Header`, `ProjectInfo`, `UserInfo`, `SprintRef`, `Period`,
      `StoriesSection`, `StoryGroup`, `StoryLine`, `EstimationsSection`, `EstimationGroup`,
      `EstimationLine`, `PokerSummary`, `SessionRecord`, `EffortSection`, `EffortGroup`, `StoryEffort`,
      `MemberEffort`, `EffortRow`, `MetricsSection` (con `Sprint *metricsdomain.SprintMetrics`,
      `SprintWarnings []metricsdomain.Warning` y `Project *metricsdomain.ProjectMetrics`),
      `DefectsSection`, `DefectLine` y `DefectTotals`, con los campos exactos de data-model.md
      secciones 2.2, 2.3 y 2.5; `errors.go` con `ErrSprintNotFound` ("el sprint no existe o es de otro
      proyecto") y `ErrSprintNotStarted` ("el sprint todavía no arrancó"); `ports.go` con
      `ReportRepository`, `GenerationRepository` y `MetricsSource` con las firmas exactas de
      data-model.md sección 2.5; y `generation.go` con `ReportGeneration`, con los tags de GORM de
      data-model.md sección 1: `ID uuid.UUID` `type:uuid;primaryKey`; `ProjectID uuid.UUID`
      `type:uuid;not null;index:idx_report_generations_project_generated,priority:1` y
      `Project *projectsdomain.Project` `foreignKey:ProjectID;references:ID`; `SprintID *uuid.UUID`
      `type:uuid` ("Nulo en el alcance `project`") y `Sprint *sprintsdomain.Sprint`;
      `Scope ReportScope` `type:varchar(8);not null`; `GeneratedByID uuid.UUID` `type:uuid;not null`
      y `GeneratedBy *authdomain.User`; `GeneratedAt time.Time`
      `type:timestamptz;not null;index:idx_report_generations_project_generated,priority:2`;
      `CreatedAt` y `UpdatedAt` `type:timestamptz;not null`. El comentario del tipo explica que los
      punteros a entidades existen solo para que `AutoMigrate` cree las claves foráneas, **sin**
      cascada, y que el registro nunca se actualiza ni se elimina. Sin funciones todavía
- [X] T003 Registrar la tabla en la plataforma: en
      `backend/internal/platform/database/postgres.go`, `&reportsdomain.ReportGeneration{}` al final
      de la lista de `AutoMigrate`, con su importación; en
      `backend/internal/platform/testsupport/database.go`, `report_generations` al final del
      `TRUNCATE … CASCADE`. Verificar con la base de test levantada que `go test ./...` sigue pasando
      y que `\d report_generations` en `smye-test-postgres-1` muestra las tres claves foráneas y el
      índice `idx_report_generations_project_generated` — depende de T002
- [X] T004 [P] Exportar en `backend/internal/metrics/delivery/dto.go` los constructores
      `newSprintMetrics` → `NewSprintMetrics` y `newProjectMetrics` → `NewProjectMetrics`, con un
      comentario que diga que también los usa el módulo de reportes para que la sección de métricas
      tenga exactamente la forma de la 008 (research.md sección 3), y actualizar sus llamadas en
      `backend/internal/metrics/delivery/handler.go`. Sin otro cambio: `go test ./internal/metrics/...`
      pasa igual
- [X] T005 Crear el esqueleto del módulo y su composición, sin lógica: en
      `backend/internal/reports/repository/report_repository.go`, `ReportRepository` con
      `NewReportRepository(db *gorm.DB)` y las ocho operaciones del puerto devolviendo
      `errors.New("no implementado")`; en `generation_repository.go`, `GenerationRepository` con
      `NewGenerationRepository(db)` y `Save` igual; en
      `backend/internal/reports/service/report_service.go`, `ReportService` con
      `NewReportService(repo domain.ReportRepository, generations domain.GenerationRepository,
      metrics domain.MetricsSource, now func() time.Time)` y los métodos
      `SprintReport(ctx, projectID, sprintID, userID uuid.UUID) (domain.Report, error)` y
      `ProjectReport(ctx, projectID, userID uuid.UUID) (domain.Report, error)` devolviendo
      `errors.New("no implementado")`; en `backend/internal/reports/delivery/handler.go`, `Handler`
      con `NewHandler(reports *service.ReportService)` y los manejadores `generateSprintReport` y
      `generateProjectReport`, que por ahora responden el `500` genérico ("Ocurrió un error
      inesperado. Intentá de nuevo más tarde."); en `routes.go`,
      `RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler)` con
      `projectGroup.POST("/reports/sprints/:sprintId", …)` y `projectGroup.POST("/reports/project", …)`.
      En `backend/cmd/app/main.go`, después del bloque de métricas, componer el módulo sobre
      `reportsrepository.NewReportRepository(db)`, `reportsrepository.NewGenerationRepository(db)`, la
      **misma** instancia `metricsCalculator` y `time.Now`, con un comentario de bloque como los
      demás (cuelga del subgrupo `/:projectId`, que ya exige sesión y membresía; reutiliza el
      servicio de métricas en proceso, research.md secciones 1 y 3). `go build ./...` y `go vet ./...`
      pasan — depende de T002, T004
- [X] T006 Crear en `backend/internal/reports/delivery/testenv_test.go` el entorno de pruebas
      compartido, con la misma estructura que `backend/internal/metrics/delivery/testenv_test.go`
      (que no se puede importar por ser `_test.go`): `reportsTestEnv` con la base de
      `testsupport.OpenDatabase`, un reloj inyectable (`Now`, `Set`, `Advance`) compartido por
      `MetricsService`, `ReportService` y los servicios de las demás features, y el router completo
      armado como en `cmd/app/main.go`; `newReportsTestEnvWith(t, compose)`, que permite reemplazar
      el `ReportRepository` o el `MetricsSource` que recibe `ReportService` (para US4-6); ayudantes
      `do`, `requireStatus`, `signUp`, `createProject`, `addMember`, `removeMember`, `finishProject`,
      `createStory(…, title, priority, points)`, `estimateStory`, `renameStory`, `changePriority`,
      `createSprint`, `startSprint`, `completeStory`, `closeSprintAt`, `logHours`, `createDefect`,
      `changeDefectStatus`, `forceRow`; `seedPokerSession(t, storyID, status, rounds, agreed,
      createdAt, closedAt)` con `forceRow` sobre `estimation_sessions` y `estimation_rounds`;
      `generateSprintReport(t, token, projectID, sprintID)` y `generateProjectReport(t, token,
      projectID)`, que devuelven el `*httptest.ResponseRecorder`; `decodeReport(t, rec) map[string]any`;
      `withoutGeneration(report)`, que borra `header.generatedAt` y `header.generatedBy`; y
      `generationRows(t, projectID) []domain.ReportGeneration`, que lee `report_generations`. Cada
      ayudante con su comentario en español — depende de T003, T005
- [X] T007 [P] Declarar los tipos del frontend: en `frontend/src/types/report.ts`, con comentario de
      cabecera ("Tipos de la respuesta de los reportes, según
      specs/010-project-reports/contracts/openapi.yaml"), `ReportScope`, `SectionState`,
      `GroupState`, `StoryResult`, `PokerState`, `ReportSprint`, `Period`, `ReportHeader`,
      `StoryLine`, `StoryGroup`, `StoriesSection`, `PokerSummary`, `EstimationLine`,
      `EstimationGroup`, `EstimationsSection`, `StoryEffort`, `EffortGroup`, `MemberEffort`,
      `EffortSection`, `MetricsSection` (con `sprint: SprintMetrics | null` y
      `project: ProjectMetricsResponse | null` de `types/metrics.ts`), `DefectLine`, `DefectsSection`
      y `Report`, con los campos, nulos y valores exactos del contrato; en
      `frontend/src/types/metrics.ts` (**TOCA** la 009), sumar sin cambiar los existentes los campos
      que el dashboard no declaró y el reporte muestra: `SprintMetrics` con las nueve métricas
      (`defectsDetected` y `defectsResolved` incluidas) y su `sprint`, y en `ProjectMetricsResponse`
      `velocity.sprintsConsidered`, `effortOutsideSprintsHours`, `effortInPlannedSprintsHours`,
      `unestimatedBacklogStories` y `defects.detected` y `defects.resolved`, según
      `specs/008-metrics-calculation/contracts/openapi.yaml`; y en `frontend/src/types/pdfmake.d.ts`,
      `declare module "pdfmake/build/pdfmake"` y `declare module "pdfmake/build/vfs_fonts"` con solo
      lo que se usa (research.md sección 14): `PdfContent` (texto con estilo, `stack`, tabla con
      `headerRows`, `dontBreakRows`, `widths` y `body`), `PdfDocumentDefinition` (`content`,
      `footer(currentPage: number, pageCount: number)`, `pageMargins`, `styles`, `defaultStyle`,
      `info.title`), `createPdf(definition): { download(fileName: string): Promise<void> }` y
      `addVirtualFileSystem(vfs: Record<string, string>): void`. Sin `any`. `npx tsc --noEmit` pasa
- [X] T008 [P] Leer antes `frontend/node_modules/next/dist/docs/01-app/01-getting-started/03-layouts-and-pages.md`
      y `05-server-and-client-components.md` (CLAUDE.md). Crear
      `frontend/src/app/projects/[projectId]/reports/page.tsx`, un Server Component con
      `metadata.title = "Reportes | Software Metrics & Estimation"` que lee `params` como `Promise`,
      muestra el `h1` "Reportes" y monta `<SessionGuard><ReportsView projectId={projectId} />
      </SessionGuard>`, con la misma estructura que `app/projects/[projectId]/dashboard/page.tsx`.
      Crear `frontend/src/features/reports/components/reports-view.tsx` con `"use client"`,
      `ReportsView({ projectId }: ReportsViewProps)` y un cuerpo que todavía no muestra nada
      (`return null`), con el comentario de qué va a componer
- [X] T009 Crear en `frontend/src/features/reports/components/reports-test-backend.ts` el ayudante
      de pruebas compartido, que importan **solo** archivos `*.test.tsx`:
      `installReportsBackend(overrides?)` reemplaza `globalThis.fetch` con `vi.fn` y responde por
      método y ruta `GET /api/v1/projects/{id}/sprints`, `POST …/reports/sprints/{sprintId}` y
      `POST …/reports/project`, con `404 {"error":"El proyecto no existe."}` para cualquier otro
      proyecto; cada operación se puede reemplazar con `{ status, body }`; devuelve la lista de
      peticiones (método y ruta). Exportar las fábricas `sprintReport(overrides?)` y
      `projectReport(overrides?)`, que arman un `Report` completo y válido según el contrato con los
      datos de la spec (US1-1: 4 historias, 3 defectos con uno de cada severidad, esfuerzo de 2
      integrantes, métricas del sprint Cerrado con `finality` definitiva y provisoria; US3-1: 3
      sprints Cerrados), y `renderReports(projectId)`, que guarda un token con `writeToken` y monta
      `ReportsView` dentro de `SessionProvider`, como `project-effort-view.test.tsx`. El
      `vi.mock("next/navigation", …)` y el `vi.mock` de `export-pdf` los declara cada archivo de
      prueba, porque Vitest solo eleva los `vi.mock` del propio archivo — depende de T007, T008

**Checkpoint**: `go build ./...`, `go test ./...`, `npm run build`, `npm run lint` y `npm test`
pasan sin pruebas nuevas; las dos rutas existen detrás de sesión y membresía y responden `500`; y
`/projects/{id}/reports` existe, protegida por la guardia de sesión. El enlace desde la vista del
proyecto llega en US1 (T022), con su prueba. Las historias pueden empezar.

---

## Fase 3: Historia de Usuario 1 — Generar y ver el reporte de un sprint (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante elige un sprint Cerrado o Activo, genera su reporte y ve las seis
secciones con los datos correctos, marcado como parcial si el sprint está en curso, y la generación
queda registrada.

**Rama**: `hu/010-us1-generar-y-ver-el-reporte-de-un-sprint`

**Prueba independiente**: se cierra un sprint con 4 historias, esfuerzo y 3 defectos, se genera su
reporte y se verifican las seis secciones, el `200`, la fila de `report_generations` y la pantalla
con el `article` "Reporte".

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T010 [P] [US1] Pruebas unitarias **obligatorias** en
      `backend/internal/reports/domain/rules_test.go`, con tablas y `t.Run`: `EnsureReportable` con
      `planned` → `ErrSprintNotStarted` (RN-04), y con `active` y `closed` → nil; `IsPartial` en el
      alcance `sprint` con un Activo → `true` y con un Cerrado → `false` (RN-03); `SortStories` con
      prioridades mezcladas, dos títulos iguales en la misma prioridad y una lista ya ordenada →
      `high`, `medium`, `low`, después título y después `story_id`, y el resultado no depende del
      orden de entrada (RN-22); `StoryTitle` con la historia en la instantánea (devuelve el de la
      instantánea aunque el vigente sea otro) y fuera de ella (devuelve el vigente) (FR-021); y
      `StoriesState` con todos los grupos vacíos → `empty`, y con uno con historias → `complete`
      (RN-11). Crear las firmas mínimas para que compile — depende de T002
- [X] T011 [P] [US1] Pruebas unitarias **obligatorias** de `SelectEstimation(sessions, cutoff
      *time.Time)` en `backend/internal/reports/domain/estimation_test.go`, cubriendo RN-09, RN-10,
      FR-013 a FR-015 y research.md sección 5: sin sesiones → `none` y `Session` nil; dos `finished`
      → la de mayor `closed_at`, con sus `Rounds` y su valor; empate de `closed_at` → mayor `id`; solo
      `canceled` → `no_agreement`; una `open` → `no_agreement`; con corte, una `finished` con
      `closed_at` posterior al corte → `no_agreement` (creada antes) y una creada después del corte →
      no cuenta (`none` si es la única); con corte, la `finished` anterior al corte gana a otra
      posterior; sin corte (`nil`), todas cuentan. Crear las firmas mínimas para que compile —
      depende de T002
- [X] T012 [US1] Pruebas de escenario de integración en
      `backend/internal/reports/delivery/handler_test.go`, una función por escenario con los nombres
      de la tabla de plan.md, cada una precedida por `// Escenario: <nombre> (010/US1)`:
      `TestUS1_ReporteDeSprintCerrado` (4 historias, 2 completadas, esfuerzo de 2 integrantes y 3
      defectos más uno Descartado: `200`; `scope` `sprint`, `partial` `false`; encabezado con el
      nombre del proyecto, `sprint`, `period` igual a las fechas del sprint, `generatedAt` igual al
      reloj y `generatedBy.fullName` del que pide; historias con título, prioridad, Story Points y
      `completed`/`not_completed` de la instantánea; estimaciones; esfuerzo por historia y por
      integrante con `totalHours`; `metrics.sprint` igual al cuerpo de
      `GET …/metrics/sprints/{id}`; los 3 defectos sin el Descartado con sus columnas y
      `totals.detected`/`resolved` iguales a los de la 008), `TestUS1_SprintActivoParcial`,
      `TestUS1_HistoriaConPlanningPoker` (`poker` `agreed`, `rounds` 3, `agreedStoryPoints` 8),
      `TestUS1_HistoriaSinPlanningPoker` (`none`, `session` null), `TestUS1_SinHistoriasCompletadas`,
      `TestUS1_SprintSinHistorias` (historias y estimaciones `empty`, encabezado y métricas
      presentes), `TestUS1_MetricaNoCalculable` (sprint de 0 horas estimadas: `deviationPercent` con
      `status` `not_calculable` y `value` null), `TestUS1_SprintPlanificado` (`409` con "El sprint
      todavía no arrancó: no tiene nada que reportar." y **ninguna** fila nueva),
      `TestUS1_SprintDeOtroProyecto` (`404` con "El sprint no existe.", idéntico al de un UUID
      inexistente, sin fila) y `TestUS1_NoIntegrante` (`404` idéntico al de un proyecto inexistente,
      sin fila). Agregar, **sin** etiqueta, `TestReportRequiresSession` (`401` en las dos rutas),
      `TestReportMalformedSprintID` (`404` del sprint sin consultar), `TestReportEffortMatchesMetrics`
      (`effort.totalHours` igual a `metrics.sprint.actualHours.value`), `TestReportWritesOnlyGeneration`
      (RN-02: después de generar, el `updated_at` de proyectos, historias, sprints, defectos y
      registros de esfuerzo no cambió y la única fila nueva es la de `report_generations`) y
      `TestReportGenerationRow` (RN-21, RN-24: una fila por `200` con `scope` `sprint`, `sprint_id`,
      `generated_by_id` del titular de la sesión y `generated_at` igual a `header.generatedAt`). En
      `backend/internal/reports/service/report_service_test.go`, `TestSprintReportSaveFails`
      (SC-018): con un `GenerationRepository` de prueba que devuelve error, `SprintReport` devuelve
      error y no un reporte — depende de T006
- [X] T013 [P] [US1] Pruebas unitarias **obligatorias** en `frontend/src/features/reports/api.test.ts`
      y `frontend/src/features/reports/view-model.test.ts`: `isReport` acepta `sprintReport()` y
      rechaza un cuerpo sin `header`, con `stories.state` desconocido, con `storyPoints` `"8"` y con
      una sección faltante (Principio VII); `buildReportView(sprintReport(), "America/Argentina/Buenos_Aires")`
      devuelve siempre **seis** `SectionView` en el orden `header`, `stories`, `estimations`, `effort`,
      `metrics`, `defects`; el título "Reporte del sprint <nombre>"; `partialNote` `null` con
      `partial: false` y "Reporte parcial: incluye un sprint en curso." con `true`; los términos del
      encabezado de contracts/ui-contract.md, con "Generado el" en `DD/MM/AAAA HH:mm:ss` de la zona
      indicada; las columnas fijas de cada tabla; "Sin estimar" para `storyPoints` null; "No
      completada" para `not_completed`; "3 rondas, valor acordado 8", "1 ronda, valor acordado 5",
      "Sin valor acordado por estimación colaborativa" y "No hubo sesión de estimación"; y
      `metricText` con `available` 0 → "0", `not_calculable` en `velocity`, `deviationPercent` y
      `storyCompletionPercent` → "No calculable: <explicación>" y **nunca** "0" (RN-13), `not_applicable`
      → "No aplica", y las marcas " · Parcial", " · Provisoria" y " · Definitiva", distintas entre sí
      (RN-14). Crear las firmas mínimas para que compile — depende de T007
- [X] T014 [US1] Pruebas de escenario en
      `frontend/src/features/reports/components/reports-view.test.tsx`, montando `ReportsView` con
      `renderReports`, con `describe("010/US1 - Generar y ver el reporte de un sprint")` y un `it` por
      escenario con los nombres de la tabla de plan.md, cada uno precedido por
      `// Escenario: US1-k (<caso>)`. Cada uno elige el sprint en el `combobox` "Alcance del reporte",
      pulsa "Generar reporte" y verifica el `article` "Reporte": (1) las seis `region` en el orden de
      contracts/ui-contract.md con sus datos; (2) el `status` "Reporte parcial: incluye un sprint en
      curso." y la opción "<nombre> (Activo)"; (3) "3 rondas, valor acordado 8"; (4) "No hubo sesión
      de estimación"; (5) ninguna fila con "Completada"; (6) "Sin datos." en Historias y
      Estimaciones, con el encabezado y las métricas presentes; (7) "No calculable: …" y ninguna celda
      de esa fila con "0"; (8) `409`: el `alert` con el mensaje del backend y ningún `article`; (9) el
      `404` del sprint: "El sprint no existe."; (10) el `404` del proyecto: "El proyecto no existe." y
      ningún `combobox`. Verificar también que el selector **no** ofrece sprints Planificados y que
      `listSprints` se pidió con `status=active&status=closed`. Agregar, **sin** etiqueta,
      `it("sin sesión lleva al inicio de sesión")` (SC-014). En
      `frontend/src/features/projects/components/project-overview.test.tsx`, junto a las pruebas de
      los otros enlaces, `it("ofrece el enlace a los reportes")`: la navegación "Secciones del
      proyecto" tiene "Reportes" con `href` `/projects/{id}/reports` — depende de T009

### Implementación de la Historia 1

- [X] T015 [P] [US1] Implementar en `backend/internal/reports/domain/rules.go` `EnsureReportable`,
      `IsPartial`, `SortStories`, `StoryTitle` y `StoriesState`, como funciones puras con su
      comentario en español y la referencia a la regla (data-model.md sección 2.4) — depende de T010
- [X] T016 [P] [US1] Implementar en `backend/internal/reports/domain/estimation.go`
      `SelectEstimation(sessions []SessionRecord, cutoff *time.Time) (PokerState, *PokerSummary)`
      según research.md sección 5, sin consultar nada — depende de T011
- [X] T017 [US1] Implementar en `backend/internal/reports/repository/report_repository.go`, con
      `tx.Raw` y parámetros con nombre como `metrics/repository`, cada lectura en su propia consulta y
      cada error envuelto con `fmt.Errorf("…: %w", err)`: `LoadHeader` (proyecto por id y
      `users.full_name` del que pide); `LoadSprint` acotado al `project_id` (`ErrSprintNotFound` con
      0 filas); `LoadSprintStories` con la instantánea (`title_at_close`, `priority_at_close`,
      `story_points_at_close`, `outcome`) en un Cerrado y los compromisos abiertos
      (`outcome IS NULL`) unidos a `user_stories` vigente en el Activo (research.md sección 4);
      `LoadEstimationSessions` de las historias pedidas, con `COUNT` de `estimation_rounds` por
      sesión; `LoadEffort` con `GROUP BY sprint_id, story_id, user_id` sobre `effort_entries` unido a
      `user_stories` y `users` para los `sprint_id` pedidos (research.md sección 6); y `LoadDefects`
      con `sprintID`: `status <> 'discarded' AND (detected_sprint_id = @sprint OR resolved_sprint_id
      = @sprint)`, unido a `user_stories` y a los dos sprints, en orden `start_date` del sprint de
      detección, `created_at`, `id` (research.md sección 7). `ListSprints` y `LoadUnplannedStories`
      quedan para US3. Implementar en `generation_repository.go` `Save` con `db.Create` — depende de
      T012
- [X] T018 [US1] Implementar en `backend/internal/reports/service/report_service.go` `SprintReport`:
      tomar `now` una sola vez; `LoadSprint` y `EnsureReportable` (propagar `ErrSprintNotFound` y
      `ErrSprintNotStarted` con `%w`); `LoadHeader`; historias con `SortStories` y `StoriesState`;
      estimaciones con los mismos grupos, `SelectEstimation` con corte `ClosedAt` en el Cerrado y
      `nil` en el Activo; esfuerzo agrupado con `StoryTitle` contra las historias del sprint; métricas
      con `MetricsSource.SprintMetrics`; defectos con `StoryTitle` y totales de
      `DefectsDetected`/`DefectsResolved` de la 008; `Partial` con `IsPartial`; y por último
      `GenerationRepository.Save` con `GeneratedAt = now`; si falla, devolver el error y ningún
      reporte (research.md sección 10). En esta historia, toda falla de lectura se propaga; los
      estados `unavailable` llegan en US4 — depende de T015, T016, T017
- [X] T019 [US1] Implementar en `backend/internal/reports/delivery/dto.go` la forma JSON del contrato
      (`Report`, `Header`, secciones, grupos y líneas, con `json` en camelCase, `validate:"required"`,
      `extensions:"x-nullable"` donde el contrato lo marca y horas como `httpserver.Hundredths`), con
      `metrics.sprint` armado con `metricsdelivery.NewSprintMetrics`; y en
      `backend/internal/reports/delivery/handler.go` `generateSprintReport`, con la anotación Swagger
      completa en español (`@Summary`, `@Description`, `@Tags Reportes`, `@ID generateSprintReport`,
      `@Security BearerAuth`, `@Param`, `@Success 200 {object} Report` y `@Failure` 401, 404, 409 y
      500 con `delivery.ErrorResponse`): `uuid.Parse` del `sprintId` (mal formado → `404` "El sprint
      no existe."), `authdelivery.CurrentUserID`, y la traducción con `errors.Is` de
      `ErrSprintNotFound` → `404` y `ErrSprintNotStarted` → `409` "El sprint todavía no arrancó: no
      tiene nada que reportar."; cualquier otro error → `500` genérico registrado con `slog`
      (Principio VII) — depende de T018
- [X] T020 [P] [US1] Implementar en `frontend/src/features/reports/api.ts` el reconocedor `isReport`
      sobre `isRecord`, sin `any`, que verifica las seis secciones, sus `state` y los tipos de cada
      campo del contrato (las métricas con los reconocedores de la 009 si están exportados; si no,
      con los propios), y `generateSprintReport(projectId, sprintId)` con
      `postAuthorizedJson(path, {}, isReport)` sobre `/api/v1/projects/{projectId}/reports/sprints/{sprintId}`,
      con el comentario de que el backend no lee el cuerpo — depende de T013
- [X] T021 [P] [US1] Implementar en `frontend/src/features/reports/view-model.ts` los tipos
      `ReportView`, `SectionView` y `BlockView` de data-model.md sección 3.2 y las funciones puras
      `buildReportView(report, timeZone)` y `metricText(metric, kind)` para el alcance `sprint`, con
      los textos exactos de contracts/ui-contract.md y los formateadores y etiquetas listados en
      "Base existente"; ninguna cadena se arma fuera de este archivo (research.md sección 13). Los
      estados `empty` se resuelven con "Sin datos."; `unavailable` llega en US4 — depende de T013
- [X] T022 [US1] Implementar los componentes en `frontend/src/features/reports/components/`:
      `report-section.tsx` (`ReportSection({ section }: { section: SectionView })`: una `region` con
      `h3`, "Sin datos." con `role="note"` y los bloques con `h4` y `<table>` con `<thead>`, `<tbody>`
      y `<tfoot>`); `report-document.tsx` (`ReportDocument({ view }: { view: ReportView })`: `article`
      "Reporte" con el `h2`, el `status` de la marca parcial y las seis secciones, sin formatear nada);
      `report-scope-select.tsx` (`combobox` "Alcance del reporte" con los sprints de
      `listSprints(projectId, ["active", "closed"])` como "<nombre> (Activo)" o "<nombre> (Cerrado)");
      y en `reports-view.tsx` el flujo `idle → generating → shown | error` de data-model.md sección
      3.3, con "Generar reporte"/"Generando…", el `Alert` con los mensajes de contracts/ui-contract.md,
      y `buildReportView` con `Intl.DateTimeFormat().resolvedOptions().timeZone`, como
      `story-detail-card.tsx`. Sumar el enlace "Reportes" a `/projects/${project.id}/reports` en la
      navegación "Secciones del proyecto" de
      `frontend/src/features/projects/components/project-overview.tsx` (**TOCA**), después de
      "Dashboard" — depende de T014, T020, T021

**Checkpoint**: `go test ./internal/reports/...` y `npm test` pasan; US1 funciona sola: los pasos 1 y
4 de la sección 3 de [quickstart.md](./quickstart.md) y las verificaciones de la sección 2 sobre el
reporte de sprint.

---

## Fase 4: Historia de Usuario 2 — Exportar el reporte a PDF (Prioridad: P1)

**Objetivo**: el integrante descarga en un paso un PDF con el mismo contenido que la pantalla, con
páginas numeradas, la marca de parcial y un nombre saneado que identifica proyecto, alcance y
fecha.

**Rama**: `hu/010-us2-exportar-el-reporte-a-pdf`

**Prueba independiente**: con un reporte de sprint en pantalla, "Exportar a PDF" entrega a
`exportReportPdf` un documento cuyos textos son los de la pantalla y un nombre que cumple la regla;
si la exportación falla, la pantalla sigue igual.

### Pruebas de la Historia 2 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T023 [P] [US2] Pruebas unitarias **obligatorias** de `reportFileName` en
      `frontend/src/utils/report-file-name.test.ts`, cubriendo RN-17, FR-029, FR-030 y los ejemplos
      de contracts/ui-contract.md: "Gestión de Compras", alcance proyecto, 30/10/2026 14:32:05 →
      `Reporte_Gestion-de-Compras_proyecto_2026-10-30_14-32-05.pdf`; "Ñandú: App/Web *beta*" con
      "Sprint 3" → `Reporte_Nandu-App-Web-beta_sprint-Sprint-3_2026-10-30_09-05-00.pdf`; un nombre con
      `\ / : * ? " < > |`, tabulaciones y espacios repetidos no deja ninguno de esos caracteres ni
      guiones repetidos o en los extremos; un nombre que queda vacío después de sanear → `proyecto` o
      `sprint`; un nombre de 200 caracteres se limita a 60; dos instantes del mismo día que difieren en
      un segundo dan nombres distintos; la fecha y la hora salen de la zona horaria recibida (un
      instante de las 01:30 UTC con `America/Argentina/Buenos_Aires` cae en el día anterior). Crear la
      firma mínima para que compile
- [X] T024 [P] [US2] Pruebas unitarias **obligatorias** de `buildPdfDocument(view)` en
      `frontend/src/features/reports/pdf/pdf-document.test.ts`, sobre
      `buildReportView(sprintReport(), zona)`: un ayudante del archivo de prueba recorre `content` y
      extrae los textos en orden, y son exactamente los de `view` en el mismo orden (RN-15); el primer
      bloque es `view.title`; con `partialNote`, la nota aparece debajo del título y
      `footer(1, 3)` incluye "Página 1 de 3" y la nota; sin ella, `footer(2, 3)` es solo "Página 2 de
      3" (RN-16, FR-028); **toda** tabla tiene `headerRows: 1` y `dontBreakRows: true` y su primera
      fila son las columnas de la vista (FR-032); con un reporte de 80 historias, la tabla de historias
      tiene 81 filas y ninguna se pierde; los textos con acentos, eñes y raya llegan sin cambios
      (FR-031); y `defaultStyle.font` es `Roboto`. Crear la firma mínima para que compile — depende de
      T021
- [X] T025 [US2] Pruebas de escenario en
      `frontend/src/features/reports/components/reports-view.test.tsx`, con
      `vi.mock("../pdf/export-pdf", …)` que captura el documento y el nombre, y
      `describe("010/US2 - Exportar el reporte a PDF")` con un `it` por escenario según la tabla de
      plan.md, cada uno precedido por `// Escenario: US2-k (<caso>)`: (1) después de generar, un clic
      en "Exportar a PDF" llama una vez a `exportReportPdf` con un documento cuyos textos coinciden
      sección por sección con el `textContent` de cada `region` del `article` "Reporte" y un nombre
      con el proyecto, `sprint-<nombre>` y la fecha; (2) con un reporte parcial, el documento lleva la
      nota debajo del título y en el pie; (3) el nombre de un reporte de proyecto lleva `_proyecto_` y
      difiere del de sprint del mismo día; (4) un reporte de 80 historias: la tabla de historias del
      documento tiene 81 filas con `headerRows: 1`; (5) un proyecto "Ñandú: Gestión/Compras *beta*":
      el documento conserva el nombre y el archivo se llama
      `Reporte_Nandu-Gestion-Compras-beta_…pdf`; (6) dos generaciones con `generatedAt` distintos el
      mismo día dan dos nombres distintos; (7) con `exportReportPdf` rechazado: el `alert` "No se pudo
      generar el PDF. Intentá de nuevo.", el `article` con el mismo `textContent` que antes, el botón
      habilitado y un segundo clic que vuelve a llamar a `exportReportPdf` **sin** un nuevo `POST`;
      (8) quien no es integrante recibe el `404` y no hay ningún botón "Exportar a PDF". Agregar,
      **sin** etiqueta, `it("exportar no hace ninguna consulta")` (RN-02, research.md sección 16) —
      depende de T022

### Implementación de la Historia 2

- [X] T026 [P] [US2] Implementar en `frontend/src/utils/report-file-name.ts`
      `reportFileName({ projectName, scope, sprintName, generatedAt, timeZone })` según research.md
      sección 15, con `normalize("NFD")`, la eliminación de `\p{M}`, el reemplazo de todo lo que no sea
      `[A-Za-z0-9-]`, el colapso de guiones, el límite de 60 por parte y los valores por defecto; la
      fecha y la hora con `Intl.DateTimeFormat` en la zona recibida, como `formatInstant` — depende de
      T023
- [X] T027 [P] [US2] Implementar en `frontend/src/features/reports/pdf/pdf-document.ts`
      `buildPdfDocument(view: ReportView): PdfDocumentDefinition` según research.md sección 14 y la
      tabla "PDF" de contracts/ui-contract.md: título, nota parcial, seis secciones con su título y su
      mensaje, cada bloque con su subtítulo y su tabla (`headerRows: 1`, `dontBreakRows: true`,
      `widths` con `*` en la columna de texto largo y `auto` en las demás), `footer` con "Página N de M"
      y la nota, `pageMargins`, estilos y `defaultStyle.font = "Roboto"`. Sin formatear nada: solo lee
      `view` — depende de T024
- [X] T028 [US2] Implementar en `frontend/src/features/reports/pdf/export-pdf.ts`
      `exportReportPdf(definition, fileName): Promise<void>`, que importa con `import()`
      `pdfmake/build/pdfmake` y `pdfmake/build/vfs_fonts`, registra las fuentes con
      `addVirtualFileSystem` y llama a `createPdf(definition).download(fileName)`, con un comentario de
      por qué la carga es diferida (research.md sección 14); en `view-model.ts`, `ReportView.fileName`
      con `reportFileName`; y en `reports-view.tsx` el botón "Exportar a PDF"/"Exportando…", visible
      solo con un reporte en pantalla, con el estado `exporting` y `exportError` de data-model.md
      sección 3.3: `try/catch` alrededor de `exportReportPdf(buildPdfDocument(view), view.fileName)`,
      el `Alert` del error y **sin** tocar `report` ni `view` (FR-033). Verificar a mano en
      `npm run dev` que el archivo se descarga, con los pasos 2 a 4 de la sección 3 de quickstart.md —
      depende de T025, T026, T027

**Checkpoint**: US1 y US2 funcionan juntas: los pasos 1 a 4, 6, 7 y 8 de la sección 3 de
quickstart.md.

---

## Fase 5: Historia de Usuario 3 — Generar y ver el reporte de un proyecto completo (Prioridad: P2)

**Objetivo**: el integrante genera el reporte de todo el proyecto, con los sprints Cerrados y el
Activo en orden cronológico, el grupo final de historias no planificadas y las métricas del
proyecto.

**Rama**: `hu/010-us3-generar-y-ver-el-reporte-de-un-proyecto-completo`

**Prueba independiente**: con un proyecto de tres sprints Cerrados, uno Activo y uno Planificado, el
reporte de proyecto tiene cuatro grupos en orden, más el de no planificadas, y `partial: true`.

### Pruebas de la Historia 3 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T029 [P] [US3] Pruebas unitarias **obligatorias** en
      `backend/internal/reports/domain/rules_test.go`: `ReportableSprints` con Planificados
      intercalados → solo `active` y `closed`, con el orden recibido (`start_date ASC, id ASC`)
      intacto (RN-05, RN-07, FR-007); `ReportPeriod` con tres sprints → menor inicio y mayor fin, con
      uno → sus fechas, y sin sprints → nil; e `IsPartial` en el alcance `project` con y sin un Activo
      (RN-06). Crear las firmas mínimas para que compile — depende de T015
- [X] T030 [US3] Pruebas de escenario de integración en
      `backend/internal/reports/delivery/handler_test.go`, con los nombres de la tabla de plan.md y
      `// Escenario: <nombre> (010/US3)`: `TestUS3_ProyectoConTresSprintsCerrados` (grupos en orden
      cronológico, `period` del primero al último y `metrics.project` igual a
      `GET …/projects/{id}/metrics`), `TestUS3_ProyectoConSprintActivo` (4 grupos y `partial`
      `true`), `TestUS3_SprintPlanificadoExcluido` (ningún grupo, fila de esfuerzo ni defecto del
      Planificado, y una historia comprometida solo en él aparece en el grupo de no planificadas
      con su estado vigente), `TestUS3_ProyectoFinalizado` (`200`, `partial` `false` y la fila de
      `report_generations`, FR-035, RN-20), `TestUS3_HistoriaEnDosSprints` (la historia en los dos
      grupos, `not_completed` en el primero y `completed` en el segundo, RN-08),
      `TestUS3_HistoriasNoPlanificadas` (12 historias en el último grupo, con `sprint` null y datos
      vigentes), `TestUS3_SinHistoriasNoPlanificadas` (el último grupo con `state` `empty`),
      `TestUS3_HistoriasCambiadasDespuesDelCierre` (después de cerrar, se renombran, se cambian de
      prioridad y se reestiman, también con una sesión de Planning Poker Finalizada después del
      cierre: el grupo del Cerrado muestra los datos del cierre y la estimación de antes del corte),
      `TestUS3_ReporteReproducible` (dos generaciones con esos cambios en el medio y el reloj avanzado
      un mes: `withoutGeneration` de las dos es idéntico; RN-22, SC-017),
      `TestUS3_ProyectoSinSprints` (con subtests: sin sprints ni historias → encabezado completo
      con `period` null y las cinco secciones `empty`; sin sprints y con 3 historias → el grupo de
      no planificadas con las 3, estimaciones con esas 3 en `none`, y esfuerzo, métricas y defectos
      `empty`) y `TestUS3_ProyectoInexistente` (`404` de la 002). Agregar, **sin**
      etiqueta, `TestProjectReportEffortMatchesMetrics` (`effort.totalHours` igual a
      `metrics.project.cumulative.actualHours.value`) y `TestProjectReportActiveSprintChanges`
      (RN-23: con un Activo, renombrar una de sus historias cambia el reporte siguiente) — depende de
      T012
- [X] T031 [P] [US3] Pruebas unitarias **obligatorias** en
      `frontend/src/features/reports/view-model.test.ts` para el alcance `project`, sobre
      `projectReport()`: título "Reporte del proyecto <nombre>"; "Alcance" "Proyecto completo";
      "Período" "DD/MM/AAAA al DD/MM/AAAA" y, con `period` null, "Sin sprints reportables"; un bloque
      por grupo con el `h4` "<sprint> (DD/MM/AAAA al DD/MM/AAAA)", " — en curso" en el Activo, y el
      último "Historias no planificadas" con "Sin datos." si está vacío; y en métricas, las filas del
      acumulado con su marca, "Esfuerzo fuera de sprints", "Esfuerzo en sprints planificados" e
      "Historias sin estimar en el backlog" — depende de T021
- [X] T032 [US3] Pruebas de escenario en
      `frontend/src/features/reports/components/reports-view.test.tsx`, con
      `describe("010/US3 - Generar y ver el reporte de un proyecto completo")` y un `it` por escenario
      según la tabla de plan.md, precedido por `// Escenario: US3-k (<caso>)`, cada uno eligiendo
      "Proyecto completo" (la primera opción del `combobox`) y generando: (1) los `h4` de los tres
      sprints en el orden recibido; (2) el `status` de parcial; (3) ningún `h4` del Planificado y su historia en "Historias no
      planificadas"; (4)
      un proyecto `finished` sin `status` de parcial; (5) la misma historia en dos grupos con "No
      completada" y "Completada"; (6) el grupo "Historias no planificadas" último, con sus 12 filas;
      (7) ese grupo con "Sin datos."; (8) los títulos de la instantánea que manda el backend; (9) dos
      generaciones del mismo cuerpo salvo `generatedAt` y `generatedBy`: el `textContent` de las
      secciones distintas del encabezado es idéntico; (10) el encabezado y "Sin datos." en las cinco
      secciones; (11) el `404`: "El proyecto no existe." — depende de T025

### Implementación de la Historia 3

- [X] T033 [P] [US3] Implementar en `backend/internal/reports/domain/rules.go` `ReportableSprints`
      y `ReportPeriod`, y el alcance `project` de `IsPartial` — depende de T029
- [X] T034 [US3] Implementar en `backend/internal/reports/repository/report_repository.go`
      `ListSprints` (todos los sprints del proyecto en orden `start_date ASC, id ASC`) y
      `LoadUnplannedStories` (historias del proyecto con `NOT EXISTS (SELECT 1 FROM sprint_stories ss
      JOIN sprints s ON s.id = ss.sprint_id WHERE ss.story_id = us.id AND s.status IN ('active',
      'closed'))`, con datos vigentes; research.md sección 4), y `LoadDefects` con `sprintID` nil (todos los defectos del proyecto salvo
      `discarded`) — depende de T030
- [X] T035 [US3] Implementar en `backend/internal/reports/service/report_service.go` `ProjectReport`,
      reutilizando el armado de `SprintReport` sin duplicarlo (extraer las funciones de armado por
      grupo, DRY): `ListSprints` y `ReportableSprints`; un grupo por sprint en historias,
      estimaciones (con corte en cada Cerrado) y esfuerzo; el grupo de no planificadas al final de
      historias y estimaciones, sin corte; `ReportPeriod`; `MetricsSource.ProjectMetrics`; defectos con
      totales `Detected.Total` y `Resolved.Total` y `StoryTitle` contra la instantánea del sprint de
      detección si es Cerrado; `IsPartial`; y el `Save` con `Scope` `project` y `SprintID` nil —
      depende de T033, T034
- [X] T036 [US3] Implementar en `backend/internal/reports/delivery/handler.go`
      `generateProjectReport`, con su anotación Swagger completa (`@ID generateProjectReport`,
      `@Failure` 401, 404 y 500), y en `dto.go` `metrics.project` con
      `metricsdelivery.NewProjectMetrics` — depende de T035
- [X] T037 [P] [US3] Implementar en `frontend/src/features/reports/api.ts`
      `generateProjectReport(projectId)` sobre `/api/v1/projects/{projectId}/reports/project`, con
      `postAuthorizedJson(path, {}, isReport)` — depende de T032
- [X] T038 [US3] Implementar el alcance `project` en `frontend/src/features/reports/view-model.ts`
      (título, alcance, período, encabezados de grupo, no planificadas y filas de métricas del
      proyecto) y en `report-scope-select.tsx` la opción "Proyecto completo" como primera y valor
      inicial, con `reports-view.tsx` llamando a `generateProjectReport` o a `generateSprintReport`
      según la opción — depende de T031, T037

**Checkpoint**: US1, US2 y US3 funcionan juntas; el reporte de proyecto se exporta igual que el de
sprint (paso 5 de la sección 3 de quickstart.md).

---

## Fase 6: Historia de Usuario 4 — Leer un reporte con secciones sin datos (Prioridad: P3)

**Objetivo**: un reporte temprano o con una fuente caída sigue siendo legible: las secciones vacías
dicen "Sin datos.", las que no se pudieron leer lo dicen de otra forma, y ninguna desaparece, ni en
pantalla ni en el PDF.

**Rama**: `hu/010-us4-leer-un-reporte-con-secciones-sin-datos`

**Prueba independiente**: el reporte de un sprint sin esfuerzo ni defectos muestra esas dos
secciones con "Sin datos." y los totales en 0; con la fuente de métricas fallando, las secciones de
métricas y de defectos dicen "No se pudo obtener esta sección." y las demás se ven completas.

### Pruebas de la Historia 4 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T039 [P] [US4] Pruebas unitarias **obligatorias** en
      `backend/internal/reports/domain/rules_test.go` de `EffortState` (sin filas → `empty`),
      `DefectsState` (listado vacío → `empty` con los totales conservados) y `ProjectMetricsState`
      (alcance `project` sin sprints reportables → `empty`; con alguno → `complete`), según la tabla
      de research.md sección 8 (RN-11). Crear las firmas mínimas para que compile — depende de T033
- [X] T040 [US4] Pruebas de escenario de integración con los nombres de la tabla de plan.md y
      `// Escenario: <nombre> (010/US4)`: en `backend/internal/reports/delivery/handler_test.go`,
      `TestUS4_SprintSinEsfuerzo` (`effort.state` `empty`, `totalHours` `0.00`, grupos sin filas),
      `TestUS4_SprintSinDefectos` (`defects.state` `empty`, `defects` vacío y `totals` en 0),
      `TestUS4_HistoriasSinEstimar` (todas con `storyPoints` null y las métricas iguales a las de la
      008) y `TestUS4_SeccionesSinDatos` (proyecto sin sprints ni historias: las cinco secciones
      `empty` y el encabezado completo); y con `newReportsTestEnvWith`,
      `TestUS4_SeccionNoObtenible`, con subtests: el `MetricsSource` fallando → `metrics` y `defects`
      `unavailable` con sus datos en null, y historias, estimaciones y esfuerzo `complete`; el
      repositorio fallando solo en `LoadEffort` → solo `effort` `unavailable`; en los dos casos
      `200` y la fila de `report_generations` (FR-025); y el repositorio fallando en `LoadHeader` →
      `500` genérico y **ninguna** fila en `report_generations` (FR-025, excepción del encabezado) — depende de T030
- [ ] T041 [P] [US4] Pruebas unitarias **obligatorias** en
      `frontend/src/features/reports/view-model.test.ts` y
      `frontend/src/features/reports/pdf/pdf-document.test.ts`: con `state` `unavailable`, la
      `SectionView` tiene "No se pudo obtener esta sección." y ningún bloque, y con `empty` tiene "Sin
      datos." (textos distintos, RN-12); una sección de defectos `empty` conserva "Detectados: 0" y
      "Resueltos: 0"; y el documento PDF conserva el título y el mensaje de cada sección `empty` y
      `unavailable`, en el mismo orden que la vista (US4-5) — depende de T024, T031
- [ ] T042 [US4] Pruebas de escenario en
      `frontend/src/features/reports/components/reports-view.test.tsx`, con
      `describe("010/US4 - Leer un reporte con secciones sin datos")` y un `it` por escenario según
      la tabla de plan.md, precedido por `// Escenario: US4-k (<caso>)`: (1) "Sin datos." con
      `role="note"` en Esfuerzo; (2) "Sin datos." en Defectos y "Detectados: 0" y "Resueltos: 0"; (3)
      "Sin estimar" en todas las filas y las métricas tal como llegan; (4) seis `region` con su `h3`,
      cinco con "Sin datos." y el encabezado completo; (5) el documento capturado por el `vi.mock` de
      `export-pdf` conserva esas secciones con su mensaje; (6) una sección `unavailable` con
      "No se pudo obtener esta sección." con `role="alert"` y otra `empty` con `role="note"`, con
      textos distintos — depende de T032

### Implementación de la Historia 4

- [ ] T043 [P] [US4] Implementar en `backend/internal/reports/domain/rules.go` `EffortState`,
      `DefectsState` y `ProjectMetricsState` — depende de T039
- [ ] T044 [US4] Implementar en `backend/internal/reports/service/report_service.go` los estados por
      sección de research.md sección 8: cada lectura independiente; si falla la de historias,
      historias **y** estimaciones `unavailable`; si falla la de sesiones, solo estimaciones; si falla
      la de esfuerzo, solo esfuerzo; si falla `MetricsSource`, métricas **y** defectos; si falla la de
      defectos, solo defectos. Cada falla se registra con
      `slog.Error("no se pudo obtener la sección del reporte", "section", …, "projectId", …, "error", err)`
      y no se propaga. `LoadSprint`, `LoadHeader` y `Save` siguen propagando su error (`404`, `409` o
      `500`). Aplicar `EffortState`, `DefectsState` y `ProjectMetricsState`. En
      `backend/internal/reports/delivery/dto.go`, las secciones `unavailable` con sus datos en `null`
      según el contrato — depende de T040, T043
- [ ] T045 [US4] Implementar el estado `unavailable` y los totales de una sección de defectos vacía en
      `frontend/src/features/reports/view-model.ts`, en `report-section.tsx` el mensaje
      "No se pudo obtener esta sección." con `role="alert"`, y en `pdf/pdf-document.ts` el mensaje de
      las dos clases de sección — depende de T041, T042

**Checkpoint**: las cuatro historias funcionan juntas; las secciones 1 a 3 de quickstart.md pasan
completas.

---

## Fase 7: Polish y cuestiones transversales

**Propósito**: documentación viva, cobertura y verificación de las puertas de calidad.

**Rama**: `fase/010-polish`

- [ ] T046 [P] Regenerar Swagger en `backend/docs/` con `swag init -g cmd/app/main.go -o docs`, ejecutado desde
      `backend/` como indica el `README.md`, sumar "y generación de reportes de proyecto y de
      sprint" a la `@description` de `backend/cmd/app/main.go`, y verificar en
      `/swagger/index.html` que la etiqueta "Reportes" tiene las dos operaciones, en español y con los
      mismos códigos, campos y nulos que [contracts/openapi.yaml](./contracts/openapi.yaml)
- [ ] T047 [P] Actualizar `README.md`: la sección introductoria y "Panel de Control y Reportes" con
      qué es un reporte, sus dos alcances, la exportación a PDF y la marca de parcial; `pdfmake 0.3.13`
      en la tabla del stack del frontend; y `report_generations` donde se listan las tablas, aclarando
      que guarda solo quién, cuándo y con qué alcance. Mantener el paso a paso de ejecución sin
      cambios, porque no hay servicios ni variables nuevas
- [ ] T048 Ejecutar las puertas de calidad y dejar constancia del resultado: `gofmt -l backend`
      vacío, `go vet ./...`, `go test ./... -coverprofile=coverage.out` contra la base de test,
      `govulncheck ./...`, `npm run lint`, `npm run format:check` y `npm run test:coverage`; `grep -rn
      "010/US" backend frontend/src` con los 26 escenarios del backend y los 35 del frontend de la
      tabla de plan.md; ninguna aparición de `any` en `frontend/src/features/reports`,
      `frontend/src/types/report.ts` y `frontend/src/types/pdfmake.d.ts`; y ninguna dependencia nueva
      fuera de `pdfmake@0.3.13`. Si algo falla, detenerse e informarlo con la salida — depende de T046,
      T047
- [ ] T049 Validar a mano con [quickstart.md](./quickstart.md) secciones 2 y 3, en particular el
      PDF de 80 historias (paso 3.5), los caracteres especiales (paso 3.6) y la falla de exportación
      (paso 3.8), y anotar en el pull request de esta fase qué se verificó — depende de T048

---

## Phase 8: Convergence and Security

Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.

- [ ] T050 Agregar prueba de integración que verifica que LoadEffort y LoadEstimationSessions no devuelven filas de sprints o historias de otro proyecto aunque se les pase su id, en backend/internal/reports/repository/ per hallazgo de seguridad: consultas de esfuerzo y sesiones sin filtro de proyecto (security) — origen: seguridad, detectado en 010/US1
- [ ] T051 Recibir projectID en LoadEffort y LoadEstimationSessions y filtrar por él (JOIN con sprints y user_stories) en backend/internal/reports/repository/report_repository.go y en la interfaz de backend/internal/reports/domain/ per hallazgo de seguridad: consultas de esfuerzo y sesiones sin filtro de proyecto (security) — origen: seguridad, detectado en 010/US1
- [ ] T052 Justificar o quitar la opción "Proyecto completo" y su valor inicial con el botón deshabilitado en frontend/src/features/reports/components/report-scope-select.tsx y frontend/src/features/reports/components/reports-view.tsx hasta que T038 la incorpore, ajustando la prueba de frontend/src/features/reports/components/reports-view.test.tsx per plan: la opción "Proyecto completo" la agrega T038 (unrequested) — origen: converge, detectado en hu/010-us1-generar-y-ver-el-reporte-de-un-sprint
- [ ] T053 Cubrir con una prueba o quitar la traducción de metrics.project en NewReport de backend/internal/reports/delivery/dto.go hasta US3 per Constitución Principio II (unrequested) — origen: converge, detectado en hu/010-us1-generar-y-ver-el-reporte-de-un-sprint

---

## Dependencias y orden de ejecución

### Dependencias entre fases

- **Fase 1 Setup**: sin dependencias.
- **Fase 2 Foundational**: depende de la Fase 1 y **bloquea** todas las historias.
- **Fases 3 a 6, historias**: dependen de la Fase 2 y van en orden de prioridad
  (P1 → P1 → P2 → P3).
- **Fase 7 Polish**: depende de que estén completas las cuatro historias.

### Dependencias entre historias

Cada historia se prueba sola, pero no todas son independientes en su implementación:

- **US1 Reporte de sprint (P1)**: puede empezar después de la Fase 2. **Aporta** el endpoint, el
  repositorio, el servicio, el registro de generación, el modelo de presentación y la pantalla.
- **US2 Exportar (P1)**: necesita US1 (`ReportView` y la pantalla). No toca el backend.
- **US3 Reporte de proyecto (P2)**: necesita US1, de cuyo armado reutiliza las funciones por grupo.
  No depende de US2, pero sus pruebas montan la pantalla con US2 ya hecha.
- **US4 Secciones sin datos (P3)**: necesita US1 y US3, porque US4-4 es un reporte de proyecto, y US2
  para US4-5.

Si la 009 no estuviera integrada, el backend de US1 y US3 puede avanzar igual; el frontend espera.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio, repositorio antes que servicio y servicio antes que delivery.
- En el frontend, `api.ts` y `view-model.ts` antes que los componentes.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 2: T002, T004, T007 y T008 en paralelo; T003 y T005 después de T002; T006 después de T003 y
  T005; T009 después de T007 y T008.
- US1: T010, T011 y T013 en paralelo; T012 y T014 en paralelo con las anteriores (archivos
  distintos); T015, T016, T020 y T021 en paralelo.
- US2: T023 y T024 en paralelo; T026 y T027 en paralelo.
- US3: T029 y T031 en paralelo; T033 y T037 en paralelo.
- US4: T039 y T041 en paralelo.
- Las tareas que tocan `report_service.go` (T018, T035, T044), `handler_test.go` (T012, T030, T040),
  `reports-view.test.tsx` (T014, T025, T032, T042) o `reports-view.tsx` (T022, T028, T038) **no** son
  paralelizables entre sí.
- Fase 7: T046 y T047 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Pruebas en archivos distintos:
T010  backend/internal/reports/domain/rules_test.go
T011  backend/internal/reports/domain/estimation_test.go
T013  frontend/src/features/reports/api.test.ts + view-model.test.ts

# Implementaciones independientes entre sí:
T015  backend/internal/reports/domain/rules.go
T016  backend/internal/reports/domain/estimation.go
T020  frontend/src/features/reports/api.ts
T021  frontend/src/features/reports/view-model.ts
```

## Ejemplo de paralelismo: Historia de Usuario 2

```text
# Pruebas en archivos distintos:
T023  frontend/src/utils/report-file-name.test.ts
T024  frontend/src/features/reports/pdf/pdf-document.test.ts

# Implementaciones independientes entre sí:
T026  frontend/src/utils/report-file-name.ts
T027  frontend/src/features/reports/pdf/pdf-document.ts
```

## Ejemplo de paralelismo: Historia de Usuario 3

```text
# Pruebas en archivos distintos:
T029  backend/internal/reports/domain/rules_test.go
T031  frontend/src/features/reports/view-model.test.ts

# Implementaciones independientes entre sí:
T033  backend/internal/reports/domain/rules.go
T037  frontend/src/features/reports/api.ts
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar las Fases 1 y 2 (Setup y Foundational). **CRÍTICO**: bloquean todas las historias.
2. Completar la Fase 3 (US1 Reporte de sprint).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con la sección 2 de la guía de
   validación sobre el reporte de sprint y los pasos 1 y 4 de la sección 3.
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Setup y Foundational → base lista (un pull request).
2. US1 Reporte de sprint → probar sola → entregar (**MVP**: el reporte para la retrospectiva, en
   pantalla).
3. US2 Exportar → probar sola → entregar (el reporte sale del sistema; es la mitad del motivo de la
   feature).
4. US3 Reporte de proyecto → probar sola → entregar (el documento de cierre).
5. US4 Secciones sin datos → probar sola → entregar (un reporte temprano o con una fuente caída sigue
   siendo creíble).
6. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Setup y Foundational en conjunto.
2. Durante US1, una persona hace el backend (T010 a T012 y T015 a T019) y otra el frontend (T013,
   T014 y T020 a T022), contra el contrato y el backend de prueba.
3. Con US1 integrada, US2 (solo frontend) y el backend de US3 avanzan en paralelo.
4. US4 al final, porque prueba los estados de todas las secciones en los dos alcances.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia. Buscar `010/US1` a `010/US4` en el código de
  pruebas devuelve la trazabilidad completa de los 35 escenarios que exige el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 010/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b), con el ámbito `reportes`. Los commits los registra **una
  persona**, nunca un agente.
- Las pruebas del backend corren contra la base de test (`smye-test`, puerto 5433) y las del frontend
  sustituyen `fetch`. Nunca se apuntan al entorno de desarrollo (Principio V).
- **Dependencias nuevas**: solo `pdfmake@0.3.13`, autorizada el 2026-10-30 (plan, punto 1). **No** se
  instala `@types/pdfmake` (plan, punto 2). Si alguna tarea parece necesitar otra, detenerse y pedir
  autorización (Principio IV).
- **Pendiente fuera de esta lista, del responsable humano**: la enmienda PATCH de la constitución
  que suma pdfmake a la tabla de dependencias autorizadas (plan, punto 1). No es una tarea de agente,
  porque la enmienda requiere la aprobación explícita del responsable del proyecto. Las interpretaciones
  de US2-8, US3-10 y US4-4 y la regla de las historias no planificadas ya se incorporaron a la spec
  el 2026-10-30. Quedan pendientes de llevar a la spec, también a cargo de una persona, la
  descripción en el listado de defectos (FR-018) y el corte de las estimaciones al cierre (plan,
  punto 4).
- **Código de otras features que esta feature modifica**, todo señalado en el plan: los constructores
  exportados de `metrics/delivery` (T004), la migración y el vaciado de la plataforma (T003), la
  composición en `main.go` (T005), los campos sumados a `types/metrics.ts` de la 009 (T007) y el
  enlace "Reportes" de `ProjectOverview` (T022).
- **Revisión de seguridad**: antes de abrir el pull request de cada historia con backend (US1, US3 y
  US4) conviene ejecutar `/revisar-seguridad`, porque el reporte reúne datos de seis features y es lo
  que sale del sistema sin control de acceso (RC-11).
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: las anotaciones Swagger se escriben en la misma tarea que cada manejador (T019, T036),
  pero `backend/docs/` se regenera y el `README.md` se actualiza una sola vez, en la Fase 7 (T046,
  T047). Entre el merge de una historia y la Fase 7, la documentación publicada no lista los reportes.
  Es el mismo criterio que aplicaron la 001 a la 009.

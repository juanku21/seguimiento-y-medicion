---

description: "Lista de tareas de la feature 003 — Product Backlog"
---

# Tareas: Product Backlog

**Entrada**: documentos de diseño de `specs/003-product-backlog/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) y
[`specs/002-project-members`](../002-project-members/spec.md) están integradas en `main`. Esta
feature reutiliza el middleware de sesión, el accesor `authdelivery.CurrentUserID`, el subgrupo
`/api/v1/projects/:projectId` con `RequireMembership`, los esquemas de error de la 001, el límite
global de 1 MiB del cuerpo y el ayudante `testsupport.OpenDatabase`. **No hay fase de Setup**: no se
agregan dependencias, servicios ni variables de entorno.

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para las
validaciones, **la escala de estimación en Story Points** y las reglas de negocio RN-01 a RN-18.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/backlog/{domain,repository,service,delivery}/`; se
  tocan de forma puntual `backend/cmd/app/main.go`,
  `backend/internal/platform/database/postgres.go`,
  `backend/internal/platform/testsupport/database.go`,
  `backend/internal/projects/repository/backlog_summary.go` y los ayudantes de prueba de
  `backend/internal/projects/delivery/handler_test.go`
- **Frontend**: `frontend/src/app/projects/[projectId]/backlog/`, `frontend/src/features/backlog/`,
  `frontend/src/types/story.ts`, `frontend/src/utils/story-labels.ts`; se tocan
  `frontend/src/features/projects/components/project-overview.tsx` y
  `frontend/src/features/projects/components/project-status-card.tsx`
- **Pruebas**: junto al código que prueban (Go: `*_test.go` en el mismo paquete; Vitest:
  `*.test.ts`/`*.test.tsx` junto al módulo)

## Ciclo obligatorio en cada tarea

Según el Principio II y la skill `/speckit-implement`:

1. **Tarea de pruebas** → se escribe la prueba, se verifica que **falla** y se **detiene**. El
   desarrollador registra el commit con `/redactar-commit`, pie `TDD: red`.
2. **Tarea de implementación** → código mínimo que hace pasar la prueba (`TDD: green`), pausa, y
   refactor si corresponde (`TDD: refactor`).
3. **Tarea técnica** → sin prueba asociada, sin pie TDD.

Ningún agente avanza de RED a GREEN sin ese alto.

## Cómo se preparan los escenarios que dependen de otras features

- Los estados `in_progress` y `completed` (US4-8, US5-5, US5-10) se siembran escribiendo la columna
  `status` directamente en la base de test: ningún endpoint de esta feature los produce (FR-027).
- Las historias comprometidas en un sprint abierto, con historial de sprint, con esfuerzo, con
  defectos o con sesiones de Planning Poker (US3-3, US4-6, US5-8, US6-4, US6-6, US6-7) se preparan
  con **dobles de `testify/mock`** de los cuatro puertos, inyectados al componer el router de prueba.
- Las pruebas sin sesión y la comparación byte a byte de los `404` **no** llevan la etiqueta
  `003/USn`, porque no derivan de un escenario de la spec y no deben alterar el conteo de 58.

---

## Fase 1: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el dominio del módulo nuevo, sus contratos de repositorio y de puertos, las
implementaciones de los puertos que informan "sin historial", la migración de las dos tablas y el
aislamiento de las pruebas. Todas las tareas son **técnicas**: declaran estructuras y contratos, sin
lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/003-foundational`

- [X] T001 [P] Declarar la entidad `UserStory` en `backend/internal/backlog/domain/user_story.go` con
      los campos de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`;
      `ProjectID uuid NOT NULL` con FK a `projects.id` (campo `Project *projectsdomain.Project` solo
      para la FK, nunca precargado); `Title varchar(150) NOT NULL` **sin índice único**;
      `Description varchar(2000)` **anulable** (`*string`); `Priority varchar(8) NOT NULL`;
      `Status varchar(16) NOT NULL`; `StoryPoints smallint` **anulable** (`*int`: `NULL` es "sin
      estimar" y 0 es una estimación válida); `CreatedByID uuid NOT NULL` con FK a `users.id` (campo
      `CreatedBy *authdomain.User` solo para la FK); `CreatedAt` y `UpdatedAt` `timestamptz NOT
      NULL`; e índice compuesto `idx_user_stories_project_order (project_id, priority, created_at,
      id)`. **Sin** columnas `ready_to_plan`, `priority_rank`, `deleted_at` ni `updated_by_id`
      (plan.md, reevaluación del Principio I)
- [X] T002 [P] Declarar la entidad `AcceptanceCriterion` en
      `backend/internal/backlog/domain/acceptance_criterion.go` con los campos de data-model.md
      sección 2: `ID uuid` PK; `StoryID uuid NOT NULL` con FK a `user_stories.id`;
      `Position integer NOT NULL` (**desde 0, consecutivo dentro de la historia**);
      `Text varchar(500) NOT NULL`; `CreatedAt` y `UpdatedAt` `timestamptz NOT NULL`; e **índice
      único** `idx_acceptance_criteria_story_position (story_id, position)`. Sin
      `ON DELETE CASCADE`: el borrado de los criterios es explícito en el repositorio (research.md
      sección 12)
- [X] T003 [P] Declarar el tipo `StoryPriority` con sus tres constantes `high`, `medium` y `low` en
      `backend/internal/backlog/domain/priority.go`, como constantes de Go y **no** como `enum` de
      PostgreSQL (data-model.md sección 3). El rango de orden es regla de negocio y se implementa en
      US2
- [X] T004 [P] Declarar el tipo `StoryStatus` con sus tres constantes `pending`, `in_progress` y
      `completed` en `backend/internal/backlog/domain/status.go`, con un comentario en español que
      deje claro que **esta feature nunca escribe el estado después de la creación**: sus
      transiciones las produce `specs/004-sprint-management` (FR-027, RC-04)
- [X] T005 [P] Declarar los errores de dominio en `backend/internal/backlog/domain/errors.go`
      (data-model.md sección 9): `ErrStoryNotFound` (historia inexistente, eliminada o ajena;
      FR-044, FR-047), `ErrBacklogReadOnly` (FR-048), `ErrStoryCompleted` (FR-030),
      `ErrCommittedStoryMustStayReady` (FR-018, FR-022) y el tipo `DeletionBlockedError` con
      `Reasons []DeletionBlocker`, donde `DeletionBlocker` tiene los cuatro códigos
      `sprint_history`, `effort`, `defects` y `estimation_sessions` **en ese orden** (FR-043)
- [X] T006 Declarar en `backend/internal/backlog/domain/repository.go` la interfaz `StoryRepository`
      con **exactamente** las ocho operaciones de data-model.md sección 9 —
      `FindAccessible(ctx, storyID, userID) (*StoryAccess, error)`,
      `Create(ctx, story *UserStory, criteria []string) error`,
      `FindDetail(ctx, storyID) (*StoryDetail, error)`,
      `List(ctx, projectID, filter StoryFilter) ([]StoryListItem, error)`,
      `HasStories(ctx, projectID) (bool, error)`,
      `Update(ctx, storyID, changes StoryChanges) error`,
      `UpdateStoryPoints(ctx, storyID, storyPoints *int) error` y
      `Delete(ctx, storyID) error` — más los tipos de lectura `StoryAccess`, `StoryListItem`,
      `StoryDetail` (con `CreatedBy` de solo `ID` y `FullName`), `StoryFilter` y `StoryChanges` de
      data-model.md sección 6. **Prohibido** declarar cambio de estado, cambio de proyecto, búsqueda
      por texto, paginación o listados fuera de un proyecto (Principio III) — depende de T001 a T004
- [X] T007 Declarar los cinco puertos en `backend/internal/backlog/domain/ports.go` con los tipos de
      data-model.md sección 7: `SprintAssignments.Find(ctx, storyID) (SprintAssignment, error)` con
      `CommittedToOpenSprint` y `EverAssigned`; `EffortRecords.HasEffort(ctx, storyID) (bool,
      error)`; `DefectRecords.HasDefects(ctx, storyID) (bool, error)`;
      `EstimationSessions.HasSessions(ctx, storyID) (bool, error)`, que cuenta las sesiones
      Finalizadas **y** las Canceladas (RC-10); y `ProjectDirectory.IsFinished(ctx, projectID)
      (bool, error)`. Cada comentario indica qué feature producirá el dato
- [X] T008 [P] Implementar los cuatro puertos hacia otras features en
      `backend/internal/backlog/repository/sprint_assignments.go`,
      `backend/internal/backlog/repository/effort_records.go`,
      `backend/internal/backlog/repository/defect_records.go` y
      `backend/internal/backlog/repository/estimation_sessions.go`, informando **ninguna historia
      está ni estuvo en un sprint, ni tiene esfuerzo, defectos o sesiones de estimación**, con un
      comentario en español en cada archivo que indique qué spec (004, 006, 007 y 005) reemplazará
      su cuerpo. No es un dato inventado: hoy esas features no existen (research.md sección 11) —
      depende de T007
- [X] T009 Agregar `UserStory` y `AcceptanceCriterion` al `AutoMigrate` de
      `backend/internal/platform/database/postgres.go`, **después** de `Project` y `ProjectMember`,
      de modo que el índice compuesto de `user_stories` y el índice único de `acceptance_criteria`
      queden creados por las etiquetas de GORM y no por un `CREATE INDEX` en crudo — depende de T001
      y T002
- [X] T010 Extender el vaciado del ayudante de pruebas en
      `backend/internal/platform/testsupport/database.go` a
      `TRUNCATE users, sessions, auth_events, projects, project_members, user_stories,
      acceptance_criteria CASCADE`, manteniendo la verificación de que la base es `smye_test` y el
      lock de la base de test — depende de T009

**Checkpoint**: las dos tablas migradas en la base de test, el dominio y los contratos declarados,
los puertos con sus implementaciones actuales y las pruebas aisladas. Las historias pueden empezar.

---

## Fase 2: Historia de Usuario 1 — Registrar una historia en el backlog (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante registra una historia con título, prioridad y, si los tiene,
descripción, Story Points y criterios de aceptación; nace Pendiente y atribuida a quien la creó.

**Rama**: `hu/003-us1-registrar-una-historia`

**Prueba independiente**: `POST /api/v1/projects/{projectId}/stories` devuelve `201` con
`status: pending`, `createdBy` del titular de la sesión y `readiness` correcta, y un valor de 4
Story Points responde `400` con la escala, sin necesitar ninguna otra historia.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T011 [P] [US1] Pruebas unitarias **obligatorias** de la escala en
      `backend/internal/backlog/domain/story_points_test.go`, cubriendo RN-06 y RN-07:
      `IsValidStoryPoints` acepta exactamente **0, 1, 2, 3, 5, 8, 13, 20, 40 y 100** y rechaza 4, 6,
      7, 50, 101 y -1 (SC-007) — depende de T010
- [X] T012 [P] [US1] Pruebas unitarias **obligatorias** de `Readiness` en
      `backend/internal/backlog/domain/readiness_test.go`, cubriendo RN-12 y RN-13 con las cinco
      combinaciones de data-model.md sección 5: (5, 2) lista; (0, 1) lista; (`nil`, 2) falta
      `story_points`; (3, 0) falta `acceptance_criteria`; (`nil`, 0) faltan las dos, **en ese
      orden** (SC-009) — depende de T010
- [X] T013 [P] [US1] Pruebas unitarias **obligatorias** de validación en
      `backend/internal/backlog/delivery/validation_test.go`, cubriendo RN-03 a RN-08: título
      **obligatorio, entre 1 y 150 runas tras `TrimSpace`**, con ausente o solo espacios rechazado y
      150 válido y 151 rechazado (RN-03); descripción **opcional de hasta 2000 runas tras
      `TrimSpace`**, guardada como nula si queda vacía, con 2000 válido y 2001 rechazado (RN-04);
      prioridad **obligatoria, solo `high`, `medium` o `low`** (RN-05); Story Points leídos como
      `json.Number` y validados sobre el texto literal, rechazando `6.5`, `5.0`, `1e1`, `-1` y los
      enteros fuera de la escala con el mensaje que enumera la escala completa, y aceptando `null`
      como "sin estimar" y `0` como estimación (RN-06, RN-07); y cada criterio **recortado, de entre
      1 y 500 runas**, con 1 y 500 válidos, vacío, solo espacios y 501 rechazados, el campo
      informado como `acceptanceCriteria[i]` y el orden de la lista conservado (RN-08). La validación
      devuelve **todos** los campos inválidos a la vez — depende de T010
- [X] T014 [P] [US1] Pruebas unitarias **obligatorias** de la creación en el servicio en
      `backend/internal/backlog/service/story_service_test.go`, con dobles de `testify/mock` de
      `StoryRepository` y de los cinco puertos: la historia nace `pending` (RN-09) con un UUID
      nuevo, el autor es el actor recibido y no un dato de la entrada (FR-049), un actor que no es
      el propietario del proyecto crea igual (RN-01, RN-02), y con `ProjectDirectory.IsFinished`
      verdadero devuelve `ErrBacklogReadOnly` sin llamar a `Create` (RN-15) — depende de T010
- [X] T015 [US1] Pruebas de integración de la historia en
      `backend/internal/backlog/delivery/handler_test.go`, con un ayudante `newBacklogTestEnv` que
      compone el router como `cmd/app` (sesión de la 001, rutas de la 002 con su subgrupo
      `/:projectId`, rutas del backlog y `httpserver.LimitRequestBody`) y que permite inyectar dobles
      de los cuatro puertos. Una función por escenario de spec.md precedida por
      `// Escenario: <nombre del escenario> (003/US1)`: `TestUS1_CreacionCompleta` (`201`, `pending`,
      los dos criterios en orden, `createdBy` del titular, `readiness.ready` verdadero),
      `TestUS1_SoloTituloYPrioridad`, `TestUS1_ProyectoPlanificado`, `TestUS1_CeroStoryPoints`
      (`storyPoints: 0`, distinto de `null`), `TestUS1_LimitesDelTitulo` (150 y 151),
      `TestUS1_LimitesDelCriterio` (500 y 1), `TestUS1_TituloRepetido`, `TestUS1_CamposObligatorios`
      (subtests sin título, solo espacios y sin prioridad), `TestUS1_StoryPointsFueraDeEscala`
      (subtests 4, 7, negativo y decimal), `TestUS1_CriterioVacio`, `TestUS1_ProyectoFinalizado`
      (`409` y nada creado) y `TestUS1_NoIntegrante` (`404` con **el mismo cuerpo** que un
      `projectId` inexistente). En todos los rechazos se verifica contando filas que **no se creó
      ni la historia ni ningún criterio** (FR-013) — depende de T010
- [X] T016 [P] [US1] Pruebas de frontend en
      `frontend/src/features/backlog/components/story-form.test.tsx` y
      `frontend/src/features/backlog/validation.test.ts`, con
      `describe("003/US1 - Registrar una historia en el backlog")` y un `it` por escenario de la
      spec, incluido que el formulario **muestra antes de enviar** el máximo de 150 caracteres del
      título, los 2000 de la descripción y los 500 de cada criterio, y que el selector de Story
      Points ofrece solo los valores de la escala más "sin estimar"

### Implementación de la Historia 1

- [X] T017 [US1] Implementar la escala en `backend/internal/backlog/domain/story_points.go`: el
      conjunto cerrado `{0, 1, 2, 3, 5, 8, 13, 20, 40, 100}` declarado **una sola vez** e
      `IsValidStoryPoints(int) bool` (RN-06) — depende de T011
- [X] T018 [US1] Implementar `Readiness(storyPoints *int, criteriaCount int)` en
      `backend/internal/backlog/domain/readiness.go`, devolviendo `ready` y la lista `missing` con
      `story_points` y `acceptance_criteria` en ese orden. Es la **única** fuente de la regla
      (research.md sección 10) — depende de T012
- [X] T019 [US1] Implementar la normalización y validación de la creación en
      `backend/internal/backlog/delivery/validation.go`, con los límites 150, 2000 y 500 declarados
      como **constantes nombradas en un único lugar**, los Story Points recibidos como `json.Number`
      y validados sobre el texto literal con `IsValidStoryPoints`, y un `FieldError` por cada campo
      inválido. La validación ocurre antes de la capa de servicio (Principio VII) — depende de T013 y
      T017
- [X] T020 [US1] Implementar `IsFinished` en `backend/internal/backlog/repository/project_directory.go`
      sobre el modelo `projectsdomain.Project`, **seleccionando solo la columna `status`** y
      comparándola con `projectsdomain.ProjectStatusFinished` (research.md sección 14) — depende de
      T007 y T015
- [X] T021 [US1] Implementar `Create` y `FindDetail` en
      `backend/internal/backlog/repository/story_repository.go`: `Create` inserta la historia y sus
      criterios con posiciones `0..n-1` en **una sola transacción**, de modo que FR-013 sea atómico
      por construcción; `FindDetail` lee la historia con un `JOIN` a `users` que **selecciona solo
      `users.id` y `users.full_name`** (nunca `password_hash`) y los criterios ordenados por
      `position`, en dos consultas, y devuelve `ErrStoryNotFound` si no hay fila — depende de T006 y
      T015
- [X] T022 [US1] Implementar en `backend/internal/backlog/service/story_service.go` el constructor
      `NewStoryService`, que recibe `StoryRepository` y **los cinco puertos de T007**, para que su
      firma no cambie en las historias siguientes; el método `Create(ctx, projectID, actorID,
      input)`, que rechaza con `ErrBacklogReadOnly` si el proyecto está Finalizado y construye la
      historia con UUID, estado `pending` y autor el actor; y el armado del detalle
      (`buildDetail`), que usa `Readiness` y `SprintAssignments.Find` para `everAssignedToSprint`.
      Lo reutilizan todas las historias siguientes — depende de T014, T018, T020 y T021
- [X] T023 [US1] Declarar en `backend/internal/backlog/delivery/dto.go` los esquemas de
      contracts/openapi.yaml `StoryInput`, `StoryDetail`, `Readiness` y `AuthorRef`, reutilizando
      `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de la 001. `storyPoints` y
      `description` se serializan como `null` cuando no tienen valor y `acceptanceCriteria` como
      `[]`, nunca como `null` (FR-040) — depende de T022
- [X] T024 [US1] Implementar el manejador `createStory` en
      `backend/internal/backlog/delivery/handler.go` con anotaciones `swag` en español, que lee al
      titular con `authdelivery.CurrentUserID` y el proyecto del parámetro `projectId` ya validado
      por `RequireMembership`; crear `RegisterRoutes` en `backend/internal/backlog/delivery/routes.go`
      registrando `POST /stories` en el **subgrupo `/api/v1/projects/:projectId` que devuelve
      `projectsdelivery.RegisterRoutes`**, sin que `backlog/delivery` importe `projects/delivery`; y
      componer el módulo en `backend/cmd/app/main.go` (servicio, repositorio y los cinco puertos)
      tomando ese subgrupo, más el `@description` general de la API para que mencione el backlog.
      Respuestas: `201` con `StoryDetail`, `400` con `ValidationErrorResponse`, `401`, `404` de
      `RequireMembership` y `409` (`El proyecto está finalizado y su backlog es de solo lectura.`)
      (research.md sección 2) — depende de T015, T019 y T023
- [X] T025 [P] [US1] Declarar los tipos de petición y respuesta en `frontend/src/types/story.ts` y,
      en `frontend/src/utils/story-labels.ts`, las etiquetas en español de la prioridad (`high` →
      Alta, `medium` → Media, `low` → Baja) y del estado (`pending` → Pendiente, `in_progress` → En
      progreso, `completed` → Completada), y la escala `[0, 1, 2, 3, 5, 8, 13, 20, 40, 100]`.
      **Prohibido el tipo `any`** (Principio VIII)
- [X] T026 [US1] Implementar la llamada de creación en `frontend/src/features/backlog/api.ts` sobre el
      cliente `fetch` de la 001, y las reglas del formulario en
      `frontend/src/features/backlog/validation.ts` con los límites 150, 2000 y 500 como constantes
      nombradas en un único lugar — depende de T016 y T025
- [X] T027 [US1] Implementar `StoryForm` en `frontend/src/features/backlog/components/story-form.tsx`
      y `AcceptanceCriteriaEditor` en
      `frontend/src/features/backlog/components/acceptance-criteria-editor.tsx` (agregar, quitar y
      reordenar criterios), con los mensajes en español y los límites visibles antes de enviar.
      Antes de escribir código, leer la documentación de Next.js de
      `frontend/node_modules/next/dist/docs/` — depende de T026
- [X] T028 [US1] Crear la pantalla `frontend/src/app/projects/[projectId]/backlog/new/page.tsx`,
      protegida con la guardia de sesión de la 001 — depende de T027

**Checkpoint**: US1 funciona y se prueba sola. El MVP está entregable.

---

## Fase 3: Historia de Usuario 2 — Ver el backlog ordenado y filtrado (Prioridad: P1)

**Objetivo**: un integrante ve todas las historias del proyecto en orden de prioridad y antigüedad,
filtra por estado y por prioridad, y distingue un backlog vacío de unos filtros sin coincidencias.

**Rama**: `hu/003-us2-ver-el-backlog`

**Prueba independiente**: creando historias Alta, Media y Baja en distinto orden,
`GET /api/v1/projects/{projectId}/stories` las devuelve en el orden de FR-032, cada filtro las acota
y un proyecto sin historias responde `emptyReason: backlog_empty`.

### Pruebas de la Historia 2 ⚠️

- [X] T029 [P] [US2] Pruebas unitarias **obligatorias** del rango de prioridad en
      `backend/internal/backlog/domain/priority_test.go` (`high` 0, `medium` 1, `low` 2) y de la
      validación de filtros en `backend/internal/backlog/delivery/validation_test.go`: `status` y
      `priority` repetibles, valores fuera de `pending`/`in_progress`/`completed` o
      `high`/`medium`/`low` rechazados indicando los valores admitidos, y ausencia de filtros
      aceptada (RN-05, RN-17, FR-037) — depende de T028
- [X] T030 [US2] Pruebas de integración de la historia en
      `backend/internal/backlog/delivery/handler_test.go`, con `// Escenario: <nombre> (003/US2)`:
      `TestUS2_OrdenPorPrioridadYFecha`, `TestUS2_FiltroPorEstado` (estados sembrados en la base),
      `TestUS2_FiltrosCombinados`, `TestUS2_BacklogVacio` (`200`, `stories: []` y `emptyReason:
      backlog_empty`), `TestUS2_FiltroSinCoincidencias` (`emptyReason: no_matches` y
      `appliedFilters`), `TestUS2_OrdenEstableAnteEmpate` (dos historias con el mismo `created_at`
      escrito en la base, dos consultas, mismo orden por `id`), `TestUS2_FiltroInvalido` y
      `TestUS2_NoIntegrante`. Además, sin etiqueta de escenario, la prueba de orden con **50
      historias** de las tres prioridades (SC-004), la de que el listado **no incluye historias de
      otro proyecto** (FR-038) y la de **varios valores en un mismo filtro**:
      `?status=pending&status=in_progress` devuelve las historias de los dos estados y ninguna
      `completed`, y `?priority=high&priority=low` devuelve las Altas y las Bajas sin las Medias
      (FR-034) — depende de T028
- [X] T031 [P] [US2] Pruebas de frontend en
      `frontend/src/features/backlog/components/story-list.test.tsx` y
      `frontend/src/features/backlog/components/backlog-filters.test.tsx`, con
      `describe("003/US2 - Ver el backlog ordenado y filtrado")` y un `it` por escenario, incluido
      que el mensaje de backlog vacío y el de filtros sin coincidencias son **distintos** y que cada
      fila muestra prioridad, estado, Story Points o "Sin estimar", cantidad de criterios y si está
      lista para planificar — depende de T028

### Implementación de la Historia 2

- [X] T032 [US2] Implementar el rango de prioridad en `backend/internal/backlog/domain/priority.go` —
      depende de T029
- [X] T033 [US2] Implementar el parseo y la validación de los filtros en
      `backend/internal/backlog/delivery/validation.go` a partir de `c.QueryArray("status")` y
      `c.QueryArray("priority")`, devolviendo un `StoryFilter` o los `FieldError` de cada valor
      inválido — depende de T029
- [X] T034 [US2] Implementar `List` y `HasStories` en
      `backend/internal/backlog/repository/story_repository.go`: `List` es **una sola consulta**
      sobre `user_stories` del proyecto con una subconsulta de conteo de `acceptance_criteria`,
      filtros `IN` combinados con `AND` y el orden
      `CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, created_at ASC, id ASC`
      (FR-032); `HasStories` es un `SELECT 1 ... LIMIT 1` (research.md sección 9) — depende de T030
      y T032
- [X] T035 [US2] Implementar `List(ctx, projectID, filter)` en
      `backend/internal/backlog/service/story_service.go`, que calcula `readyToPlan` con `Readiness`
      y elige `emptyReason`: `null` con resultados, `backlog_empty` si el proyecto no tiene historias
      (aunque haya filtros) y `no_matches` si las tiene pero ninguna cumple los filtros, llamando a
      `HasStories` **solo** cuando el resultado filtrado viene vacío — depende de T034
- [X] T036 [US2] Declarar `StoryListItem`, `AppliedFilters` y `StoryListResponse` en
      `backend/internal/backlog/delivery/dto.go` (`stories` y los dos filtros aplicados nunca son
      `null`), implementar el manejador `listStories` en `backend/internal/backlog/delivery/handler.go`
      con anotaciones `swag` en español y registrar `GET /stories` en el subgrupo del proyecto en
      `backend/internal/backlog/delivery/routes.go` — depende de T030, T033 y T035
- [X] T037 [US2] Implementar la llamada de listado en `frontend/src/features/backlog/api.ts`, con los
      filtros como parámetros repetidos de `URLSearchParams`, y los componentes `StoryList` en
      `frontend/src/features/backlog/components/story-list.tsx` y `BacklogFilters` en
      `frontend/src/features/backlog/components/backlog-filters.tsx` — depende de T031 y T036
- [X] T038 [US2] Crear la pantalla `frontend/src/app/projects/[projectId]/backlog/page.tsx`, protegida
      con la guardia de sesión, con el acceso a "Nueva historia" — depende de T037
- [X] T089 [US2] Agregar pruebas unitarias de `StoryService.List` en
      `backend/internal/backlog/service/story_service_test.go` con `mockStoryRepository`: con
      resultados, `emptyReason` nulo, `readyToPlan` calculado con `Readiness` y `HasStories` sin
      llamar; con resultado vacío y `HasStories` en `false`, `backlog_empty` con y sin filtros; con
      resultado vacío y `HasStories` en `true`, `no_matches`; y errores de `List` y de `HasStories`
      propagados per FR-035, FR-036 (partial)

**Checkpoint**: US1 y US2 funcionan de forma independiente. El backlog ya se registra y se consulta.

---

## Fase 4: Historia de Usuario 3 — Consultar el detalle de una historia (Prioridad: P2)

**Objetivo**: un integrante ve todo lo que se sabe de una historia —criterios en orden, qué le falta
para estar lista, autor y fechas— y una historia ajena es indistinguible de una inexistente.

**Rama**: `hu/003-us3-consultar-el-detalle`

**Prueba independiente**: creando una historia con tres criterios, `GET /api/v1/stories/{storyId}`
los devuelve en orden, y la misma consulta hecha por quien no es integrante responde un `404`
idéntico byte a byte al de un identificador inventado.

### Pruebas de la Historia 3 ⚠️

- [X] T039 [P] [US3] Pruebas de integración de `RequireStoryAccess` en
      `backend/internal/backlog/delivery/middleware_test.go`, montado sobre un manejador de prueba:
      un integrante que no es el propietario pasa y recibe en el contexto un `StoryAccess` con la
      historia, su proyecto y si está Finalizado (RN-01, RN-02); la historia inexistente, la de un
      proyecto ajeno, la de un integrante ya quitado y un identificador que no es UUID responden
      **el mismo `404` byte a byte** (`{"error":"La historia no existe."}`) con una sola consulta;
      y sin titular en el contexto responde el `401` de la 001 (research.md sección 3) — depende de
      T038
- [X] T040 [US3] Pruebas de integración de la historia en
      `backend/internal/backlog/delivery/handler_test.go`, con `// Escenario: <nombre> (003/US3)`:
      `TestUS3_DetalleCompleto` (descripción, prioridad Media, 8 Story Points, tres criterios en
      orden, autor, `createdAt`, `updatedAt` y `readiness.ready` verdadero),
      `TestUS3_SinEstimarNiCriterios` (`readiness.missing` con los dos motivos),
      `TestUS3_AsignadaASprint` (doble de `SprintAssignments` con `EverAssigned` verdadero →
      `everAssignedToSprint: true`), `TestUS3_CeroStoryPoints`, `TestUS3_HistoriaAjena` y
      `TestUS3_HistoriaInexistente`, comparando byte a byte los cuerpos de los dos últimos.
      Además, sin etiqueta de escenario, la prueba de **autoría conservada**: Bruno crea una
      historia, Ana lo quita del proyecto con `DELETE /api/v1/projects/{projectId}/members/{userId}`
      de la 002, y el detalle consultado por Ana sigue devolviendo la historia con
      `createdBy.fullName` igual a "Bruno Gómez", mientras que Bruno recibe `404` (FR-050, caso
      límite "Integrante quitado del proyecto") — depende de T038
- [X] T041 [P] [US3] Pruebas de frontend en
      `frontend/src/features/backlog/components/story-detail-card.test.tsx`, con
      `describe("003/US3 - Consultar el detalle de una historia")` y un `it` por escenario, incluido
      que 0 se muestra como "0 Story Points" y `null` como "Sin estimar", y que se indica qué falta
      para estar lista — depende de T038

### Implementación de la Historia 3

- [X] T042 [US3] Implementar `FindAccessible` en
      `backend/internal/backlog/repository/story_repository.go` con **una sola consulta**
      `user_stories JOIN project_members (project_id, user_id = titular) JOIN projects`, devolviendo
      `ErrStoryNotFound` si no hay fila (research.md sección 3) — depende de T039
- [X] T043 [US3] Implementar `RequireStoryAccess` en `backend/internal/backlog/delivery/middleware.go`
      con **una única salida `404`** para la historia inexistente, la ajena y el identificador mal
      formado, que deja el `StoryAccess` en el contexto; cualquier otro error responde `500`
      genérico registrado en el log — depende de T042
- [X] T044 [US3] Implementar `Get(ctx, storyID)` en `backend/internal/backlog/service/story_service.go`
      sobre `FindDetail` y `buildDetail` de T022 — depende de T040
- [X] T045 [US3] Implementar el manejador `getStory` en `backend/internal/backlog/delivery/handler.go`
      con anotaciones `swag` en español, y crear en `backend/internal/backlog/delivery/routes.go` el
      grupo plano `/api/v1/stories/:storyId` con el middleware de sesión inyectado desde
      `cmd/app/main.go` y `RequireStoryAccess`, registrando `GET` en él — depende de T043 y T044
- [X] T046 [US3] Implementar la llamada de detalle en `frontend/src/features/backlog/api.ts`, el
      componente `StoryDetailCard` en
      `frontend/src/features/backlog/components/story-detail-card.tsx` y la pantalla
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx`, con el enlace desde cada
      fila de `StoryList` — depende de T041 y T045

**Checkpoint**: US1 a US3 funcionan de forma independiente. El aislamiento por historia está activo.

---

## Fase 5: Historia de Usuario 4 — Estimar una historia en Story Points (Prioridad: P2)

**Objetivo**: un integrante asigna un valor de la escala a una historia, la reestima o la devuelve a
"sin estimar", respetando la historia Completada, el proyecto Finalizado y la historia comprometida
en un sprint abierto.

**Rama**: `hu/003-us4-estimar-una-historia`

**Prueba independiente**: creando una historia sin estimar con dos criterios,
`PUT /api/v1/stories/{storyId}/story-points` con 13 la deja estimada y lista para planificar, y con
50 responde `400` sin cambiar el valor.

### Pruebas de la Historia 4 ⚠️

- [X] T047 [P] [US4] Pruebas unitarias **obligatorias** de `Estimate` en
      `backend/internal/backlog/service/story_service_test.go`, con dobles: historia `completed` →
      `ErrStoryCompleted` (RN-14); proyecto Finalizado → `ErrBacklogReadOnly` (RN-15), evaluado
      **antes** que las demás reglas; historia comprometida en un sprint abierto con `nil` →
      `ErrCommittedStoryMustStayReady`, y con otro valor de la escala → aceptada (RN-18); en todos
      los rechazos, `UpdateStoryPoints` no se llama — depende de T046
- [X] T048 [P] [US4] Pruebas unitarias de la validación del cuerpo de la estimación en
      `backend/internal/backlog/delivery/validation_test.go`: `storyPoints` **obligatorio** (cuerpo
      `{}` rechazado), `null` aceptado como "sin estimar" y los valores fuera de la escala
      rechazados con la escala en el mensaje (FR-016, FR-018, FR-019) — depende de T046
- [X] T049 [US4] Pruebas de integración de la historia en
      `backend/internal/backlog/delivery/handler_test.go`, con `// Escenario: <nombre> (003/US4)`:
      `TestUS4_EstimacionInicial` (13 y lista para planificar), `TestUS4_Reestimacion` (5 → 8 y
      `updatedAt` posterior), `TestUS4_VolverASinEstimar`, `TestUS4_SinCriterios`,
      `TestUS4_ExtremosDeLaEscala` (0 y 100), `TestUS4_ComprometidaNoVuelveASinEstimar` (doble de
      `SprintAssignments` con `CommittedToOpenSprint` → `409` y valor intacto),
      `TestUS4_ValorFueraDeEscala` (subtests 50, 4 y 6,5, valor intacto),
      `TestUS4_HistoriaCompletada` (estado sembrado) y `TestUS4_ProyectoFinalizado` — depende de
      T046
- [X] T050 [P] [US4] Pruebas de frontend en
      `frontend/src/features/backlog/components/estimate-control.test.tsx`, con
      `describe("003/US4 - Estimar una historia en Story Points")` y un `it` por escenario, incluido
      que el control ofrece **solo** los valores de la escala más "Sin estimar" y que la estimación
      se hace en no más de 3 pasos (SC-002) — depende de T046

### Implementación de la Historia 4

- [X] T051 [US4] Declarar `StoryPointsInput` en `backend/internal/backlog/delivery/dto.go` con
      `storyPoints` como `json.RawMessage`, para distinguir el campo ausente del `null`, e
      implementar su validación en `backend/internal/backlog/delivery/validation.go` reutilizando la
      validación de la escala de T019 (research.md sección 8) — depende de T048
- [X] T052 [US4] Implementar `UpdateStoryPoints` en
      `backend/internal/backlog/repository/story_repository.go`, que escribe **solo**
      `story_points` y `updated_at` (FR-017) — depende de T049
- [X] T053 [US4] Implementar `Estimate(ctx, access, storyPoints *int)` en
      `backend/internal/backlog/service/story_service.go` con las reglas en el orden de research.md
      sección 16: solo lectura, Completada y comprometida (`SprintAssignments.Find`). Es el método que
      reutilizará `specs/005-planning-poker` para registrar el valor acordado (RC-06) — depende de
      T047 y T052
- [X] T054 [US4] Implementar el manejador `estimateStory` en
      `backend/internal/backlog/delivery/handler.go` con anotaciones `swag` en español y registrar
      `PUT /story-points` en el grupo de la historia en `backend/internal/backlog/delivery/routes.go`.
      Respuestas: `200` con `StoryDetail`, `400`, `401`, `404` y `409` con los mensajes de
      contracts/openapi.yaml — depende de T051 y T053
- [X] T055 [US4] Implementar la llamada de estimación en `frontend/src/features/backlog/api.ts` y el
      componente `EstimateControl` en
      `frontend/src/features/backlog/components/estimate-control.tsx`, integrado en la pantalla de
      detalle — depende de T050 y T054

**Checkpoint**: US1 a US4 funcionan de forma independiente. El backlog ya produce Story Points
utilizables por las métricas.

---

## Fase 6: Historia de Usuario 5 — Refinar una historia (Prioridad: P3)

**Objetivo**: un integrante corrige título, descripción, prioridad o criterios de una historia,
aplicando solo los campos que envía, sin que dos ediciones simultáneas de campos distintos se pisen.

**Rama**: `hu/003-us5-refinar-una-historia`

**Prueba independiente**: creando una historia y enviando `PATCH` con solo la prioridad, el detalle
refleja la prioridad nueva y conserva el resto; y un `PATCH` con `status` responde `400` sin cambiar
el estado.

### Pruebas de la Historia 5 ⚠️

- [X] T056 [P] [US5] Pruebas unitarias **obligatorias** de la decodificación y validación del
      `PATCH` en `backend/internal/backlog/delivery/validation_test.go` (research.md sección 6):
      campo ausente → sin cambio; `title` y `priority` presentes validados como en la creación y
      **`null` o vacío rechazados**; `description` en `null`, `""` o solo espacios → se borra;
      `acceptanceCriteria` en `[]` → se vacía y en `null` → rechazado; cuerpo sin campos editables →
      rechazado con `field: body`; y `status` y `storyPoints` presentes → rechazados con su campo
      (RN-03 a RN-05, RN-08, RN-10). Un campo inválido junto a otros válidos rechaza el cuerpo
      entero (FR-023) — depende de T055
- [X] T057 [P] [US5] Pruebas unitarias **obligatorias** de `Update` en el servicio en
      `backend/internal/backlog/service/story_service_test.go`, con dobles: historia `completed` →
      `ErrStoryCompleted` (RN-14); proyecto Finalizado → `ErrBacklogReadOnly` (RN-15); historia
      comprometida con `acceptanceCriteria: []` → `ErrCommittedStoryMustStayReady`, y con una lista
      no vacía → aceptada (RN-18); `StoryChanges` **nunca** incluye el estado, el proyecto, el autor
      ni el momento de creación (FR-024, RN-10, RN-11) — depende de T055
- [X] T058 [US5] Pruebas de integración de la historia en
      `backend/internal/backlog/delivery/handler_test.go`, con `// Escenario: <nombre> (003/US5)`:
      `TestUS5_RefinamientoCompleto` (`id`, `status`, `createdBy` y `createdAt` sin cambios),
      `TestUS5_HistoriaDeOtroIntegrante`, `TestUS5_ReordenDeCriterios`, `TestUS5_VaciarCriterios`,
      `TestUS5_HistoriaEnProgreso` (estado sembrado), `TestUS5_ComprometidaNoQuedaSinCriterios`
      (doble), `TestUS5_EntradaInvalidaSinCambioParcial` (subtests título vacío, título de 151,
      descripción de 2001, prioridad inexistente y criterio de 501, cada uno junto a un cambio
      válido que **tampoco** se aplica), `TestUS5_HistoriaCompletada`, `TestUS5_ProyectoFinalizado`,
      `TestUS5_HistoriaAjena` y `TestUS5_CambioDeEstadoNoDisponible` (`PATCH` con `status` → `400`
      y estado `pending` intacto) — depende de T055
- [X] T059 [P] [US5] Pruebas de integración de concurrencia en
      `backend/internal/backlog/delivery/story_concurrency_test.go`, con
      `// Escenario: <nombre> (003/US5)`: `TestUS5_EdicionSimultaneaCamposDistintos`, que envía a
      la vez desde dos goroutines un `PATCH` de prioridad y otro de descripción y verifica que la
      historia queda **con los dos cambios** (FR-025, SC-013), repetido varias veces para que el
      intercalado sea real; y `TestUS5_EdicionSimultaneaDeCriterios`, que envía a la vez dos listas
      distintas y verifica que la historia queda con **una de las dos completa**, nunca con una
      mezcla ni con más criterios que la lista más larga — depende de T055
- [X] T060 [P] [US5] Pruebas de frontend en
      `frontend/src/features/backlog/components/story-edit-view.test.tsx`, con
      `describe("003/US5 - Refinar una historia")` y un `it` por escenario, incluido que el
      formulario de edición **envía solo los campos que la persona cambió**, que es lo que hace
      posible FR-025 desde la interfaz, y que no ofrece cambiar el estado — depende de T055

### Implementación de la Historia 5

- [X] T061 [US5] Declarar `StoryPatch` en `backend/internal/backlog/delivery/dto.go` con sus campos
      como `json.RawMessage`, e implementar en `backend/internal/backlog/delivery/validation.go` su
      decodificación a `StoryChanges` reutilizando las validaciones de la creación de T019 — depende
      de T056
- [X] T062 [US5] Implementar `Update(ctx, storyID, changes)` en
      `backend/internal/backlog/repository/story_repository.go` en **una transacción** que toma un
      bloqueo de fila sobre la historia con `clause.Locking{Strength: "UPDATE"}`, actualiza **solo
      las columnas presentes** más `updated_at` (nunca un `Save` del registro entero) y, si
      `AcceptanceCriteria` está presente, borra los criterios de la historia e inserta la lista
      nueva con posiciones `0..n-1` (research.md sección 7) — depende de T058 y T059
- [X] T063 [US5] Implementar `Update(ctx, access, changes)` en
      `backend/internal/backlog/service/story_service.go` con las reglas de solo lectura, Completada
      y comprometida, en ese orden — depende de T057 y T062
- [X] T064 [US5] Implementar el manejador `updateStory` en `backend/internal/backlog/delivery/handler.go`
      con anotaciones `swag` en español y registrar `PATCH` en el grupo de la historia en
      `backend/internal/backlog/delivery/routes.go`. Respuestas: `200` con `StoryDetail`, `400`,
      `401`, `404` y `409` — depende de T061 y T063
- [X] T065 [US5] Implementar la llamada de modificación en `frontend/src/features/backlog/api.ts`,
      el componente `StoryEditView` en
      `frontend/src/features/backlog/components/story-edit-view.tsx`, que reutiliza `StoryForm`
      precargado y **arma el cuerpo solo con los campos modificados**, y la pantalla
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/edit/page.tsx` — depende de T060 y
      T064

**Checkpoint**: US1 a US5 funcionan de forma independiente.

---

## Fase 7: Historia de Usuario 6 — Eliminar una historia que nunca se trabajó (Prioridad: P3)

**Objetivo**: un integrante borra definitivamente una historia que nunca entró en un sprint ni tiene
historial, y un intento sobre una historia con historial se rechaza indicando qué lo impide.

**Rama**: `hu/003-us6-eliminar-una-historia`

**Prueba independiente**: creando una historia y eliminándola, desaparece del backlog y un segundo
intento responde `404`; con un doble que informa historial de sprint, el intento responde `409` con
el motivo `sprint_history`.

### Pruebas de la Historia 6 ⚠️

- [X] T066 [P] [US6] Pruebas unitarias **obligatorias** de `Delete` en
      `backend/internal/backlog/service/story_service_test.go`, con dobles de los cuatro puertos:
      cada motivo por separado (`sprint_history`, `effort`, `defects`, `estimation_sessions`), dos y
      cuatro motivos a la vez **en ese orden**, ningún motivo → `Delete` del repositorio llamado, y
      proyecto Finalizado → `ErrBacklogReadOnly` sin consultar los puertos (RN-15, RN-16); en todo
      rechazo, `Delete` del repositorio no se llama — depende de T065
- [X] T067 [US6] Pruebas de integración de la historia en
      `backend/internal/backlog/delivery/handler_test.go`, con `// Escenario: <nombre> (003/US6)`:
      `TestUS6_EliminacionExitosa` (`204`, ausente del listado y sin filas en
      `acceptance_criteria`), `TestUS6_ListaParaPlanificarSeElimina`,
      `TestUS6_IntegranteNoPropietario`, `TestUS6_QuitadaDeUnSprint` (doble con `EverAssigned`
      verdadero y `CommittedToOpenSprint` falso → `409` `sprint_history`), `TestUS6_UnicaHistoria`
      (el listado responde `emptyReason: backlog_empty`), `TestUS6_EsfuerzoODefectos` (subtests con
      cada doble → `409` con su motivo e historia intacta), `TestUS6_SesionesDeEstimacion`,
      `TestUS6_ProyectoFinalizado`, `TestUS6_HistoriaAjena` y `TestUS6_YaEliminada` (`404`) —
      depende de T065
- [X] T068 [P] [US6] Pruebas de frontend en
      `frontend/src/features/backlog/components/delete-story-button.test.tsx`, con
      `describe("003/US6 - Eliminar una historia que nunca se trabajó")` y un `it` por escenario,
      incluido que pide confirmación antes de eliminar y que muestra **cada** motivo del rechazo —
      depende de T065

### Implementación de la Historia 6

- [X] T069 [US6] Implementar `Delete` en `backend/internal/backlog/repository/story_repository.go`,
      que borra los criterios de la historia y la historia en **una transacción**, sin borrado
      lógico (research.md sección 12) — depende de T067
- [X] T070 [US6] Implementar `Delete(ctx, access)` en `backend/internal/backlog/service/story_service.go`:
      solo lectura primero; después consulta los cuatro puertos y, si alguno informa historial,
      devuelve `DeletionBlockedError` con **todos** los motivos en el orden de T005 — depende de T066
      y T069
- [X] T071 [US6] Declarar `DeletionBlockedReason` y `DeletionBlockedResponse` en
      `backend/internal/backlog/delivery/dto.go`, implementar el manejador `deleteStory` en
      `backend/internal/backlog/delivery/handler.go` con anotaciones `swag` en español y registrar
      `DELETE` en el grupo de la historia en `backend/internal/backlog/delivery/routes.go`.
      Respuestas: `204`, `401`, `404` y `409` con `DeletionBlockedResponse` o `ErrorResponse` — depende
      de T070
- [X] T072 [US6] Implementar la llamada de eliminación en `frontend/src/features/backlog/api.ts` y el
      componente `DeleteStoryButton` en
      `frontend/src/features/backlog/components/delete-story-button.tsx`, con confirmación, integrado
      en la pantalla de detalle y con regreso al listado al eliminar — depende de T068 y T071

**Checkpoint**: las seis historias funcionan de forma independiente.

---

## Fase 8: Integration (Integración con la vista de estado de la 002)

**Propósito**: que la vista de estado de `specs/002-project-members` informe el backlog real (RC-07),
reemplazando el cuerpo del proveedor que la 002 dejó informando vacío, y que la interfaz del proyecto
muestre los estados en español y lleve al backlog (research.md sección 13). Puede hacerse en cualquier
momento después de US1.

**Rama**: `fase/003-integration`

- [X] T073 Prueba de integración del proveedor del resumen en
      `backend/internal/projects/repository/backlog_summary_test.go`, contra la base de test con
      historias creadas con `StoryRepository.Create`: sin historias → `CountsByState` vacío (no
      nulo) y `EstimatedStoryPoints` 0; con historias `pending`, `in_progress` y `completed`
      sembradas → la cantidad por cada clave presente; las "sin estimar" se cuentan pero **no
      suman**, las estimadas en 0 suman 0 (FR-044 y FR-045 de la 002); y las historias de otro
      proyecto no cuentan — depende de T021
- [X] T074 Reemplazar el cuerpo de `backend/internal/projects/repository/backlog_summary.go` por una
      consulta agrupada sobre el modelo `backlogdomain.UserStory`
      (`SELECT status, COUNT(*), COALESCE(SUM(story_points), 0) ... GROUP BY status`), con
      `NewBacklogSummaryProvider(db *gorm.DB)`; actualizar la composición en
      `backend/cmd/app/main.go` y los ayudantes de prueba de
      `backend/internal/projects/delivery/handler_test.go` que lo construyen. En
      `newStatusTestEnv`, el proveedor recibe **la misma sesión con contador (`serviceDB`)** que
      los repositorios del servicio, para que la prueba de conteo de consultas de la 002 cuente
      también la consulta del backlog y T075 verifique de verdad el límite de 4. **No** se toca
      `projects/service` — depende de T073
- [X] T075 Verificar que la prueba de conteo de consultas de la vista de estado de la 002 (T056 de
      `specs/002-project-members/tasks.md`) siga dentro del límite de **4 consultas** de su SC-003
      ahora que el proveedor del backlog hace una consulta real, y ajustar su comentario si cuenta
      un número distinto — depende de T074
- [X] T076 [P] Pruebas de frontend en
      `frontend/src/features/projects/components/project-status-card.test.tsx` y
      `frontend/src/features/projects/components/project-overview.test.tsx`: la tarjeta muestra
      "Pendiente", "En progreso" y "Completada" en lugar de las claves crudas, y la vista del
      proyecto ofrece un enlace al backlog — depende de T025
- [X] T077 Usar las etiquetas de `frontend/src/utils/story-labels.ts` en
      `frontend/src/features/projects/components/project-status-card.tsx` y agregar el enlace al
      backlog en `frontend/src/features/projects/components/project-overview.tsx` — depende de T038 y
      T076

**Checkpoint**: la vista de estado de la 002 informa el backlog real y la navegación llega al backlog.

---

## Fase 9: Polish (Cierre y asuntos transversales)

**Propósito**: documentación viva, cobertura, vulnerabilidades y verificación de las puertas de
calidad. Todas las tareas son **técnicas**, salvo T078, que es una prueba de caracterización sin RED.

**Rama**: `fase/003-polish`

- [X] T078 Pruebas de integración de caracterización en
      `backend/internal/backlog/delivery/handler_test.go`, **sin** etiqueta `003/USn`:
      `TestStoryEndpoints_SinSesion`, con un subtest por cada uno de los **seis** endpoints sin
      cabecera y con token adulterado → `401` y ningún cambio (SC-005);
      `TestStoryEndpoints_NotFoundIndistinguible`, que compara byte a byte el `404` de historia ajena
      e inexistente en las **cuatro** acciones de ítem y el de proyecto ajeno e inexistente en las
      **dos** de colección (SC-006); y una aserción de que ninguna respuesta contiene
      `password_hash` ni `passwordHash` (Principio VI). Prueba de caracterización: las rutas ya están
      detrás de los middlewares, así que no tiene RED; el commit lo justifica — depende de T072
- [X] T079 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las seis
      rutas, sus códigos de respuesta (todos declaran `500`) y los diecisiete esquemas; corregir las
      anotaciones si difieren (Principio VIII) — depende de T072
- [X] T080 [P] Actualizar `README.md` de la raíz en español: la sección introductoria menciona el
      Product Backlog, y se agregan los seis endpoints y las cuatro pantallas nuevas, manteniendo el
      criterio de que alguien que no conoce el proyecto pueda ejecutarlo siguiendo el documento —
      depende de T072
- [X] T081 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [X] T082 [P] Ejecutar los comandos de cobertura de los dos lados —
      `go test ./... -coverprofile=coverage.out` más `go tool cover -func=coverage.out` en
      `backend/`, y `npm run test:coverage` en `frontend/` — y revisar que el módulo
      `internal/backlog/` y `frontend/src/features/backlog/` estén cubiertos — depende de T072
- [X] T083 Verificar la trazabilidad y las puertas de seguridad recorriendo `backend/internal/backlog/`
      y `frontend/src/features/backlog/`: que buscar `003/US1` a `003/US6` en el código de pruebas
      devuelva los **58** escenarios; que las **18** reglas RN-01 a RN-18 tengan prueba unitaria
      (SC-014); que los **seis** endpoints tengan su prueba de `401` (SC-005); que los `404` sean
      indistinguibles (SC-006); que ninguna operación de esta feature escriba `user_stories.status`
      después de la creación (FR-027); y que ninguna respuesta incluya `password_hash` — depende de
      T078
- [X] T084 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluidas la
      vista de estado de la 002 con historias reales, las pruebas de concurrencia y la verificación
      de que las pruebas corrieron contra `smye_test` en el puerto 5433 y no contra desarrollo —
      depende de T075, T077, T079, T080 y T083

---

## Phase 10: Convergence and Security

Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.

- [ ] T085 Agregar en `backend/internal/backlog/delivery/validation_test.go` una prueba unitaria de
      `ParseStoryFilter` con miles de valores repetidos de `status` y de `priority` que verifique que
      el filtro resultante contiene cada valor admitido una sola vez, y en
      `backend/internal/backlog/delivery/handler_test.go` una prueba de integración de
      `GET /api/v1/projects/{projectId}/stories` con miles de `status=pending` que espere `200` y no
      `500` per hallazgo de seguridad: filtros repetibles sin tope que arman una cláusula IN con un
      parámetro por valor (security) — origen: seguridad, detectado en 003/US2
- [ ] T086 Eliminar los valores duplicados de cada filtro en `ParseStoryFilter` de
      `backend/internal/backlog/delivery/validation.go`, de modo que la cláusula `IN` reciba como
      máximo los tres valores admitidos por filtro per hallazgo de seguridad: filtros repetibles sin
      tope que arman una cláusula IN con un parámetro por valor (security) — origen: seguridad,
      detectado en 003/US2
- [ ] T087 Agregar pruebas de frontend en `frontend/src/features/backlog/api.test.ts` y
      `frontend/src/features/projects/api.test.ts` que llamen a cada función de los clientes de API
      con un identificador que contenga `../` y `/`, y verifiquen que la URL pedida a `fetch`
      conserva la ruta esperada con el segmento codificado per hallazgo de seguridad: identificadores
      interpolados sin codificar en las rutas de los clientes de API (security) — origen: seguridad,
      detectado en 003/US2
- [ ] T088 Codificar con `encodeURIComponent` cada segmento de ruta interpolado en
      `frontend/src/features/backlog/api.ts` y `frontend/src/features/projects/api.ts`, incluidas las
      llamadas de `DELETE` y `PATCH` per hallazgo de seguridad: identificadores interpolados sin
      codificar en las rutas de los clientes de API (security) — origen: seguridad, detectado en
      003/US2
- [ ] T090 Corregir los comentarios de `isValidStatus` e `isValidPriority` en
      `backend/internal/backlog/delivery/validation.go` para indicar que la comparación es exacta y
      rechaza otras grafías per Constitución VIII (contradicts) — origen: converge, detectado en
      hu/003-us2-ver-el-backlog
- [ ] T091 Agregar pruebas de integración en `backend/internal/backlog/delivery/handler_test.go`
      que estimen una historia y creen una historia en un proyecto que pasa a `finished` después
      de resolver el acceso, y verifiquen que se rechazan con `409` de solo lectura y que la base
      no cambia per hallazgo de seguridad: regla de solo lectura del proyecto Finalizado verificada
      fuera de la escritura (security) — origen: seguridad, detectado en 003/US4
- [ ] T092 Hacer que `UpdateStoryPoints` y `Create` de
      `backend/internal/backlog/repository/story_repository.go` comprueben en la propia escritura
      que el proyecto no está Finalizado (condición en el `UPDATE` o bloqueo `FOR SHARE` de la fila
      del proyecto dentro de la transacción), devolviendo `ErrBacklogReadOnly` cuando corresponda
      per hallazgo de seguridad: regla de solo lectura del proyecto Finalizado verificada fuera de
      la escritura (security) — origen: seguridad, detectado en 003/US4
- [ ] T093 Agregar prueba de integración en `backend/internal/backlog/delivery/handler_test.go` que
      elimina con DELETE /api/v1/stories/{storyId} una historia de un proyecto que pasa a `finished`
      después de que se resuelve el acceso, y verifica que responde 409 con el mensaje de solo lectura
      y que la historia y sus criterios siguen en la base per hallazgo de seguridad: regla de solo
      lectura del proyecto Finalizado verificada fuera de la escritura en la eliminación (security)
      — origen: seguridad, detectado en 003/US6
- [ ] T094 Hacer que `Delete` de `backend/internal/backlog/repository/story_repository.go` compruebe
      dentro de su transacción que el proyecto de la historia no está Finalizado (bloqueo `FOR SHARE`
      de la fila de `projects` o condición en el `DELETE` de `user_stories`) y devuelva
      `ErrBacklogReadOnly` en ese caso, para que `respondServiceError` lo traduzca al 409 de solo
      lectura per hallazgo de seguridad: regla de solo lectura del proyecto Finalizado verificada
      fuera de la escritura en la eliminación (security) — origen: seguridad, detectado en 003/US6
- [X] T095 Corregir las etiquetas de trazabilidad en
      `backend/internal/backlog/delivery/middleware_test.go`: quitar `003/US3` de los comentarios
      `// Escenario:` de `TestUS3_AccesoIntegranteNoPropietarioPasa`,
      `TestUS3_AccesoHistoriaAjenaIgualAInexistente` y `TestUS3_AccesoSinSesion`, y renombrarlas sin
      el prefijo `TestUS3_` por ser pruebas transversales de T039 (plan.md: no alteran el conteo de
      58); y unificar `it("Story Points fuera de escala")` de
      `frontend/src/features/backlog/components/story-form.test.tsx` con el nombre del escenario
      "Story Points fuera de la escala". Solo renombra: no tiene RED — origen: T083, trazabilidad
      003/US3 con 9 nombres en lugar de 6

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Foundational**: sin dependencias dentro de la feature; parte de `main` con la 001 y la
  002 integradas. **BLOQUEA** todas las historias.
- **Fases 2 a 7 (historias)**: todas dependen de la Fase 1, en orden de prioridad
  (P1 → P1 → P2 → P2 → P3 → P3).
- **Fase 8 Integration**: depende de US1 (T028) para el backend y de US2 (T038) para el enlace.
- **Fase 9 Polish**: depende de que estén completas las historias y la integración.

### Dependencias entre historias

Como en la 002, las historias **no son todas independientes en su implementación**, aunque cada una
se prueba sola:

- **US1 Registrar (P1)**: puede empezar después de la Fase 1. **Aporta** la validación (T019), el
  servicio con `buildDetail` (T022) y la composición en `cmd/app/main.go` (T024) que usan todas las
  demás.
- **US2 Ver el backlog (P1)**: necesita historias creadas por US1.
- **US3 Consultar el detalle (P2)**: necesita US1 y **aporta** `RequireStoryAccess` y el grupo
  `/api/v1/stories/:storyId` (T043, T045) que usan US4 a US6.
- **US4 Estimar (P2)**: necesita el grupo de la historia de US3 y la validación de la escala de US1.
- **US5 Refinar (P3)**: necesita el grupo de la historia de US3 y las validaciones de US1.
- **US6 Eliminar (P3)**: necesita el grupo de la historia de US3 y los puertos de la Fase 1 (T008).

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T001 a T005 en paralelo; después T006 y T007, y T008 en paralelo con T009.
- Dentro de cada historia: las pruebas unitarias, las de concurrencia y las de frontend marcadas
  `[P]` en paralelo entre sí. Las de integración **no** son paralelizables entre historias porque
  las seis comparten `handler_test.go`.
- Fase 8: T076 en paralelo con T073.
- Fase 9: T080 y T082 en paralelo.
- Con varias personas: el orden US1 → US3 → {US4, US5, US6} refleja la dependencia real del
  middleware de la historia; US2 puede avanzar en paralelo con US3 una vez terminada US1.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las cinco tareas de pruebas marcadas [P] pueden escribirse a la vez:
T011  backend/internal/backlog/domain/story_points_test.go
T012  backend/internal/backlog/domain/readiness_test.go
T013  backend/internal/backlog/delivery/validation_test.go
T014  backend/internal/backlog/service/story_service_test.go
T016  frontend/src/features/backlog/components/story-form.test.tsx

# T015 (handler_test.go) va aparte: ese archivo lo comparten las seis historias.

# En la implementación, estas no se pisan:
T017  backend/internal/backlog/domain/story_points.go
T020  backend/internal/backlog/repository/project_directory.go
T025  frontend/src/types/story.ts + frontend/src/utils/story-labels.ts
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar la Fase 1 (Foundational) — **CRÍTICO**: bloquea todas las historias.
2. Completar la Fase 2 (US1 Registrar una historia).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con la guía de validación.
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Foundational → base lista (una fase técnica, un pull request).
2. US1 Registrar → probar sola → entregar (**MVP**).
3. US2 Ver el backlog → probar sola → entregar (con esto el backlog es utilizable de punta a punta).
4. US3 Consultar el detalle → probar sola → entregar (activa el aislamiento por historia).
5. US4 Estimar → probar sola → entregar (el backlog ya produce Story Points para las métricas).
6. US5 Refinar → probar sola → entregar.
7. US6 Eliminar → probar sola → entregar.
8. Integration → la vista de estado de la 002 informa el backlog real.
9. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Foundational en conjunto.
2. US1 primero: aporta la validación, el servicio y la composición que usan todas las demás.
3. Terminada US1, US2 y US3 en paralelo; la Fase 8 también puede avanzar.
4. Terminada US3, US4, US5 y US6 en paralelo, coordinando `handler.go`, `handler_test.go`,
   `story_service.go` y `story_service_test.go`, que las tres tocan.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `003/US1` a `003/US6` en el código de
  pruebas devuelve la trazabilidad completa de los 58 escenarios exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 003/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b). Los commits los registra **una persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni de npm. Si alguna tarea parece necesitar
  una, detenerse y pedir autorización (Principio IV).
- Tres puntos del plan esperan confirmación del desarrollador y afectan al contrato o al modelo: el
  rechazo de `status` y `storyPoints` en el `PATCH` (T056, T061), la lectura de FR-049 que persiste
  solo el autor (T001) y la modificación de código de la 002 en la Fase 8 (T074, T077).
- **Verificaciones que esta feature no puede cerrar sola.** Seis reglas se prueban acá con dobles
  de los puertos, y su verificación de punta a punta queda para las features que producen esos
  datos. Hay que anotarlo al planificarlas:

  | Requisito | Cómo se verifica hoy | Quién la cierra |
  | --- | --- | --- |
  | FR-018, FR-022, RN-18 — historia comprometida no deja de estar lista | T047, T049, T057, T058 con dobles de `SprintAssignments` | `specs/004-sprint-management`, reemplazando `sprint_assignments.go` |
  | FR-039, FR-041 — historial de sprint en el detalle y en la eliminación | T040, T067 con dobles | `specs/004-sprint-management` |
  | FR-041, FR-043 — esfuerzo registrado | T066, T067 con dobles de `EffortRecords` | `specs/006-effort-tracking`, reemplazando `effort_records.go` |
  | FR-041, FR-043 — defectos asociados | T066, T067 con dobles de `DefectRecords` | `specs/007-defect-tracking`, reemplazando `defect_records.go` |
  | FR-041, RC-10 — sesiones de Planning Poker | T066, T067 con dobles de `EstimationSessions` | `specs/005-planning-poker`, reemplazando `estimation_sessions.go` |
  | FR-027, RC-04 — solo los sprints cambian el estado | T058 y T083: el backlog nunca escribe `status` | `specs/004-sprint-management`, que es la única que lo escribe |

  Además, cuando esas features creen tablas que referencien `user_stories`, la violación de clave
  foránea al borrar una historia debe traducirse al mismo `409` de eliminación bloqueada
  (research.md sección 12), y Planning Poker debe registrar el valor acordado con
  `StoryService.Estimate` (T053).
- **Criterios de éxito no automatizables.** SC-001 (registrar una historia en menos de 2 minutos,
  95 % al primer intento), SC-002 (estimar en menos de 30 segundos y en no más de 3 pasos) y SC-003
  (encontrar las Altas sin estimar en menos de 15 segundos) son métricas de experiencia de uso: se
  validan a mano siguiendo la sección "Flujo en la interfaz" de [quickstart.md](./quickstart.md).
  T050 verifica la parte automatizable de SC-002: la cantidad de pasos.

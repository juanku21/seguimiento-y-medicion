---

description: "Lista de tareas de la feature 004 — Gestión de Sprints"
---

# Tareas: Gestión de Sprints

**Entrada**: documentos de diseño de `specs/004-sprint-management/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) y
[`specs/002-project-members`](../002-project-members/spec.md) están integradas en `main`.
**[`specs/003-product-backlog`](../003-product-backlog/spec.md) DEBE estar integrada en `main` antes
de empezar la Fase 1**: esta feature usa su módulo `internal/backlog/` (modelo `UserStory`, tipos
`StoryStatus` y `StoryPriority`, función `Readiness`, error `ErrStoryNotFound`, middleware
`RequireStoryAccess`, puerto `SprintAssignments` y sus endpoints). Si la 003 se implementó con
nombres distintos de los de su plan, se ajustan las referencias de estas tareas sin cambiar las
decisiones (research.md sección 0). Se reutilizan además el middleware de sesión, el accesor
`authdelivery.CurrentUserID`, el subgrupo `/api/v1/projects/:projectId` con `RequireMembership`, los
esquemas de error de la 001, el límite global de 1 MiB, la zona `APP_TIMEZONE` y el ayudante
`testsupport.OpenDatabase`. **No hay fase de Setup**: no se agregan dependencias, servicios ni
variables de entorno.

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para las
validaciones, la superposición de períodos, las transiciones del sprint y de la historia, la
admisión de historias, **los Story Points congelados de la instantánea**, los días restantes y las
reglas de negocio RN-01 a RN-23.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/sprints/{domain,repository,service,delivery}/`; se
  tocan de forma puntual `backend/cmd/app/main.go`,
  `backend/internal/platform/database/postgres.go`,
  `backend/internal/platform/testsupport/database.go`,
  `backend/internal/backlog/domain/user_story.go`,
  `backend/internal/backlog/repository/sprint_assignments.go`,
  `backend/internal/projects/repository/sprint_directory.go` y los ayudantes de prueba de
  `backend/internal/projects/delivery/handler_test.go`
- **Frontend**: `frontend/src/app/projects/[projectId]/sprints/`, `frontend/src/features/sprints/`,
  `frontend/src/types/sprint.ts`, `frontend/src/utils/sprint-labels.ts`; se tocan
  `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx` y
  `frontend/src/features/projects/components/project-overview.tsx`
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

- Las historias se crean y se modifican con los **endpoints reales de la 003**, que el router de
  prueba compone junto con el módulo de sprints. Así US2-4, US2-14 y US7-7 ejercitan el puerto
  `SprintAssignments` reemplazado por esta feature.
- El esfuerzo registrado (US7-8, US7-14) se prepara con un **doble de `testify/mock`** del puerto
  `SprintEffort`, inyectado al componer el router de prueba.
- Los instantes (US4-5, US5-7, US5-8, US6-4 a US6-6, US6-9) se fijan con el **reloj inyectado** en el
  servicio (`now func() time.Time`) y la zona de `backend/.env.test`.
- El cambio del factor del proyecto (US5-4) y la finalización del proyecto usan los endpoints reales
  de la 002.
- Las pruebas sin sesión, la comparación byte a byte de los `404` y la de 10 inicios simultáneos
  **no** llevan la etiqueta `004/USn`, porque no derivan de un escenario de la spec y no deben
  alterar el conteo de 82.

---

## Fase 1: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el dominio del módulo nuevo, su contrato de repositorio y su puerto hacia la 006, la
columna nueva de `user_stories`, la migración de las dos tablas con sus índices parciales y el
aislamiento de las pruebas. Todas las tareas son **técnicas**: declaran estructuras y contratos, sin
lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/004-foundational`

**Precondición (verificar antes de T001)**: `git log main --oneline -- backend/internal/backlog/`
devuelve commits y existen `backend/internal/backlog/domain/user_story.go`,
`backend/internal/backlog/repository/sprint_assignments.go` y
`backend/internal/backlog/delivery/middleware.go` (`RequireStoryAccess`). Si alguno falta, la 003
no está integrada y esta fase **no empieza**. Si la 003 se integró con nombres distintos de los de
su plan, se corrigen las rutas de T007, T013 y T033 antes de ejecutarlas.

- [X] T001 [P] Declarar la entidad `Sprint` en `backend/internal/sprints/domain/sprint.go` con los
      campos de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`;
      `ProjectID uuid NOT NULL` con FK a `projects.id` (campo `Project *projectsdomain.Project` solo
      para la FK, nunca precargado); `Name varchar(100) NOT NULL` **no único**;
      `Goal varchar(500) NOT NULL`; `StartDate date NOT NULL` y `EndDate date NOT NULL` (medianoche
      UTC, como las fechas del proyecto de la 002); `Status varchar(16) NOT NULL`;
      `CommittedStoryPoints integer` **anulable** (`*int`, se fija al iniciar);
      `StartedAt timestamptz` y `ClosedAt timestamptz` **anulables**; `ClosedByID uuid` **anulable**
      con FK a `users.id`; `HoursPerStoryPointHundredths integer` **anulable** (factor copiado al
      cerrar, en centésimas como en la 002); `CreatedByID uuid NOT NULL` con FK a `users.id`;
      `CreatedAt` y `UpdatedAt` `timestamptz NOT NULL`. Índice compuesto
      `idx_sprints_project_start (project_id, start_date, id)` e **índice único parcial**
      `idx_sprints_one_active_per_project` sobre `(project_id) WHERE status = 'active'`, declarado
      con la opción `where` de la etiqueta de índice de GORM. **Sin** tabla ni columnas de totales
      planificados o completados: se derivan (research.md sección 11)
- [X] T002 [P] Declarar la entidad `SprintStory` en `backend/internal/sprints/domain/sprint_story.go`
      con los campos de data-model.md sección 2: `ID uuid` PK; `SprintID uuid NOT NULL` con FK a
      `sprints.id`; `StoryID uuid NOT NULL` con FK a `user_stories.id`;
      `CommittedAt timestamptz NOT NULL`; `CompletedAt timestamptz` **anulable**;
      `TitleAtClose varchar(150)`, `PriorityAtClose varchar(8)`, `StoryPointsAtClose smallint` y
      `Outcome varchar(16)` **anulables** (`NULL` mientras el sprint está abierto; `Outcome` toma
      `completed` o `not_completed` al cerrar); `CreatedAt` y `UpdatedAt` `timestamptz NOT NULL`.
      **Índice único** `idx_sprint_stories_sprint_story (sprint_id, story_id)`, **índice único
      parcial** `idx_sprint_stories_one_open_per_story` sobre `(story_id) WHERE outcome IS NULL` e
      índice `idx_sprint_stories_story (story_id, committed_at)`. Sin `ON DELETE CASCADE`
- [X] T003 [P] Declarar el tipo `SprintStatus` con sus tres constantes `planned`, `active` y `closed`
      en `backend/internal/sprints/domain/status.go`, y el tipo `StoryOutcome` con `completed` y
      `not_completed` en `backend/internal/sprints/domain/sprint_story.go`, como constantes de Go y
      **no** como `enum` de PostgreSQL. Las transiciones son regla de negocio y se implementan en
      US3
- [X] T004 [P] Declarar los errores de dominio en `backend/internal/sprints/domain/errors.go` con la
      tabla de data-model.md sección 9: `ErrSprintNotFound`, `ErrSprintsReadOnly`,
      `ErrInvalidSprintTransition`, `ErrSprintNotPlanned`, `ErrSprintNotActive`, `ErrSprintClosed`,
      `ErrSprintAlreadyStarted`, `ErrSprintHasEffort`, `ErrCompletedStoryRemoval`,
      `ErrProgressOutsideActiveSprint` y `ErrInvalidStoryTransition`; los tipos con datos
      `SprintOverlapError` y `ActiveSprintExistsError` (ambos con un `ConflictingSprint` de `ID` y
      `Name`) y `StoriesNotAdmittedError` (con `[]AdmissionProblem`, cuyo `Reason` es
      `not_ready`, `completed` o `committed_elsewhere`, más `Missing []string` y
      `Sprint *ConflictingSprint`). Para la historia inexistente se reutiliza `ErrStoryNotFound` de
      la 003
- [X] T005 Declarar en `backend/internal/sprints/domain/repository.go` la interfaz
      `SprintRepository` con **exactamente** las trece operaciones de data-model.md sección 9
      (`FindAccessible`, `Create`, `Update`, `Delete`, `Start`, `Close`, `AssignStories`,
      `RemoveStories`, `ChangeStoryProgress`, `List`, `HasSprints`, `FindDetail`, `ListForStory`),
      con las firmas de esa sección, más los tipos de lectura de la sección 8: `SprintAccess`,
      `SprintInput`, `SprintListItem`, `SprintResults`, `SprintDetail` (con `CreatedBy` y
      `ClosedBy` de solo `ID` y `FullName`), `SprintStoryView`, `StorySprintHistoryItem` y
      `ConflictingSprint`. Las operaciones de escritura reciben el instante como parámetro. **Prohibido**
      declarar cambio de proyecto, reapertura o listados fuera de un proyecto (Principio III) —
      depende de T001 a T004
- [X] T006 Declarar el puerto `SprintEffort.HasEffort(ctx, sprintID) (bool, error)` en
      `backend/internal/sprints/domain/ports.go` e implementarlo en
      `backend/internal/sprints/repository/sprint_effort.go` informando que **ningún sprint tiene
      esfuerzo registrado**, con un comentario en español que indique que
      `specs/006-effort-tracking` reemplazará su cuerpo y deberá traducir la violación de su clave
      foránea hacia `sprints.id` al mismo `409` (research.md sección 16)
- [X] T007 Agregar el campo `EverCommitted bool` con la etiqueta `not null;default:false` al modelo
      `UserStory` en `backend/internal/backlog/domain/user_story.go`, con un comentario en español
      que indique que **solo la feature de sprints lo escribe**, que pasa a `true` al primer
      compromiso y nunca vuelve a `false` (research.md sección 7; data-model.md sección 3)
- [X] T008 Agregar `Sprint` y `SprintStory` al `AutoMigrate` de
      `backend/internal/platform/database/postgres.go`, **después** de `UserStory` y
      `AcceptanceCriterion`, de modo que los índices —incluidos los dos parciales con su cláusula
      `WHERE`— queden creados por las etiquetas de GORM y no por un `CREATE INDEX` en crudo, y la
      columna `ever_committed` se agregue a `user_stories` — depende de T001, T002 y T007
- [X] T009 Extender el vaciado del ayudante de pruebas en
      `backend/internal/platform/testsupport/database.go` para incluir `sprints` y `sprint_stories`
      en el `TRUNCATE ... CASCADE`, manteniendo la verificación de que la base es `smye_test` y el
      lock de la base de test — depende de T008

**Checkpoint**: las dos tablas y la columna migradas en la base de test, con los índices parciales
visibles en `pg_indexes` (quickstart.md sección 2), el dominio y los contratos declarados, y las
pruebas aisladas. Las historias pueden empezar.

---

## Fase 2: Historia de Usuario 1 — Planificar un sprint (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante crea el próximo sprint con nombre, fechas y Sprint Goal; nace
Planificado, vacío y atribuido a quien lo creó, sin superponerse con otro sprint del proyecto.

**Rama**: `hu/004-us1-planificar-un-sprint`

**Prueba independiente**: `POST /api/v1/projects/{projectId}/sprints` devuelve `201` con
`status: planned`, `stories: []` y `createdBy` del titular de la sesión, y un segundo sprint que se
solapa un día responde `409` con el nombre del primero.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T010 [P] [US1] Pruebas unitarias **obligatorias** de `PeriodsOverlap` en
      `backend/internal/sprints/domain/period_test.go`, cubriendo RN-05 y SC-004: comparten solo el
      día de borde (10-01→10-14 y 10-14→10-28) → **no** se superponen, en los dos órdenes; un día de
      solape (10-01→10-14 y 10-13→10-20) → sí; período idéntico → sí; contención total en los dos
      sentidos → sí; disjuntos con un día de separación → no — depende de T009
- [X] T011 [P] [US1] Pruebas unitarias **obligatorias** de validación en
      `backend/internal/sprints/delivery/validation_test.go`, cubriendo RN-03 y RN-04: nombre
      **obligatorio, entre 1 y 100 runas tras `TrimSpace`**, con ausente o solo espacios rechazado y
      100 válido y 101 rechazado; Sprint Goal **obligatorio, entre 1 y 500 runas tras `TrimSpace`**,
      con 500 válido y 501 rechazado; `startDate` y `endDate` **obligatorias con formato
      `YYYY-MM-DD`**, con ausentes y mal formadas rechazadas; `endDate` igual o anterior a
      `startDate` rechazada en el campo `endDate`, y el día siguiente aceptado. La validación
      devuelve **todos** los campos inválidos a la vez — depende de T009
- [X] T012 [P] [US1] Pruebas unitarias **obligatorias** de la creación en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles de `testify/mock` de
      `SprintRepository` y `SprintEffort` y un reloj fijo: el sprint nace `planned` (FR-006) con un
      UUID nuevo, sin `CommittedStoryPoints`, `StartedAt` ni datos de cierre; el autor es el actor
      recibido y no un dato de la entrada (FR-060); un actor que no es el propietario crea igual
      (RN-01); y los errores `ErrSprintsReadOnly` y `SprintOverlapError` del repositorio se
      propagan sin envolverlos de forma que se pierdan (RN-19) — depende de T009
- [X] T013 [US1] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con un ayudante `newSprintTestEnv` que
      compone el router como `cmd/app` (sesión de la 001, rutas de la 002 con su subgrupo
      `/:projectId`, rutas de la 003 con `RequireStoryAccess`, rutas de sprints y
      `httpserver.LimitRequestBody`) y que permite inyectar el reloj y un doble de `SprintEffort`.
      Una función por escenario de spec.md precedida por `// Escenario: <nombre del escenario>
      (004/US1)`: `TestUS1_CreacionEnProyectoEnCurso` (`201`, `planned`, `stories: []`,
      `createdBy` del titular), `TestUS1_ProyectoPlanificado` (el proyecto **sigue `planned`**,
      leído con el `GET` de la 002), `TestUS1_VariosPlanificados`, `TestUS1_FechasContiguas`,
      `TestUS1_LimiteDelSprintGoal` (500 y 501), `TestUS1_DuracionMinima`,
      `TestUS1_CamposObligatorios` (subtests sin nombre, sin Sprint Goal, sin inicio y sin fin),
      `TestUS1_FinNoPosteriorAlInicio` (subtests igual y anterior), `TestUS1_Superposicion` (`409`
      con `sprint.id` y `sprint.name` del sprint en conflicto), `TestUS1_ProyectoFinalizado` (`409`)
      y `TestUS1_NoIntegrante` (`404` con **el mismo cuerpo** que un `projectId` inexistente). En
      todos los rechazos se verifica contando filas que **no se creó ningún sprint** (FR-011).
      Además crea `backend/internal/sprints/delivery/sprint_concurrency_test.go` con
      `TestSprintCreate_Concurrency`, **sin** etiqueta `004/USn`: diez goroutines crean a la vez
      sprints del mismo proyecto con períodos que se superponen de a pares; al terminar, ningún par
      de sprints del proyecto cumple `PeriodsOverlap` (SC-004). Repetida varias veces para que el
      intercalado sea real — depende de T009
- [X] T014 [P] [US1] Pruebas de frontend en
      `frontend/src/features/sprints/components/sprint-form.test.tsx` y
      `frontend/src/features/sprints/validation.test.ts`, con
      `describe("004/US1 - Planificar un sprint")` y un `it` por escenario de la spec, incluido que
      el formulario **muestra antes de enviar** el máximo de 100 caracteres del nombre y de 500 del
      Sprint Goal, y que avisa cuando la fecha de fin no es posterior a la de inicio

### Implementación de la Historia 1

- [X] T015 [US1] Implementar `Period` y `PeriodsOverlap(a, b Period) bool` en
      `backend/internal/sprints/domain/period.go` con la condición
      `a.Start < b.End && b.Start < a.End` (research.md sección 4) — depende de T010
- [X] T016 [US1] Implementar la normalización y validación del cuerpo `SprintInput` en
      `backend/internal/sprints/delivery/validation.go`, con los límites 100 y 500 declarados como
      **constantes nombradas en un único lugar**, las fechas parseadas con `time.DateOnly` y un
      `FieldError` por cada campo inválido. Se reutilizará sin cambios en la modificación (US7). La
      validación ocurre antes de la capa de servicio (Principio VII) — depende de T011
- [X] T017 [US1] Implementar `Create` y `FindDetail` en
      `backend/internal/sprints/repository/sprint_repository.go`: `Create` abre **una transacción**
      que toma el bloqueo de fila del proyecto con `clause.Locking{Strength: "UPDATE"}` leyendo
      `projects.status` (Finalizado → `ErrSprintsReadOnly`), busca el sprint del proyecto que se
      superpone con `start_date < ? AND ? < end_date` ordenado por `start_date, id` (si existe →
      `SprintOverlapError` con ese sprint) e inserta; `FindDetail` lee el sprint con `JOIN` a `users`
      que **selecciona solo `users.id` y `users.full_name`** (nunca `password_hash`) y sus historias
      vivas desde `sprint_stories JOIN user_stories`, en dos consultas, y devuelve
      `ErrSprintNotFound` si no hay fila (research.md secciones 4 y 19) — depende de T005, T013 y
      T015
- [X] T018 [US1] Implementar en `backend/internal/sprints/service/sprint_service.go` el constructor
      `NewSprintService(repo SprintRepository, effort SprintEffort, location *time.Location, now
      func() time.Time)`, con la firma definitiva para que no cambie en las historias siguientes; el
      método `Create(ctx, projectID, actorID, input)`, que construye el sprint con UUID, estado
      `planned` y autor el actor; y el armado del detalle (`buildDetail`), que reutilizarán todas las
      historias — depende de T012 y T017
- [X] T019 [US1] Declarar en `backend/internal/sprints/delivery/dto.go` los esquemas de
      contracts/openapi.yaml `SprintInput`, `SprintDetail`, `SprintStoryItem`, `UserRef`,
      `SprintRef` y `SprintConflictResponse`, reutilizando `ErrorResponse`, `FieldError` y
      `ValidationErrorResponse` de la 001. Las fechas se serializan con `time.DateOnly`; `startedAt`,
      `closedAt`, `closedBy`, `results`, `hoursPerStoryPoint` y `schedule` como `null` cuando no
      tienen valor, y `stories` como `[]`, nunca como `null` — depende de T018
- [X] T020 [US1] Implementar el manejador `createSprint` en
      `backend/internal/sprints/delivery/handler.go` con anotaciones `swag` en español, que lee al
      titular con `authdelivery.CurrentUserID` y el proyecto del parámetro `projectId` ya validado
      por `RequireMembership`; crear `RegisterRoutes` en
      `backend/internal/sprints/delivery/routes.go` registrando `POST /sprints` en el **subgrupo
      `/api/v1/projects/:projectId` que devuelve `projectsdelivery.RegisterRoutes`**, sin que
      `sprints/delivery` importe `projects/delivery`; y componer el módulo en
      `backend/cmd/app/main.go` (servicio con `cfg.Location` y `time.Now`, repositorio y puerto de
      esfuerzo), más el `@description` general de la API para que mencione los sprints. Respuestas:
      `201` con `SprintDetail`, `400`, `401`, `404` de `RequireMembership` y `409` con
      `SprintConflictResponse` o `ErrorResponse` (`El proyecto está finalizado y sus sprints son de
      solo lectura.`) — depende de T013, T016 y T019
- [X] T021 [P] [US1] Declarar los tipos de petición y respuesta en `frontend/src/types/sprint.ts` y,
      en `frontend/src/utils/sprint-labels.ts`, las etiquetas en español del estado del sprint
      (`planned` → Planificado, `active` → Activo, `closed` → Cerrado) y del resultado
      (`completed` → Completada, `not_completed` → No completada). **Prohibido el tipo `any`**
      (Principio VIII)
- [X] T022 [US1] Implementar la llamada de creación en `frontend/src/features/sprints/api.ts` sobre
      el cliente `fetch` de la 001, y las reglas del formulario en
      `frontend/src/features/sprints/validation.ts` con los límites 100 y 500 como constantes
      nombradas en un único lugar — depende de T014 y T021
- [X] T023 [US1] Implementar `SprintForm` en
      `frontend/src/features/sprints/components/sprint-form.tsx` con los mensajes en español, los
      límites visibles antes de enviar y la indicación del sprint en conflicto cuando la respuesta es
      `409`. Antes de escribir código, leer la documentación de Next.js de
      `frontend/node_modules/next/dist/docs/` — depende de T022
- [X] T024 [US1] Crear la pantalla `frontend/src/app/projects/[projectId]/sprints/new/page.tsx`,
      protegida con la guardia de sesión de la 001 — depende de T023

**Checkpoint**: US1 funciona y se prueba sola. El MVP está entregable.

---

## Fase 3: Historia de Usuario 2 — Comprometer historias en un sprint (Prioridad: P1)

**Objetivo**: un integrante compromete en un sprint abierto historias del backlog listas para
planificar y quita las que ya no entran; cada lote es todo o nada, y una historia comprometida no
puede dejar de estar lista para planificar.

**Rama**: `hu/004-us2-comprometer-historias`

**Prueba independiente**: creando un sprint Planificado y tres historias listas,
`POST /api/v1/sprints/{sprintId}/stories` las compromete; un lote con una historia sin criterios
responde `409` y no asigna ninguna; y quitarlas las devuelve al backlog como `pending`.

### Pruebas de la Historia 2 ⚠️

- [X] T025 [P] [US2] Pruebas unitarias **obligatorias** de `Admission` en
      `backend/internal/sprints/domain/admission_test.go`, cubriendo RN-08 y SC-007: historia lista,
      `pending` y sin compromiso abierto → sin problemas; sin Story Points, sin criterios y sin
      ninguno de los dos → `not_ready` con `missing` igual al de `Readiness` de la 003;
      `completed` → `completed`; con compromiso abierto en otro sprint o en este mismo →
      `committed_elsewhere` con ese sprint; y una historia que cumple varias condiciones informa
      **todos** sus motivos — depende de T024
- [X] T026 [P] [US2] Pruebas de integración de `RequireSprintAccess` en
      `backend/internal/sprints/delivery/middleware_test.go`, montado sobre un manejador de prueba:
      un integrante que no es el propietario pasa y recibe en el contexto un `SprintAccess` con el
      sprint, su proyecto y si está Finalizado (RN-01); el sprint inexistente, el de un proyecto
      ajeno, el de un integrante ya quitado y un identificador que no es UUID responden **el mismo
      `404` byte a byte** (`{"error":"El sprint no existe."}`) con una sola consulta; y sin titular
      en el contexto responde el `401` de la 001 (research.md sección 3) — depende de T024
- [X] T027 [P] [US2] Pruebas unitarias **obligatorias** de asignar y quitar en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles: proyecto Finalizado →
      `ErrSprintsReadOnly` (RN-19); sprint `closed` → `ErrSprintClosed` al asignar y al quitar
      (RN-09, RN-18); problemas de admisión → `StoriesNotAdmittedError` con todos los motivos;
      historia `completed` en el lote a quitar → `ErrCompletedStoryRemoval` (RN-21) — depende de
      T024
- [X] T028 [US2] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con `// Escenario: <nombre> (004/US2)`:
      `TestUS2_AsignacionDeTresHistorias` (las tres en `stories`, `pending`, y una cuarta asignación a
      otro sprint abierto rechazada), `TestUS2_AsignacionEnSprintActivo` (sprint `active` sembrado en
      la base), `TestUS2_QuitarDeSprintPlanificado` (vuelve `pending`, sin fila en `sprint_stories` y
      asignable a otro sprint), `TestUS2_ReestimacionDeComprometida` (`PUT story-points` de la 003 de
      5 a 8 → `200`), `TestUS2_QuitarUnicaHistoriaDeSprintActivo` (sigue `active` y vacío),
      `TestUS2_TercerSprint` (dos sprints cerrados sembrados con su fila congelada `not_completed`,
      asignación a un tercero aceptada, y en la base **tres** filas de `sprint_stories` para la
      historia: dos con `outcome = 'not_completed'` y una con `outcome IS NULL`; el endpoint de
      historial se verifica en `TestUS6_HistorialDeUnaHistoria`, T074), `TestUS2_HistoriaNoLista` (subtests sin Story Points y sin
      criterios), `TestUS2_HistoriaCompletada`, `TestUS2_ComprometidaEnOtroSprint` (`409` con el
      sprint), `TestUS2_SprintCerrado` (asignar y quitar), `TestUS2_LoteParcialmenteInvalido`
      (`409` y **ninguna** fila creada), `TestUS2_HistoriaDeOtroProyecto` (`404` `La historia no
      existe.`), `TestUS2_QuitarHistoriaCompletada` (`409`, la historia conserva `completed` y su
      `completed_at`) y `TestUS2_ComprometidaNoDejaDeEstarLista` (subtests `PUT story-points` con
      `null` y `PATCH` con `acceptanceCriteria: []` de la 003 → `409` y la historia intacta). Además,
      sin etiqueta de escenario: el lote con un identificador repetido o con más de 100 → `400`, y
      `ever_committed` en `true` tras asignar y tras quitar (RC-06) — depende de T024
- [X] T029 [P] [US2] Prueba de integración del puerto de la 003 en
      `backend/internal/backlog/repository/sprint_assignments_test.go`, contra la base de test: sin
      compromisos → `CommittedToOpenSprint` y `EverAssigned` falsos; con compromiso abierto → los dos
      verdaderos; con la fila congelada de un sprint cerrado → `CommittedToOpenSprint` falso y
      `EverAssigned` verdadero; tras quitar la historia → `EverAssigned` sigue verdadero — depende de
      T024
- [X] T030 [P] [US2] Pruebas de frontend en
      `frontend/src/features/sprints/components/commit-stories-dialog.test.tsx` y
      `frontend/src/features/sprints/components/sprint-story-list.test.tsx`, con
      `describe("004/US2 - Comprometer historias en un sprint")` y un `it` por escenario, incluido que
      el diálogo ofrece **solo** historias del backlog listas, no completadas y sin sprint abierto,
      que muestra **cada** motivo del `409` por historia, y que la lista no ofrece quitar una
      historia Completada — depende de T024

### Implementación de la Historia 2

- [X] T031 [US2] Implementar `Admission` en `backend/internal/sprints/domain/admission.go`, usando
      `backlogdomain.Readiness` como **única** fuente de "lista para planificar" (research.md
      sección 8) — depende de T025
- [X] T032 [US2] Implementar `FindAccessible` en
      `backend/internal/sprints/repository/sprint_repository.go` con **una sola consulta**
      `sprints JOIN project_members (project_id, user_id = titular) JOIN projects`, devolviendo
      `ErrSprintNotFound` si no hay fila, e implementar `RequireSprintAccess` en
      `backend/internal/sprints/delivery/middleware.go` con **una única salida `404`** para el
      sprint inexistente, el ajeno y el identificador mal formado, que deja el `SprintAccess` en el
      contexto; cualquier otro error responde `500` genérico registrado en el log — depende de T026
- [X] T033 [US2] Reemplazar el cuerpo de `backend/internal/backlog/repository/sprint_assignments.go`
      para que `Find` lea `EXISTS (sprint_stories WHERE story_id = ? AND outcome IS NULL)` y
      `user_stories.ever_committed` sobre los modelos de Go (no nombres de tabla en texto), con
      constructor que recibe la conexión; actualizar su composición en `backend/cmd/app/main.go` y en
      los ayudantes de prueba de `backend/internal/backlog/delivery/handler_test.go`. **No** se toca
      `backlog/service` (research.md secciones 9 y 17.2) — depende de T029
- [X] T034 [US2] Implementar `AssignStories` y `RemoveStories` en
      `backend/internal/sprints/repository/sprint_repository.go`, cada uno en **una transacción**:
      bloquear el sprint (`FOR UPDATE`) y leer el estado del proyecto; bloquear las historias pedidas
      (`FOR UPDATE`, **ordenadas por `id`**); si alguna no existe o es de otro proyecto →
      `ErrStoryNotFound`. `AssignStories` evalúa `Admission` con los datos releídos y, si hay
      problemas, los devuelve **sin escribir nada**; si no, inserta una fila de `sprint_stories` por
      historia con `committed_at` y marca `ever_committed = true`, traduciendo
      `gorm.ErrDuplicatedKey` del índice parcial a `committed_elsewhere`. `RemoveStories` rechaza
      con `ErrCompletedStoryRemoval` si alguna está `completed`, responde `ErrStoryNotFound` si
      alguna no está comprometida en este sprint, y si no borra las filas y deja las historias en
      `pending` (FR-022, FR-023) — depende de T028, T031 y T033
- [X] T035 [US2] Implementar `AssignStories(ctx, access, storyIDs)` y
      `RemoveStories(ctx, access, storyIDs)` en `backend/internal/sprints/service/sprint_service.go`,
      con solo lectura primero y estado del sprint después, y devolviendo el detalle actualizado con
      `buildDetail` — depende de T027 y T034
- [X] T036 [US2] Declarar `StoryIdsInput`, `AdmissionProblem` y `StoriesNotAdmittedResponse` en
      `backend/internal/sprints/delivery/dto.go`, e implementar en
      `backend/internal/sprints/delivery/validation.go` la validación de la lista de identificadores
      del cuerpo y de los parámetros `storyId` repetidos: **entre 1 y 100, UUID válidos y sin
      repetidos**, con el campo `storyIds[i]` o `storyId` en el `FieldError` — depende de T028
- [X] T037 [US2] Crear en `backend/internal/sprints/delivery/routes.go` el grupo plano
      `/api/v1/sprints/:sprintId` con el middleware de sesión inyectado desde `cmd/app/main.go` y
      `RequireSprintAccess`; implementar en `backend/internal/sprints/delivery/handler.go` con
      anotaciones `swag` en español los manejadores `assignSprintStories`
      (`POST /stories`), `removeSprintStories` (`DELETE /stories?storyId=...`) y un `getSprint`
      básico (`GET`, detalle con las historias vivas, que US6 completa con instantánea y días
      restantes). Respuestas según contracts/openapi.yaml — depende de T032, T035 y T036
- [X] T038 [US2] Implementar las llamadas de detalle, asignación y baja en
      `frontend/src/features/sprints/api.ts`, los componentes `CommitStoriesDialog` en
      `frontend/src/features/sprints/components/commit-stories-dialog.tsx` (lista las historias del
      backlog con la llamada de listado de la 003) y `SprintStoryList` en
      `frontend/src/features/sprints/components/sprint-story-list.tsx`, y la pantalla
      `frontend/src/app/projects/[projectId]/sprints/[sprintId]/page.tsx`, protegida con la guardia
      de sesión, a la que redirige la creación de US1 — depende de T030 y T037

**Checkpoint**: US1 y US2 funcionan de forma independiente. El sprint ya es un plan con historias, y
la regla de la 003 sobre historias comprometidas usa datos reales.

---

## Fase 4: Historia de Usuario 3 — Iniciar un sprint (Prioridad: P2)

**Objetivo**: un integrante arranca un sprint Planificado; queda Activo con su momento real de
inicio y sus Story Points comprometidos congelados, sin tocar el estado del proyecto y sin permitir
dos sprints Activos.

**Rama**: `hu/004-us3-iniciar-un-sprint`

**Prueba independiente**: creando un sprint Planificado con historias,
`POST /api/v1/sprints/{sprintId}/start` lo deja `active` con `startedAt`; un segundo inicio en el
proyecto responde `409` con el nombre del activo; y diez inicios simultáneos dejan un solo Activo.

### Pruebas de la Historia 3 ⚠️

- [X] T039 [P] [US3] Pruebas unitarias **obligatorias** de `SprintStatus.ValidateTransition` en
      `backend/internal/sprints/domain/status_test.go`, recorriendo la **matriz completa de 9
      pares** origen-destino: solo `planned → active` y `active → closed` devuelven `nil`; los
      otros siete, incluida la permanencia, `ErrInvalidSprintTransition` (RN-02, RN-18, SC-006) —
      depende de T038
- [X] T040 [P] [US3] Pruebas unitarias **obligatorias** de `Start` en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles y reloj fijo: proyecto
      Finalizado → `ErrSprintsReadOnly`; sprint `active` → `ErrInvalidSprintTransition`; sprint
      `closed` → `ErrSprintClosed`; otro activo → `ActiveSprintExistsError` con ese sprint (RN-06);
      el instante pasado al repositorio es el del reloj; y el servicio **no llama a ninguna
      operación de proyectos** (RN-07) — depende de T038
- [X] T041 [US3] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con `// Escenario: <nombre> (004/US3)`:
      `TestUS3_InicioConHistorias` (`active`, `startedAt` igual al reloj y
      `committed_story_points` igual a la suma en la base), `TestUS3_ProyectoPlanificadoNoCambia`
      (un integrante no propietario inicia y el proyecto sigue `planned`),
      `TestUS3_ProyectoEnCursoNoCambia`, `TestUS3_SprintVacio` (`committed_story_points` 0),
      `TestUS3_InicioAntesDeLaFecha`, `TestUS3_OtroSprintActivo` (`409` con `sprint`),
      `TestUS3_SprintYaActivo`, `TestUS3_SprintCerrado` y `TestUS3_ProyectoFinalizado` — depende de
      T038
- [X] T042 [P] [US3] Agregar al archivo
      `backend/internal/sprints/delivery/sprint_concurrency_test.go` de T013 la prueba, **sin**
      etiqueta `004/USn`:
      `TestSprintStart_Concurrency` crea diez sprints Planificados no superpuestos en el mismo
      proyecto y los inicia a la vez desde diez goroutines; exactamente una respuesta es `200`, nueve
      son `409`, y la base tiene **un** sprint `active` (SC-005). Repetida varias veces para que el
      intercalado sea real — depende de T038
- [X] T043 [P] [US3] Prueba de integración del puerto de la 002 en
      `backend/internal/projects/repository/sprint_directory_test.go`, contra la base de test: sin
      sprints o solo con Planificados y Cerrados → `(nil, nil)`; con uno Activo → su `ID` y su
      `Name`; y el Activo de otro proyecto no cuenta — depende de T038
- [X] T044 [P] [US3] Pruebas de frontend en
      `frontend/src/features/sprints/components/sprint-lifecycle-actions.test.tsx`, con
      `describe("004/US3 - Iniciar un sprint")` y un `it` por escenario, incluido que el botón de
      inicio solo aparece en un sprint Planificado y que el `409` muestra el nombre del sprint
      activo — depende de T038

### Implementación de la Historia 3

- [X] T045 [US3] Implementar `ValidateTransition(to)` en `backend/internal/sprints/domain/status.go`
      con el mismo patrón de mapa de transiciones que `ProjectStatus` de la 002 — depende de T039
- [X] T046 [US3] Implementar `Start` en `backend/internal/sprints/repository/sprint_repository.go` en
      **una transacción**: bloquear el proyecto (`FOR UPDATE`) y el sprint; Finalizado →
      `ErrSprintsReadOnly`; validar la transición con el estado releído; buscar otro sprint `active`
      del proyecto (si existe → `ActiveSprintExistsError`); fijar `status = active`,
      `started_at = now` y `committed_story_points` con la suma de `user_stories.story_points` de
      sus compromisos (`COALESCE(..., 0)`). Traducir `gorm.ErrDuplicatedKey` del índice parcial al
      mismo `ActiveSprintExistsError` (research.md sección 5) — depende de T041, T042 y T045
- [X] T047 [US3] Implementar `Start(ctx, access)` en `backend/internal/sprints/service/sprint_service.go`
      con el instante del reloj inyectado — depende de T040 y T046
- [X] T048 [US3] Implementar el manejador `startSprint` en
      `backend/internal/sprints/delivery/handler.go` con anotaciones `swag` en español y registrar
      `POST /start` en el grupo del sprint en `backend/internal/sprints/delivery/routes.go`.
      Respuestas: `200` con `SprintDetail`, `401`, `404` y `409` con `SprintConflictResponse` o
      `ErrorResponse` — depende de T047
- [X] T049 [US3] Reemplazar el cuerpo de `backend/internal/projects/repository/sprint_directory.go`
      por la lectura real del sprint `active` sobre el modelo `sprintsdomain.Sprint` (solo `id` y
      `name`), con `NewSprintDirectory(db *gorm.DB)`; actualizar su composición en
      `backend/cmd/app/main.go` (las dos instancias, la del servicio de proyectos y la del de
      estado) y en los ayudantes de prueba de `backend/internal/projects/delivery/handler_test.go`.
      En `newStatusTestEnv`, el directorio recibe **la misma sesión con contador** que los
      repositorios del servicio. **No** se toca `projects/service` (research.md sección 17.1) —
      depende de T043
- [X] T050 [US3] Verificar que la prueba de conteo de consultas de la vista de estado de la 002 siga
      dentro del límite de **4 consultas** de su SC-003 ahora que el directorio de sprints hace una
      consulta real, y que la prueba de la 002 que rechaza finalizar con un sprint activo pase con un
      sprint `active` sembrado en la base; ajustar sus comentarios si cuentan un número distinto —
      depende de T049
- [X] T051 [US3] Implementar la llamada de inicio en `frontend/src/features/sprints/api.ts` y el
      componente `SprintLifecycleActions` en
      `frontend/src/features/sprints/components/sprint-lifecycle-actions.tsx` con la acción de
      iniciar, integrado en la pantalla de detalle — depende de T044 y T048

**Checkpoint**: US1 a US3 funcionan de forma independiente. La vista de estado de la 002 muestra el
sprint activo real y la finalización con sprint activo se rechaza con datos reales.

---

## Fase 5: Historia de Usuario 4 — Registrar el avance de las historias del sprint (Prioridad: P2)

**Objetivo**: durante el sprint Activo, un integrante pasa sus historias a En progreso y a
Completada, o las devuelve a En progreso, registrando la fecha y hora del último completado.

**Rama**: `hu/004-us4-registrar-el-avance`

**Prueba independiente**: con un sprint Activo y una historia comprometida,
`PUT /api/v1/stories/{storyId}/status` con `in_progress` y luego `completed` registra `completedAt`,
y el salto directo de `pending` a `completed` responde `409` con las transiciones admitidas.

### Pruebas de la Historia 4 ⚠️

- [X] T052 [P] [US4] Pruebas unitarias **obligatorias** de `ValidateStoryProgress` en
      `backend/internal/sprints/domain/progress_test.go`, recorriendo los **9 pares** de
      `pending`/`in_progress`/`completed`: solo `pending → in_progress`, `in_progress → completed` y
      `completed → in_progress` devuelven `nil`; los demás, incluido el salto a `completed` y todo
      destino `pending`, `ErrInvalidStoryTransition` (RN-11, RN-12, FR-038) — depende de T051
- [X] T053 [P] [US4] Pruebas unitarias de la validación del cuerpo del avance en
      `backend/internal/sprints/delivery/validation_test.go`: `status` **obligatorio** y limitado a
      `pending`, `in_progress` o `completed`; ausente o con otro valor → `400` en el campo `status`
      — depende de T051
- [X] T054 [P] [US4] Pruebas unitarias **obligatorias** de `ChangeStoryProgress` en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles y reloj fijo: el
      instante pasado al repositorio es el del reloj, y `ErrProgressOutsideActiveSprint`,
      `ErrSprintClosed`, `ErrInvalidStoryTransition` y `ErrSprintsReadOnly` del repositorio se
      propagan (RN-11, SC-011) — depende de T051
- [X] T055 [US4] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con `// Escenario: <nombre> (004/US4)`:
      `TestUS4_AvanceHastaCompletada` (`completedAt` igual al reloj en la respuesta y en
      `sprint_stories`), `TestUS4_DesmarcarCompletada` (`completedAt: null`),
      `TestUS4_RecompletarRegistraElUltimo` (dos instantes distintos del reloj; queda el segundo),
      `TestUS4_ReestimacionDuranteElSprint` (reestimación con la 003 y completado aceptado),
      `TestUS4_UltimoDiaDelSprint` (reloj en la fecha de fin), `TestUS4_SprintPlanificado`,
      `TestUS4_HistoriaSinSprint`, `TestUS4_SprintCerrado` (historia `completed` de un sprint cerrado
      → `409` de sprint cerrado) y `TestUS4_SaltoDePendienteACompletada`. En todos los rechazos el
      estado de la historia y `completed_at` quedan intactos. Además, sin etiqueta de escenario, que
      el `404` de una historia ajena sea **idéntico byte a byte** al de la 003 — depende de T051
- [X] T056 [P] [US4] Pruebas de frontend en
      `frontend/src/features/sprints/components/story-progress-control.test.tsx`, con
      `describe("004/US4 - Registrar el avance de las historias del sprint")` y un `it` por
      escenario, incluido que el control solo ofrece las transiciones admitidas desde el estado
      actual y que completar una historia lleva **no más de 3 pasos** (SC-002) — depende de T051

### Implementación de la Historia 4

- [X] T057 [US4] Implementar `ValidateStoryProgress(from, to backlogdomain.StoryStatus) error` en
      `backend/internal/sprints/domain/progress.go` — depende de T052
- [X] T058 [US4] Implementar `ChangeStoryProgress` en
      `backend/internal/sprints/repository/sprint_repository.go` en **una transacción**: bloquear la
      historia (`FOR UPDATE`) y leer el estado de su proyecto (Finalizado → `ErrSprintsReadOnly`);
      buscar su compromiso abierto y tomar `FOR SHARE` sobre la fila del sprint. Sin compromiso
      abierto: si la historia está `completed` y su último compromiso es de un sprint `closed` →
      `ErrSprintClosed`; si no → `ErrProgressOutsideActiveSprint`. Con compromiso en un sprint
      `planned` → `ErrProgressOutsideActiveSprint`. Validar con `ValidateStoryProgress`; escribir
      `user_stories.status` y `sprint_stories.completed_at` (`now` al completar, `NULL` al volver a
      `in_progress`) (research.md sección 13) — depende de T055 y T057
- [X] T059 [US4] Implementar `ChangeStoryProgress(ctx, storyID, to)` en
      `backend/internal/sprints/service/sprint_service.go` con el instante del reloj — depende de
      T054 y T058
- [X] T060 [US4] Declarar `StoryStatusInput` y `StoryProgressResponse` en
      `backend/internal/sprints/delivery/dto.go`, implementar su validación en
      `backend/internal/sprints/delivery/validation.go` y el manejador `changeStoryStatus` en
      `backend/internal/sprints/delivery/handler.go` con anotaciones `swag` en español; crear en
      `backend/internal/sprints/delivery/routes.go` el registro de `PUT /api/v1/stories/:storyId/status`
      con el middleware de sesión y **el `RequireStoryAccess` de la 003 recibidos como
      `gin.HandlerFunc` desde `backend/cmd/app/main.go`**, sin que `sprints/delivery` importe
      `backlog/delivery` (research.md sección 2). El mensaje de `ErrInvalidStoryTransition` enumera
      las tres transiciones admitidas — depende de T053 y T059
- [X] T061 [US4] Implementar la llamada de avance en `frontend/src/features/sprints/api.ts` y el
      componente `StoryProgressControl` en
      `frontend/src/features/sprints/components/story-progress-control.tsx`, integrado en cada fila
      de `SprintStoryList` cuando el sprint está Activo — depende de T056 y T060

**Checkpoint**: US1 a US4 funcionan de forma independiente. Las historias ya llegan a Completada.

---

## Fase 6: Historia de Usuario 5 — Cerrar un sprint y congelar su resultado (Prioridad: P2)

**Objetivo**: un integrante cierra el sprint Activo; se congela la instantánea con los Story Points
comprometidos, planificados y completados, cada historia con su título, prioridad y Story Points al
cierre, y el factor vigente, y las no completadas vuelven al backlog.

**Rama**: `hu/004-us5-cerrar-un-sprint`

**Prueba independiente**: con un sprint Activo de cuatro historias por 21 Story Points y dos
completadas por 13, `POST /api/v1/sprints/{sprintId}/close` devuelve `results` con 21 planificados y
13 completados, las otras dos vuelven `pending`, y reestimar después no cambia nada.

### Pruebas de la Historia 5 ⚠️

- [X] T062 [P] [US5] Pruebas unitarias **obligatorias** de `Outcome` en
      `backend/internal/sprints/domain/progress_test.go` (`completed` → `completed`; `pending` e
      `in_progress` → `not_completed`) (RN-15). Los totales de la instantánea no tienen prueba
      unitaria propia: se calculan solo en SQL y los verifica T064 contra la base (FR-045 con
      `TestUS5_SprintVacio`) — depende de T061
- [X] T063 [P] [US5] Pruebas unitarias **obligatorias** de `Close` en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles y reloj fijo: proyecto
      Finalizado → `ErrSprintsReadOnly`; sprint `planned` → `ErrSprintNotActive`; sprint `closed` →
      `ErrSprintClosed` (RN-18); el actor llega como `closedBy` y el instante es el del reloj; y el
      servicio no llama a ninguna operación de proyectos (RN-07) — depende de T061
- [X] T064 [US5] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con `// Escenario: <nombre> (004/US5)`:
      `TestUS5_CierreParcial` (21 planificados, 13 completados, las dos no completadas `pending` y sin
      compromiso abierto, y **cuatro** filas congeladas), `TestUS5_TodasCompletadas`,
      `TestUS5_AlcanceAgregado` (20 al iniciar, dos historias de 8 sumadas con el sprint Activo → 20
      y 36; y con una historia quitada durante el sprint, diferencia negativa — SC-009),
      `TestUS5_InmutableTrasElCierre` (renombrar y reestimar con la 003 y cambiar el factor con la
      002; el detalle y el historial son **idénticos byte a byte** antes y después — SC-008),
      `TestUS5_SinCompletadas`, `TestUS5_SprintVacio` (todo en 0), `TestUS5_CierreAnticipado`,
      `TestUS5_CierreTardio`, `TestUS5_SprintPlanificado`, `TestUS5_SprintCerrado` (cerrar de nuevo e
      iniciar → `409`) y `TestUS5_ProyectoFinalizado` (proyecto sembrado `finished` con el sprint
      Activo). Además, sin etiqueta de escenario, que el estado del proyecto no cambia al cerrar
      (RN-07) y que `closedBy` es el titular de la sesión (FR-040) — depende de T061
- [X] T065 [P] [US5] Pruebas de frontend en
      `frontend/src/features/sprints/components/sprint-lifecycle-actions.test.tsx` y
      `frontend/src/features/sprints/components/sprint-results.test.tsx`, con
      `describe("004/US5 - Cerrar un sprint y congelar su resultado")` y un `it` por escenario,
      incluido que el cierre **pide confirmación** e informa cuántas historias vuelven al backlog, y
      que el resultado muestra la diferencia entre comprometidos al inicio y planificados al cierre —
      depende de T061

### Implementación de la Historia 5

- [X] T066 [US5] Implementar `Outcome(status)` en `backend/internal/sprints/domain/progress.go` —
      depende de T062
- [X] T067 [US5] Implementar `Close` en `backend/internal/sprints/repository/sprint_repository.go` en
      **una transacción**: bloquear el sprint (`FOR UPDATE`) y leer el estado del proyecto;
      validar la transición con el estado releído; leer y bloquear las historias comprometidas
      (`FOR UPDATE`, **ordenadas por `id`**); completar en cada fila `title_at_close`,
      `priority_at_close`, `story_points_at_close` y `outcome` con `Outcome`; dejar en `pending` las
      no completadas; copiar `projects.hours_per_story_point_hundredths`; y fijar `closed_at = now`,
      `closed_by_id` y `status = closed`. Si algo falla, nada cambia (research.md sección 14).
      Declarar en este archivo un único fragmento de consulta, `sprintResultsSelect`, con
      `COUNT(*)`, `COUNT(*) FILTER (WHERE outcome = 'completed')`,
      `COALESCE(SUM(story_points_at_close), 0)` y su versión filtrada por `completed`, que es la
      **única** fuente de los totales derivados (research.md sección 11) y que `FindDetail` usa
      para un sprint `closed` — depende de T064 y T066
- [X] T068 [US5] Implementar `Close(ctx, access, actorID)` en
      `backend/internal/sprints/service/sprint_service.go` con el instante del reloj, y extender
      `buildDetail` para un sprint `closed`: historias desde las filas congeladas, `results` leído por
      `FindDetail` con `sprintResultsSelect`, `hoursPerStoryPoint` desde las centésimas copiadas y `closedBy` — depende de T063
      y T067
- [X] T069 [US5] Declarar `SprintResults` en `backend/internal/sprints/delivery/dto.go`, implementar
      el manejador `closeSprint` en `backend/internal/sprints/delivery/handler.go` con anotaciones
      `swag` en español y registrar `POST /close` en el grupo del sprint en
      `backend/internal/sprints/delivery/routes.go`. Respuestas: `200` con `SprintDetail`, `401`, `404`
      y `409` — depende de T068
- [X] T070 [US5] Implementar la llamada de cierre en `frontend/src/features/sprints/api.ts`, la acción
      de cerrar con confirmación en
      `frontend/src/features/sprints/components/sprint-lifecycle-actions.tsx` y el componente
      `SprintResults` en `frontend/src/features/sprints/components/sprint-results.tsx`, integrado en
      la pantalla de detalle — depende de T065 y T069
- [X] T097 [US5] Prueba de frontend en
      `frontend/src/features/sprints/components/sprint-story-list.test.tsx`, dentro de
      `describe("004/US5 - Cerrar un sprint y congelar su resultado")`, que verifique que en un sprint
      Cerrado cada historia muestra su resultado congelado (Completada / No completada según
      `outcome`) y no su estado vigente en el backlog per FR-043 (partial)
- [X] T098 [US5] Mostrar en `frontend/src/features/sprints/components/sprint-story-list.tsx` el
      resultado de cada historia con `STORY_OUTCOME_LABELS` cuando el sprint está Cerrado,
      conservando el estado vigente para los sprints abiertos per FR-043 (partial) — depende de T097

**Checkpoint**: US1 a US5 funcionan de forma independiente. Cada sprint cerrado deja una instantánea
inmutable lista para las métricas (008).

---

## Fase 7: Historia de Usuario 6 — Consultar el historial de sprints (Prioridad: P3)

**Objetivo**: un integrante lista los sprints del proyecto y el historial de cerrados con sus números
congelados, ve los días restantes del sprint Activo y consulta en qué sprints estuvo una historia.

**Rama**: `hu/004-us6-consultar-el-historial`

**Prueba independiente**: cerrando dos sprints con resultados distintos,
`GET /api/v1/projects/{projectId}/sprints?status=closed` los devuelve con sus totales congelados, y
el detalle de un sprint Activo con fin el 2026-10-14 consultado el 2026-10-04 informa
`remainingDays: 10`.

### Pruebas de la Historia 6 ⚠️

- [X] T071 [P] [US6] Pruebas unitarias **obligatorias** de `Schedule` en
      `backend/internal/sprints/domain/schedule_test.go`, cubriendo RN-23 con la tabla de
      data-model.md sección 6: fin 2026-10-14 y hoy 2026-10-04 → `remainingDays` 10 y `overdueDays`
      `nil`; hoy 2026-10-14 → 0 y `nil`; hoy 2026-10-17 → `nil` y 3; y el **borde de medianoche**:
      un instante que en UTC ya es el día siguiente pero en la zona del sistema todavía es el día de
      fin devuelve 0 — depende de T070
- [X] T072 [P] [US6] Pruebas unitarias de la validación del filtro del listado en
      `backend/internal/sprints/delivery/validation_test.go`: `status` repetible, valores fuera de
      `planned`/`active`/`closed` rechazados indicando los admitidos, y ausencia de filtro aceptada —
      depende de T070
- [X] T073 [P] [US6] Pruebas unitarias del listado y del detalle en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles y reloj fijo:
      `emptyReason` `null` con resultados, `no_sprints` sin sprints aunque haya filtro y `no_matches`
      con sprints que no cumplen el filtro, llamando a `HasSprints` **solo** cuando el resultado
      viene vacío; y `schedule` presente solo para un sprint `active` (FR-053) — depende de T070
- [X] T074 [US6] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con `// Escenario: <nombre> (004/US6)`:
      `TestUS6_HistorialDeCerrados` (tres cerrados con nombre, Sprint Goal, cuatro fechas y los cinco
      totales), `TestUS6_ListadoCompletoYFiltro` (Cerrado, Activo y Planificado en el listado; solo
      el Cerrado con `?status=closed`; orden `start_date DESC, id`), `TestUS6_HistorialDeUnaHistoria`
      (tres sprints hasta completarse, en orden ascendente con `not_completed`, `not_completed` y
      `completed`), `TestUS6_DiasRestantes`, `TestUS6_TerminaHoy`, `TestUS6_Vencido`,
      `TestUS6_SinDiasRestantesFueraDeActivo`, `TestUS6_DetalleDesdeLaInstantanea` (título, prioridad
      y Story Points del cierre tras renombrar y reestimar con la 003),
      `TestUS6_CuatroFechas`, `TestUS6_ProyectoSinSprints` (`sprints: []` y `emptyReason:
      no_sprints`), `TestUS6_CerradosSinCompletadas`, `TestUS6_NoIntegrante` (`404` de proyecto) y
      `TestUS6_SprintInexistente` (`404` de sprint). Además, sin etiqueta de escenario, que el
      listado **no incluye sprints de otro proyecto** y que una historia quitada antes del cierre no
      figura en el historial de ese sprint — depende de T070
- [X] T075 [P] [US6] Pruebas de frontend en
      `frontend/src/features/sprints/components/sprint-list.test.tsx`,
      `frontend/src/features/sprints/components/sprint-history-table.test.tsx`,
      `frontend/src/features/sprints/components/sprint-schedule.test.tsx` y
      `frontend/src/features/sprints/components/story-sprint-history.test.tsx`, con
      `describe("004/US6 - Consultar el historial de sprints")` y un `it` por escenario, incluido que
      `overdueDays` se muestra como "Vencido hace N días" y nunca como un número negativo, que el
      listado vacío muestra la indicación de que todavía no hay sprints, y que el sprint Activo
      concentra objetivo, fechas, días restantes e historias en una pantalla (SC-003) — depende de
      T070

### Implementación de la Historia 6

- [X] T076 [US6] Implementar `Schedule(endDate, now time.Time, location *time.Location)` en
      `backend/internal/sprints/domain/schedule.go`, tomando la fecha de calendario de `now` en la
      zona y devolviendo `remainingDays` u `overdueDays`, **nunca un número negativo** (research.md
      sección 15) — depende de T071
- [X] T077 [US6] Implementar `List`, `HasSprints` y `ListForStory` en
      `backend/internal/sprints/repository/sprint_repository.go`: `List` es **una sola consulta**
      sobre `sprints` del proyecto con el fragmento `sprintResultsSelect` de T067 como subconsulta
      agregada por sprint,
      filtro `IN` por estado y orden `start_date DESC, id ASC` (FR-052); `HasSprints` es un
      `SELECT 1 ... LIMIT 1`; `ListForStory` une `sprint_stories` con `sprints` por la historia y
      ordena por `start_date ASC, sprints.id ASC` — depende de T066, T067 y T074
- [X] T078 [US6] Implementar `List(ctx, projectID, statuses)`, completar `Get(ctx, access)` con
      `schedule` calculado con `Schedule`, el reloj y la zona solo para un sprint `active`, e
      implementar `StoryHistory(ctx, storyID)` en `backend/internal/sprints/service/sprint_service.go`
      — depende de T073, T076 y T077
- [X] T079 [US6] Declarar `SprintListItem`, `SprintListResponse`, `SprintSchedule`,
      `StorySprintHistoryItem` y `StorySprintHistoryResponse` en
      `backend/internal/sprints/delivery/dto.go` (`sprints` y `appliedFilters.status` nunca son
      `null`); implementar el parseo del filtro con `c.QueryArray("status")` en
      `backend/internal/sprints/delivery/validation.go`; implementar los manejadores `listSprints` y
      `listStorySprints` y completar las anotaciones de `getSprint` en
      `backend/internal/sprints/delivery/handler.go`; y registrar `GET /sprints` en el subgrupo del
      proyecto y `GET /api/v1/stories/:storyId/sprints` con el `RequireStoryAccess` inyectado en
      `backend/internal/sprints/delivery/routes.go` — depende de T072 y T078
- [X] T080 [US6] Implementar las llamadas de listado e historial de una historia en
      `frontend/src/features/sprints/api.ts`; los componentes `SprintList` en
      `frontend/src/features/sprints/components/sprint-list.tsx`, `SprintHistoryTable` en
      `frontend/src/features/sprints/components/sprint-history-table.tsx`, `SprintSchedule` en
      `frontend/src/features/sprints/components/sprint-schedule.tsx` (integrado en el detalle) y
      `StorySprintHistory` en `frontend/src/features/sprints/components/story-sprint-history.tsx`; la
      pantalla `frontend/src/app/projects/[projectId]/sprints/page.tsx` con el listado, la vista de
      cerrados y el acceso a "Nuevo sprint"; y la sección "Sprints en los que estuvo" en
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx` de la 003 (FR-055) —
      depende de T075 y T079
- [X] T101 [US6] Agregar en `backend/internal/sprints/delivery/validation_test.go` una prueba
      unitaria de `ParseSprintStatusFilter` con miles de valores repetidos de `status` (por ejemplo,
      70.000 `closed` intercalados con `planned`) que verifique que el filtro resultante contiene cada
      valor admitido una sola vez, en el orden de su primera aparición, y en
      `backend/internal/sprints/delivery/handler_test.go` una prueba de integración de
      `GET /api/v1/projects/{projectId}/sprints` con más de 65.535 `status=closed` que espere `200`,
      no `500`, y `appliedFilters.status` igual a `["closed"]` per hallazgo de seguridad: filtro de
      estado repetible sin deduplicar que arma una cláusula IN con un parámetro por valor (security)
- [X] T102 [US6] Eliminar los valores duplicados en `ParseSprintStatusFilter` de
      `backend/internal/sprints/delivery/validation.go`, conservando el orden de la primera
      aparición, de modo que la cláusula `IN` de `List` y `appliedFilters.status` reciban como máximo
      los tres estados admitidos per hallazgo de seguridad: filtro de estado repetible sin deduplicar
      que arma una cláusula IN con un parámetro por valor (security) — depende de T101

**Checkpoint**: US1 a US6 funcionan de forma independiente. El historial congelado y los días
restantes ya están disponibles para el dashboard (009) y los reportes (010).

---

## Fase 8: Historia de Usuario 7 — Corregir o descartar un sprint planificado (Prioridad: P3)

**Objetivo**: un integrante corrige nombre, fechas o Sprint Goal de un sprint que no arrancó, o lo
elimina devolviendo sus historias al backlog, salvo que tenga esfuerzo registrado.

**Rama**: `hu/004-us7-corregir-o-descartar`

**Prueba independiente**: creando un sprint Planificado con dos historias, `PUT` cambia sus datos y
conserva las historias; `DELETE` responde `204`, libera el período y devuelve las historias al
backlog; y los mismos intentos sobre un sprint Activo responden `409`.

### Pruebas de la Historia 7 ⚠️

- [X] T081 [P] [US7] Pruebas unitarias **obligatorias** de `Update` y `Delete` en el servicio en
      `backend/internal/sprints/service/sprint_service_test.go`, con dobles: proyecto Finalizado →
      `ErrSprintsReadOnly`; modificar `active` → `ErrSprintNotPlanned` y `closed` →
      `ErrSprintClosed` (RN-18); eliminar `active` o `closed` → `ErrSprintAlreadyStarted`; con
      `SprintEffort.HasEffort` verdadero → `ErrSprintHasEffort` **sin llamar a `Delete` del
      repositorio** (RN-20); y la modificación no pasa estado, historias ni instantánea al
      repositorio (FR-015) — depende de T080
- [X] T082 [US7] Pruebas de integración de la historia en
      `backend/internal/sprints/delivery/handler_test.go`, con `// Escenario: <nombre> (004/US7)`:
      `TestUS7_ModificacionCompleta` (estado e historias sin cambios), `TestUS7_EliminacionConHistorias`
      (`204`, sin filas en `sprints` ni `sprint_stories`, historias `pending` y un sprint nuevo en el
      mismo período aceptado), `TestUS7_MoverFechas`, `TestUS7_MismoNombre`,
      `TestUS7_FechasContiguas`, `TestUS7_EliminacionSinHistorias`,
      `TestUS7_HistoriaDeSprintEliminado` (`DELETE` de la historia en la 003 → `409` con
      `sprint_history`), `TestUS7_EsfuerzoEliminadoAntes` (doble de `SprintEffort` que primero informa
      esfuerzo y después no → la segunda eliminación se acepta), `TestUS7_DatosInvalidos` (subtests
      nombre vacío, Sprint Goal vacío, fin no posterior y Sprint Goal de 501, sin cambio parcial),
      `TestUS7_Superposicion` (`409` con el sprint en conflicto, y sin conflicto consigo mismo),
      `TestUS7_ModificarActivo`, `TestUS7_ModificarCerrado`, `TestUS7_EliminarActivoOCerrado`,
      `TestUS7_EsfuerzoRegistrado` (`409`, sprint e historias intactos — SC-016) y
      `TestUS7_SprintEliminado` (`GET`, `PUT` y `DELETE` → `404`) — depende de T080
- [X] T083 [P] [US7] Pruebas de frontend en
      `frontend/src/features/sprints/components/sprint-edit-view.test.tsx` y
      `frontend/src/features/sprints/components/delete-sprint-button.test.tsx`, con
      `describe("004/US7 - Corregir o descartar un sprint planificado")` y un `it` por escenario,
      incluido que editar y eliminar solo se ofrecen en un sprint Planificado, que la eliminación
      pide confirmación y que el rechazo por esfuerzo explica qué hay que hacer primero — depende de
      T080

### Implementación de la Historia 7

- [X] T084 [US7] Implementar `Update` y `Delete` en
      `backend/internal/sprints/repository/sprint_repository.go`: `Update` en **una transacción** que
      bloquea el proyecto y el sprint, verifica solo lectura y estado `planned` con los datos
      releídos, busca superposición **excluyendo al propio sprint** y escribe **solo** `name`,
      `goal`, `start_date`, `end_date` y `updated_at` (FR-013 a FR-015); `Delete` en **una
      transacción** que bloquea el proyecto y el sprint, verifica `planned`, deja las historias
      comprometidas en `pending`, borra sus filas de `sprint_stories` y el sprint, dejando
      `ever_committed` en `true` (research.md sección 16) — depende de T082
- [X] T085 [US7] Implementar `Update(ctx, access, input)` y `Delete(ctx, access)` en
      `backend/internal/sprints/service/sprint_service.go`; `Delete` consulta `SprintEffort.HasEffort`
      después de las reglas de solo lectura y estado y antes de llamar al repositorio — depende de
      T081 y T084
- [X] T086 [US7] Implementar los manejadores `updateSprint` (reutiliza la validación de T016) y
      `deleteSprint` en `backend/internal/sprints/delivery/handler.go` con anotaciones `swag` en
      español, y registrar `PUT` y `DELETE` en el grupo del sprint en
      `backend/internal/sprints/delivery/routes.go`. Respuestas según contracts/openapi.yaml —
      depende de T085
- [X] T087 [US7] Implementar las llamadas de modificación y eliminación en
      `frontend/src/features/sprints/api.ts`, el componente `SprintEditView` en
      `frontend/src/features/sprints/components/sprint-edit-view.tsx` que reutiliza `SprintForm`
      precargado, `DeleteSprintButton` en
      `frontend/src/features/sprints/components/delete-sprint-button.tsx` con confirmación y regreso
      al listado, y la pantalla `frontend/src/app/projects/[projectId]/sprints/[sprintId]/edit/page.tsx`
      — depende de T083 y T086

**Checkpoint**: las siete historias funcionan de forma independiente.

---

## Fase 9: Polish (Cierre y asuntos transversales)

**Propósito**: navegación desde el proyecto, documentación viva, cobertura, vulnerabilidades y
verificación de las puertas de calidad. Todas las tareas son **técnicas**, salvo T088, T089 y T090:
T088 es de caracterización sin RED y T089 y T090 siguen el ciclo normal.

**Rama**: `fase/004-polish`

- [X] T088 Pruebas de integración de caracterización en
      `backend/internal/sprints/delivery/handler_test.go`, **sin** etiqueta `004/USn`:
      `TestSprintEndpoints_SinSesion`, con un subtest por cada uno de los **once** endpoints sin
      cabecera y con token adulterado → `401` y ningún cambio (SC-013);
      `TestSprintEndpoints_NotFoundIndistinguible`, que compara byte a byte el `404` de sprint ajeno e
      inexistente en las **siete** acciones de sprint y el de proyecto ajeno e inexistente en las
      **dos** de colección (SC-014); `TestSprintEndpoints_ProyectoFinalizado`, con las **ocho**
      escrituras → `409` y ningún cambio, y las tres lecturas → `200` (SC-012); y una aserción de
      que ninguna respuesta contiene `password_hash` ni `passwordHash` (Principio VI). Prueba de
      caracterización: las rutas ya están detrás de los middlewares y las reglas ya existen, así que
      no tiene RED; el commit lo justifica — depende de T087
- [ ] T089 [P] Prueba de frontend en
      `frontend/src/features/projects/components/project-overview.test.tsx`: la vista del proyecto
      ofrece un enlace a `/projects/{projectId}/sprints` — depende de T080
- [ ] T090 Implementar el enlace a los sprints en
      `frontend/src/features/projects/components/project-overview.tsx` — depende de T089
- [ ] T091 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las **once**
      operaciones, sus códigos de respuesta (todas declaran `500`) y sus esquemas; corregir las
      anotaciones si difieren (Principio VIII) — depende de T087
- [ ] T092 [P] Actualizar `README.md` de la raíz en español: la sección introductoria menciona la
      gestión de sprints, y se agregan los once endpoints y las cuatro pantallas nuevas, manteniendo
      el criterio de que alguien que no conoce el proyecto pueda ejecutarlo siguiendo el documento —
      depende de T087
- [ ] T093 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T094 [P] Ejecutar los comandos de cobertura de los dos lados —
      `go test ./... -coverprofile=coverage.out` más `go tool cover -func=coverage.out` en
      `backend/`, y `npm run test:coverage` en `frontend/` — y revisar que el módulo
      `internal/sprints/` y `frontend/src/features/sprints/` estén cubiertos — depende de T087
- [ ] T095 Verificar la trazabilidad y las puertas de seguridad recorriendo
      `backend/internal/sprints/` y `frontend/src/features/sprints/`: que buscar `004/US1` a
      `004/US7` en el código de pruebas devuelva los **82** escenarios; que las **23** reglas RN-01 a
      RN-23 tengan prueba asociada (SC-017); que los **once** endpoints tengan su prueba de `401`
      (SC-013); que los `404` sean indistinguibles (SC-014); que **ninguna** operación de esta
      feature escriba `projects` (FR-049); que solo esta feature escriba `user_stories.status` y
      `ever_committed`; y que ninguna respuesta incluya `password_hash` — depende de T088
- [ ] T096 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluidas la
      vista de estado de la 002 con el sprint activo real, el puerto de la 003 con historias
      comprometidas, las pruebas de concurrencia y la verificación de que las pruebas corrieron contra
      `smye_test` en el puerto 5433 y no contra desarrollo — depende de T050, T090, T091, T092 y
      T095

---

## Phase 10: Convergence and Security

Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.

- [ ] T099 Prueba de concurrencia en `backend/internal/sprints/delivery/sprint_concurrency_test.go`
      que lance a la vez el cierre de un sprint Activo y avances de sus historias, y verifique que
      ninguna petición responde `500` y que el resultado es coherente (o el avance entra antes del
      cierre o se rechaza con `409`) per plan: research.md §13 serialización del avance contra el
      cierre (partial) — origen: converge, detectado en hu/004-us5-cerrar-un-sprint
- [ ] T100 Reordenar los bloqueos de `ChangeStoryProgress` en
      `backend/internal/sprints/repository/sprint_repository.go` para que tome el bloqueo del sprint
      antes que el de la historia (mismo orden que `Close`) y revalide el compromiso abierto tras
      bloquear, eliminando el interbloqueo con el cierre per plan: research.md §13 (partial) —
      origen: converge, detectado en hu/004-us5-cerrar-un-sprint
- [ ] T103 Marcar `outcome` como `nullable: true` en el esquema `StorySprintHistoryItem` de
      `specs/004-sprint-management/contracts/openapi.yaml`, con la descripción "`null` mientras el
      sprint está abierto", alineándolo con data-model.md y con la respuesta de
      `GET /api/v1/stories/{storyId}/sprints` per plan: contrato de la API (contradicts) — origen:
      converge, detectado en hu/004-us6-consultar-el-historial
- [ ] T104 Agregar pruebas de integración que verifiquen que
      `GET /api/v1/projects/{projectId}/sprints` responde `400`, no `200` con todos los sprints,
      cuando la query no se puede interpretar (más de 10.000 parámetros `status`, `status=%zz` y
      `status=closed;x`), y el mismo caso para `GET /api/v1/projects/{projectId}/stories`, en
      `backend/internal/sprints/delivery/handler_test.go` y
      `backend/internal/backlog/delivery/handler_test.go`, más una prueba del middleware en
      `backend/internal/platform/httpserver/` per hallazgo de seguridad: query mal formada que ignora
      los filtros sin aviso (security) — origen: seguridad, detectado en 004/US6
- [ ] T105 Agregar en `backend/internal/platform/httpserver/` un middleware que valide
      `c.Request.URL.RawQuery` con `url.ParseQuery` y responda `400` con un mensaje descriptivo si
      hay error, registrarlo con `router.Use` en `backend/cmd/app/main.go` junto a
      `LimitRequestBody`, actualizar las respuestas `400` de Swagger de los listados afectados y el
      comentario de `TestSprintList_EstadoRepetidoMilesDeVeces` per hallazgo de seguridad: query mal
      formada que ignora los filtros sin aviso (security) — origen: seguridad, detectado en 004/US6

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Foundational**: parte de `main` con la 001, la 002 **y la 003** integradas. **BLOQUEA**
  todas las historias.
- **Fases 2 a 8 (historias)**: todas dependen de la Fase 1, en orden de prioridad
  (P1 → P1 → P2 → P2 → P2 → P3 → P3).
- **Fase 9 Polish**: depende de que estén completas las siete historias.

### Dependencias entre historias

Como en la 002 y la 003, las historias **no son todas independientes en su implementación**, aunque
cada una se prueba sola:

- **US1 Planificar (P1)**: puede empezar después de la Fase 1. **Aporta** la validación (T016), el
  servicio con `buildDetail` (T018), `FindDetail` (T017) y la composición en `cmd/app/main.go`
  (T020) que usan todas las demás.
- **US2 Comprometer (P1)**: necesita sprints de US1. **Aporta** `RequireSprintAccess`, el grupo
  `/api/v1/sprints/:sprintId` y el `GET` básico del detalle (T032, T037) que usan US3 a US7, y el
  puerto real de la 003 (T033).
- **US3 Iniciar (P2)**: necesita el grupo del sprint de US2. **Aporta** `ValidateTransition` (T045),
  que usan US5 y US7.
- **US4 Registrar el avance (P2)**: necesita un sprint Activo de US3 e historias comprometidas de US2.
- **US5 Cerrar (P2)**: necesita US3 y, para sus escenarios con historias completadas, US4.
- **US6 Consultar (P3)**: necesita sprints cerrados de US5 para el historial y la instantánea.
- **US7 Corregir o descartar (P3)**: necesita US1 y US2; sus escenarios con sprints Activos y
  Cerrados, US3 y US5.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T001 a T004 en paralelo; después T005 y T006; T007 en paralelo con ambas.
- Dentro de cada historia: las pruebas unitarias, las de concurrencia, las de los puertos
  reemplazados y las de frontend marcadas `[P]` en paralelo entre sí. Las de integración **no** son
  paralelizables entre historias porque las siete comparten `handler_test.go`, ni las del servicio
  porque comparten `sprint_service_test.go`.
- US3: el reemplazo del puerto de la 002 (T043, T049, T050) puede avanzar en paralelo con el inicio
  (T045 a T048).
- Fase 9: T089, T092 y T094 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las cuatro tareas de pruebas marcadas [P] pueden escribirse a la vez:
T010  backend/internal/sprints/domain/period_test.go
T011  backend/internal/sprints/delivery/validation_test.go
T012  backend/internal/sprints/service/sprint_service_test.go
T014  frontend/src/features/sprints/components/sprint-form.test.tsx

# T013 (handler_test.go) va aparte: ese archivo lo comparten las siete historias.
# T013 crea además sprint_concurrency_test.go, que T042 amplía en US3.

# En la implementación, estas no se pisan:
T015  backend/internal/sprints/domain/period.go
T016  backend/internal/sprints/delivery/validation.go
T021  frontend/src/types/sprint.ts + frontend/src/utils/sprint-labels.ts
```

## Ejemplo de paralelismo: Historia de Usuario 3

```text
# Pruebas en archivos distintos:
T039  backend/internal/sprints/domain/status_test.go
T040  backend/internal/sprints/service/sprint_service_test.go
T042  backend/internal/sprints/delivery/sprint_concurrency_test.go
T043  backend/internal/projects/repository/sprint_directory_test.go
T044  frontend/src/features/sprints/components/sprint-lifecycle-actions.test.tsx
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Verificar que la 003 está integrada en `main`.
2. Completar la Fase 1 (Foundational) — **CRÍTICO**: bloquea todas las historias.
3. Completar la Fase 2 (US1 Planificar un sprint).
4. **DETENERSE y VALIDAR**: probar US1 de forma independiente con la guía de validación.
5. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Foundational → base lista (una fase técnica, un pull request).
2. US1 Planificar → probar sola → entregar (**MVP**).
3. US2 Comprometer → probar sola → entregar (el sprint ya es un plan; la regla de la 003 usa datos
   reales).
4. US3 Iniciar → probar sola → entregar (la 002 ve el sprint activo real).
5. US4 Registrar el avance → probar sola → entregar.
6. US5 Cerrar → probar sola → entregar (con esto el ciclo completo del sprint funciona y produce la
   instantánea).
7. US6 Consultar → probar sola → entregar.
8. US7 Corregir o descartar → probar sola → entregar.
9. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Foundational en conjunto.
2. US1 y después US2 en secuencia: aportan el servicio, el detalle y el grupo del sprint que usan
   todas las demás.
3. Terminada US2, US3 y US7 (modificación y eliminación de Planificados) pueden avanzar en paralelo.
4. Terminada US3, US4 y luego US5; US6 al final, coordinando `handler.go`, `handler_test.go`,
   `sprint_service.go`, `sprint_service_test.go` y `sprint_repository.go`, que todas tocan.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `004/US1` a `004/US7` en el código de
  pruebas devuelve la trazabilidad completa de los 82 escenarios exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 004/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b). Los commits los registra **una persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni de npm. Si alguna tarea parece necesitar
  una, detenerse y pedir autorización (Principio IV).
- **Código de otras features que esta feature modifica**, todo señalado en el plan: el campo
  `EverCommitted` del modelo de la 003 (T007), el puerto `SprintAssignments` de la 003 (T033), el
  puerto `SprintDirectory` de la 002 (T049, T050), la pantalla de detalle de la historia de la 003
  (T080) y la vista del proyecto de la 002 (T089, T090).
- **Puntos del plan que esperan confirmación del desarrollador** y no tienen tarea: las dos ventanas
  de carrera en código ajeno —la verificación de historia comprometida fuera del bloqueo en la 003
  (research.md sección 9) y la finalización del proyecto sin bloqueo en la 002 (research.md sección
  17.1)—, que se recomienda tratar en `/revisar-seguridad`; y la lectura de FR-060, que persiste solo
  el autor y quien cerró (research.md sección 19).
- **Verificación que esta feature no puede cerrar sola**:

  | Requisito | Cómo se verifica hoy | Quién la cierra |
  | --- | --- | --- |
  | FR-048, RN-20, SC-016 — sprint con esfuerzo registrado | T081, T082 con dobles de `SprintEffort` | `specs/006-effort-tracking`, reemplazando `sprint_effort.go` y traduciendo la violación de su clave foránea hacia `sprints.id` al mismo `409` |

- **Criterios de éxito no automatizables.** SC-001 (planificar un sprint con cinco historias en menos
  de 3 minutos, 95 % al primer intento), SC-002 (completar una historia en menos de 15 segundos) y
  SC-003 (ver el sprint activo en una pantalla) son métricas de experiencia de uso: se validan a mano
  siguiendo la sección "Flujo en la interfaz" de [quickstart.md](./quickstart.md). T056 verifica la
  parte automatizable de SC-002 (no más de 3 pasos) y T075 la de SC-003.
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: Swagger se actualiza en la misma tarea que cada endpoint, pero el `README.md` se
  actualiza una sola vez en la Fase 9 (T092). Entre el merge de una historia y la Fase 9, el README
  no lista los endpoints de sprints ya integrados. Se acepta porque Swagger, publicado en
  `/swagger/index.html`, es la referencia actualizada de la API en cada merge, y porque los pasos
  para ejecutar la aplicación no cambian en esta feature: no hay servicios, variables ni comandos
  nuevos. Es el mismo criterio que aplicaron la 001, la 002 y la 003.

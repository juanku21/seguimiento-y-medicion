---

description: "Lista de tareas de la feature 007 — Gestión de Defectos"
---

# Tareas: Gestión de Defectos

**Entrada**: documentos de diseño de `specs/007-defect-tracking/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) a
[`specs/005-planning-poker`](../005-planning-poker/spec.md) están integradas en `main`, junto con la
fase fundacional de la 006. Esta feature usa del módulo `internal/backlog/` el modelo `UserStory`,
`StoryStatus`, `ErrStoryNotFound`, `DeletionBlockedError`, `DeletionBlockerDefects`, el middleware
`RequireStoryAccess` y el puerto `DefectRecords`; del módulo `internal/sprints/` el modelo `Sprint`,
`SprintStatus`, `ErrSprintNotFound` y el listado de sprints con el filtro `status` repetible; del
módulo `internal/projects/` el modelo `Project`, `ProjectStatus` y el subgrupo
`/api/v1/projects/:projectId` con `RequireMembership`; de la 001 el middleware de sesión,
`authdelivery.CurrentUserID` y los esquemas de error; además el límite global de 1 MiB, el patrón de
reloj inyectado y el ayudante `testsupport.OpenDatabase`. **No hay fase de Setup**: no se agregan
dependencias, servicios ni variables de entorno.

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para la
máquina de estados, los sprints admitidos como ancla, el orden entre sprints, la ventana de
corrección, la solo lectura del descartado, la validación de descripción, severidad, estado, campos
prohibidos y filtros, y las reglas de negocio RN-01 a RN-24.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/defects/{domain,repository,service,delivery}/`; se tocan
  de forma puntual `backend/cmd/app/main.go`, `backend/internal/platform/database/postgres.go`,
  `backend/internal/platform/testsupport/database.go`,
  `backend/internal/backlog/domain/user_story.go`,
  `backend/internal/backlog/repository/defect_records.go` y
  `backend/internal/backlog/repository/story_repository.go`
- **Frontend**: `frontend/src/app/projects/[projectId]/defects/` (`page.tsx`, `new/page.tsx`,
  `[defectId]/page.tsx`), `frontend/src/features/defects/`, `frontend/src/types/defect.ts`,
  `frontend/src/utils/defect-labels.ts`; se tocan
  `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx` (donde se componen las
  secciones del detalle de la historia) y
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

## La máquina de estados crece por historia

Para que cada historia tenga un RED real, el mapa `allowedDefectTransitions` de
`backend/internal/defects/domain/status.go` se amplía en la historia que introduce cada transición:

| Historia | Transiciones que agrega | Mensajes que agrega |
| --- | --- | --- |
| US2 | `open → in_progress`, `in_progress → resolved` | Salto de Abierto a Resuelto (FR-019), vuelta de En progreso a Abierto (FR-020), genérico |
| US5 | `resolved → open` | Reapertura de lo que no está Resuelto (FR-030) |
| US6 | `open → discarded`, `in_progress → discarded` | Descarte de un Resuelto (FR-039), cualquier acción sobre un Descartado (RN-22) |

La prueba completa de las **16 combinaciones** (SC-005) se cierra en US6, cuando el mapa está
completo. El frontend replica el mismo crecimiento en `frontend/src/features/defects/transitions.ts`.

## Cómo se preparan los escenarios que dependen de otras features

- Las pruebas de integración usan un entorno `newDefectTestEnv` (T011) que compone el router como
  `cmd/app` (sesión de la 001, rutas de la 002, la 003 y la 004, y las de defectos) sobre
  `httptest`, con **un reloj falso** compartido por el servicio de sprints y el de defectos.
- Las historias se crean con los **endpoints reales de la 003**; los sprints se crean, se
  comprometen, se inician y se cierran con los de la 004 (y una historia se completa con
  `PUT /api/v1/stories/{id}/status`); el proyecto Finalizado y la baja de integrantes, con los de la
  002.
- Algunos escenarios de las historias P2 necesitan defectos en estados que solo producen historias
  P3 (Descartado, de US6; reabierto, de US5): US3-1 y US4-14. Hasta que esas historias existan, el
  entorno ofrece el ayudante `forceDefectStatus(t, defectID, status, resolvedSprintID)`, que escribe
  la fila directamente en `smye_test` como **dato de preparación**. Ninguna prueba lo usa para
  verificar una transición. US4-9 (ventana reabierta) usa ese ayudante para dejar un defecto
  `open` sin sprint de resolución después de haber estado `resolved`, que es exactamente el estado
  que produce la reapertura.
- Las pruebas sin sesión, la comparación de los `404`, la de vínculos inmutables, la de autoría tras
  quitar a un integrante, las de concurrencia y la de la carrera con el borrado de la historia **no**
  llevan la etiqueta `007/USn`, porque no derivan de un escenario de la spec y no deben alterar el
  conteo de 73.

---

## Fase 1: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el dominio del módulo nuevo, su contrato de repositorio, la migración de la tabla y de
la columna nueva, y el aislamiento de las pruebas. Todas las tareas son **técnicas**: declaran
estructuras y contratos, sin lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/007-foundational`

- [X] T001 [P] Declarar la entidad `Defect` en `backend/internal/defects/domain/defect.go` con los
      campos de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`;
      `StoryID uuid NOT NULL` con FK a `user_stories.id` (campo `Story *backlogdomain.UserStory` solo
      para la FK, nunca precargado); `Description varchar(2000) NOT NULL` ("Recortada; 1 a 2000
      caracteres"); `Severity DefectSeverity varchar(8) NOT NULL`; `Status DefectStatus varchar(16)
      NOT NULL`; `DetectedSprintID uuid NOT NULL` con FK a `sprints.id` (campo
      `DetectedSprint *sprintsdomain.Sprint`); `ResolvedSprintID *uuid.UUID` **anulable** con FK a
      `sprints.id` (campo `ResolvedSprint *sprintsdomain.Sprint`; "No nulo **si y solo si**
      `status = resolved`"); `CreatedByID`, `UpdatedByID` y `StatusChangedByID uuid NOT NULL` con FK
      a `users.id` (campos `CreatedBy`, `UpdatedBy`, `StatusChangedBy *authdomain.User`);
      `StatusChangedAt timestamptz NOT NULL`; `CreatedAt` y `UpdatedAt timestamptz NOT NULL`. Índices
      `idx_defects_story_order (story_id, created_at)`, `idx_defects_detected_sprint
      (detected_sprint_id)` e `idx_defects_resolved_sprint (resolved_sprint_id)` con etiquetas de
      GORM. Claves foráneas **sin `ON DELETE CASCADE`**; la del campo `Story` debe llamarse
      `fk_defects_story` (el nombre que GORM genera por defecto). **Sin** columnas `project_id`, de
      borrado lógico ni de historial
- [X] T002 [P] Declarar en `backend/internal/defects/domain/severity.go` el tipo
      `DefectSeverity string` con las constantes `DefectSeverityCritical = "critical"`,
      `DefectSeverityHigh = "high"`, `DefectSeverityMedium = "medium"` y `DefectSeverityLow = "low"`;
      y en `backend/internal/defects/domain/status.go` el tipo `DefectStatus string` con
      `DefectStatusOpen = "open"`, `DefectStatusInProgress = "in_progress"`,
      `DefectStatusResolved = "resolved"` y `DefectStatusDiscarded = "discarded"`, más el tipo de
      lectura `SprintAnchor` (`ID uuid.UUID`, `Status sprintsdomain.SprintStatus`,
      `StartDate time.Time`). Las funciones (`Valid`, `SeverityRank`, `AllowedTransitions`,
      `PlanTransition`) se implementan en las historias
- [X] T003 [P] Agregar a `UserStory` en `backend/internal/backlog/domain/user_story.go` el campo
      `EverHadDefects bool` con la etiqueta `gorm:"type:boolean;not null;default:false"` (columna
      `ever_had_defects`), y extender el comentario del modelo: "pasa a true cuando la historia
      recibe su primer defecto —por alta o por revinculación— y nunca vuelve a false, aunque el
      defecto se revincule a otra historia o se descarte (RC-05, RN-19); solo la escribe la feature
      de defectos", igual que `EverCommitted` y `EverHadEffort`
- [X] T004 [P] Declarar los errores de dominio en `backend/internal/defects/domain/errors.go` con la
      tabla de data-model.md sección 8: `ErrDefectNotFound`, `ErrDefectsReadOnly`,
      `ErrNoDetectionSprint`, `ErrDefectDiscarded`, `ErrLinksFrozen`, `ErrDetectionSprintPlanned`,
      `ErrResolutionSprintRequired`, `ErrResolutionSprintPlanned`, `ErrResolutionBeforeDetection` e
      `ErrInvalidFilter`; y los tipos con datos
      `InvalidTransitionError{Current, Requested DefectStatus; Allowed []DefectStatus}` e
      `InvalidFilterError{Field string}` (este con `Is(ErrInvalidFilter)`, como el de la 006). El
      texto visible para la persona lo define `delivery`, no estos mensajes — depende de T002
- [X] T005 Declarar en `backend/internal/defects/domain/repository.go` la interfaz `DefectRepository`
      con **exactamente** las ocho operaciones de data-model.md sección 10 (`FindAccessible`,
      `Create`, `ChangeStatus`, `Update`, `FindView`, `ListForProject`, `ValidateFilter`,
      `HasProjectDefects`) con sus firmas, y los tipos de lectura de la sección 9: `DefectAccess`,
      `UserRef` (solo `ID` y `FullName`), `StoryRef`, `SprintRef` (`ID`, `Name`, `Status`,
      `StartDate`), `DefectView`, `DefectInput`, `DefectPatch` (punteros; `nil` = no cambia),
      `StatusChange` (`To`, `ResolvedSprintID *uuid.UUID`) y `DefectFilter` (`Statuses`,
      `Severities`, `StoryID *uuid.UUID`, `SprintID *uuid.UUID`). Las escrituras reciben el actor y,
      las que fijan `status_changed_at`, el instante `now` — depende de T001, T002, T004
- [X] T006 Agregar `Defect` al `AutoMigrate` de `backend/internal/platform/database/postgres.go`
      **después** de `SprintStory` (apunta a `user_stories`, `sprints` y `users`) y actualizar el
      comentario del orden y de la columna nueva `ever_had_defects` de `UserStory` — depende de T001,
      T003
- [X] T007 Extender el vaciado del ayudante de pruebas en
      `backend/internal/platform/testsupport/database.go` para incluir `defects` en el
      `TRUNCATE ... CASCADE`, manteniendo la verificación de que la base es `smye_test` y el lock de
      la base de test — depende de T006

**Checkpoint**: `defects` migrada en la base de test con sus seis claves foráneas `NO ACTION` y
`fk_defects_story` (quickstart.md sección 2), `user_stories.ever_had_defects` creada, el dominio y el
contrato declarados, y las pruebas aisladas. Las historias pueden empezar.

---

## Fase 2: Historia de Usuario 1 — Registrar un defecto (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante registra un defecto con descripción, severidad, historia y sprint de
detección; nace Abierto, sin sprint de resolución, atribuido a quien lo registró, y la historia queda
marcada como no eliminable.

**Rama**: `hu/007-us1-registrar-un-defecto`

**Prueba independiente**: `POST /api/v1/projects/{projectId}/defects` con severidad `high`, una
historia del proyecto y el sprint Activo devuelve `201` con `status: open`, `resolvedSprint: null` y
`createdBy` igual a la sesión; con un sprint `planned` devuelve `400` en `detectedSprintId`; en un
proyecto sin sprints iniciados devuelve `409 no_detection_sprint`.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T008 [P] [US1] Pruebas unitarias **obligatorias** de `DefectSeverity.Valid() bool` y
      `SeverityRank(DefectSeverity) int` en `backend/internal/defects/domain/severity_test.go`,
      cubriendo RN-04: los cuatro valores válidos con rangos `critical` 0, `high` 1, `medium` 2,
      `low` 3; `"urgent"`, `""` y `"Critical"` inválidos — depende de T007
- [X] T009 [P] [US1] Pruebas unitarias **obligatorias** de
      `EnsureSprintUsableForDetection(status sprintsdomain.SprintStatus) error` en
      `backend/internal/defects/domain/rules_test.go`, cubriendo RN-08: `planned` →
      `ErrDetectionSprintPlanned`; `active` y `closed` → `nil` — depende de T007
- [X] T010 [P] [US1] Pruebas unitarias **obligatorias** de `ValidateDefectInput` en
      `backend/internal/defects/delivery/validation_test.go`, cubriendo RN-03, RN-04, RN-06 y
      FR-016: descripción ausente, `""` y `"   "` → "La descripción es obligatoria."; de exactamente
      **2000 runas** con acentos aceptada y recortada; de **2001** rechazada con "La descripción
      admite como máximo 2000 caracteres."; severidad ausente o `urgent` → "La severidad debe ser
      Crítica, Alta, Media o Baja."; `storyId` ausente → "La historia relacionada es obligatoria.";
      `detectedSprintId` ausente → "El sprint de detección es obligatorio."; `storyId` o
      `detectedSprintId` mal formados → "…no es un identificador válido."; varios campos inválidos
      devueltos **todos a la vez**; un `createdBy` o `createdById` en el cuerpo se ignora — depende de
      T007
- [X] T011 [US1] Pruebas de integración de la historia en
      `backend/internal/defects/delivery/handler_test.go`, creando en
      `backend/internal/defects/delivery/testenv_test.go` el ayudante `newDefectTestEnv` (router como
      `cmd/app` con las rutas de la 001 a la 004 y las de defectos, reloj falso compartido por los
      servicios de sprints y de defectos, ayudantes para crear cuentas, proyectos, integrantes,
      historias y sprints con los endpoints reales, y `forceDefectStatus` descripto arriba). Una
      función por escenario de spec.md precedida por `// Escenario: <nombre del escenario> (007/US1)`:
      `TestUS1_RegistroEnSprintActivo` (`201`, `open`, `resolvedSprint: null`, `createdBy`,
      `updatedBy` y `statusChangedBy` = sesión, `statusChangedAt` = `createdAt`),
      `TestUS1_SprintActivoPorDefecto` (con un sprint Activo, registrar con un sprint Cerrado se
      acepta), `TestUS1_HistoriaCompletada` (historia completada con la 004), `TestUS1_SinSprintActivo`
      (solo sprints Cerrados), `TestUS1_VariosDefectosPorHistoria`,
      `TestUS1_DescripcionDeDosMilCaracteres` (subtests 2000 `201` y 2001 `400`),
      `TestUS1_CuatroSeveridades` (subtests por severidad; SC-004), `TestUS1_HistoriaConCincuentaDefectos`
      (el 51 se acepta; la cantidad de filas de la historia es 51), `TestUS1_DescripcionVacia`
      (subtests ausente y espacios), `TestUS1_SeveridadInvalida`, `TestUS1_CamposObligatorios`
      (subtests historia y sprint faltantes), `TestUS1_SprintPlanificado` (`400` en
      `detectedSprintId`), `TestUS1_VinculoDeOtroProyecto` (subtests historia → `404`
      `{"error":"La historia no existe."}` y sprint → `404` `{"error":"El sprint no existe."}`,
      idénticos byte a byte a los de un UUID inexistente), `TestUS1_ProyectoFinalizado` (`409
      read_only`), `TestUS1_NoIntegrante` (`404` con el cuerpo de la 002) y
      `TestUS1_ProyectoSinSprintIniciado` (subtests con un sprint `planned` y con un UUID inexistente
      en el cuerpo: ambos `409 no_detection_sprint`). En cada rechazo se verifica que la cantidad de
      filas de `defects` no cambió y que `ever_had_defects` de la historia sigue en `false` — depende
      de T007
- [X] T012 [P] [US1] Pruebas de integración contra `smye_test` en `backend/internal/backlog/repository/`:
      en `defect_records_test.go`, `HasDefects` es `false` sin defectos y `true` con
      `ever_had_defects = true`, y **sigue** `true` después de mover el único defecto a otra historia
      con un `UPDATE` directo (RC-05, RN-19); en `story_repository_test.go`,
      `TestStoryDelete_DefectRace` (**sin** etiqueta): con un defecto insertado directamente sobre una
      historia cuya marca sigue en `false` (simula la carrera de research.md sección 10),
      `StoryRepository.Delete` devuelve `DeletionBlockedError` con el motivo `defects` y no un error
      genérico, y la historia no se borra — depende de T007
- [X] T013 [P] [US1] Pruebas de frontend en `frontend/src/features/defects/validation.test.ts`
      (descripción y severidad con las mismas reglas de forma que el backend),
      `frontend/src/utils/defect-labels.test.ts` (etiquetas en español de las cuatro severidades, los
      cuatro estados, `no_defects`, `no_matches` y los cinco `code` de conflicto) y
      `frontend/src/features/defects/components/defect-form.test.tsx`, con
      `describe("007/US1 - Registrar un defecto")` y un `it` por escenario con comportamiento visible:
      el sprint Activo propuesto y los Cerrados como alternativa, ningún sprint `planned` en el
      selector, sin sprint propuesto cuando no hay Activo, el aviso y el envío deshabilitado cuando no
      hay sprints Activos ni Cerrados, la historia preseleccionada con `?storyId=`, los mensajes de
      campo del `400` junto a cada campo, el mensaje de proyecto de solo lectura, y que el formulario
      **no** tiene campo de autor

### Implementación de la Historia 1

- [X] T014 [P] [US1] Implementar `DefectSeverity.Valid` y `SeverityRank` en
      `backend/internal/defects/domain/severity.go` — depende de T008
- [X] T015 [P] [US1] Implementar `EnsureSprintUsableForDetection` en
      `backend/internal/defects/domain/rules.go` — depende de T009
- [X] T016 [US1] Implementar en `backend/internal/defects/delivery/validation.go`
      `ValidateDefectInput` (recorte, 1 a 2000 runas, severidad válida, identificadores obligatorios y
      bien formados) y declarar el DTO `DefectInput` en `backend/internal/defects/delivery/dto.go`
      (**sin** campo de autor), con los mensajes como constantes nombradas en un único lugar y todos
      los campos inválidos a la vez (Principio VII) — depende de T010, T014
- [X] T017 [US1] Implementar `Create` en `backend/internal/defects/repository/defect_repository.go`
      con el orden de bloqueos de research.md sección 10, en una transacción: (1) proyecto con
      `clause.Locking{Strength: "SHARE"}`, `finished` → `ErrDefectsReadOnly`; (2) `EXISTS` de un
      sprint del proyecto con `status IN ('active','closed')`, si no → `ErrNoDetectionSprint`; (3)
      sprint de detección `WHERE id = ? AND project_id = ?` con
      `clause.Locking{Strength: "KEY SHARE"}`, sin fila → `sprintsdomain.ErrSprintNotFound`, y
      `EnsureSprintUsableForDetection`; (4)
      `UPDATE user_stories SET ever_had_defects = true WHERE id = ? AND project_id = ?` con
      `UpdateColumn` (sin tocar `updated_at`), cero filas → `backlogdomain.ErrStoryNotFound`; (5)
      inserción en `open`, `resolved_sprint_id` nulo, `created_by_id`, `updated_by_id` y
      `status_changed_by_id` = actor y `status_changed_at` = `now`. Implementar también `FindView`,
      que devuelve el `DefectView` con historia, ambos sprints y los tres `UserRef` (`users.id` y
      `users.full_name`, **nunca** `password_hash`); sin fila → `ErrDefectNotFound` — depende de T005,
      T011, T015
- [X] T018 [US1] Implementar en `backend/internal/defects/service/defect_service.go` el constructor
      `NewDefectService(repo domain.DefectRepository, now func() time.Time)`, con la firma definitiva
      para que no cambie en las historias siguientes, y `Create(ctx, projectID, actorID, input)`, que
      toma el actor **siempre** del titular de la sesión (FR-056) y el instante del reloj una sola vez
      — depende de T017
- [X] T019 [US1] Declarar en `backend/internal/defects/delivery/dto.go` los esquemas de
      contracts/openapi.yaml `Defect` (con `projectId` derivado del proyecto de la historia),
      `UserRef`, `StoryRef`, `SprintRef` y `DefectConflictResponse` (`code`, `error`,
      `currentStatus` anulable, `allowedStatuses` siempre arreglo, nunca `null`), reutilizando
      `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de la 001. `resolvedSprint` se
      serializa `null` cuando no aplica — depende de T018
- [X] T020 [US1] Implementar el manejador `createDefect` en
      `backend/internal/defects/delivery/handler.go` con anotaciones `swag` en español, y la
      traducción de errores: `ErrDetectionSprintPlanned` → `400` con `field: detectedSprintId` ("Un
      defecto solo se detecta en un sprint Activo o Cerrado."); `backlogdomain.ErrStoryNotFound` →
      `404` "La historia no existe."; `sprintsdomain.ErrSprintNotFound` → `404` "El sprint no
      existe."; `ErrDefectsReadOnly` → `409 read_only` ("El proyecto está finalizado y sus defectos
      son de solo lectura."); `ErrNoDetectionSprint` → `409 no_detection_sprint` ("Todavía no hay
      ningún sprint Activo ni Cerrado que pueda ser sprint de detección."); cualquier otro → `500`
      genérico registrado con `slog` — depende de T016, T019
- [X] T021 [US1] Crear en `backend/internal/defects/delivery/routes.go`
      `RegisterProjectRoutes(projectGroup, handler)` con `POST /defects` sobre el subgrupo de la 002,
      sin importar `projects/delivery`; y componer el módulo en `backend/cmd/app/main.go`:
      repositorio, servicio con `time.Now`, registro sobre `projectGroup`, más el `@description`
      general de la API para que mencione la gestión de defectos — depende de T020
- [X] T022 [US1] Reemplazar el puerto de la 003 y traducir la clave foránea: en
      `backend/internal/backlog/repository/defect_records.go`, `NewDefectRecords(db *gorm.DB)` y
      `HasDefects` que lee `user_stories.ever_had_defects`; en
      `backend/internal/backlog/repository/story_repository.go`, `Delete` reconoce además la
      violación de `fk_defects_story` (constante con el nombre y función `isDefectsFKViolation` con el
      mismo patrón que `isEstimationSessionsFKViolation`) y devuelve
      `DeletionBlockedError{Reasons: []DeletionBlocker{DeletionBlockerDefects}}`; actualizar sus
      comentarios (ya no son provisorios) y pasar `db` al puerto en `backend/cmd/app/main.go`.
      `backlog/service` no cambia. Verificación de punta a punta en
      `backend/internal/defects/delivery/handler_test.go`, **sin** etiqueta `007/USn`:
      `TestDefectPorts_BloqueanEliminacion`, que con los endpoints reales registra un defecto sobre
      una historia y verifica que `DELETE /api/v1/stories/{id}` responde `409` con el motivo
      `defects` (RC-05). Se escribe primero y falla con el puerto provisorio — depende de T012, T021
- [X] T023 [P] [US1] Declarar los tipos de petición y respuesta en `frontend/src/types/defect.ts` (sin
      `any`; uniones literales para severidad, estado, `emptyReason` y `code`), implementar
      `frontend/src/utils/defect-labels.ts` y `frontend/src/features/defects/validation.ts` — depende
      de T013
- [X] T024 [US1] Implementar en `frontend/src/features/defects/api.ts` `createDefect` y
      `listDetectionSprints` (sobre `GET /api/v1/projects/{projectId}/sprints?status=active&status=closed`
      de la 004), `DefectForm` en `frontend/src/features/defects/components/defect-form.tsx`
      (descripción, severidad, historia, sprint propuesto; cinco pasos, cuatro si se llega desde la
      historia) y la página `frontend/src/app/projects/[projectId]/defects/new/page.tsx`, que al crear
      navega al detalle del defecto. Antes, leer la documentación de la versión instalada en
      `frontend/node_modules/next/dist/docs/` — depende de T013, T021, T023

**Checkpoint**: US1 funciona sola: se registran defectos con su sprint de detección validado, y la
003 ya bloquea la eliminación de una historia que tuvo defectos.

---

## Fase 3: Historia de Usuario 2 — Seguir y resolver un defecto (Prioridad: P1)

**Objetivo**: un integrante pasa un defecto Abierto a En progreso y lo resuelve indicando el sprint
de resolución; las transiciones no permitidas se rechazan contra el estado vigente.

**Rama**: `hu/007-us2-seguir-y-resolver-un-defecto`

**Prueba independiente**: `PUT /api/v1/defects/{id}/status` con `in_progress` y luego con `resolved` y
el sprint Activo devuelve `200` y deja el defecto `resolved` con ese sprint; `resolved` directo desde
`open` devuelve `409 invalid_transition` con `currentStatus: open`; sin `resolvedSprintId` devuelve
`400`.

### Pruebas de la Historia 2 ⚠️

- [X] T025 [P] [US2] Pruebas unitarias **obligatorias** en
      `backend/internal/defects/domain/status_test.go`, cubriendo RN-11 a RN-14 para las transiciones
      de esta historia: `DefectStatus.Valid` con los cuatro valores y uno inexistente;
      `AllowedTransitions(open)` = `[in_progress]` y `AllowedTransitions(in_progress)` = `[resolved]`;
      `PlanTransition(current, to, detected SprintAnchor, resolution *SprintAnchor) (*uuid.UUID, error)`
      con `open → in_progress` → `nil, nil`; `in_progress → resolved` con un sprint `active` y otro
      `closed` posteriores → su ID; con **el mismo sprint** que detección → su ID (FR-025); sin
      `resolution` → `ErrResolutionSprintRequired`; con `planned` → `ErrResolutionSprintPlanned`; con
      `StartDate` anterior → `ErrResolutionBeforeDetection`; y como `InvalidTransitionError` con
      `Current`, `Requested` y `Allowed`: `open → resolved`, `in_progress → open`, `open → open`,
      `in_progress → in_progress`, `resolved → in_progress` y `resolved → resolved`. Las transiciones
      de reapertura y descarte se prueban en US5 y US6 — depende de T024
- [X] T026 [P] [US2] Pruebas unitarias **obligatorias** de `ValidateStatusChange` en
      `backend/internal/defects/delivery/validation_test.go`, cubriendo FR-023: `status` ausente o
      `closed` → "El estado debe ser Abierto, En progreso, Resuelto o Descartado."; `resolved` sin
      `resolvedSprintId` → `field: resolvedSprintId` "El sprint de resolución es obligatorio para
      resolver."; `in_progress`, `open` o `discarded` **con** `resolvedSprintId` → "El sprint de
      resolución solo se indica al resolver."; `resolvedSprintId` mal formado → "…no es un
      identificador válido." — depende de T024
- [X] T027 [P] [US2] Pruebas del middleware `RequireDefectAccess` en
      `backend/internal/defects/delivery/middleware_test.go`, cubriendo FR-053 y FR-055: sin titular →
      `401` con el cuerpo de la 001; identificador mal formado, defecto inexistente, defecto de un
      proyecto ajeno y defecto registrado por una persona ya quitada del proyecto, pedido por ella →
      `404` con **el mismo cuerpo byte a byte** (`{"error":"El defecto no existe."}`); integrante →
      sigue la cadena con el `DefectAccess` en el contexto, sea o no el autor — depende de T024
- [X] T028 [US2] Pruebas de integración en `backend/internal/defects/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (007/US2)`: `TestUS2_CicloCompleto` (dos `200`;
      `GET /api/v1/defects/{id}` muestra `resolved`, el sprint de resolución, y
      `statusChangedBy`/`statusChangedAt` de la última transición), `TestUS2_MismoSprint`,
      `TestUS2_SprintPosterior` (detectado en un sprint ya Cerrado, resuelto en el Activo),
      `TestUS2_OtroIntegrante` (`updatedBy` y `statusChangedBy` = quien actuó; `createdBy` sin
      cambio), `TestUS2_ResolucionEnSprintCerrado`, `TestUS2_UnicoSprint`,
      `TestUS2_ResolucionSinSprint` (`400`, sigue `in_progress`; SC-006),
      `TestUS2_ResolucionAnteriorALaDeteccion` (`400` en `resolvedSprintId`, sin cambios; SC-007),
      `TestUS2_ResolucionEnSprintPlanificado` (`400`; SC-008), `TestUS2_SaltoAResuelto` (`409
      invalid_transition`, `currentStatus: open`, mensaje "Un defecto Abierto primero debe pasar a En
      progreso."), `TestUS2_VueltaAAbierto` (`409`, mensaje "Para volver a Abierto, resolvé el defecto
      y después reabrilo."), `TestUS2_SprintDeOtroProyecto` (`404` "El sprint no existe.", idéntico al
      de un UUID inexistente) y `TestUS2_ProyectoFinalizado` (`409 read_only`). En cada rechazo se
      verifica que la fila del defecto quedó idéntica — depende de T024
- [X] T029 [US2] Prueba de concurrencia `TestDefectTransition_ConcurrentStart` en
      `backend/internal/defects/delivery/defect_concurrency_test.go`, **sin** etiqueta `007/USn`: dos
      integrantes envían a la vez `in_progress` sobre el mismo defecto `open`; exactamente uno recibe
      `200` y el otro `409 invalid_transition` con `currentStatus: in_progress`, ninguno `500`.
      Repetida **10** veces para que el intercalado sea real (FR-018, RN-24) — depende de T028
- [X] T030 [P] [US2] Pruebas de frontend en `frontend/src/features/defects/transitions.test.ts`
      (`allowedTransitions` coincide con el mapa del backend para las transiciones de esta historia)
      y `frontend/src/features/defects/components/defect-status-actions.test.tsx`, con
      `describe("007/US2 - Seguir y resolver un defecto")`: un defecto Abierto ofrece solo "Pasar a En
      progreso"; uno En progreso ofrece "Resolver", que pide el sprint de resolución entre los
      sprints Activos y Cerrados; el `400` se muestra junto al selector; ante un `409
      invalid_transition` se muestra el mensaje y se refresca el defecto con el `currentStatus`
      recibido — depende de T024

### Implementación de la Historia 2

- [X] T031 [US2] Implementar en `backend/internal/defects/domain/status.go` `DefectStatus.Valid`, el
      mapa `allowedDefectTransitions` con `open: [in_progress]` e `in_progress: [resolved]`,
      `AllowedTransitions` y `PlanTransition`, que nunca devuelve ni modifica la historia ni el
      sprint de detección (FR-027) — depende de T025
- [X] T032 [US2] Implementar `ValidateStatusChange` en `backend/internal/defects/delivery/validation.go`
      con el DTO `DefectStatusChange` en `backend/internal/defects/delivery/dto.go` — depende de T026,
      T031
- [X] T033 [US2] Implementar en `backend/internal/defects/repository/defect_repository.go`
      `FindAccessible` (una consulta `defects → user_stories → project_members (sesión) → projects`;
      sin fila → `ErrDefectNotFound`) y `ChangeStatus` en una transacción con el orden de
      research.md sección 10: (1) proyecto del defecto con
      `clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "projects"}}`, `finished` →
      `ErrDefectsReadOnly`; (2) defecto con `clause.Locking{Strength: "UPDATE"}` y su sprint de
      detección releídos; (3) si hay `ResolvedSprintID`, sprint `WHERE id = ? AND project_id = ?` con
      `KEY SHARE`, sin fila → `sprintsdomain.ErrSprintNotFound`; (4) `PlanTransition` con el estado
      **releído**; (5) escritura de `status`, `resolved_sprint_id` (siempre la salida de
      `PlanTransition`), `status_changed_at`, `status_changed_by_id` y `updated_by_id`. Devuelve
      `FindView` dentro de la misma transacción — depende de T027, T028, T031
- [X] T034 [US2] Implementar `RequireDefectAccess` en `backend/internal/defects/delivery/middleware.go`
      con una única salida `404` y el mensaje `El defecto no existe.` — depende de T033
- [X] T035 [US2] Implementar `ChangeStatus` y `Get` en
      `backend/internal/defects/service/defect_service.go` con el actor de la sesión y el instante del
      reloj — depende de T033
- [X] T036 [US2] Implementar los manejadores `changeDefectStatus` y `getDefect` con anotaciones `swag`
      en `backend/internal/defects/delivery/handler.go`: `InvalidTransitionError` → `409
      invalid_transition` con `currentStatus` y `allowedStatuses`, y el mensaje elegido por
      `(Current, Requested)` en una función de `delivery` (salto de Abierto a Resuelto, vuelta de En
      progreso a Abierto, genérico "No se puede pasar de X a Y. Desde X se permite: …"); los tres
      errores del sprint de resolución → `400` con `field: resolvedSprintId`. En
      `backend/internal/defects/delivery/routes.go`, `RegisterDefectRoutes(router, requireSession,
      repo, handler)` con el grupo plano `/api/v1/defects/:defectId`, `GET ""` y `PUT /status`,
      registrado en `backend/cmd/app/main.go` sobre la misma instancia del repositorio — depende de
      T032, T034, T035
- [X] T037 [US2] Implementar `frontend/src/features/defects/transitions.ts` (mapa con las
      transiciones de esta historia), `getDefect` y `changeDefectStatus` en
      `frontend/src/features/defects/api.ts`, `DefectDetail` en
      `frontend/src/features/defects/components/defect-detail.tsx`, `DefectStatusActions` en
      `frontend/src/features/defects/components/defect-status-actions.tsx`, `ResolveDefectForm` en
      `frontend/src/features/defects/components/resolve-defect-form.tsx` y la página
      `frontend/src/app/projects/[projectId]/defects/[defectId]/page.tsx` — depende de T030, T036

**Checkpoint**: US1 y US2 funcionan; un defecto se registra, se trabaja y se resuelve con su sprint.

---

## Fase 4: Historia de Usuario 3 — Consultar y filtrar los defectos (Prioridad: P2)

**Objetivo**: un integrante recorre los defectos del proyecto con filtros combinables por estado,
severidad, historia y sprint, y ve los defectos de una historia desde su detalle.

**Rama**: `hu/007-us3-consultar-y-filtrar-los-defectos`

**Prueba independiente**: con defectos de distintas severidades, estados, historias y sprints,
`GET /api/v1/projects/{projectId}/defects?severity=critical&status=in_progress` devuelve solo los que
cumplen ambas; `?sprintId=<id>` devuelve detectados y resueltos en ese sprint; `?status=closed`
devuelve `400` en `status`.

### Pruebas de la Historia 3 ⚠️

- [X] T038 [P] [US3] Pruebas unitarias **obligatorias** de
      `ParseDefectFilter(statuses, severities []string, storyID, sprintID string)` en
      `backend/internal/defects/delivery/validation_test.go`, cubriendo FR-046 y FR-047: sin
      parámetros → filtro vacío; `status` y `severity` repetidos → listas en el orden recibido; un
      valor inexistente en cada uno → `FieldError` con los valores admitidos; `storyId` y `sprintId`
      mal formados → `FieldError` con "El filtro no corresponde al proyecto."; varios filtros
      inválidos a la vez — depende de T037
- [X] T039 [US3] Pruebas de integración en `backend/internal/defects/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (007/US3)`: `TestUS3_SinFiltros` (defectos en los
      cuatro estados —el Descartado preparado con `forceDefectStatus`— y de las cuatro severidades,
      todos con descripción, severidad, estado, historia, sprint de detección y de resolución
      cuando lo tienen), `TestUS3_DefectosDeUnaHistoria` (`GET /api/v1/stories/{id}/defects`),
      `TestUS3_FiltroPorEstado`, `TestUS3_SeveridadYEstado`, `TestUS3_FiltroPorSprint` (detectados y
      resueltos en el sprint, y uno detectado y resuelto en él aparece una sola vez),
      `TestUS3_ProyectoSinDefectos` (`defects: []`, `emptyReason: no_defects`),
      `TestUS3_HistoriaSinDefectos`, `TestUS3_SinCoincidencias` (`emptyReason: no_matches`),
      `TestUS3_OrdenEstable` (50 defectos de una historia con severidades mezcladas: orden por rango,
      `created_at DESC`, `id DESC`, idéntico en tres consultas; SC-014), `TestUS3_FiltroDeOtroProyecto`
      (subtests historia y sprint ajenos: `400` con el **mismo** cuerpo que un UUID inexistente),
      `TestUS3_ValorDeFiltroInvalido` (subtests estado y severidad) y `TestUS3_NoIntegrante` (`404` de
      la 002 en el listado del proyecto y de la 003 en el de la historia). Además, subtests con al
      menos tres combinaciones de dos o más filtros (SC-013) — depende de T037
- [X] T040 [P] [US3] Pruebas de frontend en
      `frontend/src/features/defects/components/defect-filters.test.tsx`,
      `frontend/src/features/defects/components/defect-list.test.tsx` y
      `frontend/src/features/defects/components/story-defects-section.test.tsx`, con
      `describe("007/US3 - Consultar y filtrar los defectos")`: los filtros arman la query con
      `status` y `severity` repetidos; cada fila muestra severidad, estado, historia, sprint de
      detección y de resolución; los avisos distintos para `no_defects` y `no_matches`; la sección de
      la historia lista sus defectos y ofrece "Registrar defecto" con la historia preseleccionada
      (SC-003) — depende de T037

### Implementación de la Historia 3

- [X] T041 [US3] Implementar `ParseDefectFilter` en `backend/internal/defects/delivery/validation.go`
      sobre `c.QueryArray("status")`, `c.QueryArray("severity")`, `storyId` y `sprintId` — depende de
      T038
- [X] T042 [US3] Implementar en `backend/internal/defects/repository/defect_repository.go`
      `ValidateFilter` (historia y sprint del proyecto; cualquier otro → `InvalidFilterError{Field}`
      sin distinguir inexistente de ajeno), `ListForProject` (filtros conjuntivos sobre `defects`
      unido a `user_stories` por `project_id`, `status IN ?`, `severity IN ?`, `story_id = ?` y
      `(detected_sprint_id = ? OR resolved_sprint_id = ?)`; orden
      `CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
      created_at DESC, id DESC`; sin paginación) y `HasProjectDefects`, reutilizando la consulta de
      `FindView` para las columnas — depende de T039
- [X] T043 [US3] Implementar `ListForProject(ctx, projectID, filter)` y `ListForStory(ctx, access)` en
      `backend/internal/defects/service/defect_service.go`: valida el filtro, lista y, con la lista
      vacía, calcula `emptyReason` (`no_defects` si `HasProjectDefects` es `false`, si no
      `no_matches`; siempre `no_defects` para una historia) — depende de T042
- [X] T044 [US3] Declarar `ProjectDefects` y `StoryDefects` en `backend/internal/defects/delivery/dto.go`
      (listas vacías como `[]`), implementar los manejadores `listProjectDefects` (`InvalidFilterError`
      → `400` con `ValidationErrorResponse`) y `listStoryDefects` con anotaciones `swag` en
      `backend/internal/defects/delivery/handler.go`, agregar `GET /defects` a `RegisterProjectRoutes`
      y crear `RegisterStoryRoutes(router, requireSession, requireStoryAccess, handler)` con
      `GET /api/v1/stories/:storyId/defects` en `backend/internal/defects/delivery/routes.go`, sin
      importar `backlog/delivery`; registrarlo en `backend/cmd/app/main.go` con el
      `RequireStoryAccess` de la 003 sobre la misma instancia del repositorio de historias — depende
      de T041, T043
- [X] T045 [US3] Implementar `listProjectDefects` y `listStoryDefects` en
      `frontend/src/features/defects/api.ts`, `DefectFilters`, `DefectList` y `DefectRow` en
      `frontend/src/features/defects/components/`, la página
      `frontend/src/app/projects/[projectId]/defects/page.tsx`, `StoryDefectsSection` en
      `frontend/src/features/defects/components/story-defects-section.tsx` compuesta en
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx`, y el enlace **Defectos** en
      `frontend/src/features/projects/components/project-overview.tsx` — depende de T040, T044

**Checkpoint**: US1 a US3 funcionan; lo registrado se encuentra y se lee.

---

## Fase 5: Historia de Usuario 4 — Corregir los datos de un defecto (Prioridad: P2)

**Objetivo**: un integrante corrige la descripción y la severidad de un defecto en cualquier estado
salvo Descartado, y su historia y su sprint de detección solo mientras está Abierto.

**Rama**: `hu/007-us4-corregir-los-datos-de-un-defecto`

**Prueba independiente**: `PATCH /api/v1/defects/{id}` con descripción y severidad `critical` sobre
un defecto Abierto devuelve `200` con estado, sprints y autor sin cambios; con `storyId` distinto
sobre uno En progreso devuelve `409 links_frozen`; con `status` devuelve `400`.

### Pruebas de la Historia 4 ⚠️

- [X] T046 [P] [US4] Pruebas unitarias **obligatorias** en
      `backend/internal/defects/domain/rules_test.go`, cubriendo RN-16, RN-18 y RN-22:
      `EnsureDefectEditable` con `open`, `in_progress` y `resolved` → `nil`, `discarded` →
      `ErrDefectDiscarded`; `EnsureLinksCorrectable` con `open` → `nil`, y `in_progress`, `resolved` y
      `discarded` → `ErrLinksFrozen` — depende de T045
- [X] T047 [P] [US4] Pruebas unitarias **obligatorias** de `ValidateDefectPatch` en
      `backend/internal/defects/delivery/validation_test.go`, cubriendo FR-034, FR-035 y FR-037:
      cuerpo `{}` → "Indicá al menos un dato para modificar."; solo `severity` → ok; las mismas reglas
      de descripción, severidad e identificadores que el alta; `status` y `resolvedSprintId`
      presentes, **aunque valgan `null`** → `FieldError` en ese campo con "El estado y el sprint de
      resolución se cambian con las acciones de estado del defecto."; `projectId`, `createdBy` y
      `createdById` presentes → "El proyecto y el autor de un defecto no se modifican." — depende de
      T045
- [X] T048 [US4] Pruebas de integración en `backend/internal/defects/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (007/US4)`: `TestUS4_DescripcionYSeveridad` (estado,
      sprints y `createdBy` sin cambio; `updatedBy` = quien modificó),
      `TestUS4_RevinculacionDeHistoria` (deja de aparecer en la historia anterior y aparece en la
      nueva; `ever_had_defects` de **ambas** queda en `true`), `TestUS4_DescripcionDeResuelto`,
      `TestUS4_SeveridadEnProgreso`, `TestUS4_CorreccionDeSprintDeDeteccion` (subtests otro Activo y
      un Cerrado), `TestUS4_AmbosVinculos`, `TestUS4_SinCambios` (mismos valores, incluidos los dos
      vínculos sobre un defecto En progreso: `200` sin efecto en los datos),
      `TestUS4_DescripcionDeDosMilCaracteres`, `TestUS4_VentanaReabierta` (defecto resuelto y llevado a
      `open` sin sprint de resolución con `forceDefectStatus`: la corrección del sprint de detección se
      acepta), `TestUS4_DatosInvalidos` (subtests descripción vacía y severidad inválida: `400` y el
      defecto exactamente como estaba), `TestUS4_HistoriaDeOtroProyecto` (`404` "La historia no
      existe."), `TestUS4_VinculosCongelados` (subtests historia y sprint de detección sobre un defecto
      En progreso y sobre uno Resuelto: `409 links_frozen` con `currentStatus`; SC-011),
      `TestUS4_SprintDeDeteccionInvalido` (subtests `planned` → `400` y de otro proyecto → `404`),
      `TestUS4_DefectoDescartado` (preparado con `forceDefectStatus`: `409 defect_discarded`),
      `TestUS4_EstadoDesdeModificacion` (subtests `status` y `resolvedSprintId`: `400`) y
      `TestUS4_ProyectoFinalizado` (`409 read_only`) — depende de T045
- [X] T049 [US4] Agregar a `backend/internal/defects/delivery/defect_concurrency_test.go` la variante
      `TestDefectLinks_ConcurrentCorrectionAndStart`, **sin** etiqueta: a la vez, sobre un defecto
      `open`, una corrección de su historia y un paso a `in_progress`; en 10 repeticiones el
      resultado es siempre uno de dos: la corrección se aplicó y después el paso, o el paso se aplicó
      y la corrección recibió `409 links_frozen`; ninguna `500` (FR-018, FR-038) — depende de T048
- [X] T050 [P] [US4] Pruebas de frontend en
      `frontend/src/features/defects/components/defect-edit-form.test.tsx`, con
      `describe("007/US4 - Corregir los datos de un defecto")`: descripción y severidad editables en
      Abierto, En progreso y Resuelto; historia y sprint de detección editables **solo** en Abierto;
      se envían solo los campos cambiados; los mensajes del `400`, del `404` y de `links_frozen` se
      muestran sin perder lo escrito — depende de T045

### Implementación de la Historia 4

- [X] T051 [P] [US4] Implementar `EnsureDefectEditable` y `EnsureLinksCorrectable` en
      `backend/internal/defects/domain/rules.go` — depende de T046
- [X] T052 [US4] Implementar `ValidateDefectPatch` en `backend/internal/defects/delivery/validation.go`
      con el DTO `DefectPatch` en `backend/internal/defects/delivery/dto.go`: cada campo como
      `json.RawMessage` para distinguir ausente de `null`, y `status`, `resolvedSprintId`,
      `projectId`, `createdBy` y `createdById` decodificados **solo para rechazarlos** con
      `swaggerignore:"true"` (patrón de `StoryPatch` de la 003) — depende de T047
- [X] T053 [US4] Implementar `Update` en `backend/internal/defects/repository/defect_repository.go` en
      una transacción con el orden de research.md sección 10: proyecto `FOR SHARE OF projects`
      (`finished` → `ErrDefectsReadOnly`); defecto `FOR UPDATE` releído; `EnsureDefectEditable`; para
      cada vínculo presente **cuyo valor difiere del vigente**, `EnsureLinksCorrectable`; si cambia el
      sprint de detección, el sprint nuevo `WHERE id = ? AND project_id = ?` con `KEY SHARE` y
      `EnsureSprintUsableForDetection`; si cambia la historia, el `UPDATE` de `ever_had_defects` sobre
      la historia nueva con `project_id` (cero filas → `backlogdomain.ErrStoryNotFound`); escritura de
      los campos presentes y de `updated_by_id`. **Nunca** escribe `status`, `resolved_sprint_id`,
      `created_by_id` ni `status_changed_*` (FR-035) — depende de T048, T051
- [X] T054 [US4] Implementar `Update` en `backend/internal/defects/service/defect_service.go` con el
      actor de la sesión — depende de T053
- [ ] T055 [US4] Implementar el manejador `updateDefect` con anotaciones `swag` en
      `backend/internal/defects/delivery/handler.go` (`ErrDefectDiscarded` → `409 defect_discarded`
      "El defecto está Descartado y es de solo lectura."; `ErrLinksFrozen` → `409 links_frozen` "La
      historia relacionada y el sprint de detección solo se corrigen mientras el defecto está
      Abierto." con el `currentStatus` releído y sus `allowedStatuses`), y agregar `PATCH ""` a
      `RegisterDefectRoutes` en `backend/internal/defects/delivery/routes.go` — depende de T052, T054
- [ ] T056 [US4] Implementar `updateDefect` en `frontend/src/features/defects/api.ts` y
      `DefectEditForm` en `frontend/src/features/defects/components/defect-edit-form.tsx`, compuesto en
      `DefectDetail` — depende de T050, T055

**Checkpoint**: US1 a US4 funcionan; los datos son corregibles y los vínculos se congelan al empezar
a trabajar el defecto.

---

## Fase 6: Historia de Usuario 5 — Reabrir un defecto resuelto (Prioridad: P3)

**Objetivo**: un integrante devuelve a Abierto un defecto Resuelto que sigue ocurriendo; pierde el
sprint de resolución y conserva el de detección.

**Rama**: `hu/007-us5-reabrir-un-defecto-resuelto`

**Prueba independiente**: `PUT /api/v1/defects/{id}/status` con `open` sobre un defecto Resuelto
devuelve `200` con `status: open`, `resolvedSprint: null` y el mismo sprint de detección; sobre uno En
progreso devuelve `409 invalid_transition`.

### Pruebas de la Historia 5 ⚠️

- [ ] T057 [P] [US5] Ampliar las pruebas unitarias **obligatorias** de
      `backend/internal/defects/domain/status_test.go`, cubriendo RN-15 y RN-17:
      `AllowedTransitions(resolved)` = `[open]`; `PlanTransition(resolved, open, …)` → `nil, nil`
      (sprint de resolución borrado); `PlanTransition(open, open, …)` e
      `PlanTransition(in_progress, open, …)` siguen siendo `InvalidTransitionError` — depende de T056
- [ ] T058 [US5] Pruebas de integración en `backend/internal/defects/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (007/US5)`: `TestUS5_Reapertura` (`resolvedSprint:
      null`, detección y historia sin cambio; SC-009), `TestUS5_SegundaResolucion`,
      `TestUS5_VariasReaperturas` (vale la última resolución), `TestUS5_ResolucionEnElMismoSprint`
      (detectado en S1, resuelto en S2, reabierto y resuelto otra vez en S2),
      `TestUS5_SinSprintActivo` (único sprint Cerrado), `TestUS5_DefectoNoResuelto` (subtests Abierto y
      En progreso: `409` con "Solo se reabre un defecto Resuelto.") y `TestUS5_ProyectoFinalizado` —
      depende de T056
- [ ] T059 [P] [US5] Pruebas de frontend en
      `frontend/src/features/defects/components/defect-status-actions.test.tsx` y
      `frontend/src/features/defects/transitions.test.ts`, con
      `describe("007/US5 - Reabrir un defecto resuelto")`: el botón "Reabrir" aparece solo en un
      Resuelto; después de reabrir, el detalle muestra el sprint de resolución vacío y vuelven a
      habilitarse los campos de vínculo — depende de T056

### Implementación de la Historia 5

- [ ] T060 [US5] Agregar `resolved: [open]` al mapa de `backend/internal/defects/domain/status.go` y,
      en la función de mensajes de `backend/internal/defects/delivery/handler.go`, el caso de
      reapertura de lo que no está Resuelto ("Solo se reabre un defecto Resuelto.") — depende de T057,
      T058
- [ ] T061 [US5] Agregar la reapertura a `frontend/src/features/defects/transitions.ts` y el botón
      "Reabrir" a `DefectStatusActions` en
      `frontend/src/features/defects/components/defect-status-actions.tsx` — depende de T059, T060

**Checkpoint**: US1 a US5 funcionan; un defecto que vuelve a aparecer se reabre sin duplicar el
registro.

---

## Fase 7: Historia de Usuario 6 — Descartar un defecto que no corresponde (Prioridad: P3)

**Objetivo**: un integrante descarta un defecto Abierto o En progreso que resultó ser un duplicado o
no era un defecto; queda Descartado, definitivo y de solo lectura, visible en los listados.

**Rama**: `hu/007-us6-descartar-un-defecto-que-no-corresponde`

**Prueba independiente**: `PUT /api/v1/defects/{id}/status` con `discarded` sobre un defecto Abierto
devuelve `200` con `status: discarded` y `resolvedSprint: null`; cualquier transición o modificación
posterior devuelve `409`; resolver y descartar a la vez el mismo defecto En progreso aplica
exactamente una de las dos.

### Pruebas de la Historia 6 ⚠️

- [ ] T062 [P] [US6] Completar las pruebas unitarias **obligatorias** de
      `backend/internal/defects/domain/status_test.go`, cubriendo RN-12, RN-21 a RN-23 y SC-005:
      `AllowedTransitions(open)` = `[in_progress, discarded]`, `AllowedTransitions(in_progress)` =
      `[resolved, discarded]` y `AllowedTransitions(discarded)` = `[]`; descarte desde `open` e
      `in_progress` → `nil, nil` (nunca sprint de resolución); desde `resolved` y `discarded` →
      `InvalidTransitionError`; y una prueba de tabla con **las 16 combinaciones** de origen y
      destino: exactamente 5 válidas y 11 inválidas, incluidas las 4 salidas desde `discarded` —
      depende de T061
- [ ] T063 [US6] Pruebas de integración en `backend/internal/defects/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (007/US6)`: `TestUS6_DescarteDesdeAbierto` (conserva
      la detección, `resolvedSprint: null`), `TestUS6_DescarteDesdeEnProgreso`,
      `TestUS6_ListadoConDescartados` (aparece en el listado sin filtros con `status: discarded`),
      `TestUS6_FiltroPorDescartado` (y `status=open&status=in_progress&status=resolved` lo excluye),
      `TestUS6_HistoriaNoEliminable` (con la 003: `409` con el motivo `defects`; RN-19),
      `TestUS6_DetalleDeDescartado`, `TestUS6_DescarteDeResuelto` (`409` con "Solo se descarta un
      defecto Abierto o En progreso; para descartarlo, primero reabrilo."),
      `TestUS6_DescartadoDefinitivo` (subtests descartar, pasar a En progreso, resolver y reabrir →
      `409 invalid_transition` con "El defecto está Descartado, que es un estado definitivo.", y
      modificar → `409 defect_discarded`; SC-012) y `TestUS6_ProyectoFinalizado` — depende de T061
- [ ] T064 [US6] Agregar a `backend/internal/defects/delivery/defect_concurrency_test.go`
      `TestDefectTransition_Concurrency`, **sin** etiqueta: a la vez, sobre el mismo defecto
      `in_progress`, una resolución y un descarte de dos integrantes; en **10 de 10** repeticiones
      exactamente uno recibe `200` y el otro `409 invalid_transition` con el estado vigente, y el
      defecto nunca queda `resolved` después de `discarded` (SC-019); y la variante
      `TestDefectTransition_ConcurrentUpdateAndDiscard`: una modificación de la descripción y un
      descarte a la vez; si el descarte se aplicó primero, la modificación recibe `409
      defect_discarded` (FR-018, última oración) — depende de T063
- [ ] T065 [P] [US6] Pruebas de frontend en
      `frontend/src/features/defects/components/defect-status-actions.test.tsx`,
      `frontend/src/features/defects/components/defect-detail.test.tsx` y
      `frontend/src/features/defects/transitions.test.ts`, con
      `describe("007/US6 - Descartar un defecto que no corresponde")`: "Descartar" aparece en Abierto y
      En progreso y pide confirmación; un Descartado se muestra sin acciones ni campos editables;
      `transitions.ts` coincide con las 16 combinaciones del backend; el filtro de estado ofrece
      "Descartado" — depende de T061

### Implementación de la Historia 6

- [ ] T066 [US6] Agregar `discarded` a los destinos de `open` y de `in_progress` y `discarded: []` al
      mapa de `backend/internal/defects/domain/status.go`, y en la función de mensajes de
      `backend/internal/defects/delivery/handler.go` los casos de descarte de un Resuelto y de
      cualquier acción sobre un Descartado — depende de T062, T063
- [ ] T067 [US6] Agregar el descarte a `frontend/src/features/defects/transitions.ts`, el botón
      "Descartar" con confirmación a `DefectStatusActions` y la vista de solo lectura del Descartado a
      `DefectDetail` en `frontend/src/features/defects/components/` — depende de T065, T066

**Checkpoint**: las seis historias funcionan; un duplicado deja de contar sin borrar el registro.

---

## Fase 8: Polish (Cierre y asuntos transversales)

**Propósito**: pruebas transversales de seguridad y de invariantes, documentación viva, cobertura,
vulnerabilidades y verificación de las puertas de calidad.

**Rama**: `fase/007-polish`

- [ ] T068 Pruebas de integración de caracterización en
      `backend/internal/defects/delivery/handler_test.go`, **sin** etiqueta `007/USn`:
      `TestDefectEndpoints_SinSesion`, con un subtest por cada uno de los **seis** endpoints sin
      cabecera y con token adulterado → `401` y ningún cambio (SC-016);
      `TestDefectEndpoints_NotFoundIndistinguible`, que compara byte a byte el `404` ajeno e
      inexistente de proyecto (dos rutas), de historia (una ruta), de defecto (tres rutas), de
      historia y sprint en el cuerpo (alta, cambio de estado y modificación), y el `400` de filtro
      ajeno e inexistente (SC-017); `TestDefectLinks_Immutable`, que recorre en progreso, resolución,
      reapertura, segunda resolución y descarte, y compara historia y sprint de detección al
      principio y al final (SC-010); `TestDefectAuthor_RemovedMember`, que quita con la 002 a quien
      registró defectos y verifica que siguen todos, con su nombre, en el listado y el detalle, y que
      otro integrante los sigue trabajando (FR-055, SC-018); y una aserción de que ninguna respuesta
      contiene `password_hash` ni `passwordHash` (Principio VI). Caracterización: las reglas ya
      existen, así que no tiene RED; el commit lo justifica — depende de T067
- [ ] T069 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las **seis**
      operaciones, sus códigos de respuesta (todas declaran `500`), sus esquemas y los cinco `code` de
      `DefectConflictResponse`; corregir las anotaciones si difieren (Principio VIII) — depende de
      T068
- [ ] T070 [P] Actualizar `README.md` de la raíz en español: la sección introductoria menciona la
      gestión de defectos, y se agregan los seis endpoints, la sección "Defectos" del detalle de la
      historia y las páginas de defectos del proyecto, manteniendo el criterio de que alguien que no
      conoce el proyecto pueda ejecutarlo siguiendo el documento — depende de T068
- [ ] T071 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T072 [P] Ejecutar los comandos de cobertura de los dos lados —
      `go test -p 1 ./... -coverprofile=coverage.out` más `go tool cover -func=coverage.out` en
      `backend/`, y `npm run test:coverage` en `frontend/` — y revisar que `internal/defects/`,
      `frontend/src/features/defects/` y `frontend/src/utils/defect-labels.ts` estén cubiertos; correr
      además `go test -race -p 1 ./internal/defects/...` — depende de T068
- [ ] T073 Verificar la trazabilidad y las puertas de seguridad recorriendo `backend/internal/defects/`
      y `frontend/src/features/defects/`: que buscar `007/US1` a `007/US6` en el código de pruebas
      devuelva los **73** escenarios; que las **24** reglas RN-01 a RN-24 tengan prueba asociada según
      la tabla de plan.md (SC-020); que los **seis** endpoints tengan su prueba de `401` (SC-016); que
      los `404` sean indistinguibles (SC-017); que fuera de `defects` esta feature solo escriba
      `user_stories.ever_had_defects`; que no haya dependencias nuevas en `go.mod` ni en
      `package.json`; y que ninguna respuesta incluya `password_hash` — depende de T068
- [ ] T074 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluida la
      validación manual de 19 pasos, la prueba de concurrencia con `-count=10` y la verificación de que
      las pruebas corrieron contra `smye_test` en el puerto 5433 y no contra desarrollo — depende de
      T069, T070, T073

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Foundational**: parte de `main` con la 001 a la 005 y la fase fundacional de la 006
  integradas. **BLOQUEA** todas las historias.
- **Fases 2 a 7 (historias)**: todas dependen de la Fase 1, en orden de prioridad
  (P1 → P1 → P2 → P2 → P3 → P3).
- **Fase 8 Polish**: depende de que estén completas las seis historias.

### Dependencias entre historias

Las historias **no son todas independientes en su implementación**, aunque cada una se prueba sola:

- **US1 Registrar (P1)**: puede empezar después de la Fase 1. **Aporta** el entorno de pruebas
  (T011), el servicio con su firma definitiva, `FindView`, los DTO base, la composición en
  `cmd/app/main.go` y el puerto real de la 003, que usan todas las demás.
- **US2 Seguir y resolver (P1)**: necesita defectos de US1. **Aporta** `PlanTransition`,
  `RequireDefectAccess`, el endpoint de estado, el detalle y su página, que usan US4, US5 y US6.
- **US3 Consultar y filtrar (P2)**: necesita defectos de US1 y, para el filtro por sprint, la
  resolución de US2.
- **US4 Corregir (P2)**: necesita `RequireDefectAccess` y el detalle de US2; no depende de US3.
- **US5 Reabrir (P3)**: amplía la máquina de estados de US2.
- **US6 Descartar (P3)**: amplía la máquina de estados de US2 y US5; cierra la prueba de las 16
  combinaciones y la de concurrencia de SC-019.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T001 a T003 en paralelo; T004 después de T002; luego T005; T006 y T007 en secuencia.
- Dentro de cada historia: las pruebas unitarias de dominio, de validación, del middleware, del
  puerto y de frontend marcadas `[P]` en paralelo entre sí. Las de integración **no** son
  paralelizables entre historias porque las seis comparten `handler_test.go`, ni las de concurrencia
  porque comparten `defect_concurrency_test.go`.
- En las implementaciones, las funciones puras de dominio marcadas `[P]` (cada una en su archivo) en
  paralelo; repositorio, servicio y `delivery` en secuencia, porque cada historia toca
  `defect_repository.go`, `defect_service.go`, `handler.go`, `dto.go` y `routes.go`.
- Fase 8: T070 y T072 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las tareas de pruebas marcadas [P] pueden escribirse a la vez:
T008  backend/internal/defects/domain/severity_test.go
T009  backend/internal/defects/domain/rules_test.go
T010  backend/internal/defects/delivery/validation_test.go
T012  backend/internal/backlog/repository/defect_records_test.go + story_repository_test.go
T013  frontend validation.test.ts + defect-labels.test.ts + defect-form.test.tsx

# T011 (handler_test.go + testenv_test.go) va aparte: lo comparten las seis historias.

# En la implementación, estas no se pisan:
T014  backend/internal/defects/domain/severity.go
T015  backend/internal/defects/domain/rules.go
T023  frontend/src/types/defect.ts + utils/defect-labels.ts + features/defects/validation.ts
```

## Ejemplo de paralelismo: Historia de Usuario 2

```text
# Pruebas en archivos distintos:
T025  backend/internal/defects/domain/status_test.go
T026  backend/internal/defects/delivery/validation_test.go
T027  backend/internal/defects/delivery/middleware_test.go
T030  frontend transitions.test.ts + defect-status-actions.test.tsx
```

## Ejemplo de paralelismo: Historia de Usuario 4

```text
# Pruebas en archivos distintos:
T046  backend/internal/defects/domain/rules_test.go
T047  backend/internal/defects/delivery/validation_test.go
T050  frontend/src/features/defects/components/defect-edit-form.test.tsx
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar la Fase 1 (Foundational) — **CRÍTICO**: bloquea todas las historias.
2. Completar la Fase 2 (US1 Registrar un defecto).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con los pasos 0 a 3 y 15 de la guía
   de validación.
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Foundational → base lista (una fase técnica, un pull request).
2. US1 Registrar → probar sola → entregar (**MVP**: los defectos entran al sistema con su sprint de
   detección validado; la 003 ya bloquea eliminar historias con defectos).
3. US2 Seguir y resolver → probar sola → entregar (el defecto tiene ciclo de vida y sprint de
   resolución: la 008 ya puede contar detectados y resueltos).
4. US3 Consultar y filtrar → probar sola → entregar (lo registrado se encuentra).
5. US4 Corregir → probar sola → entregar (el dato es confiable).
6. US5 Reabrir → probar sola → entregar.
7. US6 Descartar → probar sola → entregar (los duplicados dejan de contar).
8. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Foundational en conjunto.
2. US1 y US2 en secuencia: aportan el entorno de pruebas, el servicio, el middleware de defecto y la
   máquina de estados que usan todas.
3. Terminada US2, US3 y US4 pueden avanzar en paralelo en el backend coordinando
   `defect_repository.go`, `defect_service.go`, `handler.go`, `dto.go`, `routes.go`,
   `validation.go` y `handler_test.go`; el frontend de cada una, en archivos distintos.
4. US5 y US6 al final, en ese orden, porque las dos amplían `status.go` y `transitions.ts`.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `007/US1` a `007/US6` en el código de
  pruebas devuelve la trazabilidad completa de los 73 escenarios exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 007/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b), con el ámbito `defectos`. Los commits los registra **una
  persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni de npm ni variables de entorno. Si alguna
  tarea parece necesitar una, detenerse y pedir autorización (Principio IV).
- **Código de otras features que esta feature modifica**, todo señalado en el plan: `UserStory` de
  la 003 (T003), el puerto `DefectRecords` y el `Delete` de historias de la 003 (T022), la migración
  y el ayudante de pruebas de la plataforma (T006, T007), el detalle de la historia de la 003 (T045)
  y el resumen del proyecto de la 002 (T045).
- **Ayudante `forceDefectStatus`**: solo prepara datos en US3-1, US4-9 y US4-14 mientras las
  transiciones de US5 y US6 no existen. Una vez completada US6, puede reemplazarse por las
  transiciones reales en el refactor de la Fase 8, sin cambiar lo que verifican esas pruebas.
- **Puntos del plan que esperan confirmación del desarrollador** y no tienen tarea: aceptar un
  vínculo reenviado con el mismo valor fuera de Abierto (research.md sección 11; afecta T047, T048 y
  T053 si se cambia), y la definición de defectos "abiertos" que hará la 008.
- **Criterios de éxito no automatizables.** SC-001 (registrar en menos de 1 minuto y 5 pasos, 95 % al
  primer intento), SC-002 (Críticos abiertos en menos de 30 segundos) y SC-003 (defectos de una
  historia desde la propia historia) son métricas de experiencia de uso: las pruebas de componentes
  verifican la estructura, y la medición se hace a mano siguiendo la sección 4 de
  [quickstart.md](./quickstart.md).
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: Swagger se actualiza en la misma tarea que cada endpoint, pero la descripción de la
  feature y sus endpoints en el `README.md` se agrega una sola vez en la Fase 8 (T070). Entre el
  merge de una historia y la Fase 8, el README no lista los endpoints de defectos ya integrados. Se
  acepta porque Swagger es la referencia actualizada de la API en cada merge. Es el mismo criterio
  que aplicaron la 001 a la 006.

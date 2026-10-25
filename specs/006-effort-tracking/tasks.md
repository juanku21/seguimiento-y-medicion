---

description: "Lista de tareas de la feature 006 — Registro de Esfuerzo"
---

# Tareas: Registro de Esfuerzo

**Entrada**: documentos de diseño de `specs/006-effort-tracking/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) a
[`specs/005-planning-poker`](../005-planning-poker/spec.md) están integradas en `main`. Esta feature
usa del módulo `internal/backlog/` el modelo `UserStory`, `StoryStatus`, el middleware
`RequireStoryAccess` y el puerto `EffortRecords`; del módulo `internal/sprints/` los modelos `Sprint`
y `SprintStory`, `SprintStatus`, `StoryOutcome`, el puerto `SprintEffort` y `ErrSprintHasEffort`; del
módulo `internal/projects/` el modelo `Project`, `ProjectStatus` y el subgrupo
`/api/v1/projects/:projectId` con `RequireMembership`; de la 001 el middleware de sesión,
`authdelivery.CurrentUserID` y los esquemas de error; además el límite global de 1 MiB, el patrón de
reloj y zona inyectados y el ayudante `testsupport.OpenDatabase`. **No hay fase de Setup**: no se
agregan dependencias ni servicios; la única variable nueva (`EFFORT_GRACE_PERIOD_HOURS`) es opcional
y se agrega en US1 con su prueba, porque su validación es lógica (research.md sección 8).

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para la
lectura de horas, actividad, fecha y filtros, el tope diario, el anclaje al sprint, el plazo de
gracia, la autoría, la comparación con su redondeo, el formato de 2 decimales, la configuración del
plazo y las reglas de negocio RN-01 a RN-26.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/effort/{domain,repository,service,delivery}/`; se tocan
  de forma puntual `backend/cmd/app/main.go`, `backend/internal/platform/config/config.go`,
  `backend/.env.example`, `backend/internal/platform/database/postgres.go`,
  `backend/internal/platform/testsupport/database.go`,
  `backend/internal/backlog/domain/user_story.go`,
  `backend/internal/backlog/repository/effort_records.go`,
  `backend/internal/sprints/repository/sprint_effort.go` y
  `backend/internal/sprints/repository/sprint_repository.go`
- **Frontend**: `frontend/src/app/projects/[projectId]/effort/page.tsx`,
  `frontend/src/features/effort/`, `frontend/src/types/effort.ts`,
  `frontend/src/utils/effort-format.ts`; se tocan
  `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx` (o
  `frontend/src/features/backlog/components/story-detail-view.tsx`, donde se compone el detalle) y
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

## Cómo se preparan los escenarios que dependen de otras features o del tiempo

- Las pruebas de integración usan un entorno `newEffortTestEnv` (T015) que compone el router como
  `cmd/app` (sesión de la 001, rutas de la 002, la 003 y la 004, y las de esfuerzo) sobre
  `httptest`, con **un único reloj falso** compartido por el servicio de sprints y el de esfuerzo,
  una zona horaria inyectada y un plazo de gracia de **48 horas** salvo que la prueba indique otro.
  Ninguna prueba espera tiempo real.
- Las historias se crean y se estiman con los **endpoints reales de la 003**; los sprints se crean,
  se comprometen, se inician, se cierran y se les quitan historias con los de la 004 (el avance a
  `completed` también); el proyecto Finalizado, el cambio de factor y la baja y reincorporación de
  integrantes, con los de la 002.
- El plazo de gracia se ejercita cerrando el sprint con el reloj fijado (por ejemplo, viernes
  2026-10-23 18:00 en la zona inyectada) y avanzando el reloj hasta cada frontera.
- Las pruebas sin sesión, la comparación de los `404`, la de autoría por cada forma de indicarla, la
  suma de 100 cuartos de hora, las de concurrencia, la del ancla inmutable y la de las fronteras del
  plazo **no** llevan la etiqueta `006/USn`, porque no derivan de un escenario de la spec y no deben
  alterar el conteo de 74.

---

## Fase 1: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el dominio del módulo nuevo, su contrato de repositorio, la migración de la tabla y de
la columna nueva, y el aislamiento de las pruebas. Todas las tareas son **técnicas**: declaran
estructuras y contratos, sin lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/006-foundational`

- [X] T001 [P] Declarar la entidad `EffortEntry` en `backend/internal/effort/domain/entry.go` con los
      campos de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`;
      `StoryID uuid NOT NULL` con FK a `user_stories.id` (campo `Story *backlogdomain.UserStory` solo
      para la FK, nunca precargado); `UserID uuid NOT NULL` con FK a `users.id` (campo
      `User *authdomain.User`); `SprintID uuid` **anulable** (`*uuid.UUID`) con FK a `sprints.id`
      (campo `Sprint *sprintsdomain.Sprint`); `EntryDate date NOT NULL` (medianoche UTC en Go);
      `Activity varchar(300) NOT NULL`; `HoursHundredths integer NOT NULL` ("Centésimas de hora: 25 a
      2400, múltiplo de 25"); `CreatedAt` y `UpdatedAt` `timestamptz NOT NULL`. Índices
      `idx_effort_entries_story_order (story_id, entry_date, created_at)`,
      `idx_effort_entries_user_date (user_id, entry_date)` e `idx_effort_entries_sprint (sprint_id)`
      con etiquetas de GORM. Claves foráneas **sin `ON DELETE CASCADE`**. **Sin** columnas
      `project_id`, de borrado lógico ni de "definitivo"
- [X] T002 [P] Agregar a `UserStory` en `backend/internal/backlog/domain/user_story.go` el campo
      `EverHadEffort bool` con la etiqueta `gorm:"type:boolean;not null;default:false"` (columna
      `ever_had_effort`), y extender el comentario del modelo: "pasa a true con el primer registro de
      esfuerzo y nunca vuelve a false, aunque se eliminen todos los registros (RC-05, RN-22); solo la
      escribe la feature de esfuerzo", igual que `EverCommitted`
- [X] T003 [P] Declarar en `backend/internal/effort/domain/anchor.go` los tipos `OpenCommitment`
      (`SprintID`) y `ClosedCommitment` (`SprintID`, `ClosedAt time.Time`,
      `Outcome sprintsdomain.StoryOutcome`); en `backend/internal/effort/domain/grace.go` el tipo
      `AnchoredSprint` (`ID`, `Name`, `Status sprintsdomain.SprintStatus`, `ClosedAt *time.Time`); y
      en `backend/internal/effort/domain/daily_cap.go` las constantes `MaxDailyHundredths = 2400`,
      `MinEntryHundredths = 25`, `MaxEntryHundredths = 2400` y `HoursStepHundredths = 25`. Las
      funciones (`ResolveAnchor`, `GraceOpen`, `EnsureEntryWritable`, `CheckDailyCap`) se
      implementan en las historias
- [X] T004 [P] Declarar los errores de dominio en `backend/internal/effort/domain/errors.go` con la
      tabla de data-model.md sección 7: `ErrEntryNotFound`, `ErrEffortReadOnly`,
      `ErrSprintFinalized`, `ErrEffortFinalized`, `ErrNotEntryAuthor`, `ErrEntryDateInFuture` e
      `ErrInvalidFilter` (con el nombre del parámetro: tipo `InvalidFilterError{Field string}`); y
      los tipos con datos `DailyCapExceededError{AvailableHundredths int}` y
      `EntryDateBeforeProjectStartError{MinDate time.Time}`
- [X] T005 Declarar en `backend/internal/effort/domain/repository.go` la interfaz `EffortRepository`
      con **exactamente** las nueve operaciones de data-model.md sección 9 (`FindAccessible`,
      `Create`, `Update`, `Delete`, `ListForStory`, `ListForProject`, `ValidateFilter`,
      `HasProjectEffort`, `ComparisonRows`) con sus firmas, y los tipos de lectura de la sección 8:
      `EntryAccess`, `EntryView`, `UserRef` (solo `ID` y `FullName`), `StoryRef`, `MemberEffort`,
      `StoryEffort`, `EntryFilter`, `ComparisonRow`, `EntryInput` y `EntryPatch`. Las escrituras
      reciben el instante, la zona y el plazo como parámetros — depende de T001, T003, T004
- [X] T006 Agregar `EffortEntry` al `AutoMigrate` de `backend/internal/platform/database/postgres.go`
      **después** de `SprintStory` (apunta a `user_stories`, `users` y `sprints`) y actualizar el
      comentario del orden y de la columna nueva `ever_had_effort` de `UserStory` — depende de T001,
      T002
- [X] T007 Extender el vaciado del ayudante de pruebas en
      `backend/internal/platform/testsupport/database.go` para incluir `effort_entries` en el
      `TRUNCATE ... CASCADE`, manteniendo la verificación de que la base es `smye_test` y el lock de
      la base de test — depende de T006

**Checkpoint**: `effort_entries` migrada en la base de test con sus tres claves foráneas `NO ACTION`
(quickstart.md sección 2), `user_stories.ever_had_effort` creada, el dominio y el contrato
declarados, y las pruebas aisladas. Las historias pueden empezar.

---

## Fase 2: Historia de Usuario 1 — Registrar el esfuerzo trabajado (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante registra horas sobre una historia con fecha y actividad; el registro
queda atribuido a él, anclado al sprint correcto y sujeto a las validaciones de horas, fecha,
actividad y tope diario.

**Rama**: `hu/006-us1-registrar-el-esfuerzo-trabajado`

**Prueba independiente**: `POST /api/v1/stories/{storyId}/effort-entries` con 6,5 h de hoy sobre una
historia del sprint Activo devuelve `201` con `hours: 6.50`, el autor igual a la sesión y el sprint
Activo; 1,3 h devuelve `400` en `hours`; 5 h con 20 ya cargadas devuelve `409 daily_cap_exceeded`
con `availableHours: 4.00`.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T008 [P] [US1] Pruebas unitarias **obligatorias** de `EFFORT_GRACE_PERIOD_HOURS` en
      `backend/internal/platform/config/config_test.go`, cubriendo RC-11: ausente o vacía → 48 horas;
      `48` → 48; `1` → 1; `0`, `-1`, `48h`, `1.5` y `abc` → error que impide el arranque y nombra la
      variable, acumulado con los demás problemas como `APP_TIMEZONE` — depende de T007
- [X] T009 [P] [US1] Pruebas unitarias **obligatorias** de la validación del alta en
      `backend/internal/effort/delivery/validation_test.go`, cubriendo RN-02, RN-03 y SC-006:
      `ParseHours(json.RawMessage)` con `0.25` → 25, `6.5` → 650, `6.50` → 650, `24` → 2400, y rechazos
      `0`, `-1`, `24.25`, `25` (mensaje de rango "Las horas deben ser mayores que 0 y como máximo
      24."), `1.3`, `1.1`, `0.10` (mensaje "Las horas se cargan en múltiplos de 0,25 (un cuarto de
      hora)."), `6.555`, `1e1`, el número entre comillas `"6.5"`, `"abc"`, `null`, `true` y ausencia
      (mensaje "Las horas son obligatorias y deben ser un número."); actividad
      vacía, `"   "` (mensaje "La actividad es obligatoria."), de exactamente **300 runas** con
      acentos aceptada y recortada, de **301** rechazada ("La actividad admite como máximo 300
      caracteres."); `entryDate` ausente, `2026-02-30` y `24/10/2026` rechazadas con el mensaje de
      formato `AAAA-MM-DD`; varios campos inválidos devueltos **todos a la vez** — depende de T007
- [X] T010 [P] [US1] Pruebas unitarias **obligatorias** del tipo de 2 decimales en
      `backend/internal/effort/delivery/dto_test.go`, cubriendo SC-015 y SC-022: 650 → `6.50`, 2000
      → `20.00`, −2000 → `-20.00`, 25 → `0.25`, 0 → `0.00`, −5 → `-0.05`, 1310 → `13.10`; un
      puntero nulo se serializa `null`; el resultado es un número JSON válido (se vuelve a decodificar
      con `json.Unmarshal`) — depende de T007
- [X] T011 [P] [US1] Pruebas unitarias **obligatorias** de
      `ValidateEntryDate(date, projectStart, today time.Time) error` en
      `backend/internal/effort/domain/entry_date_test.go`, cubriendo RN-05 y SC-007: hoy → ok;
      mañana → `ErrEntryDateInFuture`; día de inicio → ok; día anterior al inicio →
      `EntryDateBeforeProjectStartError` con `MinDate` igual al inicio; proyecto con inicio futuro →
      toda fecha rechazada (pasado por inicio, futuro por hoy) — depende de T007
- [X] T012 [P] [US1] Pruebas unitarias **obligatorias** de
      `CheckDailyCap(alreadyLogged, requested int) error` en
      `backend/internal/effort/domain/daily_cap_test.go`, cubriendo RN-04: 0 + 2400 → ok; 2000 + 400
      → ok; 2000 + 500 → `DailyCapExceededError{AvailableHundredths: 400}`; 2400 + 25 →
      `AvailableHundredths: 0`; un `alreadyLogged` mayor que 2400 nunca da disponible negativo —
      depende de T007
- [X] T013 [P] [US1] Pruebas unitarias **obligatorias** de
      `ResolveAnchor(open *OpenCommitment, lastClosed *ClosedCommitment, now time.Time, grace time.Duration) (*uuid.UUID, error)`
      en `backend/internal/effort/domain/anchor_test.go` con la tabla completa de data-model.md
      sección 6.1, cubriendo RN-07, RN-24 y RN-25: con `open` → su sprint, aunque haya `lastClosed`
      en plazo (recomprometida); sin `open` y `lastClosed` en plazo `completed` → su sprint; en plazo
      `not_completed` → su sprint; vencido `completed` → `ErrSprintFinalized`; vencido
      `not_completed` → `nil` sin error; sin compromisos → `nil` sin error. Fronteras con cierre
      viernes 18:00 y 48 h: domingo 17:59 en plazo, domingo **18:00** vencido, 18:01 vencido —
      depende de T007
- [X] T014 [P] [US1] Pruebas unitarias **obligatorias** de
      `GraceOpen(closedAt, now time.Time, grace time.Duration) bool` en
      `backend/internal/effort/domain/grace_test.go`, cubriendo RN-23: justo después del cierre →
      `true`; `closedAt + grace − 1ns` → `true`; `closedAt + grace` → `false` ("el plazo se cuenta
      cumplido al alcanzarse"); después → `false` — depende de T007
- [X] T015 [US1] Pruebas de integración de la historia en
      `backend/internal/effort/delivery/handler_test.go`, creando en
      `backend/internal/effort/delivery/testenv_test.go` el ayudante `newEffortTestEnv` (router como
      `cmd/app` con las rutas de la 001 a la 004 y las de esfuerzo, reloj falso avanzable compartido
      por los servicios de sprints y de esfuerzo, zona inyectada, plazo configurable con defecto 48 h,
      y ayudantes para crear cuentas, proyectos, integrantes, historias, estimaciones y sprints con
      los endpoints reales). Una función por escenario de spec.md precedida por
      `// Escenario: <nombre del escenario> (006/US1)`: `TestUS1_RegistroEnSprintActivo` (`201`,
      autor = sesión, sprint Activo, total de la historia +6,50 leído en la base),
      `TestUS1_HistoriaSoloEnBacklog` (`sprint: null`), `TestUS1_SprintPlanificado`,
      `TestUS1_HistoriaCompletadaEnSprintActivo`, `TestUS1_SegundoRegistroDelDia` (dos filas, 5 h de
      la persona en la fecha), `TestUS1_HistoriaSinEstimar` (con subtests `pending`, `in_progress` y
      `completed` para RN-13), `TestUS1_CompletadaDentroDelPlazo` (cierre viernes 18:00, alta domingo
      10:00 → sprint Cerrado), `TestUS1_DevueltaAlBacklogDentroDelPlazo` (con un subtest en el que la
      historia no se completa en el sprint A, se recompromete en B, B se cierra dentro del plazo de A
      y el alta queda anclada a B, el de `closed_at` más reciente; FR-014), `TestUS1_LimiteDelPlazo`
      (subtests 17:59 aceptado y 18:01 rechazado), `TestUS1_VeinticuatroHorasExactas`,
      `TestUS1_CuartoDeHora`, `TestUS1_ActividadDeTrescientosCaracteres` (300 `201`, 301 `400`),
      `TestUS1_DiaDeInicioDelProyecto`, `TestUS1_HoySegunZonaDelSistema` (reloj a las 23:30 de la
      zona del sistema, que en UTC ya es el día siguiente: la fecha local de hoy se acepta),
      `TestUS1_TopeEntreProyectos` (20 h en dos proyectos + 4 h en un tercero → `201`),
      `TestUS1_TopeDiarioSuperado` (`409`, `code: daily_cap_exceeded`, `availableHours: 4.00`, el
      mensaje **no** contiene el nombre de ningún proyecto, nada creado),
      `TestUS1_HorasFueraDeRango` (subtests 0, −1 y 24,25), `TestUS1_HorasNoMultiplo`,
      `TestUS1_ActividadVacia` (subtests ausente y espacios), `TestUS1_FechaFutura`
      (`400` en `entryDate`), `TestUS1_FechaAnteriorAlInicio`, `TestUS1_SprintDefinitivo` (cierre
      hace cinco días, `409 effort_finalized`, nada creado), `TestUS1_ProyectoFinalizado` (con el
      plazo de gracia del último sprint vigente: `409 read_only`), `TestUS1_NoIntegrante` (`404` con
      el cuerpo de la 003) y `TestUS1_AutorAjenoIgnorado` (`userId` de otra persona en el cuerpo →
      registro a nombre de la sesión). En cada rechazo se verifica que la cantidad de filas de
      `effort_entries` no cambió — depende de T007
- [X] T016 [US1] Prueba de concurrencia `TestEffortDailyCap_Concurrency` en
      `backend/internal/effort/delivery/effort_concurrency_test.go`, **sin** etiqueta `006/USn`: la
      misma cuenta envía a la vez **10** altas de 3 h para la misma fecha, repartidas en dos
      proyectos; exactamente 8 reciben `201` y 2 `409 daily_cap_exceeded`, ninguna `500`, y la suma
      de la fecha en la base es 24,00 (SC-005). Repetida varias veces para que el intercalado sea
      real — depende de T015
- [X] T017 [P] [US1] Pruebas de integración de los puertos reemplazados, contra `smye_test`: en
      `backend/internal/backlog/repository/effort_records_test.go`, `HasEffort` es `false` sin
      registros, `true` después de insertar un registro con `ever_had_effort = true`, y **sigue**
      `true` después de borrar todas las filas de `effort_entries` (RN-22); en
      `backend/internal/sprints/repository/sprint_effort_test.go`, `HasEffort` del sprint es `false`
      sin registros y `true` con uno anclado; y en el mismo archivo, `SprintRepository.Delete` sobre
      un sprint `planned` con un registro anclado insertado directamente (simula la carrera de
      research.md sección 10) devuelve `ErrSprintHasEffort` y no `500`. Además, que el `DELETE` de una
      historia, de un sprint y de un usuario con registros falla en la base (RN-21) — depende de T007
- [X] T018 [P] [US1] Pruebas de frontend en `frontend/src/features/effort/validation.test.ts`
      (horas, actividad y fecha con las mismas reglas de forma que el backend),
      `frontend/src/utils/effort-format.test.ts` (`6.5` → `6,5 h`, `0.25` → `0,25 h`, `20` → `+20,00 %`,
      `-20` → `−20,00 %`, `null` → `No calculable`) y
      `frontend/src/features/effort/components/effort-entry-form.test.tsx`, con
      `describe("006/US1 - Registrar el esfuerzo trabajado")` y un `it` por escenario con
      comportamiento visible: la fecha de hoy precargada, los mensajes de campo del `400` junto a
      cada campo, el mensaje del tope con las horas disponibles, el de sprint definitivo y el de
      proyecto de solo lectura, y que el formulario **no** tiene campo de autor

### Implementación de la Historia 1

- [X] T019 [US1] Implementar `EFFORT_GRACE_PERIOD_HOURS` en
      `backend/internal/platform/config/config.go` (campo `EffortGracePeriod time.Duration`, defecto
      48 h si falta o está vacía, entero `≥ 1` o error acumulado) y agregar a `backend/.env.example`
      la línea `EFFORT_GRACE_PERIOD_HOURS=48` con un comentario en español: "Plazo de gracia, en
      horas enteras, para registrar y corregir esfuerzo después de cerrar un sprint (por defecto
      48)"; agregar la misma clave con el mismo valor a `backend/.env` y `backend/.env.test` (copia
      de trabajo, no versionados), para que cada `.env` tenga las mismas claves que su
      `.env.example` (Principio VI); y agregar en `README.md` de la raíz, en la sección de variables
      de entorno del backend, la fila de `EFFORT_GRACE_PERIOD_HOURS` con su defecto y su significado
      (Principio VIII: los pasos para ejecutar cambian en este mismo cambio) — depende de T008
- [X] T020 [P] [US1] Implementar el tipo de 2 decimales en
      `backend/internal/effort/delivery/dto.go` (por ejemplo `hundredths int` con `MarshalJSON` que
      escribe `%d.%02d` con signo) — depende de T010
- [X] T021 [P] [US1] Implementar `ValidateEntryDate` en
      `backend/internal/effort/domain/entry_date.go` — depende de T011
- [X] T022 [P] [US1] Implementar `CheckDailyCap` en `backend/internal/effort/domain/daily_cap.go` —
      depende de T012
- [X] T023 [US1] Implementar `ResolveAnchor` en `backend/internal/effort/domain/anchor.go`,
      usando `GraceOpen` para la vigencia — depende de T013, T024
- [X] T024 [P] [US1] Implementar `GraceOpen` en `backend/internal/effort/domain/grace.go` — depende
      de T014
- [X] T025 [US1] Implementar en `backend/internal/effort/delivery/validation.go` `ParseHours`, la
      validación de actividad (recorte, 1 a 300 runas) y de `entryDate` (`AAAA-MM-DD`, fecha real, a
      medianoche UTC) y `ValidateEntryInput`, con los mensajes como constantes nombradas en un único
      lugar y todos los campos inválidos a la vez (Principio VII) — depende de T009
- [X] T026 [US1] Implementar `Create` en `backend/internal/effort/repository/effort_repository.go`
      con el orden de bloqueos de research.md sección 10, en una transacción: (1) historia unida a su
      proyecto con `clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "projects"}}`,
      `finished` → `ErrEffortReadOnly` **antes** de evaluar el plazo; (2) `ValidateEntryDate` con
      `today` en la zona recibida; (3) compromiso abierto (`sprint_stories.outcome IS NULL`) y último
      congelado por `sprints.closed_at DESC` con su `outcome`, **sin bloqueo**, y `ResolveAnchor`; (4)
      fila del autor en `users` con `clause.Locking{Strength: "NO KEY UPDATE"}`, suma de
      `hours_hundredths` del autor en la fecha en **todos** los proyectos y `CheckDailyCap`; (5)
      inserción; (6) `UpdateColumn("ever_had_effort", true)` con `WHERE ever_had_effort = false`.
      Devuelve el `EntryView` con autor (`users.id` y `users.full_name`, **nunca** `password_hash`),
      historia y sprint — depende de T005, T015, T021, T022, T023
- [ ] T027 [US1] Implementar en `backend/internal/effort/service/effort_service.go` el constructor
      `NewEffortService(repo EffortRepository, location *time.Location, grace time.Duration, now func() time.Time)`,
      con la firma definitiva para que no cambie en las historias siguientes, y `Create(ctx, access,
      actorID, input)`, que toma el autor **siempre** del titular de la sesión (FR-002) y el instante
      del reloj una sola vez — depende de T026
- [ ] T028 [US1] Declarar en `backend/internal/effort/delivery/dto.go` los esquemas de
      contracts/openapi.yaml `EffortEntryInput` (`entryDate`, `activity`, `hours` como
      `json.RawMessage` con `swaggertype:"number"`, para rechazar un número entre comillas; **sin**
      campo de autor), `EffortEntry` (con `definitive`), `UserRef`,
      `StoryRef`, `SprintRef` y `EffortConflictResponse` (`code`, `error`, `availableHours`),
      reutilizando `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de la 001. `sprint` y
      `availableHours` se serializan `null` cuando no aplican — depende de T020, T027
- [ ] T029 [US1] Implementar el manejador `createEffortEntry` en
      `backend/internal/effort/delivery/handler.go` con anotaciones `swag` en español, y la
      traducción de errores: `ErrEntryDateInFuture` y `EntryDateBeforeProjectStartError` → `400` con
      `field: entryDate` ("No se registran horas en una fecha futura." y "La fecha no puede ser
      anterior al inicio del proyecto (AAAA-MM-DD)."); `ErrEffortReadOnly` → `409 read_only`;
      `ErrSprintFinalized` → `409 effort_finalized`; `DailyCapExceededError` → `409
      daily_cap_exceeded` con "Superarías las 24 horas registradas para ese día. Te quedan N horas
      disponibles." y `availableHours`; cualquier otro → `500` genérico registrado con `slog` —
      depende de T025, T028
- [ ] T030 [US1] Crear en `backend/internal/effort/delivery/routes.go`
      `RegisterStoryRoutes(router, requireSession, requireStoryAccess, handler)` con
      `POST /api/v1/stories/:storyId/effort-entries`, sin importar `backlog/delivery`; y componer el
      módulo en `backend/cmd/app/main.go`: repositorio, servicio con `cfg.Location`,
      `cfg.EffortGracePeriod` y `time.Now`, `RequireStoryAccess` de la 003 sobre la misma instancia
      del repositorio de historias, más el `@description` general de la API para que mencione el
      registro de esfuerzo — depende de T019, T029
- [ ] T031 [US1] Reemplazar los puertos de la 003 y la 004: en
      `backend/internal/backlog/repository/effort_records.go`, `NewEffortRecords(db *gorm.DB)` y
      `HasEffort` que lee `user_stories.ever_had_effort`; en
      `backend/internal/sprints/repository/sprint_effort.go`, `NewSprintEffort(db *gorm.DB)` y
      `HasEffort` con `EXISTS` sobre `effort_entries.sprint_id`; en
      `backend/internal/sprints/repository/sprint_repository.go`, `Delete` traduce
      `gorm.ErrForeignKeyViolated` del `DELETE` del sprint a `domain.ErrSprintHasEffort`; actualizar
      sus comentarios (ya no son provisorios) y pasarles `db` en `backend/cmd/app/main.go`. Ni
      `backlog/service` ni `sprints/service` cambian. Verificación de punta a punta en
      `backend/internal/effort/delivery/handler_test.go`, **sin** etiqueta `006/USn`:
      `TestEffortPorts_BloqueanEliminaciones`, que con los endpoints reales registra esfuerzo en una
      historia de un sprint `planned`, y verifica que `DELETE /api/v1/sprints/{id}` responde `409`
      (RC-07) y que, después de borrar con la 006 todos los registros de la historia,
      `DELETE /api/v1/stories/{id}` sigue respondiendo `409` con el motivo `effort` (RC-05, RN-22).
      Es una prueba de integración de los puertos: se escribe primero y falla con los puertos
      provisorios — depende de T017, T030
- [ ] T032 [P] [US1] Declarar los tipos de petición y respuesta en `frontend/src/types/effort.ts`
      (sin `any`; "no calculable" como `number | null`), implementar
      `frontend/src/utils/effort-format.ts` y `frontend/src/features/effort/validation.ts` — depende
      de T018
- [ ] T033 [US1] Implementar `createEffortEntry` en `frontend/src/features/effort/api.ts`,
      `EffortEntryForm` en `frontend/src/features/effort/components/effort-entry-form.tsx` (fecha
      precargada con hoy, actividad, horas con paso 0,25; tres pasos: escribir la actividad, escribir
      las horas, confirmar) y una primera versión de
      `frontend/src/features/effort/components/story-effort-section.tsx` con el formulario, compuesta
      en el detalle de la historia
      (`frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx` o
      `frontend/src/features/backlog/components/story-detail-view.tsx`). Antes, leer la documentación
      de la versión instalada en `frontend/node_modules/next/dist/docs/` — depende de T018, T029, T032

**Checkpoint**: US1 funciona sola: se registra esfuerzo desde el detalle de la historia, con el
sprint correcto, el tope invariante y los puertos de la 003 y la 004 reales.

---

## Fase 3: Historia de Usuario 2 — Consultar el esfuerzo de una historia (Prioridad: P1)

**Objetivo**: un integrante ve en el detalle de la historia el total de horas, el desglose por
persona y la lista de registros con su sprint.

**Rama**: `hu/006-us2-consultar-el-esfuerzo-de-una-historia`

**Prueba independiente**: con 10 h de una persona en tres registros y 6 h de otra en dos,
`GET /api/v1/stories/{storyId}/effort` devuelve `totalHours: 16.00`, dos líneas en `byMember` y cinco
registros en orden de fecha descendente.

### Pruebas de la Historia 2 ⚠️

- [ ] T034 [US2] Pruebas de integración en `backend/internal/effort/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (006/US2)`: `TestUS2_TotalYDesglose`,
      `TestUS2_RegistrosDeVariosSprints` (dos sprints y uno sin sprint, cada registro con su sprint y
      el total los incluye a todos), `TestUS2_ExIntegrante` (baja con la 002: sus registros siguen,
      con su nombre y en el total), `TestUS2_HistoriaSinRegistros` (`totalHours: 0.00`, `byMember:
      []` y `entries: []`, nunca `null`), `TestUS2_CuartoDeHora` (`0.25`), `TestUS2_UnaLineaPorPersona`
      (tres fechas → una línea en el desglose y tres registros), `TestUS2_NoIntegrante` (`404`, ningún
      dato) y `TestUS2_HistoriaInexistente`. Orden verificado: `entry_date DESC, created_at ASC, id
      ASC`; desglose por `full_name ASC, user_id ASC` — depende de T033
- [ ] T035 [P] [US2] Pruebas de frontend en
      `frontend/src/features/effort/components/effort-breakdown.test.tsx` y
      `frontend/src/features/effort/components/effort-entry-list.test.tsx`, con
      `describe("006/US2 - Consultar el esfuerzo de una historia")`: total y una línea por persona;
      cada registro con fecha, autor, actividad, horas y sprint o "Sin sprint"; el aviso "Todavía no
      hay esfuerzo registrado." con la lista vacía — depende de T033

### Implementación de la Historia 2

- [ ] T036 [US2] Implementar `ListForStory` en
      `backend/internal/effort/repository/effort_repository.go`: una consulta de `effort_entries`
      unida a `users` (solo `id` y `full_name`) y con `LEFT JOIN` a `sprints` (`id`, `name`, `status`,
      `closed_at`), en el orden de FR-035 — depende de T034
- [ ] T037 [US2] Implementar `StoryEffort(ctx, storyID)` en
      `backend/internal/effort/service/effort_service.go`: total y desglose agregados en Go sobre la
      lista ya leída (research.md sección 11), y `definitive` de cada registro como "sprint `closed` y
      `!GraceOpen`" con el reloj del servicio — depende de T036
- [ ] T038 [US2] Declarar `StoryEffort` y `MemberEffort` en `backend/internal/effort/delivery/dto.go`,
      implementar el manejador `getStoryEffort` con anotaciones `swag` en
      `backend/internal/effort/delivery/handler.go` y registrar
      `GET /api/v1/stories/:storyId/effort` en `RegisterStoryRoutes` de
      `backend/internal/effort/delivery/routes.go`. Listas vacías como `[]` — depende de T037
- [ ] T039 [US2] Implementar `getStoryEffort` en `frontend/src/features/effort/api.ts`,
      `EffortBreakdown` en `frontend/src/features/effort/components/effort-breakdown.tsx`,
      `EffortEntryList` en `frontend/src/features/effort/components/effort-entry-list.tsx`, y
      completar `StoryEffortSection` para mostrar total, desglose y lista y recargarlos después de
      cada alta (SC-002: todo en una sola pantalla) — depende de T035, T038

**Checkpoint**: US1 y US2 funcionan; el dato real entra y se ve.

---

## Fase 4: Historia de Usuario 3 — Corregir o eliminar un registro propio (Prioridad: P2)

**Objetivo**: el autor corrige la fecha, la actividad o las horas de su registro, o lo elimina,
mientras el proyecto no esté Finalizado y el sprint asociado no sea definitivo; nadie más puede.

**Rama**: `hu/006-us3-corregir-o-eliminar-un-registro-propio`

**Prueba independiente**: `PATCH /api/v1/effort-entries/{id}` de 4 a 6 h devuelve `200` con el mismo
sprint; desde otro integrante, `403`; `DELETE` del autor devuelve `204` y el total baja.

### Pruebas de la Historia 3 ⚠️

- [ ] T040 [P] [US3] Pruebas unitarias **obligatorias** de
      `EnsureEntryWritable(sprint *AnchoredSprint, now time.Time, grace time.Duration) error` en
      `backend/internal/effort/domain/grace_test.go` con la tabla de data-model.md sección 6.2,
      cubriendo RN-09 y RN-10: sin sprint → ok; `planned` y `active` → ok; `closed` hace 10 h con
      48 h → ok; `closed` hace exactamente 48 h → `ErrEffortFinalized`; `closed` hace cinco días →
      `ErrEffortFinalized` — depende de T039
- [ ] T041 [P] [US3] Pruebas unitarias **obligatorias** de `EnsureAuthor(authorID, actorID uuid.UUID) error`
      en `backend/internal/effort/domain/author_test.go`, cubriendo RN-11: el autor → ok; otro
      integrante y el propietario del proyecto → `ErrNotEntryAuthor` — depende de T039
- [ ] T042 [P] [US3] Pruebas unitarias **obligatorias** de `ValidateEntryPatch` en
      `backend/internal/effort/delivery/validation_test.go`, cubriendo RN-12 y FR-023: cuerpo vacío
      `{}` → error "Hay que indicar al menos un campo a modificar."; solo `hours` → ok; las mismas
      reglas de horas, actividad y fecha que el alta; `storyId`, `userId` y `sprintId` presentes,
      **aunque valgan `null`** → `FieldError` en ese campo con "La historia, el integrante y el sprint
      de un registro no se modifican; para mover horas, eliminá el registro y creá uno nuevo." —
      depende de T039
- [ ] T043 [P] [US3] Pruebas del middleware `RequireEffortEntryAccess` en
      `backend/internal/effort/delivery/middleware_test.go`, cubriendo FR-059: sin titular → `401`
      con el cuerpo de la 001; identificador mal formado, registro inexistente, registro de un
      proyecto ajeno y registro propio de una persona quitada del proyecto → `404` con **el mismo
      cuerpo byte a byte** (`{"error":"El registro de esfuerzo no existe."}`); integrante → sigue la
      cadena con el `EntryAccess` en el contexto, sea o no el autor — depende de T039
- [ ] T044 [US3] Pruebas de integración en `backend/internal/effort/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (006/US3)`: `TestUS3_CorreccionDeHoras` (total +2, autor,
      historia y sprint sin cambio), `TestUS3_Eliminacion` (`204`; sale del total de la historia, del
      sprint y del día), `TestUS3_CambioDeFecha` (tope verificado sobre la fecha nueva, sprint sin
      cambio), `TestUS3_SprintPlanificadoAhoraActivo`, `TestUS3_HistoriaQuitadaDelSprint` (quita con
      la 004 y luego modificación: conserva el sprint), `TestUS3_ReduccionEnDiaCompleto` (24 → 22),
      `TestUS3_AumentoQueSuperaElTope` (`409`, el registro queda en 6),
      `TestUS3_EliminacionDelUnicoRegistro` (0 h y desglose vacío; la historia sigue sin poder
      eliminarse con la 003), `TestUS3_DentroDelPlazo` (subtests corregir y eliminar a las 10 h del
      cierre), `TestUS3_PlazoCumplido` (exactamente 48 h → `409 effort_finalized`),
      `TestUS3_RegistroAjeno` (subtests otro integrante y propietario, modificar y eliminar, con
      plazo vigente → `403`, sin cambios), `TestUS3_ModificacionDefinitiva`,
      `TestUS3_EliminacionDefinitiva` (el registro se conserva), `TestUS3_RegistroSinSprint`
      (subtests modificar y eliminar), `TestUS3_DatosInvalidos` (subtests 0, 25, 1,1, actividad vacía,
      fecha futura y anterior al inicio: `400` y el registro exactamente como estaba),
      `TestUS3_CamposInmutables` (subtests `storyId` y `userId`), `TestUS3_ProyectoFinalizado`
      (`409 read_only` en modificar y eliminar) y `TestUS3_ExIntegrante` (baja con la 002: `404` de
      registro inexistente y los registros siguen visibles para los vigentes; tras reincorporarlo
      vuelve a poder modificarlos) — depende de T039
- [ ] T045 [US3] Agregar a `backend/internal/effort/delivery/effort_concurrency_test.go` la variante
      `TestEffortDailyCap_ConcurrencyMixed`, **sin** etiqueta: con 18 h cargadas en la fecha, a la vez
      una modificación de un registro de 2 h a 6 h y tres altas de 2 h; en ningún intercalado la
      fecha supera 24,00 y ninguna respuesta es `500` (FR-024, SC-005) — depende de T044
- [ ] T046 [P] [US3] Pruebas de frontend en
      `frontend/src/features/effort/components/effort-entry-list.test.tsx`, con
      `describe("006/US3 - Corregir o eliminar un registro propio")`: los botones **Editar** y
      **Eliminar** aparecen solo en los registros propios con `definitive: false`; la edición en línea
      envía solo los campos cambiados; la eliminación pide confirmación; los mensajes del `403` y de
      los `409` se muestran sin perder lo escrito — depende de T039

### Implementación de la Historia 3

- [ ] T047 [P] [US3] Implementar `EnsureEntryWritable` en `backend/internal/effort/domain/grace.go`
      sobre `GraceOpen` — depende de T040
- [ ] T048 [P] [US3] Implementar `EnsureAuthor` en `backend/internal/effort/domain/author.go` —
      depende de T041
- [ ] T049 [US3] Implementar `ValidateEntryPatch` en `backend/internal/effort/delivery/validation.go`
      con el DTO `EffortEntryPatch` en `backend/internal/effort/delivery/dto.go`: cada campo como
      `json.RawMessage` para distinguir ausente de `null`, y `StoryID`, `UserID` y `SprintID`
      decodificados **solo para rechazarlos** con `swaggerignore:"true"` (patrón de `StoryPatch` de la
      003) — depende de T042
- [ ] T050 [US3] Implementar en `backend/internal/effort/repository/effort_repository.go`
      `FindAccessible` (una consulta `effort_entries → user_stories → project_members` con la
      membresía vigente del titular y el estado del proyecto; sin fila → `ErrEntryNotFound`),
      `Update` y `Delete` en transacción con el orden de research.md sección 10: proyecto
      `FOR SHARE` (`finished` → `ErrEffortReadOnly`), `EnsureAuthor`, registro `FOR UPDATE` releído
      con el estado y el `closed_at` de su sprint y `EnsureEntryWritable`; en `Update` además
      `ValidateEntryDate` sobre la fecha resultante, bloqueo del autor `NO KEY UPDATE` y
      `CheckDailyCap` **excluyendo** las horas vigentes del propio registro sobre la fecha
      resultante. `Update` **nunca** escribe `story_id`, `user_id` ni `sprint_id`; `Delete` no toca
      `ever_had_effort` — depende de T043, T044, T047, T048
- [ ] T051 [US3] Implementar `RequireEffortEntryAccess` en
      `backend/internal/effort/delivery/middleware.go` con una única salida `404` y el mensaje
      `El registro de esfuerzo no existe.` — depende de T050
- [ ] T052 [US3] Implementar `Update` y `Delete` en `backend/internal/effort/service/effort_service.go`
      con el actor de la sesión y el instante del reloj — depende de T050
- [ ] T053 [US3] Implementar los manejadores `updateEffortEntry` y `deleteEffortEntry` con
      anotaciones `swag` en `backend/internal/effort/delivery/handler.go` (`ErrNotEntryAuthor` →
      `403` "Solo quien registró este esfuerzo puede modificarlo o eliminarlo.";
      `ErrEffortFinalized` → `409 effort_finalized` "Este esfuerzo pertenece a un sprint cerrado y ya
      es definitivo."), y en `backend/internal/effort/delivery/routes.go`
      `RegisterEntryRoutes(router, requireSession, repo, handler)` con el grupo plano
      `/api/v1/effort-entries/:entryId` y `PATCH` y `DELETE`; registrarlo en
      `backend/cmd/app/main.go` sobre la misma instancia del repositorio — depende de T049, T051, T052
- [ ] T054 [US3] Implementar `updateEffortEntry` y `deleteEffortEntry` en
      `frontend/src/features/effort/api.ts` y `EffortEntryRow` en
      `frontend/src/features/effort/components/effort-entry-row.tsx` (edición en línea de fecha,
      actividad y horas; eliminación con confirmación), usado por `EffortEntryList` — depende de T046,
      T053

**Checkpoint**: US1 a US3 funcionan; el dato es corregible y queda congelado al vencer el plazo.

---

## Fase 5: Historia de Usuario 4 — Consultar el esfuerzo del proyecto con filtros (Prioridad: P2)

**Objetivo**: un integrante recorre el esfuerzo del proyecto y lo acota por persona, historia, sprint
(o "sin sprint") y rango de fechas, con el total del conjunto.

**Rama**: `hu/006-us4-consultar-el-esfuerzo-del-proyecto`

**Prueba independiente**: con registros de tres integrantes en dos sprints,
`GET /api/v1/projects/{projectId}/effort-entries?sprintId=<id>` devuelve solo los anclados a ese
sprint y su total; `?from=2026-10-10&to=2026-10-09` devuelve `400` en `to`.

### Pruebas de la Historia 4 ⚠️

- [ ] T055 [P] [US4] Pruebas unitarias **obligatorias** de `ParseEntryFilter` en
      `backend/internal/effort/delivery/validation_test.go`, cubriendo FR-038 y FR-043: sin
      parámetros → filtro vacío; `userId`, `storyId` y `sprintId` mal formados → `FieldError` en cada
      uno con "El filtro no corresponde al proyecto."; `sprintId=none` → `WithoutSprint`; `from` y
      `to` con formato inválido → `FieldError`; solo `from`, solo `to`, ambos iguales → ok; `to`
      anterior a `from` → `FieldError` en `to` con "La fecha de fin del rango no puede ser anterior a
      la de inicio." — depende de T054
- [ ] T056 [US4] Pruebas de integración en `backend/internal/effort/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (006/US4)`: `TestUS4_SinFiltros` (todos con fecha,
      autor, historia, actividad, horas y sprint, y el total general), `TestUS4_FiltroPorIntegrante`,
      `TestUS4_FiltroPorSprint` (por el sprint asociado, aunque la historia esté hoy en otro),
      `TestUS4_RangoEHistoria`, `TestUS4_SinSprint`, `TestUS4_RangoDeUnDia`,
      `TestUS4_SinCoincidencias` (`entries: []`, `totalHours: 0.00`, `emptyReason: no_matches`),
      `TestUS4_ProyectoSinEsfuerzo` (`emptyReason: no_effort`), `TestUS4_FiltroPorExIntegrante`,
      `TestUS4_RangoInvertido`, `TestUS4_FiltroDeOtroProyecto` (subtests historia y sprint de otro
      proyecto, e integrante ajeno: `400` con el **mismo** cuerpo que un UUID inexistente, sin datos
      del otro proyecto) y `TestUS4_NoIntegrante` (`404` de la 002). Además, subtests con al menos
      tres combinaciones de dos o más filtros (SC-019) — depende de T054
- [ ] T057 [P] [US4] Pruebas de frontend en
      `frontend/src/features/effort/components/effort-filters.test.tsx` y
      `frontend/src/features/effort/components/project-effort-view.test.tsx`, con
      `describe("006/US4 - Consultar el esfuerzo del proyecto con filtros")`: los filtros arman la
      query (incluido "Sin sprint" → `sprintId=none`), el selector de integrantes incluye a quienes
      dejaron registros aunque ya no sean integrantes, el total se muestra, y los avisos distintos
      para `no_effort` y `no_matches` — depende de T054

### Implementación de la Historia 4

- [ ] T058 [US4] Implementar `ParseEntryFilter` en `backend/internal/effort/delivery/validation.go`
      sobre los parámetros de query, con las fechas a medianoche UTC — depende de T055
- [ ] T059 [US4] Implementar en `backend/internal/effort/repository/effort_repository.go`
      `ValidateFilter` (historia y sprint del proyecto; integrante vigente **o** autor de algún
      registro del proyecto; cualquier otro → `InvalidFilterError{Field}` sin distinguir inexistente
      de ajeno), `ListForProject` (filtros conjuntivos sobre `effort_entries` unido a `user_stories`
      por `project_id`, a `users` y a `sprints`; `sprint_id IS NULL` para `none`; extremos del rango
      inclusivos; orden de FR-045) y `HasProjectEffort` — depende de T056
- [ ] T060 [US4] Implementar `ProjectEffort(ctx, projectID, filter)` en
      `backend/internal/effort/service/effort_service.go`: valida el filtro, lista, agrega el total
      en Go, calcula `definitive` y, con la lista vacía, `emptyReason` (`no_effort` si
      `HasProjectEffort` es `false`, si no `no_matches`) — depende de T059
- [ ] T061 [US4] Declarar `ProjectEffort` en `backend/internal/effort/delivery/dto.go`, implementar el
      manejador `listProjectEffort` con anotaciones `swag` (`InvalidFilterError` → `400` con
      `ValidationErrorResponse`) en `backend/internal/effort/delivery/handler.go`, y en
      `backend/internal/effort/delivery/routes.go` `RegisterProjectRoutes(projectGroup, handler)` con
      `GET /effort-entries` sobre el subgrupo de la 002, registrado en `backend/cmd/app/main.go` —
      depende de T058, T060
- [ ] T062 [US4] Implementar `listProjectEffort` en `frontend/src/features/effort/api.ts`,
      `EffortFilters` en `frontend/src/features/effort/components/effort-filters.tsx`,
      `ProjectEffortView` en `frontend/src/features/effort/components/project-effort-view.tsx`, la
      página `frontend/src/app/projects/[projectId]/effort/page.tsx` y el enlace **Esfuerzo** en
      `frontend/src/features/projects/components/project-overview.tsx`. Antes, leer la documentación
      de la versión instalada en `frontend/node_modules/next/dist/docs/` — depende de T057, T061

**Checkpoint**: US1 a US4 funcionan; el esfuerzo se lee a escala de proyecto.

---

## Fase 6: Historia de Usuario 5 — Comparar el esfuerzo estimado con el real (Prioridad: P3)

**Objetivo**: por historia y para todo el proyecto, horas estimadas (Story Points × factor vigente),
reales, diferencia absoluta y porcentual, con "no calculable" donde corresponde.

**Rama**: `hu/006-us5-comparar-el-esfuerzo-estimado-con-el-real`

**Prueba independiente**: con factor 6 y una historia de 5 Story Points con 36 h reales,
`GET /api/v1/stories/{storyId}/effort-comparison` devuelve `estimatedHours: 30.00`,
`actualHours: 36.00`, `deviationHours: 6.00` y `deviationPercent: 20.00`.

### Pruebas de la Historia 5 ⚠️

- [ ] T063 [P] [US5] Pruebas unitarias **obligatorias** de
      `Compare(storyPoints *int, factorHundredths, actualHundredths int) Comparison` y de
      `RoundPercentHundredths(numerator, denominator int) int` en
      `backend/internal/effort/domain/comparison_test.go`, cubriendo RN-16 a RN-20, RN-26, SC-012 a
      SC-014 y SC-022 con la tabla de research.md sección 12: sin estimar → estimada, diferencia y
      porcentaje `nil`; 0 puntos → estimada 0, diferencia = real, porcentaje `nil`; 5 puntos con
      factor 600 y 3600 reales → 3000, +600, +2000; con 2400 reales → −600, −2000; exacta → 0 y 0; 3
      puntos sin registros → −1800 y −10000; factor 610 (6,1); redondeo de 1/3 → 3333, 2/3 → 6667,
      −1/3 → −3333, empate positivo y negativo **mitad lejos de cero**, y 1650/12600 → 1310 (13,10 %
      de 008/US1-1); ningún `float64` en la implementación — depende de T062
- [ ] T064 [US5] Pruebas de integración en `backend/internal/effort/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (006/US5)`: `TestUS5_Exceso`, `TestUS5_Ahorro`,
      `TestUS5_Exacta`, `TestUS5_CambioDeFactor` (factor 6 → 8 con la 002: estimadas recalculadas,
      reales sin cambio), `TestUS5_SinEstimar` (`null` en los tres campos, nunca `0.00`),
      `TestUS5_CeroPuntos`, `TestUS5_SinRegistros`, `TestUS5_CeroPuntosSinRegistros`,
      `TestUS5_TodasSinEstimar` (sobre `GET /projects/{id}/effort-comparison`: todas las filas con
      `null`, sin error), `TestUS5_NoIntegrante` (`404` de la 002) y `TestUS5_HistoriaInexistente`.
      La comparación del proyecto incluye todas las historias, de cualquier estado, en el orden del
      backlog de la 003 — depende de T062
- [ ] T065 [P] [US5] Pruebas de frontend en
      `frontend/src/features/effort/components/effort-comparison-table.test.tsx` y
      `frontend/src/features/effort/components/effort-comparison-card.test.tsx`, con
      `describe("006/US5 - Comparar el esfuerzo estimado con el real")`: "No calculable" para `null`,
      el signo distingue exceso de ahorro, y una historia de 0 Story Points se distingue visiblemente
      de una "sin estimar" (FR-055) — depende de T062

### Implementación de la Historia 5

- [ ] T066 [P] [US5] Implementar `Compare` y `RoundPercentHundredths` en
      `backend/internal/effort/domain/comparison.go` con aritmética entera (cociente y resto), y un
      comentario que indique que la 008 debe reutilizar `RoundPercentHundredths` (RC-13) — depende de
      T063
- [ ] T067 [US5] Implementar `ComparisonRows` en
      `backend/internal/effort/repository/effort_repository.go`: el factor vigente de `projects` y las
      historias del proyecto (o una sola) con `story_points` y la suma de sus horas por `LEFT JOIN`
      agregado (0 sin registros), en el orden del listado del backlog de la 003 (prioridad,
      `created_at`, `id`) — depende de T064
- [ ] T068 [US5] Implementar `StoryComparison` y `ProjectComparison` en
      `backend/internal/effort/service/effort_service.go` sobre `Compare` — depende de T066, T067
- [ ] T069 [US5] Declarar `EffortComparisonRow` y `ProjectEffortComparison` en
      `backend/internal/effort/delivery/dto.go`, implementar los manejadores
      `getStoryEffortComparison` y `getProjectEffortComparison` con anotaciones `swag` en
      `backend/internal/effort/delivery/handler.go`, y registrar
      `GET /api/v1/stories/:storyId/effort-comparison` en `RegisterStoryRoutes` y
      `GET /effort-comparison` en `RegisterProjectRoutes` de
      `backend/internal/effort/delivery/routes.go` — depende de T068
- [ ] T070 [US5] Implementar `getStoryEffortComparison` y `getProjectEffortComparison` en
      `frontend/src/features/effort/api.ts`, `EffortComparisonCard` en
      `frontend/src/features/effort/components/effort-comparison-card.tsx` (en `StoryEffortSection`) y
      `EffortComparisonTable` en
      `frontend/src/features/effort/components/effort-comparison-table.tsx` (en la página de esfuerzo
      del proyecto) — depende de T065, T069

**Checkpoint**: las cinco historias funcionan; la desviación entre estimado y real existe como número.

---

## Fase 7: Polish (Cierre y asuntos transversales)

**Propósito**: pruebas transversales de seguridad y de invariantes, documentación viva, cobertura,
vulnerabilidades y verificación de las puertas de calidad.

**Rama**: `fase/006-polish`

- [ ] T071 Pruebas de integración de caracterización en
      `backend/internal/effort/delivery/handler_test.go`, **sin** etiqueta `006/USn`:
      `TestEffortEndpoints_SinSesion`, con un subtest por cada uno de los **siete** endpoints sin
      cabecera y con token adulterado → `401` y ningún cambio (SC-017);
      `TestEffortEndpoints_NotFoundIndistinguible`, que compara byte a byte el `404` ajeno e
      inexistente de historia (tres rutas), de proyecto (dos rutas) y de registro (dos rutas)
      (SC-018); `TestEffortAuthor_TodasLasFormas`, con `userId`, `authorId` y `memberId` en el cuerpo
      y `userId` en la query del alta → siempre a nombre de la sesión (SC-004);
      `TestEffortTotals_CuartosDeHora`, con 100 registros de 0,25 en varias fechas → total exacto
      `25.00` (SC-015); `TestEffortAnchor_Immutable`, que quita la historia del sprint, la compromete
      en otro y cierra el primero, y compara la asociación antes y después (SC-010);
      `TestEffortGrace_Boundaries`, con cuatro subtests por acción (alta, modificación, eliminación):
      justo después del cierre, poco antes del vencimiento, exactamente en el vencimiento y después
      (SC-020); y una aserción de que ninguna respuesta contiene `password_hash` ni `passwordHash`
      (Principio VI). Caracterización: las reglas ya existen, así que no tiene RED; el commit lo
      justifica — depende de T070
- [ ] T072 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las **siete**
      operaciones, sus códigos de respuesta (todas declaran `500`) y sus esquemas, incluidos los
      números de 2 decimales y los `null` de "no calculable"; corregir las anotaciones si difieren
      (Principio VIII) — depende de T071
- [ ] T073 [P] Actualizar `README.md` de la raíz en español: la sección introductoria menciona el
      registro de esfuerzo y la comparación entre estimado y real, y se agregan los siete endpoints
      (la variable ya está desde T019), la sección "Esfuerzo" del detalle de la
      historia y la página de esfuerzo del proyecto, manteniendo el criterio de que alguien que no
      conoce el proyecto pueda ejecutarlo siguiendo el documento — depende de T071
- [ ] T074 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T075 [P] Ejecutar los comandos de cobertura de los dos lados —
      `go test -p 1 ./... -coverprofile=coverage.out` más `go tool cover -func=coverage.out` en
      `backend/`, y `npm run test:coverage` en `frontend/` — y revisar que `internal/effort/`,
      `frontend/src/features/effort/` y `frontend/src/utils/effort-format.ts` estén cubiertos; correr
      además `go test -race -p 1 ./internal/effort/...` — depende de T071
- [ ] T076 Verificar la trazabilidad y las puertas de seguridad recorriendo `backend/internal/effort/`
      y `frontend/src/features/effort/`: que buscar `006/US1` a `006/US5` en el código de pruebas
      devuelva los **74** escenarios; que las **26** reglas RN-01 a RN-26 tengan prueba asociada según
      la tabla de plan.md (SC-023); que los **siete** endpoints tengan su prueba de `401` (SC-017);
      que los `404` sean indistinguibles (SC-018); que fuera de `effort_entries` esta feature solo
      escriba `user_stories.ever_had_effort`; que ningún cálculo de horas o porcentajes use
      `float64`; que no haya dependencias nuevas en `go.mod` ni en `package.json`; y que ninguna
      respuesta incluya `password_hash` — depende de T071
- [ ] T077 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluida la
      validación manual de 16 pasos con `EFFORT_GRACE_PERIOD_HOURS=1`, las pruebas de concurrencia
      con `-count=5` y la verificación de que las pruebas corrieron contra `smye_test` en el puerto
      5433 y no contra desarrollo — depende de T072, T073, T076

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Foundational**: parte de `main` con la 001 a la 005 integradas. **BLOQUEA** todas las
  historias.
- **Fases 2 a 6 (historias)**: todas dependen de la Fase 1, en orden de prioridad
  (P1 → P1 → P2 → P2 → P3).
- **Fase 7 Polish**: depende de que estén completas las cinco historias.

### Dependencias entre historias

Las historias **no son todas independientes en su implementación**, aunque cada una se prueba sola:

- **US1 Registrar (P1)**: puede empezar después de la Fase 1. **Aporta** el entorno de pruebas
  (T015), la configuración del plazo, el tipo de 2 decimales, el servicio con su firma definitiva,
  `GraceOpen`, los DTO base, la composición en `cmd/app/main.go` y los puertos reales de la 003 y la
  004, que usan todas las demás.
- **US2 Consultar la historia (P1)**: necesita registros de US1.
- **US3 Corregir o eliminar (P2)**: necesita registros de US1 y la lista de US2 en el frontend.
- **US4 Esfuerzo del proyecto (P2)**: necesita registros de US1; no depende de US2 ni de US3 en el
  backend.
- **US5 Comparar (P3)**: necesita registros de US1; en el frontend usa la sección de US2 y la página
  de US4.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T001 a T004 en paralelo; después T005; T006 y T007 en secuencia.
- Dentro de cada historia: las pruebas unitarias de dominio, de validación, de configuración, del
  middleware, de los puertos y de frontend marcadas `[P]` en paralelo entre sí. Las de integración
  **no** son paralelizables entre historias porque las cinco comparten `handler_test.go`, ni las de
  concurrencia porque comparten `effort_concurrency_test.go`.
- En las implementaciones, las funciones puras de dominio marcadas `[P]` (cada una en su archivo)
  en paralelo; repositorio, servicio y `delivery` en secuencia, porque cada historia toca
  `effort_repository.go`, `effort_service.go`, `handler.go`, `dto.go` y `routes.go`.
- Fase 7: T073 y T075 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las tareas de pruebas marcadas [P] pueden escribirse a la vez:
T008  backend/internal/platform/config/config_test.go
T009  backend/internal/effort/delivery/validation_test.go
T010  backend/internal/effort/delivery/dto_test.go
T011  backend/internal/effort/domain/entry_date_test.go
T012  backend/internal/effort/domain/daily_cap_test.go
T013  backend/internal/effort/domain/anchor_test.go
T014  backend/internal/effort/domain/grace_test.go
T017  backend/internal/backlog/repository/effort_records_test.go + sprints/repository/sprint_effort_test.go
T018  frontend validation.test.ts + effort-format.test.ts + effort-entry-form.test.tsx

# T015 (handler_test.go + testenv_test.go) va aparte: lo comparten las cinco historias.

# En la implementación, estas no se pisan:
T020  backend/internal/effort/delivery/dto.go (tipo de 2 decimales)
T021  backend/internal/effort/domain/entry_date.go
T022  backend/internal/effort/domain/daily_cap.go
T024  backend/internal/effort/domain/grace.go  (T023 anchor.go la usa: después)
T032  frontend/src/types/effort.ts + utils/effort-format.ts + features/effort/validation.ts
```

## Ejemplo de paralelismo: Historia de Usuario 3

```text
# Pruebas en archivos distintos:
T040  backend/internal/effort/domain/grace_test.go
T041  backend/internal/effort/domain/author_test.go
T042  backend/internal/effort/delivery/validation_test.go
T043  backend/internal/effort/delivery/middleware_test.go
T046  frontend/src/features/effort/components/effort-entry-list.test.tsx

# Implementación de las reglas puras:
T047  backend/internal/effort/domain/grace.go
T048  backend/internal/effort/domain/author.go
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar la Fase 1 (Foundational) — **CRÍTICO**: bloquea todas las historias.
2. Completar la Fase 2 (US1 Registrar el esfuerzo trabajado).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con los pasos 1 a 4 y 8 a 11 de la
   guía de validación.
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Foundational → base lista (una fase técnica, un pull request).
2. US1 Registrar → probar sola → entregar (**MVP**: el dato real entra al sistema, anclado y con el
   tope invariante; la 003 y la 004 ya bloquean eliminaciones con esfuerzo).
3. US2 Consultar la historia → probar sola → entregar (el dato se ve).
4. US3 Corregir o eliminar → probar sola → entregar (el dato es confiable).
5. US4 Esfuerzo del proyecto → probar sola → entregar.
6. US5 Comparar → probar sola → entregar (la desviación existe como número).
7. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Foundational en conjunto.
2. US1 en secuencia: aporta el entorno de pruebas, el servicio y la composición que usan todas.
3. Terminada US1, US2 y US4 pueden avanzar en paralelo en el backend coordinando
   `effort_repository.go`, `effort_service.go`, `handler.go`, `dto.go`, `routes.go` y
   `handler_test.go`; el frontend de cada una, en archivos distintos.
4. US3 después de US2 (usa su lista en el frontend); US5 al final.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `006/US1` a `006/US5` en el código de
  pruebas devuelve la trazabilidad completa de los 74 escenarios exigida por el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 006/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b), con el ámbito `esfuerzo`. Los commits los registra **una
  persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni de npm. Si alguna tarea parece necesitar
  una (por ejemplo, una biblioteca de decimales), detenerse y pedir autorización (Principio IV).
- **Código de otras features que esta feature modifica**, todo señalado en el plan: `UserStory` de
  la 003 (T002), los puertos `EffortRecords` de la 003 y `SprintEffort` de la 004 y el `Delete` de
  sprints (T031), la configuración de la plataforma (T019), el detalle de la historia de la 003
  (T033, T039, T070) y el resumen del proyecto de la 002 (T062).
- **Puntos del plan que esperan confirmación del desarrollador** y no tienen tarea: el criterio de
  empate del redondeo (mitad lejos de cero; research.md sección 5), que la 008 deberá respetar, y la
  ventana de carrera entre `HasEffort` y el borrado de una historia en la 003 (research.md, punto
  abierto 2), que se recomienda tratar en `/revisar-seguridad`.
- **Criterios de éxito no automatizables.** SC-001 (registrar en menos de 30 segundos y 4 pasos, 95 %
  sin ayuda), SC-002 (total y desglose en una sola pantalla) y SC-003 (responder "cuántas horas puso
  cada persona en este sprint" en menos de 1 minuto) son métricas de experiencia de uso: las pruebas
  de componentes verifican la estructura, y la medición se hace a mano siguiendo la sección 4 de
  [quickstart.md](./quickstart.md).
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: Swagger se actualiza en la misma tarea que cada endpoint, pero el `README.md` se
  actualiza en dos momentos: la variable `EFFORT_GRACE_PERIOD_HOURS`, en US1 junto con su
  implementación (T019), porque cambia los pasos para ejecutar la aplicación; y la descripción de la
  feature y sus endpoints, una sola vez en la Fase 7 (T073). Entre el merge de una historia y la Fase
  7, el README no lista los endpoints de esfuerzo ya integrados. Se acepta porque Swagger es la
  referencia actualizada de la API en cada merge. Es el mismo criterio que aplicaron la 001 a la 005.

---

description: "Lista de tareas de la feature 005 — Planning Poker en Tiempo Real"
---

# Tareas: Planning Poker en Tiempo Real

**Entrada**: documentos de diseño de `specs/005-planning-poker/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) a
[`specs/004-sprint-management`](../004-sprint-management/spec.md) están integradas en `main`. Esta
feature usa del módulo `internal/backlog/` el modelo `UserStory`, `StoryStatus`,
`StoryPointsScale`, `IsValidStoryPoints`, el middleware `RequireStoryAccess` y el puerto
`EstimationSessions`; además el middleware de sesión y `authdelivery.CurrentUserID` de la 001, los
esquemas de error de la 001, el límite global de 1 MiB, el patrón de reloj inyectado y el ayudante
`testsupport.OpenDatabase`. **No hay fase de Setup**: no se agregan dependencias, servicios ni
variables de entorno (research.md sección 1).

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para la
validación de cartas y del valor acordado, la revelación automática y manual, el resultado de la
ronda, el relevo del facilitador, la cancelación automática, el estado cerrado, la vista que oculta
los votos, el contador de presencia y las reglas de negocio RN-01 a RN-36.

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Backend**: módulo nuevo `backend/internal/poker/{domain,repository,service,delivery}/`; se tocan
  de forma puntual `backend/cmd/app/main.go`, `backend/internal/platform/database/postgres.go`,
  `backend/internal/platform/testsupport/database.go`,
  `backend/internal/backlog/repository/estimation_sessions.go` y, en la Fase 10,
  `backend/internal/auth/delivery/middleware.go`
- **Frontend**: `frontend/src/app/projects/[projectId]/backlog/[storyId]/poker/[sessionId]/`,
  `frontend/src/features/poker/`, `frontend/src/lib/event-stream.ts`,
  `frontend/src/types/poker.ts`, `frontend/src/utils/poker-labels.ts`; se toca
  `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx`
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

- Las pruebas de integración usan un entorno `newPokerTestEnv` (T015) que compone el router como
  `cmd/app` (sesión de la 001, rutas de la 002, la 003 y la 004, y las de Planning Poker) detrás de
  `httptest.NewServer`, con un **reloj falso** que la prueba avanza y acceso al servicio para llamar
  a `Sweep` a mano. Ninguna prueba espera 30 segundos, 5 minutos ni 60 minutos reales.
- Las incorporaciones y desconexiones se hacen **abriendo y cerrando canales SSE reales** con el
  ayudante `openStream(t, token, sessionID)`, que devuelve un lector con `Next(t)` (próxima
  instantánea, con un plazo de 2 segundos) y `Close()`. "Dos pestañas" son dos canales con el mismo
  token.
- Las historias se crean, modifican, reestiman y eliminan con los **endpoints reales de la 003**; la
  historia Completada se prepara con un sprint Activo y el avance a `completed` de la 004; el
  proyecto Finalizado y la baja de un integrante, con los endpoints de la 002.
- Las pruebas sin sesión, la comparación de los `404`, la de secreto por cada forma de consulta, las
  de concurrencia, la de escala y las de cancelación del sistema **no** llevan la etiqueta
  `005/USn`, porque no derivan de un escenario de la spec y no deben alterar el conteo de 79.

---

## Fase 1: Foundational (Prerrequisitos bloqueantes)

**Propósito**: el dominio del módulo nuevo, su contrato de repositorio, la migración de las cuatro
tablas con sus índices y el aislamiento de las pruebas. Todas las tareas son **técnicas**: declaran
estructuras y contratos, sin lógica de negocio.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/005-foundational`

- [X] T001 [P] Declarar la entidad `EstimationSession` en `backend/internal/poker/domain/session.go`
      con los campos de data-model.md sección 1: `ID uuid` PK generado con `google/uuid`;
      `StoryID uuid NOT NULL` con FK a `user_stories.id` (campo `Story *backlogdomain.UserStory` solo
      para la FK, nunca precargado); `FacilitatorID uuid NOT NULL` con FK a `users.id`;
      `Status varchar(16) NOT NULL` (`open`, `finished` o `canceled`);
      `AgreedStoryPoints smallint` **anulable** (`*int`); `CancelReason varchar(24)` **anulable**
      (`inactivity`, `story_completed`, `project_finished`); `OpenedByID uuid NOT NULL` con FK a
      `users.id`; `ClosedByID uuid` **anulable** con FK a `users.id`; `ClosedAt timestamptz`
      **anulable**; `LastActivityAt timestamptz NOT NULL`; `Version bigint NOT NULL DEFAULT 0`;
      `CreatedAt` (es el momento de apertura) y `UpdatedAt` `timestamptz NOT NULL`. **Índice único
      parcial** `idx_estimation_sessions_one_open_per_story` sobre `(story_id) WHERE status = 'open'`,
      declarado con la opción `where` de la etiqueta de índice de GORM, e índice
      `idx_estimation_sessions_story_created (story_id, created_at)`. Claves foráneas **sin
      `ON DELETE`**
- [X] T002 [P] Declarar la entidad `EstimationRound` en `backend/internal/poker/domain/round.go` con
      los campos de data-model.md sección 2: `ID uuid` PK; `SessionID uuid NOT NULL` con FK a
      `estimation_sessions.id`; `Number integer NOT NULL`; `Status varchar(16) NOT NULL` (`voting` o
      `revealed`); `RevealedAt timestamptz` **anulable**; `CreatedAt` (momento de apertura de la
      ronda) y `UpdatedAt` `timestamptz NOT NULL`. **Índice único**
      `idx_estimation_rounds_session_number (session_id, number)`. **Sin** columnas de resultado: se
      derivan (research.md sección 5)
- [X] T003 [P] Declarar la entidad `EstimationParticipant` en
      `backend/internal/poker/domain/participant.go` con los campos de data-model.md sección 3:
      `ID uuid` PK; `SessionID uuid NOT NULL` con FK a `estimation_sessions.id`;
      `UserID uuid NOT NULL` con FK a `users.id`; `Connected boolean NOT NULL`;
      `DisconnectedAt timestamptz` **anulable**; `CreatedAt` (momento de incorporación) y `UpdatedAt`
      `timestamptz NOT NULL`. **Índice único** `idx_estimation_participants_session_user
      (session_id, user_id)`. **Sin** columna de facilitador: es `estimation_sessions.facilitator_id`
- [X] T004 [P] Declarar la entidad `EstimationVote` en `backend/internal/poker/domain/vote.go` con los
      campos de data-model.md sección 4: `ID uuid` PK; `RoundID uuid NOT NULL` con FK a
      `estimation_rounds.id`; `ParticipantID uuid NOT NULL` con FK a `estimation_participants.id`;
      `Card varchar(3) NOT NULL`; `CreatedAt` y `UpdatedAt` (momento en que quedó emitida la carta
      vigente) `timestamptz NOT NULL`. **Índice único** `idx_estimation_votes_round_participant
      (round_id, participant_id)`
- [X] T005 [P] Declarar en `backend/internal/poker/domain/status.go` los tipos `SessionStatus`
      (`open`, `finished`, `canceled`), `RoundStatus` (`voting`, `revealed`), `CancelReason`
      (`inactivity`, `story_completed`, `project_finished`) y `OutcomeKind` (`consensus`,
      `divergence`, `no_numeric_votes`), y en `backend/internal/poker/domain/card.go` el tipo `Card`
      con la lista ordenada `Cards`, armada a partir de `backlogdomain.StoryPointsScale` más `"?"`
      (RC-04). Constantes de Go, **no** `enum` de PostgreSQL. Las reglas (`EnsureOpen`, `ParseCard`)
      se implementan en las historias
- [X] T006 [P] Declarar los errores de dominio en `backend/internal/poker/domain/errors.go` con la
      tabla de data-model.md sección 9: `ErrSessionNotFound`, `ErrNotParticipant`,
      `ErrFacilitatorOnly`, `ErrStoryCompleted`, `ErrProjectFinished`, `ErrRoundRevealed`,
      `ErrRoundNotRevealed`, `ErrNoVotesToReveal` y `ErrNoRevealedRound`; y los tipos con datos
      `SessionClosedError` (con `Status`, `CancelReason *CancelReason` y `AgreedStoryPoints *int`),
      `OpenSessionExistsError` (con `SessionID`) y `FacilitatorUnavailableError` (con `Facilitator`
      de `ID` y `FullName`, y `RetryAfter time.Duration`, cero si el facilitador está conectado)
- [X] T007 Declarar en `backend/internal/poker/domain/repository.go` la interfaz `SessionRepository`
      con **exactamente** las diecisiete operaciones de data-model.md sección 9 (`FindAccessible`,
      `Open`, `Connect`, `Disconnect`, `Vote`, `Reveal`, `StartRound`, `Finalize`, `Cancel`,
      `TakeFacilitator`, `AutoCancel`, `ResetPresence`, `ListOpenForSweep`,
      `ListRevokedParticipants`, `FindSnapshot`, `ListForStory`, `FindOpenForStory`), con sus firmas,
      y en `backend/internal/poker/domain/view.go` los tipos de lectura de la sección 8:
      `SessionAccess`, `SessionSnapshot`, `SessionView`, `SessionSummary`, `OpenSessionSummary`,
      `OpenSessionStatus`, `ParticipantRef`, `RoundOutcome` y `UserRef` (solo `ID` y `FullName`).
      `SessionSnapshot` **no** tiene campo de Story Points de la historia (FR-075). Las escrituras
      reciben el instante como parámetro — depende de T001 a T006
- [X] T008 Agregar `EstimationSession`, `EstimationRound`, `EstimationParticipant` y
      `EstimationVote`, en ese orden, al `AutoMigrate` de
      `backend/internal/platform/database/postgres.go`, **después** de `SprintStory`, de modo que los
      índices —incluido el parcial con su cláusula `WHERE`— queden creados por las etiquetas de GORM y
      no por un `CREATE INDEX` en crudo — depende de T001 a T004
- [X] T009 Extender el vaciado del ayudante de pruebas en
      `backend/internal/platform/testsupport/database.go` para incluir `estimation_sessions`,
      `estimation_rounds`, `estimation_participants` y `estimation_votes` en el `TRUNCATE ...
      CASCADE`, manteniendo la verificación de que la base es `smye_test` y el lock de la base de
      test — depende de T008

**Checkpoint**: las cuatro tablas migradas en la base de test, con el índice parcial visible en
`pg_indexes` (quickstart.md sección 2), el dominio y el contrato declarados, y las pruebas aisladas.
Las historias pueden empezar.

---

## Fase 2: Historia de Usuario 1 — Abrir una sesión y reunir al equipo (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante abre una sesión sobre una historia y queda como facilitador; los demás se
unen abriendo el canal de eventos y todos ven la misma historia y la misma lista de participantes,
que se actualiza al entrar y salir.

**Rama**: `hu/005-us1-abrir-una-sesion-y-reunir-al-equipo`

**Prueba independiente**: `POST /api/v1/stories/{storyId}/estimation-sessions` devuelve `201` con
`status: open` y la ronda 1 `voting`; tres canales abiertos reciben un `state` con los tres
participantes conectados, y al cerrar uno y vencer la gracia de 30 segundos los otros dos lo reciben
desconectado.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T010 [P] [US1] Pruebas unitarias **obligatorias** de
      `SessionStatus.EnsureOpen(reason *CancelReason, agreedStoryPoints *int) error` en
      `backend/internal/poker/domain/status_test.go`, cubriendo RN-22: `open` → ok (aunque reciba
      motivo o valor); `finished` con valor 8 → `SessionClosedError` con `Status` `finished` y
      `AgreedStoryPoints` 8; `canceled` con motivo `inactivity` y sin motivo → `SessionClosedError`
      con `Status` `canceled` y `CancelReason` `inactivity` o `nil` según el caso — depende de T009
- [X] T011 [P] [US1] Pruebas unitarias **obligatorias** de `Presence` en
      `backend/internal/poker/service/presence_test.go`, cubriendo RN-23 y RN-34, con instantes
      explícitos: la primera conexión de un par (sesión, usuario) informa "llegó" y la segunda no; al
      cerrar una de dos conexiones no informa nada; al cerrar la última anota el instante y no
      informa "se fue"; una conexión dentro de los 30 segundos borra la marca sin informar "llegó";
      `Expired(now)` devuelve los pares con la última conexión caída hace **30 segundos o más** (29 s
      no, 30 s sí) y los quita; `MarkPending(session, user, now)` deja anotada una caída para quien
      abrió la sesión sin canal (research.md sección 9); sesiones distintas de la misma persona se
      cuentan por separado. Seguro para uso concurrente (`go test -race`) — depende de T009
- [X] T012 [P] [US1] Pruebas unitarias del hub de difusión en
      `backend/internal/poker/service/hub_test.go`: `Subscribe(sessionID, userID)` devuelve un canal
      y una función para darse de baja; `Publish` entrega a todas las suscripciones de esa sesión y
      a ninguna de otra; con el canal lleno, la instantánea pendiente se **reemplaza** por la nueva
      ("la última gana", capacidad 1); `CloseSession` cierra todos los canales de la sesión y
      `CloseUser` los de una persona en una sesión; darse de baja dos veces no falla. Seguro para uso
      concurrente (`go test -race`) — depende de T009
- [X] T013 [P] [US1] Pruebas unitarias **obligatorias** de `ViewFor` (parte de US1) en
      `backend/internal/poker/domain/view_test.go`: la vista lleva la historia con título,
      descripción y criterios **en su orden** y sin Story Points (FR-012, FR-075); `cards` igual a
      las once cartas; cada participante con `connected` e `isFacilitator`, exactamente uno
      facilitador; las rondas con `number`, `status` y `openedAt`; `openedBy`, `openedAt`,
      `closedBy` y `closedAt` copiados del snapshot; lista de criterios vacía como `[]` y no como
      `nil` (US1-4) — depende de T009
- [X] T014 [P] [US1] Pruebas del middleware `RequireEstimationAccess` en
      `backend/internal/poker/delivery/middleware_test.go`, cubriendo RN-01: sin titular en el
      contexto → `401` con el cuerpo de la 001; identificador mal formado, sesión inexistente y sesión
      de un proyecto ajeno → `404` con **el mismo cuerpo byte a byte**
      (`{"error":"La sesión de estimación no existe."}`); integrante → sigue la cadena con el
      `SessionAccess` en el contexto, sin distinguir si es propietario — depende de T009
- [X] T015 [US1] Pruebas de integración de la historia en
      `backend/internal/poker/delivery/handler_test.go`, creando en
      `backend/internal/poker/delivery/testenv_test.go` el ayudante `newPokerTestEnv` (router como
      `cmd/app` detrás de `httptest.NewServer`, reloj falso avanzable, acceso al servicio para
      `Sweep`, alta de cuentas, proyecto, integrantes e historias con los endpoints de la 001 a la
      003) y el lector `openStream(t, token, sessionID)` con `Next(t)` y `Close()` sobre
      `net/http` y `bufio`, que interpreta las líneas `event:` y `data:`. Una función por escenario
      de spec.md precedida por `// Escenario: <nombre del escenario> (005/US1)`:
      `TestUS1_AperturaDeSesion` (`201`, `open`, ronda 1 `voting`, quien abre facilitador y
      conectado), `TestUS1_IncorporacionDeDosIntegrantes` (los tres canales reciben la lista de los
      tres con título, descripción y criterios), `TestUS1_AbandonoDeUnParticipante` (cierra un canal,
      avanza el reloj 30 s, `Sweep`, los otros dos lo reciben desconectado),
      `TestUS1_HistoriaSinCriterios`, `TestUS1_FacilitadorSolo` (sesión válida con un solo canal),
      `TestUS1_SesionesAnterioresConservadas` (prepara dos sesiones cerradas insertando filas con el
      repositorio, porque registrar y cancelar llegan en US5 y US7), `TestUS1_SegundaSesionAbierta`
      (`409` con `sessionId` de la existente y una sola fila `open`), `TestUS1_HistoriaCompletada`
      (`409`), `TestUS1_ProyectoFinalizado` (`409`) y `TestUS1_NoIntegrante` (el canal responde `404`
      con el mismo cuerpo que un `sessionId` inexistente). Cada instantánea recibida se verifica
      **sin recargar**: llega por el canal ya abierto, dentro del plazo de 2 segundos de `Next` —
      depende de T009
- [X] T016 [US1] Prueba de concurrencia `TestPokerOpen_Concurrency` en
      `backend/internal/poker/delivery/poker_concurrency_test.go`, **sin** etiqueta `005/USn`: diez
      cuentas integrantes abren a la vez una sesión sobre la misma historia; exactamente una recibe
      `201`, las nueve restantes `409` con el `sessionId` de la ganadora, ninguna `500`, y en la base
      hay una sola fila `open` de esa historia (SC-012). Repetida varias veces para que el intercalado
      sea real — depende de T015
- [X] T017 [P] [US1] Pruebas de frontend en `frontend/src/lib/event-stream.test.ts` (lector de
      `text/event-stream`: un evento en un fragmento, un evento partido en dos fragmentos, varios
      `data:` en un evento, comentarios `: ping` ignorados, líneas terminadas en `\r\n`), en
      `frontend/src/features/poker/components/participant-list.test.tsx` y en
      `frontend/src/features/poker/components/story-estimation-section.test.tsx`, con
      `describe("005/US1 - Abrir una sesión y reunir al equipo")` y un `it` por escenario con
      comportamiento visible (lista de conectados, marca de facilitador, botón **Abrir Planning
      Poker**, y ante el `409` de sesión en curso, la oferta de **Unirse** a la sesión existente)

### Implementación de la Historia 1

- [X] T018 [P] [US1] Implementar
      `SessionStatus.EnsureOpen(reason *CancelReason, agreedStoryPoints *int) error` en
      `backend/internal/poker/domain/status.go` — depende de T010
- [X] T019 [P] [US1] Implementar `Presence` en `backend/internal/poker/service/presence.go`: mapa
      protegido por `sync.Mutex` de (sesión, usuario) a cantidad de conexiones y momento de la
      última caída, con `Connect`, `Release`, `MarkPending` y `Expired(now)`; la gracia de **30
      segundos** como constante nombrada (FR-014) — depende de T011
- [ ] T020 [P] [US1] Implementar el hub en `backend/internal/poker/service/hub.go`: suscripciones por
      sesión con canales de capacidad 1 y reemplazo de la pendiente, `Subscribe`, `Publish`,
      `CloseSession` y `CloseUser`, más un mutex por sesión que `publish` del servicio usará para
      leer y difundir en orden (research.md sección 7) — depende de T012
- [ ] T021 [P] [US1] Implementar `ViewFor(snapshot, viewerID)` en
      `backend/internal/poker/domain/view.go` con lo que cubre T013; los votos se agregan en US2 —
      depende de T013
- [ ] T022 [US1] Implementar en `backend/internal/poker/repository/session_repository.go`
      `FindAccessible` (una consulta `estimation_sessions → user_stories → project_members` con la
      membresía vigente del titular; sin fila → `ErrSessionNotFound`), `Open` (transacción que
      bloquea la fila de `user_stories` con `clause.Locking{Strength: "UPDATE"}`, lee su `status` y
      el de su proyecto —`completed` → `ErrStoryCompleted`, `finished` → `ErrProjectFinished`—,
      busca la sesión `open` de la historia —si existe → `OpenSessionExistsError`— e inserta sesión
      con `facilitator_id` y `opened_by_id` igual a quien abre, `last_activity_at = now`, ronda 1
      `voting` y participante `connected = true`; la violación de
      `idx_estimation_sessions_one_open_per_story` se traduce al mismo `OpenSessionExistsError`
      releyendo la sesión abierta), `Connect` y `Disconnect` (transacción con `SELECT … FOR UPDATE`
      de la sesión, `EnsureOpen`, alta del participante si no existía o cambio de `connected` y
      `disconnected_at`, `last_activity_at = now` y `version = version + 1`), `ResetPresence`
      (participantes de sesiones `open` a desconectados con `disconnected_at = now`, sin tocar
      `version`) y `FindSnapshot` (sesión, historia con criterios en orden **sin**
      `story_points`, participantes, rondas y votos con `JOIN` a `users` que **selecciona solo
      `users.id` y `users.full_name`**) — depende de T007, T015, T018
- [ ] T023 [US1] Implementar en `backend/internal/poker/service/session_service.go` el constructor
      `NewSessionService(repo SessionRepository, now func() time.Time)`, con la firma definitiva para
      que no cambie en las historias siguientes (crea `Presence` y el hub); `Open`; `Join(ctx,
      access, userID)`, que suscribe al hub, cuenta la conexión en `Presence` y, si es la primera,
      llama a `Connect` y publica; `Leave`, que da de baja la suscripción y libera la conexión; el
      método interno `publish(sessionID)`, que bajo el mutex de la sesión lee `FindSnapshot` y
      entrega a cada suscripción `ViewFor` de su usuario. Al abrir llama a `Presence.MarkPending`
      para quien abrió. En `backend/internal/poker/service/sweeper.go`, métodos del mismo servicio:
      `Sweep(ctx)` con su primer paso (gracias vencidas de `Presence.Expired` → `Disconnect` y
      publicación), y `Run(ctx, interval)` con un `time.Ticker` que llama a `Sweep`, registra cada
      error con `slog` y sigue con la próxima pasada, sin `panic` (Principio VII) —
      depende de T019, T020, T021, T022
- [ ] T024 [US1] Implementar `RequireEstimationAccess` en
      `backend/internal/poker/delivery/middleware.go` con una única salida `404` y el mensaje
      `La sesión de estimación no existe.` — depende de T014, T022
- [ ] T025 [US1] Declarar en `backend/internal/poker/delivery/dto.go` los esquemas de
      contracts/openapi.yaml `EstimationSession`, `EstimationStory`, `EstimationParticipant`,
      `EstimationRound`, `UserRef`, `OpenSessionExistsResponse` y `SessionClosedResponse`,
      reutilizando `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de la 001. Los campos
      anulables se serializan como `null` y las listas como `[]`, nunca como `null` — depende de T023
- [ ] T026 [US1] Implementar el manejador `openEstimationSession` en
      `backend/internal/poker/delivery/handler.go` y el canal `streamEstimationSession` en
      `backend/internal/poker/delivery/stream.go`, ambos con anotaciones `swag` en español: el canal
      responde `409` con `SessionClosedResponse` si la sesión está cerrada antes de empezar a
      escribir; si no, fija `Content-Type: text/event-stream`, `Cache-Control: no-cache` y
      `X-Accel-Buffering: no`, llama a `Join`, escribe cada vista como `event: state` y
      `data: <JSON>` con `Flush`, un comentario `: ping` cada **15 segundos**, y al terminar el
      contexto de la petición o al cerrarse el canal del hub llama a `Leave`. Mensajes del `409` de
      apertura: `La historia ya tiene una sesión de estimación en curso.`,
      `Una historia completada no se estima.` y
      `El proyecto está finalizado y es de solo lectura.` — depende de T024, T025
- [ ] T027 [US1] Crear en `backend/internal/poker/delivery/routes.go` `RegisterStoryRoutes(router,
      requireSession, requireStoryAccess, handler)` con `POST /api/v1/stories/:storyId/estimation-sessions`
      y `RegisterSessionRoutes(router, requireSession, repo, handler)` con el grupo plano
      `/api/v1/estimation-sessions/:sessionId` y `GET /events`, sin importar `backlog/delivery`; y
      componer el módulo en `backend/cmd/app/main.go`: repositorio, servicio con `time.Now`,
      `ResetPresence` antes de escuchar, `go sessions.Run(ctx, time.Second)` y
      `RequireStoryAccess` de la 003 sobre la misma instancia del repositorio de historias, más el
      `@description` general de la API para que mencione el Planning Poker — depende de T026
- [ ] T028 [P] [US1] Declarar los tipos de petición, respuesta y evento en
      `frontend/src/types/poker.ts` y, en `frontend/src/utils/poker-labels.ts`, las etiquetas en
      español de data-model.md sección 5 (`open` → Abierta, `finished` → Finalizada, `canceled` →
      Cancelada; `voting` → En votación, `revealed` → Revelada; `inactivity` → Por inactividad,
      `story_completed` → Historia completada, `project_finished` → Proyecto finalizado;
      `consensus` → Consenso, `divergence` → Divergencia, `no_numeric_votes` → Sin votos numéricos).
      **Prohibido el tipo `any`** (Principio VIII)
- [ ] T029 [US1] Implementar `frontend/src/lib/event-stream.ts`: lectura de `response.body` con
      `getReader()` y `TextDecoder`, separación por línea en blanco, campos `event:` y `data:`,
      comentarios ignorados, y un callback por evento, sin dependencias — depende de T017
- [ ] T030 [US1] Implementar en `frontend/src/features/poker/api.ts` la apertura sobre el cliente
      `fetch` de la 001 (incluido el `409` con `sessionId`), y en
      `frontend/src/features/poker/use-estimation-stream.ts` el hook que abre el canal con el token
      de `session-storage` y la cabecera `Authorization`, descarta instantáneas de `version` menor
      a la vigente, se detiene ante `404` y ante `401` (`rejectSession`) y aborta la petición al
      desmontar. Antes de escribir código, leer la documentación de Next.js de
      `frontend/node_modules/next/dist/docs/` — depende de T028, T029
- [ ] T031 [US1] Implementar `StoryEstimationSection`, `PokerRoom` (historia y lista) y
      `ParticipantList` en `frontend/src/features/poker/components/` con mensajes en español —
      depende de T030
- [ ] T032 [US1] Crear la pantalla
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/poker/[sessionId]/page.tsx`, protegida
      con la guardia de sesión de la 001, y sumar `StoryEstimationSection` a
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/page.tsx` — depende de T031

**Checkpoint**: US1 funciona y se prueba sola. El MVP está entregable.

---

## Fase 3: Historia de Usuario 2 — Votar en secreto (Prioridad: P1)

**Objetivo**: cada participante juega una carta o "?", la cambia mientras la ronda esté en votación y
nadie ve el valor ajeno.

**Rama**: `hu/005-us2-votar-en-secreto`

**Prueba independiente**: con tres canales abiertos, uno vota `5`; los otros dos reciben
`hasVoted: true` y `votes: null`, y solo el autor recibe `myVote: "5"`.

### Pruebas de la Historia 2 ⚠️

- [ ] T033 [P] [US2] Pruebas unitarias **obligatorias** de `ParseCard` en
      `backend/internal/poker/domain/card_test.go`, cubriendo RN-06 y SC-010: las once cartas
      admitidas aceptadas; `4`, `6`, `7`, `50`, `101`, `-1`, `1.5`, `" 5"` y `"??"` rechazados con un
      error que enumera las cartas en orden — depende de T032
- [ ] T034 [P] [US2] Pruebas unitarias **obligatorias** de validación del voto en
      `backend/internal/poker/delivery/validation_test.go`, cubriendo RN-36: cuerpo
      `{"card":"5"}` → ok; `{"card":5}` (número JSON) y cartas inválidas → `FieldError` en `card` con
      `La carta no es válida. Cartas admitidas: 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?.`; carta
      ausente, `null` o `""` → `FieldError` en `card` con
      `Un voto emitido no se retira: solo se cambia por otra carta, incluida "?".` — depende de T032
- [ ] T035 [P] [US2] Pruebas unitarias **obligatorias** de `ViewFor` (secreto del voto) en
      `backend/internal/poker/domain/view_test.go`, cubriendo RN-08: con la ronda vigente `voting`,
      `votes` es `nil` para todo espectador, incluido el autor; `hasVoted` verdadero solo para quien
      votó; `myVote` lleva la carta del espectador y `nil` si no votó; dos espectadores distintos
      nunca reciben la carta del otro; las rondas anteriores `revealed` muestran sus votos — depende
      de T032
- [ ] T036 [US2] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (005/US2)`: `TestUS2_VotoOculto`,
      `TestUS2_PropiaCartaVisible` (también en una segunda conexión del autor, FR-024),
      `TestUS2_CambioDeVoto` (8 → 13; en la base queda **una** fila con `13`),
      `TestUS2_CartaDesconocida`, `TestUS2_ExtremosDeLaEscala`, `TestUS2_ValorFueraDeEscala`
      (subtests `4`, `50` y `1.5`; el voto anterior no cambia), `TestUS2_RondaRevelada` (prepara la
      ronda `revealed` con el repositorio, porque revelar llega en US3), `TestUS2_IntegranteNoUnido`
      (`403` `No participás de esta sesión de estimación.`) y `TestUS2_RetiroDeVoto` (`400`, el voto
      sigue en 5). `TestUS2_CambioSimultaneoConRevelacion` se escribe en US3 (T047) porque necesita la
      revelación — depende de T032
- [ ] T037 [P] [US2] Pruebas de frontend en
      `frontend/src/features/poker/components/card-deck.test.tsx`, con
      `describe("005/US2 - Votar en secreto")`: el mazo muestra las once cartas, resalta la propia
      (`myVote`), muestra a los demás como "ya votó" sin valor y deshabilita el mazo con la ronda
      revelada

### Implementación de la Historia 2

- [ ] T038 [P] [US2] Implementar `ParseCard` en `backend/internal/poker/domain/card.go` — depende de
      T033
- [ ] T039 [US2] Implementar la validación del cuerpo del voto en
      `backend/internal/poker/delivery/validation.go`, con los dos mensajes como constantes nombradas
      en un único lugar y sin llegar al servicio ante un error (Principio VII) — depende de T034, T038
- [ ] T040 [P] [US2] Completar `ViewFor` en `backend/internal/poker/domain/view.go` con `hasVoted`,
      `myVote` y los votos de las rondas reveladas — depende de T035
- [ ] T041 [US2] Implementar `Vote` en `backend/internal/poker/repository/session_repository.go`:
      bloqueo de la sesión, `EnsureOpen`, participante con `connected = true` (si no →
      `ErrNotParticipant`), ronda vigente `voting` (si no → `ErrRoundRevealed`), upsert de la fila
      única `(round_id, participant_id)` actualizando `card` y `updated_at`, `last_activity_at = now`
      y `version + 1` — depende de T036
- [ ] T042 [US2] Implementar `Vote` en `backend/internal/poker/service/session_service.go`, con
      publicación después de confirmar — depende de T040, T041
- [ ] T043 [US2] Implementar el manejador `castEstimationVote` (`PUT /vote`) con anotaciones `swag`
      en `backend/internal/poker/delivery/handler.go`, la traducción de errores (`403`, `409`
      `La ronda ya está revelada. Esperá una ronda nueva para votar.`) y su registro en
      `backend/internal/poker/delivery/routes.go`; `DTO` `VoteRequest` en `dto.go` — depende de T039,
      T042
- [ ] T044 [US2] Implementar la llamada de voto en `frontend/src/features/poker/api.ts` y
      `CardDeck` en `frontend/src/features/poker/components/card-deck.tsx`, con la marca "ya votó"
      en `ParticipantList` — depende de T037, T043

**Checkpoint**: US1 y US2 funcionan; el voto es secreto.

---

## Fase 4: Historia de Usuario 3 — Revelar los votos y ver las diferencias (Prioridad: P1)

**Objetivo**: la ronda se revela sola cuando votaron todos los conectados, o la revela el
facilitador; todos ven los votos, el consenso o la divergencia con mínimo y máximo y sus autores.

**Rama**: `hu/005-us3-revelar-los-votos-y-ver-las-diferencias`

**Prueba independiente**: tres participantes votan 3, 5 y 13; al tercer voto los tres canales
reciben la ronda `revealed` con `divergence`, mínimo 3 y máximo 13 con sus autores.

### Pruebas de la Historia 3 ⚠️

- [ ] T045 [P] [US3] Pruebas unitarias **obligatorias** de `ShouldAutoReveal` y `CanRevealManually`
      en `backend/internal/poker/domain/reveal_test.go`, cubriendo RN-09, RN-10, RN-12, RN-16, RN-23 y
      SC-006: 1, 2 y 5 conectados con todos sus votos → verdadero; un conectado sin voto → falso; un
      desconectado sin voto no impide; un desconectado con voto y nadie conectado → falso; `?` cuenta
      como voto; el voto del facilitador cuenta igual; `CanRevealManually` sin votos →
      `ErrNoVotesToReveal` y con un voto de un desconectado → ok — depende de T044
- [ ] T046 [P] [US3] Pruebas unitarias **obligatorias** de `Summarize` en
      `backend/internal/poker/domain/outcome_test.go` con la tabla completa de data-model.md sección
      6, cubriendo RN-12 a RN-14, SC-008 y SC-009, más el orden de los autores por nombre y que el
      resultado no tiene ningún campo de promedio ni mediana (FR-042) — depende de T044
- [ ] T047 [US3] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una
      por escenario con `// Escenario: <nombre> (005/US3)`: `TestUS3_RevelacionAutomatica`,
      `TestUS3_RevelacionManualConConsenso` (`notVoted` con el cuarto),
      `TestUS3_InterrogacionFueraDelCalculo`, `TestUS3_ConsensoSinExtremos`,
      `TestUS3_UnicoParticipante`, `TestUS3_TodosInterrogacion`,
      `TestUS3_DesconexionDisparaRevelacion` (el cuarto cierra su canal, reloj + 30 s, `Sweep`),
      `TestUS3_AutoresMultiples`, `TestUS3_RevelarSinVotos` (`409`
      `Hace falta al menos un voto para revelar la ronda.`) y `TestUS3_RevelarSinSerFacilitador`
      (`403` `Esta acción está reservada al facilitador de la sesión.`, votos ocultos); y, con
      `// Escenario: … (005/US2)`, `TestUS2_CambioSimultaneoConRevelacion` — depende de T044
- [ ] T048 [US3] Prueba de concurrencia `TestPokerVoteReveal_Concurrency` en
      `backend/internal/poker/delivery/poker_concurrency_test.go`, **sin** etiqueta: 20 repeticiones
      en las que un participante cambia su carta y el facilitador revela a la vez; ninguna `500`, y
      las vistas de los tres canales tienen la misma `version` y el mismo conjunto de votos revelados
      (SC-007) — depende de T047
- [ ] T049 [P] [US3] Pruebas de frontend en
      `frontend/src/features/poker/components/round-result.test.tsx` y
      `frontend/src/features/poker/components/facilitator-actions.test.tsx`, con
      `describe("005/US3 - Revelar los votos y ver las diferencias")`: consenso con su valor,
      divergencia con mínimo y máximo y todos sus autores, "sin votos numéricos", "sin voto" para
      `notVoted`, y el botón **Revelar** visible solo para el facilitador

### Implementación de la Historia 3

- [ ] T050 [P] [US3] Implementar `ShouldAutoReveal` y `CanRevealManually` en
      `backend/internal/poker/domain/reveal.go` — depende de T045
- [ ] T051 [P] [US3] Implementar `RoundOutcome` y `Summarize` en
      `backend/internal/poker/domain/outcome.go` — depende de T046
- [ ] T052 [US3] Implementar `Reveal` en `backend/internal/poker/repository/session_repository.go`
      (bloqueo, `EnsureOpen`, participante conectado, `facilitator_id` igual al titular o
      `ErrFacilitatorOnly`, ronda `voting` o `ErrRoundRevealed`, `CanRevealManually`, `status =
      revealed`, `revealed_at = now`, hecho) y agregar la evaluación de `ShouldAutoReveal`, con la
      misma escritura de revelación, al final de `Vote`, `Connect` y `Disconnect` dentro de su
      transacción (FR-031) — depende de T047, T050
- [ ] T053 [US3] Completar `ViewFor` con `outcome` (desde `Summarize`) en las rondas reveladas y
      `notVoted` en la ronda vigente revelada de una sesión abierta, en
      `backend/internal/poker/domain/view.go`, e implementar `Reveal` en
      `backend/internal/poker/service/session_service.go` — depende de T051, T052
- [ ] T054 [US3] Implementar el manejador `revealEstimationRound` (`POST /reveal`) con anotaciones
      `swag` en `backend/internal/poker/delivery/handler.go`, los esquemas `RoundOutcome` y
      `OutcomeExtreme` en `dto.go` y la ruta en `routes.go` — depende de T053
- [ ] T055 [US3] Implementar `RoundResult` y `FacilitatorActions` (revelar) en
      `frontend/src/features/poker/components/` y la llamada en `api.ts` — depende de T049, T054

**Checkpoint**: las tres historias P1 funcionan; el valor central de la feature está entregado.

---

## Fase 5: Historia de Usuario 4 — Repetir la ronda hasta acercar posiciones (Prioridad: P2)

**Objetivo**: el facilitador abre una ronda nueva sobre una revelada; la anterior queda en el
historial de la sesión.

**Rama**: `hu/005-us4-repetir-la-ronda`

**Prueba independiente**: con la ronda 1 revelada, `POST /rounds` devuelve `201` con la ronda 2
`voting` y nadie con voto, y `GET /estimation-sessions/{id}` muestra la ronda 1 con sus votos.

### Pruebas de la Historia 4 ⚠️

- [ ] T056 [US4] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (005/US4)`: `TestUS4_NuevaRonda`,
      `TestUS4_HistorialDeRondas` (con `GET /estimation-sessions/{id}`),
      `TestUS4_NuevaRondaTrasConsenso`, `TestUS4_IncorporacionEnRondaPosterior`,
      `TestUS4_SinLimiteDeRondas` (once rondas; las diez anteriores completas),
      `TestUS4_FacilitadorSoloEnNuevaRonda`, `TestUS4_RondaSinRevelar` (`409`
      `Primero hay que revelar la ronda vigente.`) y `TestUS4_NuevaRondaSinSerFacilitador` (`403`).
      Además, **sin** etiqueta, `TestPokerJoin_RondaRevelada`: con la ronda 1 revelada, un integrante
      que no había participado abre su canal y recibe los votos revelados y el resultado; su voto
      responde `409` de ronda revelada, y tras la ronda nueva puede votar (Casos Límite, FR-017) —
      depende de T055
- [ ] T057 [P] [US4] Pruebas de frontend en
      `frontend/src/features/poker/components/round-history.test.tsx`, con
      `describe("005/US4 - Repetir la ronda hasta acercar posiciones")`: la sala muestra las rondas
      anteriores con sus votos y resultado, la ronda vigente sin votos, y el botón **Nueva ronda**
      solo para el facilitador con la ronda revelada

### Implementación de la Historia 4

- [ ] T058 [US4] Implementar `StartRound` en
      `backend/internal/poker/repository/session_repository.go` (bloqueo, `EnsureOpen`, participante
      conectado, facilitador, ronda vigente `revealed` o `ErrRoundNotRevealed`, inserción de la ronda
      con `number + 1` en `voting`, hecho) — depende de T056
- [ ] T059 [US4] Implementar `StartRound` y `Get(ctx, access, viewerID)` en
      `backend/internal/poker/service/session_service.go`, y los manejadores
      `startEstimationRound` (`POST /rounds`, `201`) y `getEstimationSession` (`GET` del grupo plano)
      con anotaciones `swag` en `handler.go` y sus rutas en `routes.go` — depende de T058
- [ ] T060 [US4] Implementar `RoundHistory` y el botón **Nueva ronda** de `FacilitatorActions` en
      `frontend/src/features/poker/components/` y las llamadas en `api.ts` — depende de T057, T059

**Checkpoint**: US1 a US4 funcionan.

---

## Fase 6: Historia de Usuario 5 — Registrar la estimación acordada (Prioridad: P2)

**Objetivo**: el facilitador registra un valor de la escala; la sesión queda Finalizada y los Story
Points de la historia toman ese valor.

**Rama**: `hu/005-us5-registrar-la-estimacion-acordada`

**Prueba independiente**: con una ronda revelada, `POST /finalize {"storyPoints":8}` devuelve
`status: finished` y `GET /api/v1/stories/{id}` de la 003 muestra 8 Story Points.

### Pruebas de la Historia 5 ⚠️

- [ ] T061 [P] [US5] Pruebas unitarias **obligatorias** de `ValidateAgreedValue` en
      `backend/internal/poker/domain/card_test.go`, cubriendo RN-18: los diez valores de la escala
      aceptados; 7, 4, -1 y 101 rechazados con el mensaje que enumera la escala — depende de T060
- [ ] T062 [P] [US5] Pruebas unitarias **obligatorias** de validación del cuerpo de registro en
      `backend/internal/poker/delivery/validation_test.go`: `{"storyPoints":8}` → ok;
      `{"storyPoints":"?"}`, `{"storyPoints":null}`, cuerpo vacío, `7.5` y `7` → `FieldError` en
      `storyPoints` con
      `El valor acordado debe ser uno de 0, 1, 2, 3, 5, 8, 13, 20, 40 o 100.` — depende de T060
- [ ] T063 [US5] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (005/US5)`: `TestUS5_RegistroConConsenso` (los canales
      reciben el último `state` y se cierran), `TestUS5_RegistroSinConsenso`,
      `TestUS5_ValorNoVotado`, `TestUS5_ReemplazaEstimacionAnterior`,
      `TestUS5_ReestimacionDirectaDuranteLaSesion` (la reestimación con
      `PUT /api/v1/stories/{id}/story-points` no sube la `version` de la sesión ni produce un
      `state` con otra versión; al registrar 8 la historia queda en 8),
      `TestUS5_RegistroTrasTodosInterrogacion`, `TestUS5_HistoriaSinCriteriosNoQuedaLista` (leído
      con el detalle de la 003), `TestUS5_RondasConservadas` (subtests de 1, 3 y 10 rondas, SC-016),
      `TestUS5_ValorInvalido` (subtests 7, decimal y `null`; sesión `open` e historia sin cambios),
      `TestUS5_SinRondaRevelada` (`409`
      `Primero hay que revelar al menos una ronda o cancelar la sesión.`) y
      `TestUS5_RegistroSinSerFacilitador` (`403`, Story Points sin cambios) — depende de T060
- [ ] T064 [P] [US5] Pruebas de frontend en
      `frontend/src/features/poker/components/finalize-dialog.test.tsx` y
      `frontend/src/features/poker/components/session-closed-notice.test.tsx`, con
      `describe("005/US5 - Registrar la estimación acordada")`: el diálogo ofrece solo los diez
      valores numéricos (sin "?" ni "sin estimar"), y la sala cerrada muestra "Finalizada" con el
      valor acordado

### Implementación de la Historia 5

- [ ] T065 [P] [US5] Implementar `ValidateAgreedValue` en `backend/internal/poker/domain/card.go`
      sobre `backlogdomain.IsValidStoryPoints` — depende de T061
- [ ] T066 [US5] Implementar la validación del cuerpo de registro en
      `backend/internal/poker/delivery/validation.go` — depende de T062, T065
- [ ] T067 [US5] Implementar `Finalize` en
      `backend/internal/poker/repository/session_repository.go`: bloqueo de la sesión, `EnsureOpen`,
      participante conectado, facilitador, al menos una ronda `revealed` en la sesión o
      `ErrNoRevealedRound`; **después** bloqueo de la fila de `user_stories` (orden sesión →
      historia, research.md sección 6); `status = finished`, `agreed_story_points`,
      `closed_by_id = titular`, `closed_at = now`, `version + 1` y, en la **misma transacción**,
      `UPDATE user_stories SET story_points = ?, updated_at = ?` — depende de T063
- [ ] T068 [US5] Implementar `Finalize` en `backend/internal/poker/service/session_service.go`
      (publica el estado final y llama a `hub.CloseSession`) y el manejador
      `finalizeEstimationSession` con anotaciones `swag`, `FinalizeRequest` en `dto.go` y la ruta en
      `routes.go`; el canal (`stream.go`) termina sin error cuando el hub cierra su canal —
      depende de T066, T067
- [ ] T069 [US5] Implementar `FinalizeDialog` y `SessionClosedNotice` en
      `frontend/src/features/poker/components/`, la llamada en `api.ts` y la detención del hook al
      recibir un `state` cerrado — depende de T064, T068

**Checkpoint**: una sesión completa termina en un valor de Story Points en el backlog.

---

## Fase 7: Historia de Usuario 6 — Sobrevivir a las desconexiones y relevar al facilitador (Prioridad: P2)

**Objetivo**: la sesión tolera cortes; quien vuelve recupera su voto; pasados 5 minutos sin el
facilitador, otro participante puede tomar el rol. La membresía revocada y los cambios de la historia
se reflejan en la sesión.

**Rama**: `hu/005-us6-desconexiones-y-relevo-del-facilitador`

**Prueba independiente**: el facilitador cierra su canal; antes de 5 minutos `POST /facilitator`
responde `409` con `retryAfterSeconds`; a los 5 minutos responde `200` y todos los canales ven al
nuevo facilitador.

### Pruebas de la Historia 6 ⚠️

- [ ] T070 [P] [US6] Pruebas unitarias **obligatorias** de `FacilitatorTakeover` en
      `backend/internal/poker/domain/facilitator_test.go`, cubriendo RN-24, RN-25, RN-26 y RN-35 y
      SC-015: facilitador conectado → `FacilitatorUnavailableError` con `RetryAfter` cero;
      desconectado hace 2 min → `RetryAfter` 3 min; 4:59 → 1 s; 5:00 y 7:00 → ok; quien pide ya es
      el facilitador → `FacilitatorUnavailableError` — depende de T069
- [ ] T071 [US6] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (005/US6)`: `TestUS6_DesconexionDisparaRevelacion`,
      `TestUS6_ReconexionConVoto`, `TestUS6_ReconexionSinVoto`,
      `TestUS6_ReconexionEnRondaPosterior`, `TestUS6_FacilitadorRetomaElRol` (se reconecta a los 2
      minutos y revela, inicia ronda y registra en subtests; el subtest de cancelar se agrega en
      T079),
      `TestUS6_TodosDesconectados`, `TestUS6_FacilitadorOriginalRelevado`,
      `TestUS6_UnicoParticipanteTomaElRol`, `TestUS6_DosPestanas` (SC-024: cierra una de dos y la
      revelación se dispara con un voto por persona), `TestUS6_FacilitadorPresente` (`409`
      `El facilitador está conectado.`), `TestUS6_RolTodaviaNoDisponible` (`409` con
      `retryAfterSeconds`) y `TestUS6_TomasSimultaneas` (dos pedidos a la vez: uno `200`, el otro
      `409` con el nuevo facilitador, un solo cambio de `facilitator_id`). Además, **sin** etiqueta,
      `TestPokerSweep_MembresiaRevocada` (la 002 da de baja a un participante conectado: su canal
      recibe `removed` y se cierra, deja de contar para la revelación y sus votos quedan; FR-019) y
      `TestPokerSweep_CambioDeHistoria` (la 003 cambia título y criterios: los canales reciben un
      `state` con el texto nuevo y la misma `version`; FR-074). Y
      `TestPokerPresence_ReinicioDelServidor`, **sin** etiqueta: con dos participantes conectados en
      una sesión abierta y otro en una sesión cerrada, `ResetPresence` deja desconectados solo a los
      de la sesión abierta, con `disconnected_at` igual al instante recibido y sin cambiar `version`
      ni `last_activity_at`; al reabrir el canal, cada uno se reincorpora como un hecho nuevo
      (research.md sección 9) — depende de T069
- [ ] T072 [US6] Prueba de concurrencia `TestPokerFacilitator_Concurrency` en
      `backend/internal/poker/delivery/poker_concurrency_test.go`, **sin** etiqueta: diez
      participantes piden el rol a la vez con el facilitador desconectado hace más de 5 minutos;
      exactamente uno `200`, nueve `409` con el facilitador vigente y ninguna `500` (SC-025) —
      depende de T071
- [ ] T073 [P] [US6] Pruebas de frontend en
      `frontend/src/features/poker/components/take-facilitator-button.test.tsx` y
      `frontend/src/features/poker/use-estimation-stream.test.tsx`, con
      `describe("005/US6 - Sobrevivir a las desconexiones y relevar al facilitador")`: el botón
      **Tomar el rol** aparece solo con `facilitatorAvailableAt` vencido y muestra cuánto falta antes;
      el hook reconecta tras un corte con espera de 1, 2 y 5 segundos (tope 5) y se detiene ante
      `409`, `404` y `401`

### Implementación de la Historia 6

- [ ] T074 [P] [US6] Implementar `FacilitatorTakeover` en
      `backend/internal/poker/domain/facilitator.go`, con los **5 minutos** como constante nombrada —
      depende de T070
- [ ] T075 [US6] Implementar `TakeFacilitator` y `ListRevokedParticipants` en
      `backend/internal/poker/repository/session_repository.go` (`TakeFacilitator`: bloqueo,
      `EnsureOpen`, participante conectado, `FacilitatorTakeover` sobre la fila del facilitador
      releída, `facilitator_id = titular`, hecho) y completar `ViewFor` con `facilitatorAvailableAt`
      (`disconnected_at + 5 min` si el facilitador está desconectado) en
      `backend/internal/poker/domain/view.go` — depende de T071, T074
- [ ] T076 [US6] Implementar `TakeFacilitator` en `backend/internal/poker/service/session_service.go`
      y sumar a `Sweep`, en `backend/internal/poker/service/sweeper.go`, los pasos de membresía
      revocada (`Disconnect`, evento `removed` con
      `hub.CloseUser`) y de cambios de la historia (comparación de `user_stories.updated_at` con el
      último visto por sesión; si cambió, `publish` sin hecho); exponer el evento `removed` en
      `stream.go`; manejador `takeEstimationFacilitator` con anotaciones `swag`,
      `FacilitatorUnavailableResponse` en `dto.go` y la ruta en `routes.go`. `ListOpenForSweep` del
      repositorio se implementa en esta tarea con las columnas que el paso de cambios necesita —
      depende de T072, T075
- [ ] T077 [US6] Implementar la reconexión con espera creciente en
      `frontend/src/features/poker/use-estimation-stream.ts`, el aviso de `removed` y
      `TakeFacilitatorButton` en `frontend/src/features/poker/components/` con su llamada en
      `api.ts` — depende de T073, T076

**Checkpoint**: la sesión sobrevive a cortes y nunca queda bloqueada por un facilitador ausente.

---

## Fase 8: Historia de Usuario 7 — Cancelar una sesión sin registrar estimación (Prioridad: P3)

**Objetivo**: el facilitador cancela sin tocar la estimación; el sistema cancela solo las sesiones
inactivas 60 minutos, las de historias completadas y las de proyectos finalizados.

**Rama**: `hu/005-us7-cancelar-una-sesion`

**Prueba independiente**: `POST /cancel` deja la sesión `canceled` con la historia intacta, y una
sesión sin hechos durante 60 minutos de reloj falso queda `canceled` con `cancelReason: inactivity`
tras `Sweep`.

### Pruebas de la Historia 7 ⚠️

- [ ] T078 [P] [US7] Pruebas unitarias **obligatorias** de `AutoCancelReason` en
      `backend/internal/poker/domain/auto_cancel_test.go` con la tabla de data-model.md sección 6,
      cubriendo RN-30, RN-31 y SC-021: historia `completed` → `story_completed`; proyecto `finished`
      → `project_finished`; las dos a la vez → `story_completed` (precedencia); 59:59 → ninguno; 60:00
      → `inactivity` — depende de T077
- [ ] T079 [US7] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (005/US7)`: `TestUS7_CancelacionConservaEstimacion`,
      `TestUS7_NuevaSesionTrasCancelar`, `TestUS7_RondasConservadas`, `TestUS7_CancelacionSinVotos`,
      `TestUS7_CancelacionPorInactividad` (reloj + 60 min, `Sweep`; los canales reciben
      `canceled` con `inactivity` y se cierran; se puede abrir una sesión nueva),
      `TestUS7_HechoReiniciaInactividad` (un subtest por tipo de hecho: incorporación, desconexión,
      voto, revelación, ronda nueva y cambio de facilitador, cada uno a los 59 minutos; a los 60 desde
      el inicio la sesión sigue `open`), `TestUS7_CancelarSinSerFacilitador` (`403`),
      `TestUS7_CancelarSesionFinalizada` (`409` de sesión cerrada, valor intacto) y
      `TestUS7_AccionTrasInactividad` (`409` `La sesión de estimación se cerró por inactividad.` al
      votar y al registrar). Además, **sin** etiqueta, `TestPokerAutoCancel_HistoriaCompletada` (avance
      a `completed` con la 004 → `story_completed` tras `Sweep`),
      `TestPokerAutoCancel_ProyectoFinalizado` (finalización con la 002 → `project_finished`; un voto
      posterior responde `409` con `El proyecto está finalizado y es de solo lectura.`),
      `TestPokerJoin_SesionCerrada` (subtests con una sesión `finished` con valor 8 y otra `canceled`
      por el facilitador: el canal responde `409` con `SessionClosedResponse`, `status` y
      `agreedStoryPoints`, no envía ningún evento y no crea ningún participante; FR-018) y
      `TestPokerAutoCancel_AccionAntesDelBarrido` (votar después de completar la historia y antes de
      `Sweep` → `409` de sesión cerrada con `story_completed`). Agrega también a
      `TestUS6_FacilitadorRetomaElRol` el subtest de cancelar, con su comentario `(005/US6)`, que
      T071 no pudo escribir porque la cancelación llega en esta historia — depende de T077
- [ ] T080 [P] [US7] Pruebas de frontend en
      `frontend/src/features/poker/components/session-closed-notice.test.tsx` y
      `frontend/src/features/poker/components/facilitator-actions.test.tsx`, con
      `describe("005/US7 - Cancelar una sesión sin registrar estimación")`: el botón **Cancelar
      sesión** pide confirmación y es solo del facilitador; el aviso de cierre muestra el motivo en
      español para cada `cancelReason`

### Implementación de la Historia 7

- [ ] T081 [P] [US7] Implementar `AutoCancelReason` en `backend/internal/poker/domain/auto_cancel.go`,
      con los **60 minutos** como constante nombrada — depende de T078
- [ ] T082 [US7] Implementar `Cancel` y `AutoCancel` en
      `backend/internal/poker/repository/session_repository.go`, completar `ListOpenForSweep` con el
      estado de la historia, el del proyecto y `last_activity_at`, y extraer el paso común de todas
      las escrituras sobre una sesión abierta (data-model.md sección 9: bloqueo, `EnsureOpen`,
      `AutoCancelReason` con cancelación confirmada y `SessionClosedError`) para que `Connect`,
      `Disconnect`, `Vote`, `Reveal`, `StartRound`, `Finalize` y `TakeFacilitator` lo usen. En la
      cancelación del facilitador `closed_by_id = titular` y `cancel_reason = NULL`; en la del
      sistema `closed_by_id = NULL` y el motivo — depende de T079, T081
- [ ] T083 [US7] Implementar `Cancel` en `backend/internal/poker/service/session_service.go`, sumar
      a `Sweep`, en `sweeper.go`, el paso de cancelaciones automáticas (publicar y
      `hub.CloseSession`), publicar
      también cuando una acción devuelve `SessionClosedError` por cancelación automática, y el
      manejador `cancelEstimationSession` con anotaciones `swag` y su ruta; el `409` de sesión
      cerrada usa `La sesión de estimación se cerró por inactividad.` cuando el motivo es
      `inactivity`, `El proyecto está finalizado y es de solo lectura.` cuando es
      `project_finished` y `La sesión de estimación ya está cerrada.` en los demás casos (research.md
      sección 19) — depende de T082
- [ ] T084 [US7] Implementar el botón **Cancelar sesión** de `FacilitatorActions` y los motivos de
      `SessionClosedNotice` en `frontend/src/features/poker/components/`, con la llamada en `api.ts`
      — depende de T080, T083

**Checkpoint**: ninguna sesión queda abierta para siempre bloqueando su historia.

---

## Fase 9: Historia de Usuario 8 — Consultar el historial de estimación de una historia (Prioridad: P3)

**Objetivo**: cualquier integrante ve las sesiones de una historia, sus rondas, quién votó qué y el
valor acordado; una historia con historial no se elimina.

**Rama**: `hu/005-us8-consultar-el-historial-de-estimacion`

**Prueba independiente**: con dos sesiones cerradas, `GET /api/v1/stories/{id}/estimation-sessions`
devuelve ambas de la más reciente a la más antigua, y `DELETE /api/v1/stories/{id}` de la 003
responde `409` con el motivo `estimation_sessions`.

### Pruebas de la Historia 8 ⚠️

- [ ] T085 [US8] Pruebas de integración en `backend/internal/poker/delivery/handler_test.go`, una por
      escenario con `// Escenario: <nombre> (005/US8)`: `TestUS8_SesionFinalizadaConRondas`,
      `TestUS8_OrdenDeSesiones`, `TestUS8_SesionCancelada`, `TestUS8_SesionEnCurso` (`openSession`
      con facilitador y número de ronda, sin votos), `TestUS8_HistoriaSinSesiones` (`200`,
      `sessions: []`, `emptyReason: no_sessions`), `TestUS8_ExIntegrante` (baja con la 002; su
      nombre sigue en los votos), `TestUS8_ProyectoFinalizado`, `TestUS8_NoIntegrante` (`404` con el
      mismo cuerpo que un `storyId` inexistente) y `TestUS8_EliminacionBloqueada` (una historia con
      una sesión `finished` y otra con una `canceled`: el `DELETE` de la 003 responde `409` con
      `estimation_sessions` y el historial sigue consultable) — depende de T084
- [ ] T086 [P] [US8] Prueba de integración del puerto reemplazado en
      `backend/internal/backlog/repository/estimation_sessions_test.go`, cubriendo RN-33 y SC-023:
      sin sesiones → `false`; con una sesión `open`, `finished` o `canceled` → `true`; las sesiones de
      otra historia no cuentan — depende de T084
- [ ] T087 [P] [US8] Pruebas de frontend en
      `frontend/src/features/poker/components/story-estimation-section.test.tsx`, con
      `describe("005/US8 - Consultar el historial de estimación de una historia")`: la sección lista
      las sesiones con estado, valor y cantidad de rondas, muestra "Sesión en curso" con **Unirse**,
      y el mensaje de historia sin sesiones

### Implementación de la Historia 8

- [ ] T088 [US8] Reemplazar el cuerpo de `backend/internal/backlog/repository/estimation_sessions.go`:
      `NewEstimationSessions(db *gorm.DB)` y `HasSessions` con un `EXISTS` sobre
      `estimation_sessions` por `story_id`, en cualquier estado, usando el modelo
      `pokerdomain.EstimationSession`; actualizar la llamada en `backend/cmd/app/main.go` y en los
      ayudantes de prueba de `backend/internal/backlog/` que construyan el servicio. `backlog/service`
      no cambia (research.md sección 15.1) — depende de T086
- [ ] T089 [US8] Implementar `ListForStory` (sesiones cerradas ordenadas por `created_at`
      descendente con el conteo de rondas por subconsulta, en una consulta) y `FindOpenForStory`
      (identificador, facilitador, número y estado de la ronda vigente, o `nil`) en
      `backend/internal/poker/repository/session_repository.go`; `History` en el servicio, con
      `emptyReason` `no_sessions` solo si no hay ninguna sesión ni abierta ni cerrada; el manejador
      `listStoryEstimationSessions` con anotaciones `swag`, los esquemas
      `StoryEstimationHistory`, `EstimationSessionSummary` y `OpenEstimationSession` en `dto.go`, y
      `GET` en `RegisterStoryRoutes` — depende de T085
- [ ] T090 [US8] Implementar el historial en `StoryEstimationSection` (lista, sesión en curso con
      **Unirse**, vacío) y la vista de una sesión cerrada en la pantalla
      `frontend/src/app/projects/[projectId]/backlog/[storyId]/poker/[sessionId]/page.tsx`
      reutilizando `RoundHistory` y `SessionClosedNotice` — depende de T087, T089

**Checkpoint**: las ocho historias funcionan y el historial de estimación está protegido.

---

## Fase 10: Polish (Cierre y asuntos transversales)

**Propósito**: pruebas transversales de seguridad y escala, cierre del canal al vencer el token,
documentación viva, cobertura, vulnerabilidades y verificación de las puertas de calidad.

**Rama**: `fase/005-polish`

- [ ] T091 Pruebas de integración de caracterización en
      `backend/internal/poker/delivery/handler_test.go`, **sin** etiqueta `005/USn`:
      `TestPokerEndpoints_SinSesion`, con un subtest por cada uno de los **diez** endpoints sin
      cabecera y con token adulterado → `401` y ningún cambio (SC-017);
      `TestPokerEndpoints_NotFoundIndistinguible`, que compara byte a byte el `404` de sesión ajena e
      inexistente en las **ocho** rutas de sesión y el de historia ajena e inexistente en las **dos**
      rutas de historia (SC-018); `TestPokerSecreto_TodasLasConsultas`, que con una ronda en votación
      pide la sesión por el canal, por `GET /estimation-sessions/{id}`, por la respuesta de cada
      acción y por el historial de la historia desde una cuenta distinta de la del autor, y verifica
      que ninguna contiene la carta (SC-005); y una aserción de que ninguna respuesta contiene
      `password_hash` ni `passwordHash` (Principio VI). Prueba de caracterización: las reglas ya
      existen, así que no tiene RED; el commit lo justifica — depende de T090
- [ ] T092 Prueba de escala `TestPokerStream_Scale` en
      `backend/internal/poker/delivery/poker_scale_test.go`, **sin** etiqueta, omitida con
      `testing.Short()`: 10 sesiones con 10 canales cada una (100 conexiones); un voto por sesión;
      las 100 conexiones reciben la instantánea con la nueva `version` en menos de 2 segundos medidos
      desde la respuesta del voto (SC-004, RC-14). Caracterización, sin RED — depende de T090
- [ ] T093 Prueba en `backend/internal/poker/delivery/stream_test.go`: con un token que vence en 2
      segundos de reloj real, el canal se cierra al vencer y no vuelve a recibir instantáneas
      (research.md punto abierto 5) — depende de T090
- [ ] T094 Exportar en `backend/internal/auth/delivery/middleware.go` el accesor
      `CurrentSessionExpiry(c *gin.Context) (time.Time, bool)` sobre los claims que ya deja el
      middleware de sesión, y usarlo en `backend/internal/poker/delivery/stream.go` para cerrar el
      canal con un temporizador al vencer el token — depende de T093
- [ ] T095 Regenerar la documentación con `swag init -g cmd/app/main.go -o docs` y verificar que
      `backend/docs/` coincida con [contracts/openapi.yaml](./contracts/openapi.yaml) en las **diez**
      operaciones (el canal como `text/event-stream`), sus códigos de respuesta (todas declaran
      `500`) y sus esquemas; corregir las anotaciones si difieren (Principio VIII) — depende de T094
- [ ] T096 [P] Actualizar `README.md` de la raíz en español: la sección introductoria menciona el
      Planning Poker en tiempo real, y se agregan los diez endpoints, el canal de eventos, la
      pantalla de la sala y la sección del detalle de la historia, y el supuesto de una sola
      instancia del backend, manteniendo el criterio de que alguien que no conoce el proyecto pueda
      ejecutarlo siguiendo el documento — depende de T094
- [ ] T097 Ejecutar `govulncheck ./...` en `backend/` y resolver o justificar por escrito cada
      vulnerabilidad reportada
- [ ] T098 [P] Ejecutar los comandos de cobertura de los dos lados —
      `go test ./... -coverprofile=coverage.out` más `go tool cover -func=coverage.out` en
      `backend/`, y `npm run test:coverage` en `frontend/` — y revisar que `internal/poker/`,
      `frontend/src/features/poker/` y `frontend/src/lib/event-stream.ts` estén cubiertos; correr
      además `go test -race ./internal/poker/...` — depende de T094
- [ ] T099 Verificar la trazabilidad y las puertas de seguridad recorriendo `backend/internal/poker/`
      y `frontend/src/features/poker/`: que buscar `005/US1` a `005/US8` en el código de pruebas
      devuelva los **79** escenarios; que las **36** reglas RN-01 a RN-36 tengan prueba asociada según
      la tabla de plan.md (SC-020); que los **diez** endpoints tengan su prueba de `401` (SC-017); que
      los `404` sean indistinguibles (SC-018); que `ViewFor` sea la única salida de una sesión hacia
      la API; que esta feature solo escriba `user_stories.story_points` y `updated_at` fuera de sus
      tablas; que no haya dependencias nuevas en `go.mod` ni en `package.json`; y que ninguna
      respuesta incluya `password_hash` — depende de T091, T092
- [ ] T100 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluido el
      flujo en la interfaz con tres navegadores, las cancelaciones del sistema, las pruebas de
      concurrencia con `-count=5` y la verificación de que las pruebas corrieron contra `smye_test`
      en el puerto 5433 y no contra desarrollo — depende de T095, T096, T099

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fase 1 Foundational**: parte de `main` con la 001 a la 004 integradas. **BLOQUEA** todas las
  historias.
- **Fases 2 a 9 (historias)**: todas dependen de la Fase 1, en orden de prioridad
  (P1 → P1 → P1 → P2 → P2 → P2 → P3 → P3).
- **Fase 10 Polish**: depende de que estén completas las ocho historias.

### Dependencias entre historias

Las historias **no son todas independientes en su implementación**, aunque cada una se prueba sola:

- **US1 Abrir y reunir (P1)**: puede empezar después de la Fase 1. **Aporta** el entorno de pruebas
  con el lector del canal (T015), `Presence`, el hub, `ViewFor`, `FindSnapshot`, el servicio con su
  firma definitiva y `publish`, el barrido, el middleware, el canal y la composición en
  `cmd/app/main.go` (T019 a T027), que usan todas las demás.
- **US2 Votar (P1)**: necesita una sesión abierta y el canal de US1.
- **US3 Revelar (P1)**: necesita los votos de US2. **Aporta** la revelación que usan US4 y US5.
- **US4 Repetir la ronda (P2)**: necesita rondas reveladas de US3. **Aporta** el `GET` de la sesión.
- **US5 Registrar (P2)**: necesita una ronda revelada de US3.
- **US6 Desconexiones y relevo (P2)**: necesita US1 a US3 y, para el subtest de registrar, US5. El
  subtest de cancelar del facilitador reconectado se escribe en T079 (US7).
- **US7 Cancelar (P3)**: necesita US1; el reinicio de inactividad por cada tipo de hecho usa US2,
  US3, US4 y US6.
- **US8 Historial (P3)**: necesita sesiones cerradas de US5 y US7.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Dominio antes que repositorio; repositorio antes que servicio; servicio antes que `delivery`.
- Backend antes que la pantalla de frontend que lo consume.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 1: T001 a T006 en paralelo; después T007; T008 y T009 en secuencia.
- Dentro de cada historia: las pruebas unitarias de dominio, las de `Presence` y del hub, las del
  middleware, la del puerto de la 003 y las de frontend marcadas `[P]` en paralelo entre sí. Las de
  integración **no** son paralelizables entre historias porque las ocho comparten
  `handler_test.go`, ni las de concurrencia porque comparten `poker_concurrency_test.go`.
- En las implementaciones, las funciones puras de dominio marcadas `[P]` (cada una en su archivo)
  en paralelo; repositorio, servicio y `delivery` en secuencia, porque cada historia toca
  `session_repository.go`, `session_service.go`, `handler.go` y `routes.go`.
- Fase 10: T096 y T098 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 1

```text
# Las tareas de pruebas marcadas [P] pueden escribirse a la vez:
T010  backend/internal/poker/domain/status_test.go
T011  backend/internal/poker/service/presence_test.go
T012  backend/internal/poker/service/hub_test.go
T013  backend/internal/poker/domain/view_test.go
T014  backend/internal/poker/delivery/middleware_test.go
T017  frontend/src/lib/event-stream.test.ts + participant-list + story-estimation-section

# T015 (handler_test.go + testenv_test.go) va aparte: lo comparten las ocho historias.

# En la implementación, estas no se pisan:
T018  backend/internal/poker/domain/status.go
T019  backend/internal/poker/service/presence.go
T020  backend/internal/poker/service/hub.go
T021  backend/internal/poker/domain/view.go
T028  frontend/src/types/poker.ts + frontend/src/utils/poker-labels.ts
```

## Ejemplo de paralelismo: Historia de Usuario 3

```text
# Pruebas en archivos distintos:
T045  backend/internal/poker/domain/reveal_test.go
T046  backend/internal/poker/domain/outcome_test.go
T049  frontend/src/features/poker/components/round-result.test.tsx + facilitator-actions.test.tsx

# Implementación de las reglas puras:
T050  backend/internal/poker/domain/reveal.go
T051  backend/internal/poker/domain/outcome.go
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar la Fase 1 (Foundational) — **CRÍTICO**: bloquea todas las historias.
2. Completar la Fase 2 (US1 Abrir una sesión y reunir al equipo).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con la guía de validación (canales con
   `curl.exe -N`).
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Foundational → base lista (una fase técnica, un pull request).
2. US1 Abrir y reunir → probar sola → entregar (**MVP**: el tiempo real funciona).
3. US2 Votar → probar sola → entregar (voto secreto).
4. US3 Revelar → probar sola → entregar (con esto el Planning Poker ya sirve para una ronda).
5. US4 Repetir la ronda → probar sola → entregar.
6. US5 Registrar → probar sola → entregar (la sesión escribe en el backlog).
7. US6 Desconexiones y relevo → probar sola → entregar.
8. US7 Cancelar → probar sola → entregar.
9. US8 Historial → probar sola → entregar (el historial queda protegido de la eliminación).
10. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Foundational en conjunto.
2. US1 en secuencia: aporta el canal, el hub, la presencia y el entorno de pruebas que usan todas.
3. Terminada US1, el frontend de cada historia puede avanzar en paralelo con el backend de la
   siguiente, coordinando `types/poker.ts` y `api.ts`.
4. US2 → US3 en secuencia; después US4 y US5 pueden avanzar en paralelo coordinando
   `session_repository.go`, `session_service.go`, `handler.go`, `routes.go` y `handler_test.go`.
5. US6, US7 y US8 al final, en ese orden.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia; buscar `005/US1` a `005/US8` en el código de
  pruebas devuelve la trazabilidad completa de los 79 escenarios exigida por el Principio II.
  `TestUS2_CambioSimultaneoConRevelacion` lleva la etiqueta `005/US2` aunque se escribe en T047 de
  US3, porque necesita la revelación; el subtest de cancelar de `TestUS6_FacilitadorRetomaElRol`
  lleva `005/US6` aunque se escribe en T079 de US7, porque necesita la cancelación.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 005/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b). Los commits los registra **una persona**, nunca un agente.
- Las pruebas corren siempre contra `smye_test` en el puerto 5433, jamás contra desarrollo
  (Principio V).
- Esta feature **no agrega ninguna dependencia** de Go ni de npm. Si alguna tarea parece necesitar
  una (por ejemplo, una biblioteca de WebSockets o un polyfill de `EventSource`), detenerse y pedir
  autorización (Principio IV).
- **Código de otras features que esta feature modifica**, todo señalado en el plan: el puerto
  `EstimationSessions` de la 003 (T088), la pantalla de detalle de la historia de la 003 (T032,
  T090) y, por el punto abierto 5 de research.md, un accesor exportado del middleware de sesión de
  la 001 (T094).
- **Puntos del plan que esperan confirmación del desarrollador** y no tienen tarea: el uso de SSE en
  lugar de WebSockets (research.md sección 2), el supuesto de una sola instancia del backend
  (sección 7) y la ventana de
  carrera entre `HasSessions` y el borrado de la historia en la 003 (sección 15.4), que se
  recomienda tratar en `/revisar-seguridad`.
- **Criterios de éxito no automatizables.** SC-001 (abrir una sesión en menos de 30 segundos y 3
  pasos), SC-002 (votar en menos de 10 segundos, un paso), SC-003 (estimar con dos rondas en menos de
  5 minutos, 95 % sin ayuda) y SC-019 (encontrar en el historial las rondas y los votos en menos de
  30 segundos) son métricas de experiencia de uso: se validan a mano siguiendo la sección "Flujo en
  la interfaz" de [quickstart.md](./quickstart.md).
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: Swagger se actualiza en la misma tarea que cada endpoint, pero el `README.md` se
  actualiza una sola vez en la Fase 10 (T096). Entre el merge de una historia y la Fase 10, el README
  no lista los endpoints del Planning Poker ya integrados. Se acepta porque Swagger, publicado en
  `/swagger/index.html`, es la referencia actualizada de la API en cada merge, y porque los pasos
  para ejecutar la aplicación no cambian en esta feature: no hay servicios, variables ni comandos
  nuevos. Es el mismo criterio que aplicaron la 001 a la 004.

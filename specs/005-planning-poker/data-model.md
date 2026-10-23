# Modelo de Datos (Fase 1): Planning Poker en Tiempo Real

**Feature**: `specs/005-planning-poker` | **Fecha**: 2026-10-22

Entidades derivadas de la sección "Entidades Clave" de [spec.md](./spec.md). Las decisiones de
diseño que las justifican están en [research.md](./research.md).

Reglas transversales obligatorias (restricciones técnicas de la constitución):

- La clave primaria de los modelos persistidos es **UUID**; prohibidos los enteros secuenciales.
- Todo modelo registra **fecha y hora de creación y de última actualización** (`created_at`,
  `updated_at`, gestionadas por GORM).
- El esquema se crea con `AutoMigrate` de GORM.
- Los identificadores del código y los valores de dominio están en inglés (Principio VIII); las
  etiquetas en español viven en el frontend.

Esta feature **no modifica** las tablas `users`, `projects`, `project_members`, `sprints` ni
`sprint_stories`: las lee o no las usa. De `user_stories` (003) escribe **solo** `story_points` y
`updated_at`, al registrar la estimación acordada (RC-03).

---

## 1. `EstimationSession` — tabla `estimation_sessions`

Encuentro de Planning Poker sobre una única historia; contenedor de sus rondas, participantes y votos.

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK; lo genera la aplicación con `google/uuid` | FR-004 |
| `story_id` | `uuid` | `NOT NULL`; FK a `user_stories.id`; no cambia nunca | FR-005 |
| `facilitator_id` | `uuid` | `NOT NULL`; FK a `users.id`; facilitador vigente | FR-002, FR-070, FR-072 |
| `status` | `varchar(16)` | `NOT NULL`; `open`, `finished` o `canceled`; nace `open` | FR-003 |
| `agreed_story_points` | `smallint` | anulable; solo en `finished`; valor de la escala | FR-049, FR-052 |
| `cancel_reason` | `varchar(24)` | anulable; solo en cancelaciones del sistema: `inactivity`, `story_completed`, `project_finished` | FR-061, FR-063 |
| `opened_by_id` | `uuid` | `NOT NULL`; FK a `users.id` | FR-010 |
| `closed_by_id` | `uuid` | anulable; FK a `users.id`; nulo si cerró el sistema | FR-010, FR-053 |
| `closed_at` | `timestamptz` | anulable; momento de cierre | FR-010, FR-053 |
| `last_activity_at` | `timestamptz` | `NOT NULL`; momento del último hecho | FR-063 |
| `version` | `bigint` | `NOT NULL DEFAULT 0`; sube con cada hecho | FR-076 (research, sección 7) |
| `created_at` | `timestamptz` | `NOT NULL`; **es el momento de apertura** | FR-010, FR-080 |
| `updated_at` | `timestamptz` | `NOT NULL` | — |

**Índices**:

- `idx_estimation_sessions_one_open_per_story` **único parcial** sobre `(story_id) WHERE status =
  'open'`: respaldo de RN-04 (research, sección 6). También sirve al barrido de sesiones abiertas.
- `idx_estimation_sessions_story_created (story_id, created_at)`: historial de una historia en
  orden (FR-080) y `HasSessions` de la 003.

### Invariantes por estado

| Estado | `agreed_story_points` | `cancel_reason` | `closed_by_id` | `closed_at` |
| --- | --- | --- | --- | --- |
| `open` | `NULL` | `NULL` | `NULL` | `NULL` |
| `finished` | no nulo | `NULL` | no nulo (facilitador) | no nulo |
| `canceled` por el facilitador | `NULL` | `NULL` | no nulo (facilitador) | no nulo |
| `canceled` por el sistema | `NULL` | no nulo | `NULL` | no nulo |

Una sesión `finished` o `canceled` no vuelve a cambiar, ni ella ni sus rondas, participantes o votos
(FR-056, RN-22).

### Transiciones (RN-22)

| Desde | Hacia | Disparador |
| --- | --- | --- |
| `open` | `finished` | El facilitador registra la estimación acordada (FR-052) |
| `open` | `canceled` | El facilitador cancela (FR-058); el barrido o una acción detectan historia completada, proyecto finalizado o 60 minutos sin hechos (FR-061, FR-063) |
| `finished`, `canceled` | — | Ninguna |

---

## 2. `EstimationRound` — tabla `estimation_rounds`

Un intento de estimación dentro de una sesión.

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK | — |
| `session_id` | `uuid` | `NOT NULL`; FK a `estimation_sessions.id` | Entidad Ronda |
| `number` | `integer` | `NOT NULL`; 1, 2, 3… sin límite | FR-003, FR-044, FR-048 |
| `status` | `varchar(16)` | `NOT NULL`; `voting` o `revealed` | FR-003, FR-030 |
| `revealed_at` | `timestamptz` | anulable; momento de la revelación | Entidad Ronda |
| `created_at` | `timestamptz` | `NOT NULL`; **es el momento de apertura de la ronda** | Entidad Ronda |
| `updated_at` | `timestamptz` | `NOT NULL` | — |

**Índices**: `idx_estimation_rounds_session_number` **único** sobre `(session_id, number)`: impide
dos rondas con el mismo número y sirve para leer la ronda vigente (`ORDER BY number DESC LIMIT 1`).

**Reglas**: la ronda vigente es la de mayor número. Solo la vigente puede estar `voting`; al abrir
una ronda nueva la anterior ya está `revealed` (FR-046) y queda congelada. El resultado (consenso,
divergencia, sin votos numéricos) **no se guarda**: lo calcula `Summarize` (sección 6).

---

## 3. `EstimationParticipant` — tabla `estimation_participants`

Presencia de un integrante en una sesión. Una fila por persona y sesión, sin importar cuántas
conexiones abra (RN-34).

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK | — |
| `session_id` | `uuid` | `NOT NULL`; FK a `estimation_sessions.id` | FR-013 |
| `user_id` | `uuid` | `NOT NULL`; FK a `users.id` | FR-013 |
| `connected` | `boolean` | `NOT NULL`; verdadero mientras tenga una conexión viva o esté dentro de la gracia de 30 s | FR-013, FR-014 |
| `disconnected_at` | `timestamptz` | anulable; momento en que pasó a desconectado | FR-068, Entidad Participante |
| `created_at` | `timestamptz` | `NOT NULL`; **es el momento de incorporación** | Entidad Participante |
| `updated_at` | `timestamptz` | `NOT NULL` | — |

**Índices**: `idx_estimation_participants_session_user` **único** sobre `(session_id, user_id)`.

**Reglas**: la fila nunca se borra; quien se reconecta reutiliza la suya (FR-015). Quién es el
facilitador no está acá: es `estimation_sessions.facilitator_id`. El contador de conexiones y el
momento de la última caída viven en memoria (`Presence`, research sección 9).

---

## 4. `EstimationVote` — tabla `estimation_votes`

La carta que un participante jugó en una ronda.

| Columna | Tipo | Restricciones | Origen |
| --- | --- | --- | --- |
| `id` | `uuid` | PK | — |
| `round_id` | `uuid` | `NOT NULL`; FK a `estimation_rounds.id` | Entidad Voto |
| `participant_id` | `uuid` | `NOT NULL`; FK a `estimation_participants.id` | Entidad Voto |
| `card` | `varchar(3)` | `NOT NULL`; `0`, `1`, `2`, `3`, `5`, `8`, `13`, `20`, `40`, `100` o `?` | FR-020, RN-06 |
| `created_at` | `timestamptz` | `NOT NULL` | — |
| `updated_at` | `timestamptz` | `NOT NULL`; **es el momento en que quedó emitida la carta vigente** | Entidad Voto |

**Índices**: `idx_estimation_votes_round_participant` **único** sobre `(round_id, participant_id)`:
un voto por participante y ronda (RN-07). Cambiar de carta actualiza la fila (FR-022); retirarla no
existe (RN-36), así que la fila nunca se borra.

---

## 5. Tipos de dominio (valores en inglés)

| Tipo | Valores | Etiqueta en el frontend |
| --- | --- | --- |
| `SessionStatus` | `open`, `finished`, `canceled` | Abierta, Finalizada, Cancelada |
| `RoundStatus` | `voting`, `revealed` | En votación, Revelada |
| `CancelReason` | `inactivity`, `story_completed`, `project_finished` | Por inactividad, Historia completada, Proyecto finalizado |
| `Card` | `0`, `1`, `2`, `3`, `5`, `8`, `13`, `20`, `40`, `100`, `?` | Igual |
| `OutcomeKind` | `consensus`, `divergence`, `no_numeric_votes` | Consenso, Divergencia, Sin votos numéricos |

`Card` se declara una sola vez a partir de `backlogdomain.StoryPointsScale` más `"?"` (RC-04).

---

## 6. Reglas de dominio puras (con prueba unitaria obligatoria)

Funciones sin acceso a la base ni al reloj (el instante llega por parámetro). Son las pruebas
unitarias obligatorias del Principio II.

| Función | Entrada | Salida | Requisitos |
| --- | --- | --- | --- |
| `ParseCard(text)` | Texto recibido | `Card` o error con la lista de cartas | FR-020, FR-025, RN-06 |
| `ValidateAgreedValue(value)` | Entero | ok o error con la escala (usa `IsValidStoryPoints`) | FR-049, FR-050, RN-18 |
| `ShouldAutoReveal(participants, votes)` | Participantes con `connected` y votos de la ronda vigente | verdadero si hay al menos un conectado y todos los conectados votaron | FR-030, RN-09, RN-12, RN-23, RN-34 |
| `CanRevealManually(votes)` | Votos de la ronda vigente | ok o `ErrNoVotesToReveal` | FR-032, FR-033, RN-10 |
| `Summarize(votes)` | Votos revelados con su autor | `RoundOutcome` | FR-036 a FR-042, RN-12 a RN-14 |
| `FacilitatorTakeover(now, facilitator, requesterID)` | Participante facilitador (`connected`, `disconnected_at`) | ok o `FacilitatorUnavailableError` con `RetryAfter` | FR-068, FR-069, RN-25 |
| `AutoCancelReason(storyStatus, projectFinished, lastActivityAt, now)` | Estado de la historia y del proyecto, último hecho | `CancelReason` o ninguno | FR-061, FR-063, RN-30, RN-31 |
| `SessionStatus.EnsureOpen(reason, agreedStoryPoints)` | Estado de la sesión, motivo de cancelación (`*CancelReason`, opcional) y valor acordado (`*int`, opcional) | ok si está `open`; si no, `SessionClosedError` con `Status`, `CancelReason` y `AgreedStoryPoints` copiados de las entradas | FR-018, FR-027, FR-055, FR-060, RN-22 |
| `ViewFor(snapshot, viewerID)` | Instantánea completa | Vista con los votos ocultos según la sección 8 de research | FR-023, FR-024, FR-082, RN-08 |

### Tabla de `Summarize`

| Votos revelados | Resultado |
| --- | --- |
| 3, 5, 13 | `divergence`, mínimo 3 y máximo 13 con sus autores |
| 3, 3, 40, 40 | `divergence`, dos autores del mínimo y dos del máximo |
| 8, 8, 8 | `consensus` en 8, sin mínimo ni máximo |
| 5, 5, `?` | `consensus` en 5 |
| 13, `?`, `?` | `consensus` en 13 |
| `?`, `?`, `?` | `no_numeric_votes`, sin consenso, sin mínimo ni máximo |
| un solo voto 0 | `consensus` en 0 |

### Tabla de `AutoCancelReason` (precedencia en este orden)

| Condición | Motivo |
| --- | --- |
| La historia está `completed` | `story_completed` |
| El proyecto está `finished` | `project_finished` |
| `now - lastActivityAt ≥ 60 min` | `inactivity` |
| Ninguna | sin cancelación |

---

## 7. Hechos de la sesión

Un **hecho** es lo que reinicia la inactividad (`last_activity_at = now`) y sube `version`.

| Hecho | ¿Hecho? | Requisito |
| --- | --- | --- |
| Incorporación (primera conexión, o reconexión de quien figuraba desconectado) | Sí | FR-063 |
| Desconexión (vence la gracia de 30 s o se revoca la membresía) | Sí | FR-014, FR-019, FR-063 |
| Emisión o cambio de voto | Sí | FR-063 |
| Revelación automática o manual | Sí | FR-063 |
| Inicio de una ronda nueva | Sí | FR-063 |
| Cambio de facilitador | Sí | FR-063 |
| Registro de la estimación o cancelación | Cierra la sesión: sube `version` | FR-073 |
| Segunda conexión de la misma persona, o reconexión dentro de la gracia | No | FR-013, FR-014 |
| Cambio del texto de la historia | No: se publica sin subir `version` | FR-074 |
| Reestimación directa desde el backlog | No: no se publica como hecho | FR-075 |
| Reinicio del servidor (todos desconectados) | No | research, sección 9 |

---

## 8. Tipos de lectura (no persistidos)

| Tipo | Contenido | Uso |
| --- | --- | --- |
| `SessionAccess` | `SessionID`, `StoryID`, `ProjectID` | `RequireEstimationAccess` |
| `SessionSnapshot` | Sesión; historia (título, descripción, criterios en orden; **sin** Story Points); facilitador, quien abrió y quien cerró con su nombre; participantes con nombre y `connected`; todas las rondas con sus votos y autores | Entrada de `ViewFor` |
| `SessionView` | Lo que sale por la API: ver `EstimationSession` en [contracts/openapi.yaml](./contracts/openapi.yaml) | Canal, `GET` de la sesión y respuesta de las acciones |
| `SessionSummary` | Estado, valor acordado, motivo, quién abrió y cerró, momentos, cantidad de rondas | Historial de la historia (FR-079, FR-080) |
| `OpenSessionSummary` | Identificador, facilitador, número y estado de la ronda vigente | `openSession` del historial (FR-081) |
| `OpenSessionStatus` | Identificador, estado de la historia, proyecto finalizado, `last_activity_at`, `user_stories.updated_at` | Barrido (research, sección 13) |
| `ParticipantRef` | `SessionID`, `UserID` | Barrido de membresías revocadas |
| `RoundOutcome` | `Kind`; `Value` en consenso; `Min` y `Max` con `Value` y `Authors` en divergencia | `Summarize` |

Los nombres se leen con un `JOIN` a `users` seleccionando solo `id` y `full_name`.

---

## 9. Interfaces de repositorio y puertos (capa `domain`)

Declaran solo las operaciones que se usan (Principio III). Ninguna devuelve
`gorm.ErrRecordNotFound` ni `(nil, nil)` para un recurso obligatorio; `FindOpenForStory` devuelve
`nil` sin error porque la sesión abierta es opcional.

```text
SessionRepository
  FindAccessible(ctx, sessionID, userID) (*SessionAccess, error)         // RequireEstimationAccess; ErrSessionNotFound
  Open(ctx, storyID, openerID, now) (*EstimationSession, error)          // bloquea la historia; OpenSessionExistsError, ErrStoryCompleted, ErrProjectFinished; inserta sesión, ronda 1 y participante conectado
  Connect(ctx, sessionID, userID, now) error                             // bloquea la sesión; alta o reconexión; evalúa revelación automática
  Disconnect(ctx, sessionID, userID, now) error                          // bloquea la sesión; connected = false; evalúa revelación automática
  Vote(ctx, sessionID, userID, card, now) error                          // bloquea; participante conectado; ronda voting; upsert; evalúa revelación automática
  Reveal(ctx, sessionID, userID, now) error                              // bloquea; facilitador; CanRevealManually
  StartRound(ctx, sessionID, userID, now) error                          // bloquea; facilitador; ronda vigente revealed; inserta la siguiente
  Finalize(ctx, sessionID, userID, storyPoints, now) error               // bloquea sesión e historia; facilitador; al menos una ronda revealed; escribe story_points
  Cancel(ctx, sessionID, userID, now) error                              // bloquea; facilitador
  TakeFacilitator(ctx, sessionID, userID, now) error                     // bloquea; FacilitatorTakeover
  AutoCancel(ctx, sessionID, now) (bool, error)                          // bloquea; reevalúa AutoCancelReason; cancela si corresponde
  ResetPresence(ctx, now) error                                          // arranque: todos los participantes de sesiones abiertas desconectados
  ListOpenForSweep(ctx) ([]OpenSessionStatus, error)
  ListRevokedParticipants(ctx) ([]ParticipantRef, error)
  FindSnapshot(ctx, sessionID) (*SessionSnapshot, error)
  ListForStory(ctx, storyID) ([]SessionSummary, error)                   // solo cerradas, de la más reciente a la más antigua
  FindOpenForStory(ctx, storyID) (*OpenSessionSummary, error)
```

Todas las escrituras sobre una sesión abierta (desde `Connect` hasta `TakeFacilitator`) siguen el
mismo esquema dentro de su transacción, igual que el repositorio de la 004:

1. `SELECT … FOR UPDATE` de la sesión; `Status.EnsureOpen(CancelReason, AgreedStoryPoints)` con los
   campos de la fila bloqueada.
2. Lectura del estado de la historia y del proyecto; si `AutoCancelReason` informa un motivo, se
   cancela la sesión, se confirma la transacción y se devuelve `SessionClosedError` con ese motivo.
3. Reglas de la sección 6 sobre los datos releídos; error de dominio si no se cumplen.
4. Escrituras; si es un hecho, `last_activity_at = now` y `version = version + 1`.

El servicio recibe el instante del reloj inyectado y, después de cada escritura confirmada (o de una
cancelación automática), publica la instantánea (research, sección 7).

### Errores de dominio (`poker/domain/errors.go`)

| Error | Situación | Código HTTP |
| --- | --- | --- |
| `ErrSessionNotFound` | Sesión inexistente o ajena | `404` |
| `SessionClosedError` (con `Status`, `CancelReason`, `AgreedStoryPoints`) | Acción o incorporación sobre una sesión cerrada | `409` |
| `ErrNotParticipant` | Quien actúa no es participante conectado | `403` |
| `ErrFacilitatorOnly` | Acción reservada al facilitador | `403` |
| `OpenSessionExistsError` (con `SessionID`) | La historia ya tiene una sesión abierta | `409` |
| `ErrStoryCompleted` | Abrir sobre una historia completada | `409` |
| `ErrProjectFinished` | Abrir en un proyecto finalizado | `409` |
| `ErrRoundRevealed` | Votar o revelar sobre una ronda revelada | `409` |
| `ErrRoundNotRevealed` | Nueva ronda con la vigente en votación | `409` |
| `ErrNoVotesToReveal` | Revelación manual sin votos | `409` |
| `ErrNoRevealedRound` | Registrar sin ninguna ronda revelada | `409` |
| `FacilitatorUnavailableError` (con `Facilitator`, `RetryAfter`) | Tomar el rol sin cumplir FR-068 | `409` |

Las validaciones de formato (`ParseCard`, carta ausente, `ValidateAgreedValue`) ocurren en
`delivery` y responden `400` antes de llegar al servicio.

### Puerto de la 003 que implementa esta feature

```text
backlogdomain.EstimationSessions
  HasSessions(ctx, storyID) (bool, error)   // implementación nueva: EXISTS sobre estimation_sessions, en cualquier estado
```

---

## 10. Relaciones

```text
user_stories (003)     1 ──── N estimation_sessions        (story_id; a lo sumo una open)
users (001)            1 ──── N estimation_sessions        (facilitator_id, opened_by_id, closed_by_id)
estimation_sessions    1 ──── N estimation_rounds          (session_id; numeradas desde 1)
estimation_sessions    1 ──── N estimation_participants    (session_id; una por usuario)
users (001)            1 ──── N estimation_participants    (user_id)
estimation_rounds      1 ──── N estimation_votes           (round_id; uno por participante)
estimation_participants 1 ─── N estimation_votes           (participant_id)
```

Las claves foráneas se crean sin `ON DELETE`, de modo que una historia con sesiones no se puede
borrar a nivel de base (respaldo de RN-33). El proyecto se alcanza a través de la historia; no se
duplica `project_id` porque una historia no cambia de proyecto (003).

### Efecto sobre la 003

- `backlog/repository/estimation_sessions.go` reemplaza su cuerpo (sección 9) y su constructor
  recibe la conexión.
- `user_stories.story_points` y `updated_at` se escriben al registrar (FR-052).

### Efecto sobre la 002 y la 004

Ninguno: no se modifica su código. Completar una historia o finalizar un proyecto se detecta con el
barrido (research, sección 13).

---

## 11. Trazabilidad: requisitos cubiertos por el modelo

| Requisitos | Dónde |
| --- | --- |
| FR-001 a FR-010 | `estimation_sessions`, índice parcial, `Open` |
| FR-011 a FR-019 | `estimation_participants`, `Connect`, `Disconnect`, `ListRevokedParticipants` |
| FR-020 a FR-029 | `estimation_votes`, `ParseCard`, `Vote`, bloqueo de la sesión |
| FR-030 a FR-042 | `ShouldAutoReveal`, `CanRevealManually`, `Summarize`, `Reveal` |
| FR-043 a FR-048 | `estimation_rounds`, `StartRound` |
| FR-049 a FR-056 | `ValidateAgreedValue`, `Finalize`, invariantes por estado |
| FR-057 a FR-063 | `Cancel`, `AutoCancel`, `AutoCancelReason`, `last_activity_at` |
| FR-064 a FR-072 | `facilitator_id`, `FacilitatorTakeover`, `TakeFacilitator` |
| FR-073 a FR-077 | `version`, `SessionSnapshot`, `ViewFor` |
| FR-078 a FR-085 | `ListForStory`, `FindOpenForStory`, `FindSnapshot` |
| FR-086 a FR-088 | `FindAccessible`, `RequireStoryAccess` de la 003 |
| FR-089 | Puerto `EstimationSessions`, claves foráneas sin `ON DELETE` |

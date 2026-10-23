# Plan de Implementación: Planning Poker en Tiempo Real

**Rama**: `005-planning-poker` | **Fecha**: 2026-10-22 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/005-planning-poker/spec.md`

---

## Resumen

Esta feature permite que los integrantes de un proyecto estimen juntos una historia del backlog,
cada uno desde su pantalla y en tiempo real: votan en secreto, las cartas se revelan a la vez, las
diferencias quedan a la vista, se repiten rondas y el facilitador registra el valor acordado en los
Story Points de la historia. Las rondas y los votos se conservan como historial de estimación, que
consumirán las métricas de la 008 y los reportes de la 010.

El enfoque técnico tiene seis decisiones que lo definen:

1. **Server-Sent Events para difundir, REST para actuar.** Abrir
   `GET /api/v1/estimation-sessions/{id}/events` es unirse a la sesión; cada acción es un endpoint
   REST común. El navegador lee el canal con `fetch`, así que la cabecera `Authorization` y los
   middlewares de sesión y de acceso se reutilizan sin cambios. **Sin dependencias nuevas.**
2. **Cada mensaje es la sesión completa, vista por quien la recibe.** Una instantánea con `version`
   monotónica; el cliente descarta las viejas y al reconectarse recibe el estado vigente. La función
   pura `ViewFor` es la única salida de una sesión hacia la API y la que oculta los votos ajenos.
3. **Un bloqueo de fila por sesión convierte "un único resultado" en literal.** Voto y revelación
   simultáneos, dos tomas del rol a la vez y la revelación automática se serializan con
   `SELECT … FOR UPDATE` sobre la sesión; la apertura bloquea la historia y tiene un índice único
   parcial de respaldo.
4. **Presencia en dos niveles.** Un contador en memoria de conexiones por persona (varias pestañas
   son un participante) y el estado conectado o desconectado en la base, que es lo que leen la
   revelación automática y el relevo del facilitador bajo el mismo bloqueo.
5. **Un barrido cada segundo para todo lo que depende del tiempo**: la gracia de 30 segundos, la
   caducidad de 60 minutos, la cancelación por historia completada o proyecto finalizado, la
   membresía revocada y los cambios del texto de la historia. Con reloj inyectado y `Sweep` invocable
   desde las pruebas. Así **la 002 y la 004 no se tocan**.
6. **Esta feature cierra el puerto que la 003 dejó abierto** (`EstimationSessions`), que bloquea la
   eliminación de una historia con historial de estimación.

Esta feature **no tiene fase de arranque**: agrega un módulo, cuatro tablas, diez endpoints, una
pantalla y una sección del detalle de la historia, sin dependencias ni variables de entorno nuevas.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.3 (backend); TypeScript sobre Node.js 24.21.0 (frontend)

**Dependencias principales**: sin cambios respecto de la 004 — Gin v1.12.0, GORM v1.31.2 con driver
`postgres` v1.6.3, `google/uuid` v1.6.0, `gin-contrib/cors` v1.7.9, `swaggo/swag` v1.16.6 con
`gin-swagger` v1.6.1 y `swaggo/files` v1.0.1; Next.js 16.3.8 con React 19.3.0 y Tailwind CSS 4.3.3.
**Esta feature no agrega ninguna dependencia**: el tiempo real se resuelve con `net/http`, `sync` y
canales en Go, y con `fetch` y `ReadableStream` en el navegador ([research.md](./research.md),
secciones 1 y 2)

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en los mismos contenedores disjuntos
de desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`). Cuatro tablas
nuevas: `estimation_sessions`, `estimation_rounds`, `estimation_participants` y `estimation_votes`.
El registro de conexiones abiertas vive en memoria del proceso

**Pruebas**: `testify` v1.12.1, `net/http/httptest` (con `httptest.NewServer` para leer el canal de
eventos) y reloj inyectado en el backend; Vitest 5.0.3 con `@testing-library/react` 16.3.3 y jsdom
30.1.2 en el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor, **una instancia**) y navegador moderno;
desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: todo hecho de la sesión llega a todos los participantes conectados en
**menos de 2 segundos** sin recargar (FR-073, SC-004), con hasta 10 participantes por sesión y 10
sesiones en paralelo, es decir **100 conexiones concurrentes** (RC-14). La difusión es inmediata
después de confirmar la transacción; los hechos que dependen del tiempo (desconexión por gracia,
cancelaciones del sistema, cambios de la historia) se detectan con un barrido cada **1 segundo**.
Cada hecho cuesta **una** lectura de la instantánea por sesión, no por participante. SC-001 a SC-003
y SC-019 son tiempos de tarea de la persona que dependen de la interfaz: abrir o unirse en un paso
desde el detalle de la historia y votar con un clic sobre la carta

**Restricciones**: cero votos ajenos expuestos antes de revelar (SC-005); misma vista para todos en
el 100 % de las revelaciones, incluidas las simultáneas con un cambio de voto (SC-007); cero segundas
sesiones abiertas bajo 10 aperturas simultáneas (SC-012); cero sesiones con dos facilitadores bajo 10
tomas simultáneas (SC-025); Story Points iguales al valor acordado o intactos (SC-013); revelación
automática con un voto por persona aunque tenga varias conexiones (SC-024); sesión y membresía en el
100 % de las acciones (SC-017); `404` indistinguible (SC-018); ningún historial perdido por cerrar
la sesión ni por eliminar la historia (SC-016, SC-023)

**Escala y alcance**: 10 endpoints HTTP (uno de ellos el canal de eventos), 4 entidades persistidas,
1 puerto ajeno reemplazado, 1 pantalla nueva (sala en vivo y resultado de una sesión) más una sección
en el detalle de la historia de la 003, 8 historias de usuario con **79** escenarios de aceptación y
36 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las decisiones de producto se cerraron en la sesión de
clarificación del 2026-09-26. Las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): mecanismo de tiempo real, forma de las rutas y de los `404`,
persistencia, concurrencia, consistencia de las vistas, secreto del voto, presencia y conexiones
múltiples, revelación y resultado, relevo del facilitador, cierres, barrido de lo que depende del
tiempo, reestimación directa, integración con la 003, historial, pruebas de tiempo real, frontend y
códigos HTTP.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **SSE en lugar de WebSockets**: la constitución prevé autorizar una dependencia de WebSockets al
   planificar esta spec; el plan **no la usa** y por eso no pide autorización. Si el equipo prefiere
   WebSockets, hay que autorizar la dependencia y rehacer las secciones 2, 7, 17 y 18 de
   [research.md](./research.md).
2. **Una sola instancia del backend**: el registro de conexiones vive en memoria (research,
   sección 7). Es lo que define el `docker-compose` actual y alcanza para RC-14.
3. **Lectura de los "2 segundos" en las desconexiones** (US1-3, US6-1): corren desde que vence la
   gracia de 30 segundos de FR-014, que es cuando la persona deja de contarse (research, sección 9).
   Confirmada el 2026-10-23 y registrada en la spec (Clarifications y FR-073).
4. **Rama de trabajo**: el plan se generó en la rama `repo/005-plan`, y `.specify/feature.json`
   (ignorado por Git) ahora apunta a `specs/005-planning-poker`. La spec indica la rama
   `005-planning-poker` para la implementación.
5. **Esta feature toca código de la 001 y de la 003**: reemplaza el cuerpo de
   `backlog/repository/estimation_sessions.go` (su constructor pasa a recibir la conexión), suma una
   sección al detalle de la historia en el frontend (research, sección 15) y exporta en
   `auth/delivery/middleware.go` el vencimiento de la sesión, para cerrar el canal al vencer el
   token (research, punto abierto 5).
6. **Dos puntos para `/revisar-seguridad`**: la ventana de carrera entre `HasSessions` y el borrado
   de la historia en la 003 (la clave foránea impide perder el historial, pero la respuesta sería
   `500` en lugar de `409`; research, sección 15.4), y el canal abierto de una sesión de
   autenticación cerrada desde otro dispositivo, que sigue vivo hasta que vence el token (research,
   punto abierto 5).

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 10 endpoints se corresponden con las 8 historias (US1 abrir y el canal; US2 votar; US3 revelar; US4 nueva ronda y el detalle de la sesión; US5 registrar; US6 tomar el rol; US7 cancelar; US8 historial). Sin chat, temporizador, escalas configurables, colas de historias, invitados, estadísticas, sugerencias, notificaciones, roles de observador, audio ni exportación: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 79 escenarios son pruebas nombradas; el entorno de test existe desde la 001. El canal se prueba con `httptest.NewServer` y el tiempo con un reloj inyectado. Validaciones de carta y de valor acordado, resultado de la ronda, revelación automática, relevo y caducidad son pruebas unitarias obligatorias. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/poker/` con sus cuatro capas; el frontend usa las capas existentes. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva: la de WebSockets que la constitución prevé autorizar no se necesita. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001; esta feature no agrega servicios. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. Sin variables nuevas. El canal exige el mismo token Bearer en cabecera, nunca en la URL. Los nombres se leen de `users` sin seleccionar `password_hash`. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Carta, valor acordado y parámetros de ruta se validan en `delivery`. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de dominio en inglés; comentarios, mensajes y Swagger en español; el canal se documenta en Swagger como `text/event-stream`. README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Cuatro tablas, una por entidad persistible de la spec, sin columnas derivables: el resultado de la ronda, la lista de "sin voto" y el booleano de facilitador por participante se descartaron (research, sección 5). Sin endpoint para salir ni para retirar el voto, que la spec no pide. Un único barrido en lugar de temporizadores por persona y de puertos en la 002 y la 004. `SessionRepository` declara 17 operaciones, todas usadas. | **PASA** |
| II | Los 79 escenarios se mapean a pruebas nombradas (tabla más abajo). Las pruebas unitarias obligatorias cubren `ParseCard`, `ValidateAgreedValue`, `ShouldAutoReveal`, `CanRevealManually`, `Summarize`, `FacilitatorTakeover`, `AutoCancelReason`, `EnsureOpen`, `ViewFor`, `Presence` y las reglas RN-01 a RN-36. El reloj inyectado y `Sweep` invocable hacen reproducibles la gracia, el relevo y la caducidad sin esperas reales. | **PASA** |
| III | `poker/domain` importa solo `backlog/domain` y `auth/domain`. `poker/delivery` no importa `backlog/delivery`: recibe `RequireStoryAccess` desde `cmd/app/`. `backlog/repository` importa `poker/domain` para su puerto; ningún `domain` importa `poker`, así que no hay ciclos. El registro de conexiones y el barrido viven en `poker/service`; `delivery` solo traduce un canal de Go a SSE. | **PASA** |
| IV | Ninguna dependencia nueva de Go ni de npm. Bloqueos con `gorm.io/gorm/clause` e índice parcial con la opción `where` de las etiquetas de GORM. | **PASA** |
| V | Sin servicios nuevos. Las tablas se crean con `AutoMigrate` en las mismas bases disjuntas; el ayudante de pruebas las vacía. | **PASA** |
| VI | La autoría y los votos seleccionan solo `users.id` y `users.full_name`. Ninguna respuesta revela datos de sesiones, historias o proyectos ajenos. El canal se cierra al vencer el token y al revocarse la membresía. El cuerpo sigue limitado a 1 MiB. | **PASA** |
| VII | Los diez endpoints validan antes del servicio. Errores de dominio traducidos a `403`, `404` y `409`; la violación del índice parcial traducida al mismo `409` de sesión en curso. Toda escritura es transaccional. El barrido registra los errores en el log y sigue con la próxima pasada, sin `panic`. La escritura en un canal cerrado termina la conexión sin afectar al resto. | **PASA** |
| VIII | El contrato OpenAPI está en español. Valores `open`/`finished`/`canceled`, `voting`/`revealed`, `consensus`/`divergence`/`no_numeric_votes` e `inactivity`/`story_completed`/`project_finished` en la API, con etiquetas en español en el frontend. `any` prohibido. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Tres decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **Estado en memoria del proceso** (registro de conexiones) **y una goroutine de fondo** (barrido).
   Son la consecuencia mínima de FR-013, FR-014, FR-063 y FR-073: no hay forma de difundir sin saber
   quién está conectado, ni de caducar una sesión sin que nadie la toque. Fijan el supuesto de una
   sola instancia (research, secciones 7, 9 y 13).
2. **Esta feature modifica código de la 001 y de la 003**: el puerto que la 003 dejó anotado para
   que esta lo implemente (research, sección 15) y un accesor de solo lectura en el middleware de
   sesión de la 001.
3. **Dos rutas de esta feature cuelgan de `/api/v1/stories/:storyId`**, el prefijo de la 003. Es lo
   que permite abrir y consultar el historial respondiendo con el mismo `404` de historia de la 003
   (research, sección 3).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/005-planning-poker/
├── spec.md              # Especificación (entrada)
├── plan.md              # Este archivo (salida de /speckit-plan)
├── research.md          # Salida de la Fase 0
├── data-model.md        # Salida de la Fase 1
├── quickstart.md        # Salida de la Fase 1
├── contracts/
│   └── openapi.yaml     # Salida de la Fase 1
├── checklists/
│   └── requirements.md  # Checklist de calidad de la spec
└── tasks.md             # Salida de /speckit-tasks — NO lo crea /speckit-plan
```

### Código fuente (raíz del repositorio)

Las líneas marcadas con `NUEVO` las crea esta feature; las marcadas con `TOCA` son archivos de las
features 001 a 004 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                          # TOCA: compone el módulo poker, pasa RequireStoryAccess, la conexión al puerto de la 003, ResetPresence y la goroutine del barrido; descripción de Swagger
├── internal/
│   ├── poker/                               # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── session.go                   # Entidad EstimationSession
│   │   │   ├── round.go                     # Entidad EstimationRound
│   │   │   ├── participant.go               # Entidad EstimationParticipant
│   │   │   ├── vote.go                      # Entidad EstimationVote
│   │   │   ├── status.go                    # SessionStatus, RoundStatus, CancelReason, EnsureOpen
│   │   │   ├── card.go                      # Card, ParseCard, ValidateAgreedValue
│   │   │   ├── reveal.go                    # ShouldAutoReveal, CanRevealManually
│   │   │   ├── outcome.go                   # RoundOutcome y Summarize
│   │   │   ├── facilitator.go               # FacilitatorTakeover
│   │   │   ├── auto_cancel.go               # AutoCancelReason
│   │   │   ├── view.go                      # SessionSnapshot, SessionView y ViewFor
│   │   │   ├── errors.go                    # Errores de dominio y errores con datos de conflicto
│   │   │   └── repository.go                # SessionRepository y tipos de lectura
│   │   ├── repository/
│   │   │   └── session_repository.go        # GORM; transacciones, bloqueos, instantánea e historial
│   │   ├── service/
│   │   │   ├── session_service.go           # Open, Join, Leave, Vote, Reveal, StartRound, Finalize, Cancel, TakeFacilitator, Get, History
│   │   │   ├── presence.go                  # Contador de conexiones por persona y momento de la última caída
│   │   │   ├── hub.go                       # Suscripciones por sesión, publicación serializada, canal "la última gana"
│   │   │   └── sweeper.go                   # Sweep y Run: gracia, cancelaciones del sistema, membresía, cambios de la historia
│   │   └── delivery/
│   │       ├── handler.go                   # 9 manejadores REST con anotaciones swag en español
│   │       ├── stream.go                    # Manejador del canal SSE: une, envía state/removed, ping cada 15 s
│   │       ├── dto.go                       # Peticiones y respuestas
│   │       ├── validation.go                # Carta, retiro del voto y valor acordado
│   │       ├── middleware.go                # RequireEstimationAccess (404 único)
│   │       └── routes.go                    # Rutas sobre la historia y grupo plano de la sesión
│   ├── backlog/
│   │   └── repository/
│   │       └── estimation_sessions.go       # TOCA: lee estimation_sessions en lugar de informar "sin sesiones"
│   ├── auth/
│   │   └── delivery/
│   │       └── middleware.go                # TOCA: accesor exportado CurrentSessionExpiry para cerrar el canal al vencer el token
│   └── platform/
│       ├── database/
│       │   └── postgres.go                  # TOCA: AutoMigrate de las cuatro entidades
│       └── testsupport/
│           └── database.go                  # TOCA: vacía las cuatro tablas
├── postgres/                                # Sin cambios
├── .env.example                             # Sin cambios
├── Dockerfile                               # Sin cambios
├── go.mod                                   # Sin cambios
└── go.sum                                   # Sin cambios

frontend/
├── src/
│   ├── app/
│   │   └── projects/
│   │       └── [projectId]/
│   │           └── backlog/
│   │               └── [storyId]/
│   │                   ├── page.tsx         # TOCA: sección "Planning Poker" (US1, US8)
│   │                   └── poker/
│   │                       └── [sessionId]/
│   │                           └── page.tsx # NUEVO: sala en vivo (US1 a US7) o resultado de una sesión cerrada (US8)
│   ├── features/
│   │   └── poker/                           # NUEVO
│   │       ├── components/                  # StoryEstimationSection, PokerRoom, CardDeck, ParticipantList,
│   │       │                                # RoundResult, RoundHistory, FacilitatorActions, FinalizeDialog,
│   │       │                                # TakeFacilitatorButton, SessionClosedNotice
│   │       ├── api.ts                       # Llamadas a los 9 endpoints REST
│   │       └── use-estimation-stream.ts     # Hook del canal: versión, reconexión con espera creciente, cierre
│   ├── lib/
│   │   └── event-stream.ts                  # NUEVO: lector de text/event-stream sobre fetch
│   ├── types/
│   │   └── poker.ts                         # NUEVO: tipos de peticiones, respuestas y eventos; sin `any`
│   └── utils/
│       └── poker-labels.ts                  # NUEVO: etiquetas de estado, motivo y resultado
├── Dockerfile                               # Sin cambios
└── package.json                             # Sin cambios: ninguna dependencia nueva

README.md                                    # TOCA: describe el Planning Poker, su canal y sus endpoints
```

**Pruebas**: en el backend, junto al código del paquete que prueban (`card_test.go`,
`reveal_test.go`, `outcome_test.go`, `facilitator_test.go`, `auto_cancel_test.go`, `status_test.go`,
`view_test.go`, `presence_test.go`, `validation_test.go`, `session_service_test.go`,
`handler_test.go`, `stream_test.go`, `middleware_test.go`, `poker_concurrency_test.go`,
`poker_scale_test.go`), más la prueba de integración del puerto reemplazado en `backlog/repository`.
En el frontend, junto al componente o módulo (`event-stream.test.ts`, `card-deck.test.tsx`,
`participant-list.test.tsx`, `round-result.test.tsx`, `facilitator-actions.test.tsx`,
`story-estimation-section.test.tsx`, `use-estimation-stream.test.tsx`). No se crea un árbol `tests/`
separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/poker/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. Un único
módulo para sesiones, rondas, participantes y votos, porque ninguno existe fuera de su sesión. El
frontend no agrega capas: la sala se anida bajo la historia para la navegación, aunque las rutas de
la API de la sesión sean planas. Antes de escribir código de Next.js se lee la documentación de la
versión instalada en `frontend/node_modules/next/dist/docs/` (CLAUDE.md).

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `005/US1` a `005/US8` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (005/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run`.

**Frontend (Vitest)**: `describe("005/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario que tenga comportamiento visible en la
interfaz (cartas propias y ajenas, resultado de la ronda, acciones del facilitador, aviso de sesión
cerrada).

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Abrir una sesión y reunir al equipo | 10 | `TestUS1_AperturaDeSesion`, `TestUS1_IncorporacionDeDosIntegrantes`, `TestUS1_AbandonoDeUnParticipante`, `TestUS1_HistoriaSinCriterios`, `TestUS1_FacilitadorSolo`, `TestUS1_SesionesAnterioresConservadas`, `TestUS1_SegundaSesionAbierta`, `TestUS1_HistoriaCompletada`, `TestUS1_ProyectoFinalizado`, `TestUS1_NoIntegrante` |
| US2 Votar en secreto | 10 | `TestUS2_VotoOculto`, `TestUS2_PropiaCartaVisible`, `TestUS2_CambioDeVoto`, `TestUS2_CartaDesconocida`, `TestUS2_ExtremosDeLaEscala`, `TestUS2_CambioSimultaneoConRevelacion`, `TestUS2_ValorFueraDeEscala`, `TestUS2_RondaRevelada`, `TestUS2_IntegranteNoUnido`, `TestUS2_RetiroDeVoto` |
| US3 Revelar y ver las diferencias | 10 | `TestUS3_RevelacionAutomatica`, `TestUS3_RevelacionManualConConsenso`, `TestUS3_InterrogacionFueraDelCalculo`, `TestUS3_ConsensoSinExtremos`, `TestUS3_UnicoParticipante`, `TestUS3_TodosInterrogacion`, `TestUS3_DesconexionDisparaRevelacion`, `TestUS3_AutoresMultiples`, `TestUS3_RevelarSinVotos`, `TestUS3_RevelarSinSerFacilitador` |
| US4 Repetir la ronda | 8 | `TestUS4_NuevaRonda`, `TestUS4_HistorialDeRondas`, `TestUS4_NuevaRondaTrasConsenso`, `TestUS4_IncorporacionEnRondaPosterior`, `TestUS4_SinLimiteDeRondas`, `TestUS4_FacilitadorSoloEnNuevaRonda`, `TestUS4_RondaSinRevelar`, `TestUS4_NuevaRondaSinSerFacilitador` |
| US5 Registrar la estimación acordada | 11 | `TestUS5_RegistroConConsenso`, `TestUS5_RegistroSinConsenso`, `TestUS5_ValorNoVotado`, `TestUS5_ReemplazaEstimacionAnterior`, `TestUS5_ReestimacionDirectaDuranteLaSesion`, `TestUS5_RegistroTrasTodosInterrogacion`, `TestUS5_HistoriaSinCriteriosNoQuedaLista`, `TestUS5_RondasConservadas`, `TestUS5_ValorInvalido`, `TestUS5_SinRondaRevelada`, `TestUS5_RegistroSinSerFacilitador` |
| US6 Desconexiones y relevo | 12 | `TestUS6_DesconexionDisparaRevelacion`, `TestUS6_ReconexionConVoto`, `TestUS6_ReconexionSinVoto`, `TestUS6_ReconexionEnRondaPosterior`, `TestUS6_FacilitadorRetomaElRol`, `TestUS6_TodosDesconectados`, `TestUS6_FacilitadorOriginalRelevado`, `TestUS6_UnicoParticipanteTomaElRol`, `TestUS6_DosPestanas`, `TestUS6_FacilitadorPresente`, `TestUS6_RolTodaviaNoDisponible`, `TestUS6_TomasSimultaneas` |
| US7 Cancelar sin registrar | 9 | `TestUS7_CancelacionConservaEstimacion`, `TestUS7_NuevaSesionTrasCancelar`, `TestUS7_RondasConservadas`, `TestUS7_CancelacionSinVotos`, `TestUS7_CancelacionPorInactividad`, `TestUS7_HechoReiniciaInactividad`, `TestUS7_CancelarSinSerFacilitador`, `TestUS7_CancelarSesionFinalizada`, `TestUS7_AccionTrasInactividad` |
| US8 Consultar el historial | 9 | `TestUS8_SesionFinalizadaConRondas`, `TestUS8_OrdenDeSesiones`, `TestUS8_SesionCancelada`, `TestUS8_SesionEnCurso`, `TestUS8_HistoriaSinSesiones`, `TestUS8_ExIntegrante`, `TestUS8_ProyectoFinalizado`, `TestUS8_NoIntegrante`, `TestUS8_EliminacionBloqueada` |

**Total: 79 escenarios.**

**Cómo se preparan los escenarios que dependen de otras features o del tiempo**:

- Las historias se crean y se modifican con los endpoints de la 003, que el router de prueba compone
  junto con el módulo de Planning Poker. Así US5-5 (reestimación directa), US5-7 (lista para
  planificar) y US8-9 (eliminación bloqueada) ejercitan la integración real.
- La historia Completada (US1-8 y la cancelación del sistema de FR-061) se prepara con los endpoints
  de la 004 (sprint Activo y avance a `completed`); el proyecto Finalizado (US1-9, US8-7), con el
  cambio de estado de la 002. El ex integrante (US8-6), con la baja de integrantes de la 002.
- La gracia de 30 segundos (US1-3, US3-7, US6-1, US6-6), el relevo a los 5 minutos (US6-7, US6-8,
  US6-11, US6-12) y la caducidad de 60 minutos (US7-5, US7-6, US7-9) se reproducen avanzando el reloj
  inyectado y llamando a `Sweep`.
- Las incorporaciones y desconexiones se hacen abriendo y cerrando canales reales contra
  `httptest.NewServer`; las "dos pestañas" (US6-9) son dos canales del mismo token.

**Pruebas transversales sin etiqueta de escenario**: la prueba sin sesión de cada uno de los diez
endpoints (SC-017), la comparación byte a byte de los `404` de historia y de sesión (SC-018), la
consulta de una ronda en votación por cada forma de consulta desde otra cuenta (SC-005), las de
concurrencia (`TestPokerOpen_Concurrency`, `TestPokerFacilitator_Concurrency`,
`TestPokerVoteReveal_Concurrency`; SC-007, SC-012, SC-025) y la de escala (`TestPokerStream_Scale`,
SC-004) se nombran sin `005/USn`, porque no derivan de un escenario de la spec y no deben alterar el
conteo de 79.

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones y reglas de negocio),
que cubren SC-020:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| `RequireEstimationAccess` y `RequireStoryAccess` con un integrante, un no integrante y un identificador inexistente | RN-01, RN-02 |
| Apertura: `facilitator_id` igual a quien abre; segunda apertura rechazada; historia Completada y proyecto Finalizado rechazados; traducción de la violación del índice parcial | RN-03, RN-04, RN-05 |
| `ParseCard`: las 11 cartas admitidas y 4, 6, 7, 50, 101, un negativo, un decimal, un número JSON y el texto vacío | RN-06 |
| Voto: un voto por ronda; cambio de carta reemplaza; retiro rechazado | RN-07, RN-36 |
| `ViewFor`: votos ajenos ocultos en `voting`, propio en `myVote`, visibles al revelar | RN-08 |
| `ShouldAutoReveal`: 1, 2 y 5 conectados; un desconectado con y sin voto; nadie conectado; `?` como voto emitido; dos conexiones de la misma persona | RN-09, RN-12, RN-23, RN-34 |
| `CanRevealManually` y la verificación de facilitador | RN-10, RN-15 |
| Voto sobre ronda `revealed` | RN-11 |
| `Summarize`: la tabla completa de data-model.md, sección 6 | RN-12, RN-13, RN-14 |
| El voto del facilitador cuenta para la revelación y el resultado | RN-16 |
| Nueva ronda solo sobre `revealed`; once rondas | RN-17 |
| `ValidateAgreedValue`: la escala, 7, `?`, `null` y decimales; valor no votado aceptado | RN-18 |
| Registrar: `finished`, Story Points escritos en la misma transacción; exige ronda revelada | RN-19, RN-20 |
| Cancelar: Story Points intactos | RN-21 |
| `EnsureOpen` con `finished` y `canceled` en cada acción | RN-22 |
| `FacilitatorTakeover`: conectado, 2 minutos, 4:59, 5:00, el que pide ya facilitador | RN-24, RN-25, RN-26, RN-35 |
| Difusión: todas las vistas con la misma versión tras cada hecho | RN-27 |
| Rondas y votos conservados tras `finished` y `canceled` | RN-28 |
| Historial en proyecto Finalizado | RN-29 |
| `AutoCancelReason`: historia `completed`, proyecto `finished`, 59 y 60 minutos, precedencia | RN-30, RN-31 |
| Reestimación directa con sesión abierta: sin hecho y última escritura | RN-32 |
| Puerto `EstimationSessions` con una sesión `finished` y otra `canceled` | RN-33 |
| `Presence`: conexiones por persona, caída de la última, vuelta dentro y fuera de la gracia | RN-23, RN-34 |

Comparación con los criterios de éxito: SC-004 con `TestPokerStream_Scale`; SC-005 con la prueba de
consulta por cada forma y las de `ViewFor`; SC-006 con `TestUS3_RevelacionAutomatica`,
`TestUS3_UnicoParticipante` y `ShouldAutoReveal` con 1, 2 y 5; SC-007 con
`TestPokerVoteReveal_Concurrency`; SC-008 y SC-009 con `Summarize`; SC-010 con `ParseCard`; SC-011
con `TestUS3_RevelarSinSerFacilitador`, `TestUS4_NuevaRondaSinSerFacilitador`,
`TestUS5_RegistroSinSerFacilitador` y `TestUS7_CancelarSinSerFacilitador`; SC-012 con
`TestPokerOpen_Concurrency`; SC-013 con US5 y US7; SC-014 con US6-2, US6-3 y US6-4; SC-015 con
`TestUS6_RolTodaviaNoDisponible` y `FacilitatorTakeover`; SC-016 con `TestUS5_RondasConservadas` y
`TestUS7_RondasConservadas` sobre 1, 3 y 10 rondas; SC-017 y SC-018 con las pruebas transversales;
SC-020 con la tabla de arriba; SC-021 con `TestUS7_CancelacionPorInactividad` y
`TestUS7_HechoReiniciaInactividad` con un subtest por tipo de hecho; SC-022 con
`TestUS5_ReestimacionDirectaDuranteLaSesion`; SC-023 con `TestUS8_EliminacionBloqueada`; SC-024 con
`TestUS6_DosPestanas`; SC-025 con `TestPokerFacilitator_Concurrency`.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

1. **Fase técnica fundacional**: las cuatro entidades, los tipos de estado, los errores de dominio,
   `SessionRepository` (firmas), `AutoMigrate` con el índice parcial, el vaciado en `testsupport`,
   `RequireEstimationAccess`, `Presence`, el hub de difusión, el manejador del canal SSE con el
   ayudante de pruebas que lo lee, `ViewFor` y el esqueleto del barrido con reloj inyectado. En el
   frontend, `lib/event-stream.ts`, los tipos y el hook del canal. Es el bloque que bloquea a todas
   las historias.
2. **US1 Abrir y reunir** (P1): apertura con bloqueo de la historia; incorporación por el canal;
   gracia de desconexión en el barrido; endpoints de apertura y canal; sección "Planning Poker" del
   detalle de la historia y sala con la lista de participantes.
3. **US2 Votar en secreto** (P1): `ParseCard`; validación del retiro; voto con bloqueo; mazo de
   cartas.
4. **US3 Revelar** (P1): `ShouldAutoReveal`, `CanRevealManually`, `Summarize`; revelación automática
   dentro del voto, la incorporación y la desconexión; revelación manual; prueba de concurrencia de
   voto y revelación; resultado de la ronda.
5. **US4 Repetir la ronda** (P2): nueva ronda; detalle de la sesión con todas las rondas; historial
   de rondas en la sala.
6. **US5 Registrar** (P2): `ValidateAgreedValue`; registro con bloqueo de sesión e historia y
   escritura de Story Points; diálogo de registro.
7. **US6 Desconexiones y relevo** (P2): reconexión con voto; `FacilitatorTakeover`; toma del rol;
   prueba de concurrencia de tomas; reinicio de presencia al arrancar; botón de tomar el rol.
8. **US7 Cancelar** (P3): cancelación del facilitador; `AutoCancelReason` en el barrido y en cada
   acción (inactividad, historia completada, proyecto finalizado); aviso de sesión cerrada.
9. **US8 Historial** (P3): historial de la historia; reemplazo del puerto `EstimationSessions` de la
   003 con su prueba de integración; pantalla de sesión cerrada; membresía revocada en el barrido
   y cambios de la historia (FR-019, FR-074) si no se cubrieron antes.
10. **Cierre**: prueba de escala, Swagger comparado con el contrato, `README.md`, comandos de
    cobertura, `govulncheck` y la verificación transversal de trazabilidad y de las puertas de
    seguridad (SC-004, SC-005, SC-017, SC-018, SC-020).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las tres concesiones
documentadas —el estado en memoria con su barrido, la modificación puntual de la 003 y las dos rutas
bajo el prefijo de historias— son consecuencias necesarias de requisitos de la spec (FR-013, FR-014,
FR-063, FR-073, FR-088, FR-089, RC-13) y no desvíos de la constitución; están justificadas en la
sección "Verificación de la Constitución" y en [research.md](./research.md).

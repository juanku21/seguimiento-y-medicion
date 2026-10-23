# Investigación (Fase 0): Planning Poker en Tiempo Real

**Feature**: `specs/005-planning-poker` | **Fecha**: 2026-10-22

Este documento resuelve las incógnitas técnicas del plan. La spec no tiene preguntas abiertas: las
decisiones de producto se cerraron en la sesión de clarificación del 2026-09-26 (sección
Clarifications de [spec.md](./spec.md)). Cada sección registra la decisión, su fundamento y las
alternativas descartadas.

---

## 0. Contexto: esta feature se construye sobre la 001 a la 004

- Las features `specs/001-user-auth`, `specs/002-project-members`, `specs/003-product-backlog` y
  `specs/004-sprint-management` están implementadas e integradas en `main` (último merge: PR #504,
  US7 de la 004).
- La 003 ya dejó preparado el lugar de esta feature: el puerto `backlogdomain.EstimationSessions`
  (`HasSessions`), su implementación provisoria en `backlog/repository/estimation_sessions.go`, que
  informa "sin sesiones", y el motivo de bloqueo `DeletionBlockerEstimationSessions`. La enmienda de
  la 003 que pide RC-13 (cuarta condición de bloqueo de la eliminación) ya está reflejada en su
  código; esta feature solo reemplaza el cuerpo del puerto (sección 15).

Esta feature reutiliza sin cambios:

- El módulo de Go `github.com/juanku21/seguimiento-y-medicion/backend`, Gin, GORM y los contenedores
  de PostgreSQL de desarrollo (`smye_dev`, puerto 5432) y de test (`smye_test`, puerto 5433).
- El middleware de sesión de la 001 (`RequireSession`), inyectado como `gin.HandlerFunc` desde
  `cmd/app/main.go`, y el accesor `authdelivery.CurrentUserID`.
- El middleware `RequireStoryAccess` de la 003, inyectado como `gin.HandlerFunc`, igual que lo hace la
  004 (sección 3).
- Los esquemas de error `ErrorResponse`, `FieldError` y `ValidationErrorResponse` de `auth/delivery`.
- La escala `backlogdomain.StoryPointsScale` y `IsValidStoryPoints` (RC-04).
- El patrón de reloj inyectado (`now func() time.Time`) de la 002 y la 004.
- El ayudante `testsupport.OpenDatabase`, con su lock de la base de test.
- En el frontend, `lib/api-client.ts`, `lib/session-storage.ts`, `SessionGuard` y los componentes
  base de `components/`.

Esta feature **no tiene fase de arranque** propia: no agrega dependencias, servicios ni variables de
entorno (sección 1).

---

## 1. Versiones del stack y dependencias: ninguna nueva

**Decisión**: se mantienen las versiones fijadas por las features anteriores (Go 1.27.3, Gin
v1.12.0, GORM v1.31.2 con `gorm.io/driver/postgres` v1.6.3, `google/uuid` v1.6.0, `testify`
v1.12.1, Next.js 16.3.8, React 19.3.0, Tailwind CSS 4.3.3, Vitest 5.0.3, Node.js 24.21.0). **Esta
feature no agrega ninguna dependencia de Go ni de npm.**

**Rationale**: el tiempo real se resuelve con Server-Sent Events escritos con `net/http` y el
`http.Flusher` que ya expone Gin, y en el navegador con `fetch` y `ReadableStream` (sección 2). La
concurrencia en memoria usa `sync` y la difusión, canales de Go. Bloqueos de fila con
`gorm.io/gorm/clause` e índice parcial con la opción `where` de las etiquetas de GORM, como en la 004.

**Alternativas consideradas**: una biblioteca de WebSockets — descartada en la sección 2. La
constitución (Principio IV) prevé autorizar esa dependencia al planificar esta spec; el plan **no
la usa**, de modo que no hace falta pedir esa autorización.

---

## 2. Mecanismo de tiempo real: SSE sobre `fetch` y comandos REST (FR-073, RC-09)

### Decisión

- **Servidor → navegador**: un endpoint `GET /api/v1/estimation-sessions/{sessionId}/events` que
  responde `text/event-stream` (Server-Sent Events) y queda abierto. Cada mensaje es un evento
  `state` con la **instantánea completa** de la sesión vista por ese participante (sección 7). Un
  comentario `: ping` cada 15 segundos mantiene viva la conexión y detecta las caídas por error de
  escritura.
- **Navegador → servidor**: cada acción (votar, revelar, nueva ronda, registrar, cancelar, tomar el
  rol) es un endpoint REST común que devuelve la instantánea resultante. No viaja ninguna acción
  por el canal de eventos.
- **Abrir el canal es unirse**: la primera conexión de un integrante lo incorpora como participante;
  cerrarla en todas sus pantallas es el inicio de la desconexión (sección 9).
- En el navegador el canal se lee con `fetch` y `response.body.getReader()`, enviando la cabecera
  `Authorization: Bearer <token>` igual que cualquier otra llamada. No se usa `EventSource`.

### Rationale

- **Sin dependencias nuevas** en ninguno de los dos lados (Principio I y IV).
- **Toda la cadena de middlewares se reutiliza sin cambios**: `RequireSession` con su `401` único y el
  middleware de acceso con su `404` indistinguible (sección 4) protegen el canal igual que a
  cualquier endpoint. Con WebSockets, el navegador no puede enviar la cabecera `Authorization` en el
  handshake y habría que pasar el token por la URL (queda en logs) o en un primer mensaje (un
  segundo camino de autenticación que probar).
- **Las acciones siguen siendo HTTP**: se prueban con `httptest` como las del resto del proyecto, se
  documentan en Swagger (Principio VIII) y devuelven los mismos códigos de error. Un protocolo de
  mensajes sobre WebSocket no se documenta en Swagger.
- **El tráfico es asimétrico**: el servidor difunde; el cliente actúa pocas veces por minuto. SSE
  cubre exactamente eso.
- **Escala de RC-14**: 100 conexiones abiertas son 100 goroutines bloqueadas en un canal; el
  `http.Server` de `main.go` no fija `WriteTimeout`, así que no corta los streams largos. Seis
  conexiones por origen en HTTP/1.1 alcanzan para varias pestañas de una misma persona.

### Alternativas consideradas

- **WebSockets** (`github.com/coder/websocket` o `github.com/gorilla/websocket`): bidireccional, pero
  suma una dependencia, un segundo mecanismo de autenticación y un protocolo de mensajes sin
  Swagger, para un canal que solo necesita ir del servidor al cliente.
- **`EventSource` nativo**: no admite cabeceras; obligaría a poner el token en la query string.
- **Polling cada segundo**: con 100 clientes son 100 peticiones por segundo contra la base solo para
  enterarse de que nada cambió, y una latencia media de medio segundo que se suma a la del cambio.

---

## 3. Un módulo `poker` y la forma de las rutas

### Decisión

Se agrega **un** módulo vertical, `backend/internal/poker/`, con sus cuatro capas (el ámbito de
commits de la constitución ya es `poker`). Expone **diez** endpoints con dos formas de ruta, con la
misma regla que la 003 y la 004: el recurso de la ruta define la respuesta de "no acceso".

| Endpoint | Historia | Recurso de la ruta | Middleware de acceso |
| --- | --- | --- | --- |
| `POST /api/v1/stories/{storyId}/estimation-sessions` | US1 | la historia | `RequireStoryAccess` (003) |
| `GET /api/v1/stories/{storyId}/estimation-sessions` | US8 | la historia | `RequireStoryAccess` (003) |
| `GET /api/v1/estimation-sessions/{sessionId}` | US4, US8 | la sesión | `RequireEstimationAccess` (nuevo) |
| `GET /api/v1/estimation-sessions/{sessionId}/events` | US1, US6 | la sesión | `RequireEstimationAccess` |
| `PUT /api/v1/estimation-sessions/{sessionId}/vote` | US2 | la sesión | `RequireEstimationAccess` |
| `POST /api/v1/estimation-sessions/{sessionId}/reveal` | US3 | la sesión | `RequireEstimationAccess` |
| `POST /api/v1/estimation-sessions/{sessionId}/rounds` | US4 | la sesión | `RequireEstimationAccess` |
| `POST /api/v1/estimation-sessions/{sessionId}/finalize` | US5 | la sesión | `RequireEstimationAccess` |
| `POST /api/v1/estimation-sessions/{sessionId}/cancel` | US7 | la sesión | `RequireEstimationAccess` |
| `POST /api/v1/estimation-sessions/{sessionId}/facilitator` | US6 | la sesión | `RequireEstimationAccess` |

### Rationale

- La spec tiene dos respuestas de "no acceso": "historia inexistente" al abrir y al consultar el
  historial (tablas de apertura y de consulta, US8-8) y "sesión inexistente" al unirse y operar
  (US1-10). Una forma de ruta por cada una, con un middleware y una única salida `404`.
- `sprints/delivery` ya recibe `RequireStoryAccess` como `gin.HandlerFunc` desde `cmd/app/`; esta
  feature hace lo mismo, así `poker/delivery` no importa `backlog/delivery`.
- **No hay endpoint para retirar un voto** (RN-36): el intento de retirarlo es un `PUT /vote` sin
  carta, que se rechaza con el mensaje que pide US2-10 (sección 10).
- **No hay endpoint para salir de la sesión**: FR-014 define la desconexión por la caída de la
  última conexión más 30 segundos, sin distinguir si la persona cerró la pantalla o perdió la red.

### Alternativas consideradas

- **Rutas anidadas `/stories/{storyId}/estimation-sessions/{sessionId}/...`**: obligan a validar que
  la sesión pertenece a esa historia y producen dos `404` posibles para la misma sesión.
- **Un módulo por entidad** (sesiones, rondas, votos): una ronda y un voto no existen fuera de su
  sesión; partirlo multiplicaría puertos sin ganar nada.

---

## 4. Acceso a una sesión: una consulta y un `404` (FR-086 a FR-088, SC-017, SC-018)

**Decisión**: `RequireEstimationAccess` resuelve en **una** consulta la existencia de la sesión, su
historia, su proyecto y la membresía vigente del titular de la sesión
(`estimation_sessions → user_stories → project_members`). Deja en el contexto un `SessionAccess` con
`SessionID`, `StoryID` y `ProjectID`. La sesión inexistente, la de un proyecto ajeno y el
identificador mal formado comparten una única salida `404` con el mismo cuerpo
("La sesión de estimación no existe."). Sin titular en el contexto corta con el `401` de la 001.

**Rationale**: es el mismo diseño que `RequireStoryAccess` (003) y `RequireSprintAccess` (004), y es
lo que permite la comparación byte a byte de SC-018.

**Membresía revocada con el canal abierto (FR-019)**: el middleware solo corre al abrir el canal. La
revocación posterior la detecta el barrido periódico (sección 13), que cierra los canales de esa
persona y la marca desconectada. Sus acciones REST, en cambio, se rechazan de inmediato con `404`,
porque cada petición vuelve a pasar por el middleware.

---

## 5. Modelo persistido: cuatro tablas, resultado derivado (FR-004, FR-010, FR-078)

### Decisión

Cuatro tablas, una por entidad persistible de la spec: `estimation_sessions`, `estimation_rounds`,
`estimation_participants` y `estimation_votes` (detalle en [data-model.md](./data-model.md)).

- **El resultado de la ronda no se guarda**: se calcula con la función pura `Summarize` a partir de
  sus votos, que después de la revelación ya no cambian (RN-11, FR-056).
- **El facilitador vigente es una columna de la sesión** (`facilitator_id`), no un booleano por
  participante: "exactamente un facilitador" (FR-072, RN-26) queda garantizado por la estructura.
- **El momento de apertura es `created_at`** de la sesión, de la ronda y del participante; el del
  voto vigente es `updated_at` del voto, que se reescribe en cada cambio de carta (FR-022: el voto
  anterior no queda registrado).
- **La carta se guarda como texto** (`"0"` … `"100"`, `"?"`): es un conjunto cerrado que incluye un
  valor no numérico. El valor numérico se deriva al calcular.
- **Motivo de cancelación**: `cancel_reason` solo se completa cuando cancela el sistema
  (`inactivity`, `story_completed`, `project_finished`); en la cancelación del facilitador queda
  nulo y `closed_by_id` es el facilitador. En las cancelaciones del sistema `closed_by_id` es nulo.
- **`version`** en la sesión: entero que sube con cada hecho, para ordenar las instantáneas en el
  cliente (sección 7). **`last_activity_at`**: momento del último hecho, del que depende la caducidad
  (FR-063).

### Rationale

Cada columna sale de un atributo nombrado en "Entidades Clave" o de una regla que lo necesita; lo
derivable se deriva, como la instantánea de la 004.

### Alternativas consideradas

- **Guardar el resultado de la ronda** (tipo, valor, mínimo, máximo): duplica lo que los votos ya
  dicen y abre la puerta a que discrepen.
- **Guardar la lista de "sin voto" al revelar**: la spec la muestra en la vista en vivo (FR-035) a
  partir de los participantes conectados; el historial pide los votos por participante (FR-079), no
  quiénes estaban conectados. Se calcula al mostrar.
- **Presencia solo en memoria**: la revelación automática y el relevo del facilitador dependen de
  quién está conectado; leerla bajo el mismo bloqueo que el voto (sección 6) exige tenerla en la
  base.

---

## 6. Concurrencia: un único resultado (FR-006, FR-029, FR-072, SC-007, SC-012, SC-025)

### Decisión

- **Toda acción sobre una sesión abierta corre en una transacción que empieza con
  `SELECT … FOR UPDATE` sobre la fila de `estimation_sessions`**. Votar, revelar, abrir ronda,
  registrar, cancelar, tomar el rol, conectar y desconectar se serializan por sesión: el voto y la
  revelación simultáneos (US2-6) se ordenan, y la revelación ve el voto completo o no lo ve; de dos
  pedidos de tomar el rol, el segundo ve al primero ya facilitador (US6-12).
- **Apertura**: la transacción bloquea la fila de `user_stories` (`FOR UPDATE`), verifica que no haya
  una sesión `open` de esa historia e inserta sesión, ronda 1 y participante. Respaldo en la base: el
  índice único parcial `idx_estimation_sessions_one_open_per_story` sobre `story_id` con
  `WHERE status = 'open'`; su violación se traduce al mismo `409` de sesión en curso, con el
  identificador de la sesión existente (SC-012).
- **Orden de bloqueos**: sesión → historia. Registrar la estimación bloquea primero la sesión y
  después la historia, y lee el estado de la historia **después** de bloquearla, de modo que si la
  004 la completó en paralelo la sesión se cancela en lugar de escribir (sección 12). La apertura
  bloquea la historia pero nunca espera el bloqueo de una sesión, así que no hay ciclo.

### Rationale

Es la misma técnica que la 004 usa para el sprint Activo único: un bloqueo de fila hace que cada
regla se verifique sobre el último estado, y el índice parcial es la red de seguridad.

### Alternativas consideradas

- **Serializar en memoria con un mutex por sesión**: funciona con una sola instancia, pero deja la
  base sin protección ante cualquier otra escritura y no cubre la apertura.
- **Nivel de aislamiento `SERIALIZABLE`**: obliga a reintentar transacciones en todos los caminos.

---

## 7. Difusión: instantánea completa por participante (FR-073, FR-076, FR-077)

### Decisión

- Después de **confirmar** cada transacción, el servicio llama a `publish(sessionID)`. `publish`
  toma un mutex propio de esa sesión, lee la instantánea desde la base y la entrega a cada conexión
  abierta, pasada por `ViewFor(snapshot, viewerID)` (sección 8).
- Cada instantánea lleva `version`. El cliente descarta las de versión menor a la que ya tiene. La
  versión no sube cuando solo cambia el texto de la historia (sección 13), así que el cliente acepta
  versiones iguales.
- Cada conexión tiene un canal con capacidad 1 y semántica "la última gana": si el cliente es lento,
  la instantánea pendiente se reemplaza por la nueva. Nadie necesita las intermedias, porque cada
  una es el estado completo.
- Al conectarse o reconectarse, la primera instantánea que recibe la persona es el estado vigente
  (FR-077): no hay que reproducir hechos perdidos.

### Rationale

- **Una sola forma de mensaje** para todo: incorporación, voto, revelación, ronda, relevo, cierre y
  cancelación son la misma instantánea con otros datos. El cliente no mantiene una máquina de
  estados que pueda desincronizarse (FR-076).
- **El costo es acotado**: 10 participantes por sesión y una lectura de la base por hecho.
- **Leer bajo el mutex de la sesión** garantiza que dos publicaciones seguidas lean estados no
  decrecientes; la versión cubre cualquier reordenamiento en la red.

### Alternativas consideradas

- **Eventos incrementales** (`vote_cast`, `round_revealed`…): más mensajes distintos que documentar y
  probar, y un cliente que debe aplicarlos en orden y pedir el estado completo igual al reconectar.
- **Armar la instantánea dentro de la transacción**: alargaría el bloqueo de la sesión con trabajo
  de lectura y de red.

### Supuesto de despliegue

El registro de conexiones vive en memoria del proceso: el backend corre en **una sola instancia**,
como lo define el `docker-compose` actual. RC-14 (100 conexiones) cabe holgado en una instancia.
Escalar a varias instancias exigiría un canal compartido entre procesos y queda fuera de alcance.

---

## 8. Secreto del voto: una vista por espectador (FR-023, FR-024, FR-082, SC-005)

**Decisión**: la función pura `ViewFor(snapshot, viewerID)` es el **único** punto por el que sale una
sesión hacia la API, tanto en el canal como en `GET /estimation-sessions/{id}` y en la respuesta de
cada acción. Mientras la ronda vigente está `voting`:

- los votos de esa ronda se reducen a `hasVoted` por participante;
- `myVote` lleva la carta del espectador, igual en todas sus conexiones;
- las rondas anteriores, ya reveladas, muestran sus votos.

El historial de una historia (`GET /stories/{id}/estimation-sessions`) no incluye votos de ninguna
ronda, solo el resumen de cada sesión y, si hay una abierta, su facilitador, número y estado de
ronda (FR-081).

**Rationale**: un solo filtro con prueba unitaria obligatoria es más fácil de auditar que filtros
repartidos en cada endpoint. SC-005 se verifica pidiendo la sesión por cada forma de consulta desde
una cuenta distinta de la del autor.

**Reestimación directa (FR-075)**: los Story Points vigentes de la historia **no** forman parte de la
instantánea; mostrar el número que alguien escribió por afuera sería el ancla que la feature evita.

---

## 9. Presencia: conexiones, gracia de 30 segundos y relevo (FR-013 a FR-017, RN-23, RN-34)

### Decisión

- Un registro en memoria (`Presence`, en `poker/service`) cuenta las conexiones abiertas por par
  (sesión, usuario) y recuerda el momento en que cayó la última.
- **Primera conexión** (de 0 a 1) de alguien que figura desconectado o que nunca participó: hecho de
  incorporación. Transacción con bloqueo: alta del participante si no existía, `connected = true`,
  `last_activity_at`, `version + 1`, evaluación de la revelación automática (FR-031), publicación.
- **Conexiones adicionales** de la misma persona: solo suben el contador; no son un hecho.
- **Caída de la última conexión**: se anota el momento; la persona **sigue conectada**. Si en 30
  segundos no vuelve, el barrido (sección 13) registra el hecho de desconexión: `connected = false`,
  `disconnected_at`, `last_activity_at`, `version + 1`, evaluación de la revelación automática,
  publicación. Si vuelve antes, no pasó nada (FR-014).
- **Quien abre la sesión** queda conectado desde la apertura (FR-002) con su "última caída" anotada
  en ese instante: si su pantalla abre el canal enseguida, no hay hecho; si no lo abre nunca, a los
  30 segundos queda desconectado como cualquiera.
- **Reconexión** dentro de la misma ronda: conserva su fila de participante y su voto (FR-015,
  FR-016); en una ronda posterior entra sin voto (US6-4).
- **Arranque del servidor**: con el registro vacío, todos los participantes de las sesiones abiertas
  pasan a desconectados con `disconnected_at` igual al arranque, sin subir la versión ni contar como
  hecho; los clientes reconectan solos (sección 18) y se reincorporan.

### Rationale

- La gracia de FR-014 y "un participante con varias conexiones" (RN-34, SC-024) son las dos caras del
  mismo contador.
- **Lectura de los "2 segundos" de US1-3 y US6-1**: el hecho de desconexión ocurre cuando vence la
  gracia de FR-014, y desde ese momento corre el límite de FR-073. Es la única lectura compatible
  con FR-014.
- **"Desconectado desde hace 5 minutos" (FR-068)** se mide desde `disconnected_at`, es decir, desde
  que la persona dejó de contarse, no desde que se cortó su red.

### Alternativas consideradas

- **Un temporizador (`time.AfterFunc`) por desconexión**: un hilo por persona y otro camino de
  ejecución que probar; el barrido ya existe por la caducidad (sección 13).
- **Endpoint explícito para salir**: no lo pide ninguna entrada de la spec.

---

## 10. Votos, revelación y resultado (FR-020 a FR-042)

### Decisión

- **Validación de la carta en `delivery`**: el cuerpo es `{"card": "<texto>"}`. Se admiten
  exactamente `"0"`, `"1"`, `"2"`, `"3"`, `"5"`, `"8"`, `"13"`, `"20"`, `"40"`, `"100"` y `"?"`
  (`domain.ParseCard`). Cualquier otro texto, un número JSON, un negativo o un decimal: `400` con el
  campo `card` y la lista de cartas admitidas. **Carta ausente, nula o vacía**: `400` con el mensaje
  "Un voto emitido no se retira: solo se cambia por otra carta, incluida \"?\"." (RN-36, US2-10). En
  ambos casos no se llega al servicio, así que el voto vigente no cambia (FR-025).
- **Revelación automática** (`ShouldAutoReveal`): hay al menos un participante conectado y todos los
  conectados tienen voto en la ronda vigente. Se evalúa dentro de la misma transacción del voto, de
  la incorporación y de la desconexión (FR-031).
- **Revelación manual**: exige al menos un voto en la ronda (FR-033); los votos de participantes
  desconectados cuentan.
- **`Summarize(votes)`** devuelve el resultado: `consensus` con su valor, `divergence` con mínimo y
  máximo y **todos** sus autores, o `no_numeric_votes`. Los `"?"` no participan (RN-12); no se
  calcula ninguna otra agregación (FR-042).
- Los participantes conectados sin voto al revelar se listan en `notVoted` de la vista en vivo
  (FR-035).

### Rationale

Las reglas puras (`ParseCard`, `ShouldAutoReveal`, `Summarize`) son las pruebas unitarias
obligatorias del Principio II para validaciones y reglas de negocio; el servicio solo las orquesta
bajo el bloqueo.

---

## 11. Rol de facilitador (FR-064 a FR-072)

**Decisión**:

- Las acciones reservadas (revelar, nueva ronda, registrar, cancelar) verifican
  `facilitator_id = titular` dentro de la transacción; si no, `403` sin efecto (SC-011).
- **Toda acción sobre la sesión exige ser participante conectado** (FR-028); el facilitador también,
  lo que hace que al reconectarse recupere sus atribuciones sin ningún paso extra (FR-067).
- **Tomar el rol** (`FacilitatorTakeover(now, facilitator)`): se admite si el facilitador vigente
  está desconectado y `now - disconnected_at ≥ 5 min`. Si no, `409` con el facilitador vigente y, si
  está desconectado, `retryAfterSeconds` (FR-069). El que pide ya facilitador recibe el mismo `409`.
  Al tomarlo cambia `facilitator_id`, sube la versión y se publica; el anterior queda como
  participante común aunque vuelva (FR-070).

**Rationale**: el bloqueo de la sesión (sección 6) convierte "el primero que el sistema procesa"
(FR-072) en literal; el segundo pedido lee al nuevo facilitador conectado y recibe el `409` con su
nombre.

---

## 12. Cierre: registrar y cancelar (FR-049 a FR-062)

**Decisión**:

- **Registrar**: el cuerpo es `{"storyPoints": <entero>}`; se valida con `IsValidStoryPoints` de la
  003 en `delivery` (`400` con la escala para `"?"`, `null`, ausente, 7 o decimales: FR-050). En la
  transacción: sesión abierta, titular facilitador conectado, al menos una ronda `revealed` en la
  sesión (FR-051), bloqueo de la historia y verificación de que no está `completed` ni su proyecto
  `finished`. Después: `status = finished`, `agreed_story_points`, `closed_by_id`, `closed_at`, y un
  `UPDATE user_stories SET story_points, updated_at` en la **misma** transacción (SC-013). La
  condición "lista para planificar" de la 003 se recalcula sola, porque se deriva (RC-07).
- **Cancelar** (facilitador): sesión abierta y titular facilitador conectado; `status = canceled`,
  `closed_by_id`, `closed_at`; la historia no se toca (FR-058).
- **Sesión cerrada**: toda acción posterior responde `409` "La sesión de estimación ya está cerrada"
  con el estado final y el motivo, incluido `inactivity` (US7-9).
- Una historia comprometida en un sprint abierto se puede estimar (RC-08): la 004 congela los Story
  Points en `sprint_stories` al iniciar el sprint, así que reemplazar el valor vivo no altera el
  sprint.

---

## 13. Barrido periódico (FR-014, FR-019, FR-061, FR-063, FR-074)

### Decisión

Un único método `Sweep(ctx)` del servicio, que `cmd/app/main.go` ejecuta en una goroutine cada
**1 segundo** con un `time.Ticker`. En cada pasada:

1. **Cancelaciones automáticas**: una consulta trae las sesiones `open` con el estado de su historia,
   el de su proyecto y `last_activity_at`. La función pura
   `AutoCancelReason(storyStatus, projectFinished, lastActivityAt, now)` decide si corresponde
   `story_completed`, `project_finished` o `inactivity` (60 minutos, FR-063). Cada una se cancela en
   su transacción con bloqueo (vuelve a verificar bajo el bloqueo), se publica y se cierran sus
   canales.
2. **Gracias vencidas**: las personas cuya última conexión cayó hace 30 segundos o más pasan a
   desconectadas (sección 9).
3. **Membresía revocada**: una consulta trae los participantes conectados que ya no son integrantes
   del proyecto; se marcan desconectados, se les envía un evento `removed` y se cierran sus canales
   (FR-019). Sus votos quedan.
4. **Cambios en la historia**: se compara `user_stories.updated_at` de cada sesión abierta con el
   último visto; si cambió, se publica la instantánea, que ya trae el título, la descripción y los
   criterios vigentes (FR-074). La 003 actualiza `updated_at` en toda modificación, criterios
   incluidos. No es un hecho de la sesión: no sube la versión ni reinicia la inactividad.

La misma `AutoCancelReason` se evalúa dentro de cada acción, así que una acción que llega entre dos
pasadas no opera sobre una sesión que ya debía estar cancelada: la cancela y responde `409` de
sesión cerrada.

### Rationale

- **Ningún cambio en la 002 ni en la 004**: completar una historia o finalizar un proyecto no
  necesita conocer el Planning Poker; la sesión se entera en menos de un segundo y avisa dentro del
  límite de 2 segundos de FR-073.
- **Un solo bucle** para cinco obligaciones con tiempo; con 10 sesiones abiertas son unas pocas
  consultas por segundo sobre el índice parcial de sesiones abiertas.
- **Reproducible en pruebas**: el reloj es inyectado y las pruebas llaman a `Sweep` directamente; no
  esperan 30 segundos, 5 minutos ni 60 minutos reales.

### Alternativas consideradas

- **Puertos en la 002 y la 004** que avisen al completar una historia o finalizar un proyecto: la
  cancelación quedaría en la misma transacción, pero obliga a tocar dos módulos ajenos y sus
  pruebas por una diferencia de menos de un segundo, y la caducidad por inactividad igual
  necesitaría un temporizador.
- **Cancelación perezosa sin barrido** (solo al leer o actuar): no avisaría a los conectados (FR-061,
  FR-063) y la historia quedaría bloqueada hasta que alguien la mire.

---

## 14. Reestimación directa con una sesión abierta (FR-075, RN-32, RC-12)

**Decisión**: no se toca la 003. `PUT /stories/{id}/story-points` sigue funcionando igual con una
sesión abierta; la sesión no se entera (la versión no sube) y su instantánea no muestra los Story
Points vigentes (sección 8). Al registrar, la sesión escribe su valor y gana la última escritura.
Los dos caminos bloquean la fila de la historia, así que se ordenan sin perder ninguna escritura.

---

## 15. Integración con la 003

### 15.1 Puerto `EstimationSessions`

`backlog/repository/estimation_sessions.go` reemplaza su cuerpo: `HasSessions` consulta con GORM si
existe alguna fila de `estimation_sessions` con ese `story_id`, en cualquier estado (FR-089, RN-33,
SC-023). Su constructor pasa a recibir la conexión, y `cmd/app/main.go` le pasa `db`.
`backlog/service` no cambia.

### 15.2 Escritura de los Story Points

El único campo de la historia que escribe esta feature es `story_points` (con `updated_at`), desde
el repositorio de `poker`, dentro de la transacción de registrar (RC-03). Igual que la 004 escribe
`status` y `ever_committed`.

### 15.3 Dirección de las importaciones

- `poker/domain` importa `backlog/domain` (historia, estado y escala) y `auth/domain` (usuario, para
  las claves foráneas).
- `backlog/repository` importa `poker/domain` para leer la tabla del puerto.
- Ningún `domain` importa `poker`, y `poker/delivery` no importa `backlog/delivery`: recibe
  `RequireStoryAccess` desde `cmd/app/`. No hay ciclos.

### 15.4 Ventana de carrera al eliminar una historia

La 003 verifica `HasSessions` y después borra. Si una sesión se abre entre ambos pasos, la clave
foránea de `estimation_sessions.story_id` hace fallar el borrado: el historial **no** se pierde, pero
la 003 responde `500` en lugar del `409`. Se recomienda tratarlo en `/revisar-seguridad` de esta
feature traduciendo la violación de clave foránea al `409` de eliminación bloqueada, como quedó
anotado para la 004 (sección 16 de su research).

### 15.5 Frontend de la 003

El detalle de la historia (`app/projects/[projectId]/backlog/[storyId]/page.tsx`) suma la sección
"Planning Poker" (sección 18).

---

## 16. Historial y consulta (FR-078 a FR-085)

**Decisión**:

- `GET /stories/{id}/estimation-sessions` devuelve `openSession` (o `null`) y `sessions`: las
  sesiones cerradas ordenadas por `created_at` descendente, con estado, valor acordado, motivo,
  quién abrió, quién cerró, momentos y cantidad de rondas. Sin sesiones: lista vacía y
  `emptyReason: "no_sessions"`, con `200` (FR-083). Se resuelve en **una** consulta para la lista,
  con el conteo de rondas por subconsulta, y otra para la sesión abierta.
- `GET /estimation-sessions/{id}` devuelve la sesión completa con todas sus rondas, pasada por
  `ViewFor`; para una sesión cerrada ya no hay nada que ocultar (US8-1).
- Los nombres salen de `users.full_name` con un `JOIN`, sin filtrar por membresía, así que los votos
  de quien dejó el proyecto conservan su nombre (FR-084). Nunca se selecciona `password_hash`.
- Las lecturas no exigen proyecto en curso (FR-085).

---

## 17. Pruebas de tiempo real

**Decisión**:

- Las pruebas de integración del canal levantan el router con `httptest.NewServer` y leen el stream
  con `net/http` y `bufio`; un ayudante de prueba devuelve la próxima instantánea con un plazo.
- El servicio recibe el reloj (`now`) y el intervalo del barrido; las pruebas avanzan el reloj y
  llaman a `Sweep`.
- **Escala (SC-004)**: `TestPokerStream_Scale` abre 10 sesiones con 10 canales cada una, emite un
  hecho por sesión y mide que las 100 conexiones reciban la instantánea en menos de 2 segundos. Se
  omite con `go test -short`.
- **Concurrencia**: `TestPokerOpen_Concurrency` (10 aperturas, SC-012),
  `TestPokerFacilitator_Concurrency` (10 tomas, SC-025) y `TestPokerVoteReveal_Concurrency` (voto y
  revelación simultáneos, 20 repeticiones comparando las vistas, SC-007).

---

## 18. Frontend

**Decisión**:

- `lib/event-stream.ts`: lector genérico de `text/event-stream` sobre `fetch` (parser de líneas
  `event:`, `data:` y comentarios), con prueba unitaria.
- `features/poker/use-estimation-stream.ts`: hook que abre el canal con el token de
  `session-storage`, descarta instantáneas viejas por `version`, reconecta con espera creciente
  (1, 2, 5 segundos, tope 5) y se detiene ante `409` (sesión cerrada: muestra el resultado), `404` y
  `401` (`rejectSession`).
- Pantalla `app/projects/[projectId]/backlog/[storyId]/poker/[sessionId]/page.tsx`: sala en vivo si
  la sesión está abierta; resultado y rondas si está cerrada.
- Sección "Planning Poker" en el detalle de la historia: abrir o unirse, sesión en curso e historial.
- Antes de escribir código de Next.js se lee la documentación de la versión instalada en
  `frontend/node_modules/next/dist/docs/` (CLAUDE.md).

---

## 19. Códigos de estado y forma de los errores

**Decisión**: se reutilizan los esquemas de error de la 001 y se fija esta correspondencia:

| Situación | Código | Requisito |
| --- | --- | --- |
| Carta inválida, retiro del voto, valor final fuera de la escala | `400` | FR-022, FR-025, FR-050 |
| Sin sesión válida | `401` | FR-086 |
| Historia inexistente o ajena (abrir, historial) | `404` | FR-088 |
| Sesión inexistente o ajena | `404` | FR-088 |
| Sesión Finalizada o Cancelada (incluye la cancelada por el sistema, con su motivo) | `409` | FR-018, FR-027, FR-055, FR-060 |
| Quien actúa no es participante conectado | `403` | FR-028 |
| Acción reservada al facilitador | `403` | FR-034, FR-047, FR-054, FR-059 |
| Historia con una sesión abierta (con su identificador) | `409` | FR-006 |
| Historia Completada | `409` | FR-007 |
| Proyecto Finalizado | `409` | FR-008 |
| Ronda revelada (votar, revelar otra vez) o en votación (nueva ronda) | `409` | FR-026, FR-046 |
| Revelar sin votos; registrar sin ninguna ronda revelada | `409` | FR-033, FR-051 |
| Tomar el rol con el facilitador presente o desconectado hace menos de 5 minutos | `409` | FR-069, FR-072 |

**Precedencia**: `401` → `404` → `400` → `409` de sesión cerrada → `403` → demás `409`. La sesión
cerrada va antes que los `403` porque una sesión cerrada no tiene participantes ni facilitador que
verificar. El `403` sigue la convención de la 002 para acciones reservadas a un rol.

**Proyecto Finalizado con una sesión abierta**: la sesión se cancela con `project_finished`
(sección 13) y toda escritura posterior responde `409` con `SessionClosedResponse`. El texto de
`error` depende del motivo, para cumplir la fila de Condiciones de Error de la spec:
`project_finished` → `El proyecto está finalizado y es de solo lectura.`; `inactivity` →
`La sesión de estimación se cerró por inactividad.`; los demás casos →
`La sesión de estimación ya está cerrada.`

---

## Resumen: incógnitas resueltas

| Incógnita | Resolución | Sección |
| --- | --- | --- |
| Dependencias nuevas | Ninguna | 1 |
| Mecanismo de tiempo real | SSE sobre `fetch` para difundir; REST para actuar | 2 |
| Organización y rutas | Un módulo `poker`; apertura e historial sobre la historia, el resto sobre la sesión | 3 |
| `404` indistinguible | `RequireEstimationAccess` con una consulta y una salida | 4 |
| Persistencia | Cuatro tablas; resultado derivado; facilitador como columna | 5 |
| Un único resultado bajo concurrencia | Bloqueo de la sesión; bloqueo de la historia e índice parcial al abrir | 6 |
| Consistencia de las vistas | Instantánea completa, versión monotónica, la última gana | 7 |
| Secreto del voto | `ViewFor` como única salida | 8 |
| Presencia, pestañas y cortes | Contador en memoria, gracia de 30 s, estado en la base | 9 |
| Revelación y resultado | `ShouldAutoReveal` y `Summarize`, puras | 10 |
| Relevo del facilitador | `FacilitatorTakeover` bajo el bloqueo | 11 |
| Registrar y cancelar | Una transacción; escritura de Story Points en la misma | 12 |
| Inactividad, cancelaciones del sistema, membresía y cambios de la historia | Barrido cada 1 s con reloj inyectado | 13 |
| Reestimación directa | Sin cambios en la 003; gana la última escritura | 14 |
| Integración con la 003 | Reemplazo del puerto `EstimationSessions` | 15 |
| Historial | Dos lecturas; nombres desde `users` | 16 |
| Pruebas de tiempo real | `httptest.NewServer`, reloj inyectado, prueba de escala | 17 |
| Frontend | Lector de stream propio, hook con reconexión, una pantalla y una sección | 18 |
| Códigos HTTP | Tabla y precedencia | 19 |

### Puntos abiertos para el desarrollador (no bloquean el diseño)

1. **SSE en lugar de WebSockets** (sección 2): la constitución prevé autorizar una dependencia de
   WebSockets para esta spec; el plan no la necesita. Si el equipo prefiere WebSockets, hay que
   pedir esa autorización y rehacer las secciones 2, 7, 17 y 18.
2. **Una sola instancia del backend** (sección 7): el registro de conexiones vive en memoria.
3. **Lectura de los "2 segundos" en las desconexiones** (sección 9): se miden desde que vence la
   gracia de 30 segundos de FR-014. Confirmada y registrada en la spec (Clarifications, sesión del
   2026-10-23, y FR-073).
4. **Ventana de carrera al eliminar una historia** (sección 15.4), para `/revisar-seguridad`.
5. **Sesión de autenticación revocada con el canal abierto**: el canal se cierra al vencer el token
   (`ExpiresAt` de los claims), pero un cierre de sesión hecho desde otro dispositivo no corta un
   canal ya abierto hasta ese vencimiento. Las acciones REST sí se rechazan de inmediato. Se
   recomienda revisarlo en `/revisar-seguridad`.

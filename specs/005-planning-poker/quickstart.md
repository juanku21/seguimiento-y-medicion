# Guía de Validación (Fase 1): Planning Poker en Tiempo Real

**Feature**: `specs/005-planning-poker` | **Fecha**: 2026-10-22

Pasos para comprobar de punta a punta que la feature funciona. Los detalles de los endpoints están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y los del modelo en
[data-model.md](./data-model.md); acá no se repiten.

---

## 1. Prerrequisitos

- Las features `specs/001-user-auth` a `specs/004-sprint-management` están integradas en `main`.
- **Sin variables de entorno nuevas**: se usan los mismos `backend/.env` y `backend/.env.test`.
- **Sin dependencias nuevas** de Go ni de npm. El tiempo real usa Server-Sent Events sobre `fetch`
  ([research.md](./research.md), sección 2).
- **Una sola instancia del backend** (research, sección 7).

---

## 2. Levantar los servicios

Sin cambios respecto de la 004:

```powershell
cd backend/postgres

# Desarrollo (puerto 5432, base smye_dev)
docker compose --env-file .env -p smye-dev up -d

# Test (puerto 5433, base smye_test)
docker compose --env-file .env.test -p smye-test up -d
```

Comprobación de aislamiento: la primera consulta debe responder `smye_test` y la segunda, el puerto
5433 publicado en el anfitrión (el mismo `DB_PORT` de `backend/.env.test`). No se usa
`inet_server_port()`: `docker exec` conecta por el socket local y devuelve el puerto vacío, y con
`-h localhost` devolvería 5432, el puerto interno del contenedor.

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -tA -c "SELECT current_database();"
docker port smye-test-postgres-1 5432
```

`AutoMigrate` crea las cuatro tablas `estimation_*` al arrancar la aplicación o la primera prueba.
Comprobación del índice parcial en la base de test (la definición debe terminar en una cláusula
`WHERE`):

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT indexname, indexdef FROM pg_indexes WHERE indexname = 'idx_estimation_sessions_one_open_per_story';"
```

---

## 3. Ejecutar las pruebas

### Backend

```powershell
cd backend
# Swagger sin instalar la CLI (con la CLI instalada: swag init -g cmd/app/main.go -o docs)
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test -p 1 ./...

# Solo el módulo de esta feature
go test -p 1 ./internal/poker/...

# Trazabilidad del Principio II: los escenarios de una historia
go test -p 1 ./internal/poker/... -run TestUS3

# Concurrencia: una sola sesión abierta, un solo facilitador, voto y revelación (SC-007, SC-012, SC-025)
go test -p 1 ./internal/poker/... -run Concurrency -count=5

# Escala del tiempo real: 10 sesiones x 10 conexiones (SC-004); se omite con -short
go test -p 1 ./internal/poker/... -run Scale

# Puerto reemplazado en la 003 (eliminación bloqueada por historial de estimación)
go test -p 1 ./internal/backlog/...

# Cobertura
go test -p 1 ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Todos los `go test` llevan `-p 1` porque los paquetes comparten la base `smye_test`: en paralelo se
quedan esperándose entre sí y fallan al superar la espera máxima (README, paso 9).

Resultado esperado: todo en `ok`. Buscar `005/US` en el código de pruebas debe devolver los 79
escenarios de la spec.

### Frontend

```powershell
cd frontend
npm test
npm run test:coverage
npm run lint
npx tsc --noEmit
```

### Vulnerabilidades

```powershell
cd backend
govulncheck ./...
```

---

## 4. Levantar la aplicación

```powershell
# Terminal 1 — backend en http://localhost:8080
cd backend
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/app/main.go -o docs
go run ./cmd/app

# Terminal 2 — frontend en http://localhost:3000
cd frontend
npm run dev
```

Swagger: <http://localhost:8080/swagger/index.html>. Debe mostrar la etiqueta **Planning Poker** con
los diez endpoints de [contracts/openapi.yaml](./contracts/openapi.yaml).

---

## 5. Preparación: cuentas, proyecto, historia y tokens

Con los endpoints de la 001 a la 003 (ver sus quickstart):

1. Registrar a Ana, Bruno, Carla y Diego e iniciar sesión con los cuatro. Los tokens se llaman
   `$ANA`, `$BRUNO`, `$CARLA` y `$DIEGO`.
2. Ana crea el proyecto "Portal de Clientes" y agrega a Bruno y a Carla. Diego **no** es
   integrante. El proyecto pasa a `in_progress`.
3. Ana crea la historia `$STORY` con un criterio de aceptación y sin estimar, y la historia
   `$BARE` sin criterios.

Las acciones siguen la forma habitual:

```powershell
$h = @{ Authorization = "Bearer $ANA" }
$session = Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/stories/$STORY/estimation-sessions" -Headers $h
$SESSION = $session.id
```

El canal de eventos se observa con `curl.exe` (`-N` desactiva el búfer), uno por persona y en
terminales separadas:

```powershell
curl.exe -N -H "Authorization: Bearer $BRUNO" "http://localhost:8080/api/v1/estimation-sessions/$SESSION/events"
```

Cada hecho produce una línea `event: state` seguida de `data:` con la sesión completa. Lo inverso no
vale: el barrido también republica un `state` con la misma `version` cuando cambia la historia
(FR-074), sin que haya un hecho nuevo en la sesión.

---

## 6. Escenarios de validación de punta a punta

Cada bloque corresponde a una historia de [spec.md](./spec.md). Las pruebas automatizadas que los
cubren se nombran `TestUSn_<Escenario>` y llevan el comentario `(005/USn)`.

### US1 — Abrir una sesión y reunir al equipo

1. Ana abre la sesión sobre `$STORY`: `201`, `status: open`, ronda 1 `voting`, Ana facilitadora y
   conectada.
2. Ana, Bruno y Carla abren su canal: cada uno recibe un `state` con los tres conectados en menos de
   2 segundos desde cada incorporación.
3. Carla corta su `curl`: a los 30 segundos, Ana y Bruno reciben un `state` con Carla
   `connected: false`.
4. Bruno intenta abrir otra sesión sobre `$STORY`: `409` con `sessionId` igual a `$SESSION`.
5. Diego pide el canal: `404` con el mismo cuerpo que un `sessionId` inventado.

### US2 — Votar en secreto

1. Bruno vota `{"card":"5"}`: `200`, `myVote: "5"`. Los canales de Ana y Carla muestran a Bruno con
   `hasVoted: true` y la ronda vigente con `votes: null`.
2. Bruno vota `"13"`: el voto pasa a 13; los demás siguen viendo solo `hasVoted`.
3. Bruno vota `"4"`, `"50"`, `"-1"` y `"1.5"`: `400` con las cartas admitidas; `myVote` sigue en 13.
4. Bruno envía `{"card":null}`: `400` "Un voto emitido no se retira…"; `myVote` sigue en 13.
5. SC-005: Ana pide `GET /estimation-sessions/$SESSION` y `GET /stories/$STORY/estimation-sessions`;
   ninguna respuesta contiene el 13 de Bruno.

### US3 — Revelar los votos y ver las diferencias

1. Con Ana, Bruno y Carla conectados, votan 3, 5 y 13: al tercer voto la ronda pasa a `revealed` sola
   y los tres reciben `outcome.kind: divergence`, `min` 3 y `max` 13 con sus autores.
2. En otra ronda, Bruno intenta revelar: `403` reservado al facilitador.
3. Con todos sin votar, Ana intenta revelar: `409` "Hace falta al menos un voto".

### US4 — Repetir la ronda

1. Con la ronda 1 revelada, Ana inicia una ronda: `201`, ronda 2 `voting`, nadie con voto.
2. `GET /estimation-sessions/$SESSION` muestra la ronda 1 con sus votos y su divergencia.
3. Con la ronda 2 en votación, Ana intenta otra ronda: `409` "Primero hay que revelar".

### US5 — Registrar la estimación acordada

1. Ana registra `{"storyPoints":7}` y luego `{"storyPoints":null}`: `400` con la escala.
2. Ana registra `{"storyPoints":8}`: `200`, `status: finished`; todos los canales reciben el último
   `state` y se cierran. `GET /api/v1/stories/$STORY` (003) muestra 8 Story Points.
3. Reestimación directa (US5-5): con una sesión nueva abierta, Bruno reestima `$STORY` en 3 con
   `PUT /api/v1/stories/$STORY/story-points`; no hay un hecho nuevo en la sesión: la `version` no
   cambia y la ronda sigue en `voting`. Unos 0,6 segundos después del `PUT`, cada canal puede
   recibir un `state` con esa misma `version`: es la republicación del barrido por el cambio de la
   historia (FR-074). Al registrar 8, la historia queda en 8.

### US6 — Desconexiones y relevo del facilitador

1. Bruno abre el canal en dos terminales y vota 8; cierra una: sigue conectado y con su voto.
2. Ana corta su canal: a los 30 segundos figura desconectada y `facilitatorAvailableAt` indica el
   momento del relevo. Bruno pide `POST /facilitator` antes: `409` con `retryAfterSeconds`.
3. Pasados 5 minutos, Bruno toma el rol: `200`, Bruno facilitador. Ana se reconecta y queda como
   participante común.

### US7 — Cancelar una sesión

1. Ana cancela: `200`, `status: canceled`; la historia conserva sus Story Points.
2. Ana vuelve a cancelar: `409` de sesión cerrada.
3. Se abre una sesión nueva sobre la misma historia: `201`.
4. Inactividad (SC-021): la prueba automatizada avanza el reloj 60 minutos y llama al barrido; la
   sesión queda `canceled` con `cancelReason: inactivity`. Para verlo a mano hace falta esperar la
   hora real.

### US8 — Historial de estimación

1. `GET /api/v1/stories/$STORY/estimation-sessions`: sesiones cerradas de la más reciente a la más
   antigua, con valor acordado y cantidad de rondas; `openSession` sin votos si hay una abierta.
2. `GET /api/v1/stories/$BARE/estimation-sessions`: `200`, `sessions: []`,
   `emptyReason: no_sessions`.
3. Ana intenta eliminar `$STORY` (003): `409` con el motivo `estimation_sessions` (SC-023).

### Cancelaciones del sistema (FR-061)

- Con una sesión abierta sobre una historia comprometida en el sprint Activo, se la pasa primero a
  `in_progress` con `PUT /api/v1/stories/{id}/status` (004), porque desde `pending` el paso a
  `completed` responde `409` "Transición no permitida…"; pasar a `in_progress` no cancela la sesión.
  Después se la marca `completed` con el mismo endpoint: en menos de 2 segundos los canales reciben
  `status: canceled`, `cancelReason: story_completed`.
- Con una sesión abierta, Ana cierra antes el sprint Activo con
  `POST /api/v1/sprints/{sprintId}/close` (004), porque con un sprint activo finalizar el proyecto
  responde `409` "No se puede finalizar el proyecto mientras tenga un sprint activo. Cerralo
  primero.". Después finaliza el proyecto (002): `cancelReason: project_finished`.

### Flujo en la interfaz

1. Ana entra al detalle de `$STORY` y pulsa **Abrir Planning Poker**; la app la lleva a la sala.
2. Bruno y Carla entran al mismo detalle, ven "Sesión en curso" y pulsan **Unirse**.
3. Cada uno elige una carta; la propia queda resaltada y las ajenas se muestran boca abajo.
4. Al votar el último, las cartas se dan vuelta solas y se marcan el mínimo y el máximo.
5. Ana registra el valor; las tres pantallas muestran la sesión finalizada sin recargar.
6. En el detalle de la historia, la sección "Planning Poker" lista la sesión con su valor y sus
   rondas.

---

## 7. Puertas de calidad antes de dar por terminada la feature

- [ ] Las 79 pruebas nombradas `005/USn` existen y pasan, escritas antes que el código.
- [ ] Las pruebas unitarias de `ParseCard`, `ValidateAgreedValue`, `ShouldAutoReveal`,
      `CanRevealManually`, `Summarize`, `FacilitatorTakeover`, `AutoCancelReason`, `EnsureOpen`,
      `ViewFor` y `Presence` pasan.
- [ ] Las pruebas de concurrencia y de escala pasan con `-count=5`.
- [ ] Las pruebas corrieron contra `smye_test`, no contra `smye_dev`.
- [ ] Sin dependencias nuevas en `go.mod` ni en `package.json`.
- [ ] Sin `any` en el frontend; sin errores de Go sin manejar; sin `panic` en el camino de una
      petición ni en el barrido.
- [ ] Cada bloque nuevo comentado en español; identificadores en inglés.
- [ ] Swagger coincide con [contracts/openapi.yaml](./contracts/openapi.yaml); `README.md` describe
      el Planning Poker y sus endpoints.
- [ ] `govulncheck ./...` sin hallazgos.

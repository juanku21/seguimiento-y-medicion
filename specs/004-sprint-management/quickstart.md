# Guía de Validación (Fase 1): Gestión de Sprints

**Feature**: `specs/004-sprint-management` | **Fecha**: 2026-10-21

Pasos para comprobar de punta a punta que la feature funciona. Los detalles de los endpoints están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y los del modelo en
[data-model.md](./data-model.md); acá no se repiten.

---

## 1. Prerrequisitos

- Las features `specs/001-user-auth`, `specs/002-project-members` y **`specs/003-product-backlog`**
  están integradas en `main`. La 003 es imprescindible: esta feature compromete sus historias y
  reemplaza su puerto `SprintAssignments`.
- **Sin variables de entorno nuevas**: se usan los mismos `backend/.env` y `backend/.env.test`. La zona
  horaria de los días restantes y de los momentos reales es `APP_TIMEZONE`, ya existente.
- **Sin dependencias nuevas** de Go ni de npm.

---

## 2. Levantar los servicios

Sin cambios respecto de la 002:

```powershell
cd backend/postgres

# Desarrollo (puerto 5432, base smye_dev)
docker compose --env-file .env -p smye-dev up -d

# Test (puerto 5433, base smye_test)
docker compose --env-file .env.test -p smye-test up -d
```

Comprobación de aislamiento: debe responder `smye_test`.

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT current_database(), inet_server_port();"
```

`AutoMigrate` crea las tablas `sprints` y `sprint_stories`, sus índices parciales y la columna
`user_stories.ever_committed` al arrancar la aplicación o la primera prueba. Comprobación de los
índices parciales en la base de test:

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT indexname, indexdef FROM pg_indexes WHERE indexname IN ('idx_sprints_one_active_per_project', 'idx_sprint_stories_one_open_per_story');"
```

Ambas definiciones deben terminar en una cláusula `WHERE`.

---

## 3. Ejecutar las pruebas

### Backend

Los comandos que abarcan varios paquetes llevan `-p 1`: el advisory lock de `backend/internal/platform/testsupport/database.go` ya serializa los paquetes, y correrlos en paralelo puede superar su espera máxima de 5 minutos.

```powershell
cd backend
swag init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test -p 1 ./...

# Solo el módulo de esta feature
go test -p 1 ./internal/sprints/...

# Trazabilidad del Principio II: los escenarios de una historia
go test -p 1 ./internal/sprints/... -run TestUS5

# Concurrencia: un solo sprint activo y cero superposiciones (SC-004, SC-005)
go test -p 1 ./internal/sprints/... -run Concurrency -count=5

# Puertos reemplazados en la 002 y la 003
go test -p 1 ./internal/projects/... ./internal/backlog/...

# Cobertura
go test -p 1 ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Resultado esperado: todo en `ok`. Buscar `004/US` en el código de pruebas debe devolver los 82
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
swag init -g cmd/app/main.go -o docs
go run ./cmd/app

# Terminal 2 — frontend en http://localhost:3000
cd frontend
npm run dev
```

Swagger: <http://localhost:8080/swagger/index.html>. Debe mostrar la etiqueta **Sprints** con los
once endpoints de [contracts/openapi.yaml](./contracts/openapi.yaml).

---

## 5. Preparación: cuentas, proyecto, historias y tokens

Con los endpoints de la 001, la 002 y la 003 (ver sus quickstart):

1. Registrar a Ana (`ana@example.com`), Bruno (`bruno@example.com`) y Carla (`carla@example.com`) e
   iniciar sesión con los tres. Los tokens se llaman `$ANA`, `$BRUNO` y `$CARLA`.
2. Ana crea el proyecto "Portal de Clientes" con factor `6.5` y agrega a Bruno. Carla **no** es
   integrante. El proyecto se llama `$PROJECT` y queda `planned`.
3. Ana crea en el backlog cinco historias listas para planificar (`$S1` a `$S5`, con 5, 8, 3, 5 y 8
   Story Points y un criterio cada una) y una sin criterios (`$S6`).

En PowerShell, cada llamada sigue esta forma:

```powershell
$h = @{ Authorization = "Bearer $ANA" }
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/projects/$PROJECT/sprints" -Headers $h -ContentType "application/json" -Body '{"name":"Sprint 1","goal":"Cerrar el alta de clientes","startDate":"2026-10-01","endDate":"2026-10-14"}'
```

---

## 6. Escenarios de validación de punta a punta

Cada bloque corresponde a una historia de [spec.md](./spec.md). Las pruebas automatizadas que los
cubren se nombran `TestUSn_<Escenario>` y llevan el comentario `(004/USn)`.

### US1 — Planificar un sprint

| Acción | Resultado esperado |
| --- | --- |
| Ana crea "Sprint 1" del 2026-10-01 al 2026-10-14 | `201`, `status: planned`, `stories: []`, `createdBy` es Ana; el proyecto sigue `planned` |
| Bruno crea "Sprint 2" del 2026-10-14 al 2026-10-28 | `201`: compartir el día de borde no superpone |
| Crear otro del 2026-10-10 al 2026-10-20 | `409` con `sprint.name: "Sprint 1"` |
| Crear uno con `endDate` igual a `startDate` | `400` en `endDate` |
| Crear uno con `goal` de 501 caracteres | `400` en `goal`; con 500, `201` |
| Carla crea un sprint en `$PROJECT` | `404` idéntico al de un proyecto inexistente |

### US2 — Comprometer historias

| Acción | Resultado esperado |
| --- | --- |
| Asignar `$S1`, `$S2`, `$S3` a "Sprint 1" | `200`; las tres en `stories` como `pending` |
| Asignar `$S3` a "Sprint 2" | `409` con `reason: committed_elsewhere` y `sprint.name: "Sprint 1"` |
| Asignar `$S4` y `$S6` a "Sprint 2" | `409` con `$S6` como `not_ready` y `missing: [acceptance_criteria]`; `$S4` **no** queda asignada |
| Estimar `$S1` en 8 desde el backlog (003) | `200`: sigue lista para planificar |
| Estimar `$S1` como `null` o vaciar sus criterios (003) | `409`: primero hay que quitarla del sprint |
| Quitar `$S3` de "Sprint 1" (`DELETE .../stories?storyId=$S3`) | `200`; `$S3` vuelve `pending` y se puede asignar a "Sprint 2" |
| Eliminar `$S3` del backlog (003) | `409` con motivo `sprint_history` |

### US3 — Iniciar un sprint

| Acción | Resultado esperado |
| --- | --- |
| Bruno inicia "Sprint 1" | `200`, `status: active`, `startedAt` informado; el proyecto sigue `planned` |
| Iniciar "Sprint 2" | `409` con `sprint.name: "Sprint 1"` |
| Iniciar "Sprint 1" otra vez | `409` de transición no permitida |
| `GET /api/v1/projects/$PROJECT` (002) | `activeSprint` es "Sprint 1" |

### US4 — Registrar el avance

| Acción | Resultado esperado |
| --- | --- |
| `PUT /api/v1/stories/$S1/status` con `in_progress` y luego `completed` | `200` y `200`; el segundo con `completedAt` |
| `$S2` de `pending` a `completed` | `409` enumerando las transiciones admitidas |
| `$S1` de `completed` a `in_progress` | `200` con `completedAt: null` |
| Avance sobre una historia de "Sprint 2" (Planificado) | `409`: solo dentro del sprint activo |
| Avance sobre `$S5` (sin sprint) | `409`: solo dentro del sprint activo |

### US5 — Cerrar el sprint

Preparación: `$S1` Completada, `$S2` Pendiente, y se suma `$S5` (8) con el sprint ya Activo.

| Acción | Resultado esperado |
| --- | --- |
| Cerrar "Sprint 1" | `200`, `status: closed`, `closedBy` es quien cerró y `hoursPerStoryPoint: 6.5` |
| Leer `results` | `committedStoryPoints` = puntos al iniciar; `plannedStoryPoints` incluye a `$S5`; `completedStoryPoints` = puntos de `$S1` |
| Ver el backlog (003) | `$S2` y `$S5` vuelven `pending` y sin sprint; `$S1` sigue `completed` |
| Renombrar `$S2`, reestimarla y cambiar el factor del proyecto a `8` | El detalle de "Sprint 1" no cambia en nada (SC-008) |
| Cerrar "Sprint 1" otra vez | `409`: el estado Cerrado es definitivo |

### US6 — Consultar el historial

| Acción | Resultado esperado |
| --- | --- |
| `GET .../sprints` | Todos los sprints, por fecha de inicio descendente, con `results` solo en los cerrados |
| `GET .../sprints?status=closed` | Solo "Sprint 1", con las cuatro fechas y los cinco totales |
| `GET /api/v1/stories/$S2/sprints` | "Sprint 1" con `outcome: not_completed` |
| Iniciar "Sprint 2" y consultar su detalle | `schedule.remainingDays` según la fecha de hoy en `APP_TIMEZONE`, o `overdueDays` si la fecha de fin ya pasó |
| `GET .../sprints` sobre un proyecto nuevo sin sprints | `sprints: []`, `emptyReason: no_sprints` |
| Carla consulta un sprint de `$PROJECT` | `404` idéntico al de un sprint inexistente |

### US7 — Corregir o descartar un sprint planificado

| Acción | Resultado esperado |
| --- | --- |
| Crear "Sprint 3", asignarle dos historias y cambiarle nombre, fechas y objetivo con `PUT` | `200`; sigue `planned` con las dos historias |
| `PUT` sobre "Sprint 2" (Activo) | `409`: solo se modifica un sprint Planificado |
| Eliminar "Sprint 3" | `204`; sus historias vuelven al backlog y su período queda libre |
| Eliminar del backlog una de esas historias (003) | `409` con `sprint_history` |
| Eliminar "Sprint 1" (Cerrado) | `409`: solo se elimina un sprint que nunca fue iniciado |
| Consultar "Sprint 3" | `404` de sprint inexistente |

### Proyecto Finalizado (FR-059, SC-012)

Cerrar el sprint activo, pasar el proyecto a `in_progress` y luego a `finished` (002). Los ocho
endpoints de escritura responden `409` de solo lectura; listado, detalle e historial de
una historia siguen respondiendo `200`.

### Flujo en la interfaz

1. Desde la vista del proyecto, entrar a **Sprints** y crear un sprint.
2. En su detalle, elegir historias del backlog listas para planificar y comprometerlas.
3. Iniciar el sprint: el detalle muestra los días restantes.
4. Mover una historia a En progreso y luego a Completada; la marca de completado aparece con su
   fecha.
5. Cerrar el sprint y confirmar: el detalle muestra la instantánea y el historial de cerrados la
   lista.
6. Abrir una historia en el backlog: su sección "Sprints en los que estuvo" muestra el sprint y su
   resultado.

---

## 7. Puertas de calidad antes de dar por terminada la feature

- [ ] `go test -p 1 ./...` y `npm test` pasan contra el entorno de test.
- [ ] Buscar `004/US` en las pruebas devuelve los 82 escenarios; las reglas RN-01 a RN-23 tienen
      prueba asociada (SC-017).
- [ ] Las respuestas `404` de sprint ajeno e inexistente son idénticas byte a byte en las siete
      acciones de sprint; las de proyecto, en las dos de colección (SC-014).
- [ ] Cada uno de los once endpoints tiene su prueba sin sesión (SC-013).
- [ ] La prueba de 10 inicios simultáneos deja exactamente un sprint Activo (SC-005).
- [ ] Swagger publicado coincide con [contracts/openapi.yaml](./contracts/openapi.yaml).
- [ ] `README.md` menciona los sprints en la introducción y en la lista de endpoints.
- [ ] `govulncheck ./...` sin hallazgos; sin `any` en el frontend (`npx tsc --noEmit` y ESLint).

# Guía de Validación (Fase 1): Product Backlog

**Feature**: `specs/003-product-backlog` | **Fecha**: 2026-10-20

Pasos para comprobar de punta a punta que la feature funciona. Los detalles de los endpoints están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y los del modelo en
[data-model.md](./data-model.md); acá no se repiten.

---

## 1. Prerrequisitos

- Las features `specs/001-user-auth` y `specs/002-project-members` están integradas en `main`
  (incluida la fase 10 de la 002, con el límite de 1 MiB del cuerpo).
- **Sin variables de entorno nuevas**: se usan los mismos `backend/.env` y `backend/.env.test`.
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

Las tablas `user_stories` y `acceptance_criteria` las crea `AutoMigrate` al arrancar la aplicación o
al correr las pruebas. El ayudante de pruebas las vacía junto con las demás:

```text
TRUNCATE users, sessions, auth_events, projects, project_members, user_stories, acceptance_criteria CASCADE;
```

---

## 3. Ejecutar las pruebas

### Backend

```powershell
cd backend
swag init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test ./...

# Solo el módulo de esta feature
go test ./internal/backlog/...

# Trazabilidad del Principio II: los escenarios de una historia
go test ./internal/backlog/... -run TestUS4

# La vista de estado de la 002 con el resumen real del backlog
go test ./internal/projects/... -run TestUS4

# Cobertura
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Resultado esperado: todo en `ok`. Buscar `003/US` en el código de pruebas debe devolver los 58
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

Swagger: <http://localhost:8080/swagger/index.html>. Debe mostrar la etiqueta **Backlog** con los
seis endpoints de [contracts/openapi.yaml](./contracts/openapi.yaml).

---

## 5. Preparación: tres cuentas, un proyecto y los tokens

Con los endpoints de la 001 y la 002 (ver el quickstart de la 002, secciones 5 y 6):

1. Registrar a Ana (`ana@example.com`), Bruno (`bruno@example.com`) y Carla (`carla@example.com`), e
   iniciar sesión con los tres. En los bloques siguientes los tokens se llaman `$ANA`, `$BRUNO` y
   `$CARLA`.
2. Ana crea el proyecto "Portal de Clientes" y agrega a Bruno como integrante. Carla **no** es
   integrante. El identificador del proyecto se llama `$PROJECT`.

---

## 6. Escenarios de validación de punta a punta

Cada bloque corresponde a una historia de [spec.md](./spec.md). Las pruebas automatizadas que los
cubren se nombran `TestUSn_<Escenario>` y llevan el comentario `(003/USn)`.

### US1 — Registrar una historia

```powershell
curl -X POST http://localhost:8080/api/v1/projects/$PROJECT/stories `
  -H "Authorization: Bearer $BRUNO" -H "Content-Type: application/json" `
  -d '{"title":"Exportar el reporte de velocidad a PDF","priority":"high","storyPoints":5,"acceptanceCriteria":["El PDF incluye la velocidad de los últimos cinco sprints.","El archivo se descarga con el nombre del proyecto."]}'
```

| Comprobación | Esperado |
| --- | --- |
| Respuesta | `201`, `status: pending`, `storyPoints: 5`, `readiness.ready: true`, `createdBy.fullName: Bruno Gómez` |
| Solo título y prioridad | `201`, `storyPoints: null`, `acceptanceCriteria: []`, `readiness.missing` con los dos motivos |
| `storyPoints: 0` con un criterio | `201`, `storyPoints: 0` y lista para planificar |
| Título de 150 caracteres / de 151 | `201` / `400` con `field: title` |
| `storyPoints` en 4, 7, -1 y 6.5 | `400` con la escala completa en el mensaje |
| Un criterio de solo espacios | `400` con `field: acceptanceCriteria[n]`; no se crea nada |
| El mismo pedido con `$CARLA` | `404` con `El proyecto no existe.`, idéntico al de un `projectId` inventado; no se crea nada |

### US2 — Ver el backlog ordenado y filtrado

Crear historias Alta, Media y Baja en distinto orden y consultar:

```powershell
curl http://localhost:8080/api/v1/projects/$PROJECT/stories -H "Authorization: Bearer $ANA"
curl "http://localhost:8080/api/v1/projects/$PROJECT/stories?priority=high&status=pending" -H "Authorization: Bearer $ANA"
curl "http://localhost:8080/api/v1/projects/$PROJECT/stories?priority=urgent" -H "Authorization: Bearer $ANA"
```

| Comprobación | Esperado |
| --- | --- |
| Orden | Altas, Medias, Bajas; dentro de cada una, la más antigua primero; dos consultas seguidas dan el mismo orden |
| Filtro combinado | Solo historias Alta **y** Pendiente; `appliedFilters` refleja los dos filtros |
| Proyecto recién creado | `200`, `stories: []`, `emptyReason: backlog_empty` |
| Filtro sin coincidencias | `200`, `stories: []`, `emptyReason: no_matches` |
| `priority=urgent` | `400` con los valores admitidos |
| `$CARLA` | `404` de proyecto inexistente |

### US3 — Consultar el detalle

```powershell
curl http://localhost:8080/api/v1/stories/$STORY -H "Authorization: Bearer $ANA"
curl http://localhost:8080/api/v1/stories/$STORY -H "Authorization: Bearer $CARLA"
curl http://localhost:8080/api/v1/stories/00000000-0000-4000-8000-000000000000 -H "Authorization: Bearer $ANA"
```

| Comprobación | Esperado |
| --- | --- |
| Integrante | `200` con los criterios en el orden en que se cargaron, autor, `createdAt`, `updatedAt` y `everAssignedToSprint: false` |
| Historia en 0 | `storyPoints: 0`, distinto de `null` |
| `$CARLA` y el identificador inventado | `404` con **el mismo cuerpo** byte a byte: `{"error":"La historia no existe."}` |

### US4 — Estimar en Story Points

```powershell
curl -X PUT http://localhost:8080/api/v1/stories/$STORY/story-points `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" -d '{"storyPoints":13}'
curl -X PUT http://localhost:8080/api/v1/stories/$STORY/story-points `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" -d '{"storyPoints":null}'
curl -X PUT http://localhost:8080/api/v1/stories/$STORY/story-points `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" -d '{"storyPoints":50}'
```

| Comprobación | Esperado |
| --- | --- |
| Estimar en 13 una historia con criterios | `200`, `readiness.ready: true`, `updatedAt` posterior al anterior |
| Devolver a `null` | `200`, `readiness.missing` incluye `story_points` |
| 0 y 100 | `200` en los dos |
| 50, 4 y 6.5 | `400` con la escala; el valor anterior no cambia |
| Cuerpo `{}` | `400`: el campo es obligatorio |

Los casos de historia Completada y de historia comprometida en un sprint abierto (US4-6, US4-8) no
se pueden producir por la API hasta que exista la feature de sprints: se verifican con las pruebas
automatizadas, que usan dobles de los puertos.

### US5 — Refinar una historia

```powershell
curl -X PATCH http://localhost:8080/api/v1/stories/$STORY `
  -H "Authorization: Bearer $BRUNO" -H "Content-Type: application/json" -d '{"priority":"low"}'
curl -X PATCH http://localhost:8080/api/v1/stories/$STORY `
  -H "Authorization: Bearer $BRUNO" -H "Content-Type: application/json" -d '{"acceptanceCriteria":[]}'
curl -X PATCH http://localhost:8080/api/v1/stories/$STORY `
  -H "Authorization: Bearer $BRUNO" -H "Content-Type: application/json" -d '{"status":"completed"}'
```

| Comprobación | Esperado |
| --- | --- |
| Solo la prioridad | `200`; título, descripción y criterios sin cambios; `id`, `status`, `createdBy` y `createdAt` iguales |
| Lista de criterios reordenada | `200` con el orden nuevo |
| `acceptanceCriteria: []` | `200`, sin criterios y `readiness.ready: false` |
| `status` en el cuerpo | `400` con `field: status`; el estado sigue `pending` |
| Título de 151, descripción de 2001 o criterio de 501 junto con un cambio válido | `400`; **ningún** campo cambia |
| `{}` | `400`: no incluye ningún campo a modificar |

La edición simultánea (US5-6, US5-7, SC-013) se verifica con la prueba de concurrencia, que envía las
dos modificaciones a la vez desde dos goroutines.

### US6 — Eliminar una historia que nunca se trabajó

```powershell
curl -X DELETE http://localhost:8080/api/v1/stories/$STORY -H "Authorization: Bearer $BRUNO" -i
curl -X DELETE http://localhost:8080/api/v1/stories/$STORY -H "Authorization: Bearer $BRUNO" -i
```

| Comprobación | Esperado |
| --- | --- |
| Primera eliminación, por un integrante que no es el propietario | `204`; la historia desaparece del listado |
| Segunda eliminación | `404`, historia inexistente |
| Única historia del backlog | El listado responde `emptyReason: backlog_empty` |

Los rechazos por historial (US6-4, US6-6, US6-7) dependen de sprints, esfuerzo, defectos y Planning
Poker, que todavía no existen: se verifican con las pruebas automatizadas y dobles de los cuatro
puertos.

### Proyecto Finalizado (FR-048, SC-012)

Ana lleva el proyecto a `finished` con el endpoint de estado de la 002. Después:

| Acción | Esperado |
| --- | --- |
| Listar y consultar el detalle | `200` |
| Crear, modificar, estimar y eliminar | `409`: `El proyecto está finalizado y su backlog es de solo lectura.`; nada cambia |

### Vista de estado de la 002 (RC-07)

Con historias creadas, `GET /api/v1/projects/$PROJECT` debe informar en `backlog.countsByState` la
cantidad por `pending`, `in_progress` y `completed`, y en `backlog.estimatedStoryPoints` la suma de
las estimadas (las "sin estimar" no suman).

### Flujo en la interfaz

1. Iniciar sesión como Bruno y abrir "Portal de Clientes". La vista de estado muestra el reparto del
   backlog con las etiquetas en español y un enlace **Backlog**.
2. En el backlog, crear una historia con título, prioridad, Story Points y dos criterios. Aparece en
   el listado marcada como lista para planificar.
3. Filtrar por prioridad Alta y por estado Pendiente; quitar los filtros. Con un filtro sin
   coincidencias se ve el mensaje "Ninguna historia coincide con los filtros", distinto del de
   backlog vacío.
4. Abrir la historia: se ven los criterios en orden, el autor y las fechas. Estimarla en 8 desde el
   selector de la escala.
5. Editarla: cambiar la prioridad y reordenar los criterios. Volver al detalle y comprobar el
   cambio.
6. Eliminarla y confirmar. Vuelve al listado sin la historia.

---

## 7. Puertas de calidad antes de dar por terminada la feature

- [ ] `go test ./...` y `npm test` pasan contra el entorno de test.
- [ ] Buscar `003/US` en las pruebas devuelve los 58 escenarios; las reglas RN-01 a RN-18 tienen
      prueba unitaria (SC-014).
- [ ] Las respuestas `404` de historia ajena e inexistente son idénticas byte a byte en las cuatro
      acciones de ítem; las de proyecto ajeno e inexistente, en las dos de colección (SC-006).
- [ ] Cada uno de los seis endpoints tiene su prueba sin sesión (SC-005).
- [ ] Swagger publicado coincide con [contracts/openapi.yaml](./contracts/openapi.yaml).
- [ ] `README.md` menciona el backlog en la introducción y en la lista de endpoints.
- [ ] `govulncheck ./...` sin hallazgos; sin `any` en el frontend (`npx tsc --noEmit` y ESLint).

# Guía de Validación (Fase 1): Gestión de Proyectos e Integrantes

**Feature**: `specs/002-project-members` | **Fecha**: 2026-10-07

Cómo levantar el entorno, correr las pruebas y validar de punta a punta las seis historias de
[spec.md](./spec.md). Los detalles de diseño están en [plan.md](./plan.md),
[research.md](./research.md), [data-model.md](./data-model.md) y
[contracts/openapi.yaml](./contracts/openapi.yaml).

---

## 1. Prerrequisitos

### La feature 001 tiene que estar implementada

Esta feature no incluye arranque de proyecto: reutiliza el módulo de Go, la aplicación Next.js, los
contenedores de PostgreSQL, la tabla `users` y el middleware de sesión de
[`specs/001-user-auth`](../001-user-auth/spec.md).

| Prerrequisito | Cómo verificarlo |
| --- | --- |
| `backend/` y `frontend/` existen | `ls backend frontend` |
| Go 1.27.1 instalado | `go version` |
| Contenedores de desarrollo y de test arriba | Sección 2 |
| Endpoints de la 001 responden | `POST /api/v1/auth/register` y `POST /api/v1/auth/login` devuelven `201` y `200` |

Las herramientas y versiones son las mismas que fijó la 001; ver
[`specs/001-user-auth/quickstart.md`](../001-user-auth/quickstart.md), secciones 1 a 3.

### Variable de entorno nueva

Esta feature agrega **una** clave a los archivos de entorno del backend que creó la 001:

```text
# backend/.env.example  (versionado)  — agregar esta línea
APP_TIMEZONE=America/Argentina/Buenos_Aires
```

Hay que replicarla en `backend/.env` y `backend/.env.test`, que **no** se versionan (Principio VI).

Si `APP_TIMEZONE` falta o no es una zona horaria válida, el backend **no arranca** y lo informa,
igual que con `JWT_SECRET` ([research.md](./research.md), sección 4). No es un secreto, pero vive en
los `.env` como el resto de la configuración.

---

## 2. Levantar los servicios

Sin cambios respecto de la feature 001: el mismo `docker-compose.yml`, los mismos dos nombres de
proyecto y los mismos puertos.

```powershell
cd backend/postgres

# Desarrollo (puerto 5432, base smye_dev)
docker compose --env-file .env -p smye-dev up -d

# Test (puerto 5433, base smye_test)
docker compose --env-file .env.test -p smye-test up -d
```

**Comprobación de aislamiento** — las pruebas nunca deben tocar la base de desarrollo:

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT current_database(), inet_server_port();"
```

Debe responder `smye_test`. Si responde `smye_dev`, el entorno está mal configurado y hay que
detenerse antes de ejecutar nada.

Las tablas `projects` y `project_members` las crea `AutoMigrate` al arrancar la aplicación o al
correr las pruebas; no hay paso de migración manual.

---

## 3. Ejecutar las pruebas

### Backend

```powershell
cd backend

# Swagger se genera antes de compilar: backend/docs/ no está versionado
swag init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test ./...

# Solo el módulo de esta feature
go test ./internal/projects/...

# Solo las pruebas de una historia (trazabilidad del Principio II)
go test ./... -run TestUS1
go test ./... -run TestUS6

# Reporte de cobertura
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out
```

Las pruebas de integración cargan `backend/.env.test` y vacían las tablas antes de cada caso. Para
esta feature el ayudante de la 001, en `backend/internal/platform/testsupport/`, tiene que vaciar
también `projects` y `project_members`:

```text
TRUNCATE users, sessions, auth_events, projects, project_members CASCADE;
```

### Frontend

```powershell
cd frontend

npm test                   # Vitest una vez
npm run test:coverage      # Reporte de cobertura
npm run lint               # ESLint
npx tsc --noEmit           # Comprobación de tipos (prohibido `any`)
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

Swagger queda publicado en <http://localhost:8080/swagger/index.html>.

---

## 5. Preparación: dos cuentas y un token

Todas las operaciones de esta feature exigen sesión válida (FR-046), así que la validación arranca
creando dos cuentas con los endpoints de la 001.

```powershell
# Ana, que será propietaria
curl -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"fullName":"Ana Pérez","email":"ana@example.com","password":"Proyecto2026"}'

# Bruno, que será integrante
curl -X POST http://localhost:8080/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"fullName":"Bruno Gómez","email":"bruno@example.com","password":"Proyecto2026"}'

# Tokens
curl -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"ana@example.com","password":"Proyecto2026"}'

curl -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"bruno@example.com","password":"Proyecto2026"}'
```

Guardar los dos tokens; en los bloques siguientes se los nombra `$ANA` y `$BRUNO`.

---

## 6. Escenarios de validación de punta a punta

Cada bloque corresponde a una historia de [spec.md](./spec.md). Las pruebas automatizadas que los
cubren se nombran según el Principio II (`TestUSn_<Escenario>` en Go,
`describe("002/USn - ...")` en Vitest), de modo que buscar `002/US1` en el código de pruebas
devuelve la trazabilidad completa.

### US1 — Crear un proyecto

```powershell
# Creación válida -> 201 en estado planned, con Ana como propietaria y única integrante
curl -X POST http://localhost:8080/api/v1/projects `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"name":"Portal de Clientes","description":"Rediseño del portal.","startDate":"2026-10-01","plannedEndDate":"2026-12-20","hoursPerStoryPoint":6.5}'

# Sin descripción -> 201 igual, porque es opcional
# Fechas iguales -> 201
# Factor 0.01 y factor 40 -> 201 los dos (mínimo y máximo admitidos)
# Nombre de 100 caracteres -> 201; de 101 -> 400

# Nombre repetido con otra capitalización y espacios -> 409
curl -X POST http://localhost:8080/api/v1/projects `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"name":"  portal de clientes  ","startDate":"2026-10-01","plannedEndDate":"2026-12-20","hoursPerStoryPoint":6.5}'

# El mismo nombre para otro propietario -> 201 (la unicidad es por propietario)
curl -X POST http://localhost:8080/api/v1/projects `
  -H "Authorization: Bearer $BRUNO" -H "Content-Type: application/json" `
  -d '{"name":"Portal de Clientes","startDate":"2026-10-01","plannedEndDate":"2026-12-20","hoursPerStoryPoint":6.5}'

# Factor inválido -> 400 (0, negativo, 40.01, y 6.555 por tener tres decimales)
# Fecha prevista anterior a la de inicio -> 400
# Campos faltantes -> 400

# Sin sesión -> 401
curl -X POST http://localhost:8080/api/v1/projects -H "Content-Type: application/json" `
  -d '{"name":"Sin sesión","startDate":"2026-10-01","plannedEndDate":"2026-12-20","hoursPerStoryPoint":6.5}'
```

| Resultado esperado | Requisito |
| --- | --- |
| `201` en estado `planned`, sin `actualEndDate`, con un solo integrante marcado propietario | FR-007, FR-008, RN-01 |
| Descripción ausente aceptada | FR-003 |
| Fechas iguales aceptadas; prevista anterior rechazada | FR-005, RN-06 |
| Factor `0.01` y `40` aceptados; `0`, negativo, `40.01` y `6.555` rechazados **sin redondear** | FR-006, RN-07 |
| Nombre de 100 aceptado, de 101 rechazado, solo espacios rechazado | FR-002, RN-04 |
| Nombre repetido del mismo propietario rechazado con `409`; mismo nombre de otro propietario aceptado | FR-004, RN-04 |
| Sin sesión, `401` y nada creado | FR-046 |

**Concurrencia (SC-007)**: enviar 10 creaciones simultáneas con el mismo nombre y el mismo token.

```powershell
docker exec smye-dev-postgres-1 psql -U smye -d smye_dev -c `
  "SELECT owner_id, name_normalized, count(*) FROM projects GROUP BY 1,2 HAVING count(*) > 1;"
```

Debe devolver **cero filas**: se creó un solo proyecto y los nueve intentos restantes recibieron
`409`.

### US2 — Ver mis proyectos y quedar aislado de los ajenos

```powershell
# Listado de Ana -> 200 con sus proyectos y ninguno ajeno
curl http://localhost:8080/api/v1/projects -H "Authorization: Bearer $ANA"

# Listado de alguien sin proyectos -> 200 con lista vacía y mensaje, no un error
# Proyecto ajeno por identificador -> 404 idéntico al de un id inexistente
curl http://localhost:8080/api/v1/projects/$PROYECTO_DE_ANA -H "Authorization: Bearer $BRUNO"
curl http://localhost:8080/api/v1/projects/00000000-0000-0000-0000-000000000000 -H "Authorization: Bearer $BRUNO"

# Sin sesión -> 401
curl http://localhost:8080/api/v1/projects
```

| Resultado esperado | Requisito |
| --- | --- |
| Cada elemento trae nombre, estado, fechas, `memberCount` e `isOwner` | FR-032 |
| Ningún proyecto ajeno en la lista | FR-033, SC-005 |
| Lista vacía con mensaje y `200` | FR-034 |
| Orden por fecha de inicio descendente y, a igual fecha, nombre ascendente | FR-035 |
| Proyecto con un solo integrante aparece con `memberCount: 1` | Escenario US2-4 |
| Las **dos** respuestas `404` son byte a byte idénticas | FR-049, SC-005 |
| Un proyecto del que se quitó a la persona ya no aparece | FR-028, escenario US2-3 |

### US3 — Gestionar los integrantes del proyecto

```powershell
# Alta por correo -> 201, y Bruno ya ve el proyecto
curl -X POST http://localhost:8080/api/v1/projects/$PROYECTO/members `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"email":"bruno@example.com"}'

# Correo con espacios y mayúsculas -> encuentra al mismo usuario
#   {"email":"  Bruno@Example.COM  "}

# Correo sin cuenta -> 422
curl -X POST http://localhost:8080/api/v1/projects/$PROYECTO/members `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"email":"nadie@example.com"}'

# Ya es integrante -> 409
# Un integrante que no es propietario intentando agregar -> 403
# Baja -> 204, y Bruno deja de ver el proyecto
curl -X DELETE http://localhost:8080/api/v1/projects/$PROYECTO/members/$BRUNO_ID `
  -H "Authorization: Bearer $ANA"

# Baja de alguien que no es integrante -> 404
# Ana intentando quitarse a sí misma -> 409
curl -X DELETE http://localhost:8080/api/v1/projects/$PROYECTO/members/$ANA_ID `
  -H "Authorization: Bearer $ANA"
```

| Resultado esperado | Requisito |
| --- | --- |
| Tras el alta, Bruno obtiene `200` en la vista de estado y el proyecto en su listado | FR-019 |
| Correo normalizado antes de buscar la cuenta | FR-020, RN-05 |
| Volver a agregar a alguien quitado le devuelve el acceso y aparece **una sola vez** | FR-029 |
| Correo sin cuenta registrada: `422`, no `404` | FR-021, RC-03 |
| Alta duplicada: `409` y la lista no cambia | FR-022 |
| Quien no es propietario: `403` y ningún efecto | FR-047, FR-050, SC-006 |
| Tras la baja, Bruno recibe `404` en la vista de estado | FR-028 |
| Baja de quien no es integrante: `404` | FR-026 |
| El propietario no puede quitarse: `409` | FR-025, RN-12 |
| Sobre un proyecto `finished`, alta y baja: `409` | FR-018, RN-11 |

**Visibilidad del correo (FR-030)**: consultar la vista de estado con `$ANA` y con `$BRUNO`. En la
respuesta de Ana cada integrante trae `email`; en la de Bruno **ningún** integrante lo trae. Los
demás campos —nombre completo, `isOwner` y `joinedAt`— son iguales en las dos.

**Conservación del historial (FR-027, SC-011)**: cuando existan las features de esfuerzo, votos y
defectos, contar sus registros del proyecto antes y después de quitar a un integrante. Los totales
deben coincidir y los registros seguir mostrando el nombre de la persona. Hoy se verifica que la
baja toca **solo** `project_members`:

```powershell
docker exec smye-dev-postgres-1 psql -U smye -d smye_dev -c `
  "SELECT count(*) FROM users; SELECT count(*) FROM project_members WHERE project_id = '$PROYECTO';"
```

### US4 — Consultar el estado del proyecto

```powershell
# Vista de estado -> 200 con todo en una sola respuesta
curl http://localhost:8080/api/v1/projects/$PROYECTO -H "Authorization: Bearer $ANA"
```

| Resultado esperado | Requisito |
| --- | --- |
| Datos generales, propietario, factor, estado, fechas, integrantes, sprint activo, reparto del backlog y horas estimadas, en **una** respuesta | FR-042, SC-003 |
| Proyecto `finished`: trae `actualEndDate` y `readOnly: true` | Escenario US4-2 |
| Sin sprint activo: `hasActiveSprint: false` y `activeSprint: null`, **sin ser un error** | FR-043 |
| Proyecto recién creado: `countsByState` vacío, `estimatedStoryPoints: 0` y `estimatedHours: 0` | FR-044 |
| Con 3, 5 y 8 Story Points estimados, una historia sin estimar y factor 6,5: `estimatedStoryPoints: 16` y `estimatedHours: 104` | FR-045, RN-08, escenario US4-5 |
| Quien no es integrante: `404` que no revela ni el nombre del proyecto | FR-049, SC-005 |
| Sin sesión: `401` | FR-046 |

El escenario US4-5 se verifica hoy con una prueba unitaria del cálculo y un doble del puerto
`BacklogSummaryProvider`; de punta a punta queda disponible cuando lleguen
`specs/003-product-backlog` y `specs/004-sprint-management`
([research.md](./research.md), sección 9).

### US5 — Modificar los datos del proyecto

```powershell
# Modificación válida -> 200; el estado y actualEndDate no cambian
curl -X PUT http://localhost:8080/api/v1/projects/$PROYECTO `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"name":"Portal de Clientes v2","description":"Fase 2.","startDate":"2026-10-01","plannedEndDate":"2027-02-28","hoursPerStoryPoint":8}'

# Renombrar con el mismo nombre que ya tenía -> 200
# Nombre de otro proyecto propio -> 409
# Entrada inválida -> 400 y ningún cambio parcial
# Integrante que no es propietario -> 403
# Proyecto finished -> 409
```

| Resultado esperado | Requisito |
| --- | --- |
| Los cinco campos se actualizan y el `status` y `actualEndDate` quedan intactos | FR-012, FR-016 |
| Un proyecto `in_progress` admite correr la fecha prevista | Escenario US5-2 |
| Renombrar con el mismo nombre se acepta | FR-014, escenario US5-3 |
| Mismas validaciones que la creación | FR-013 |
| Rechazo sin cambio parcial alguno | FR-015 |
| Factor al mínimo (0,01) y al máximo (40) aceptado | FR-006, escenario US5-5 |
| Nombre de otro proyecto propio: `409` y el proyecto queda como estaba | FR-004 |
| Quien no es propietario: `403` | FR-047, SC-006 |
| Proyecto `finished`: `409` | FR-018, RN-11 |

**Factor y sprints cerrados (FR-017, RN-15)**: el cambio del factor se aplica al proyecto y a los
cierres posteriores; los sprints ya cerrados conservan el suyo. Esta feature **no guarda** el factor
de los sprints —lo hace `specs/004-sprint-management` en su instantánea de cierre—, así que lo que
se verifica acá es que `PUT /api/v1/projects/{projectId}` no toca ninguna otra tabla.

### US6 — Avanzar el estado del proyecto

```powershell
# planned -> in_progress -> 200
curl -X PATCH http://localhost:8080/api/v1/projects/$PROYECTO/status `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"status":"in_progress"}'

# in_progress -> finished -> 200 con actualEndDate
curl -X PATCH http://localhost:8080/api/v1/projects/$PROYECTO/status `
  -H "Authorization: Bearer $ANA" -H "Content-Type: application/json" `
  -d '{"status":"finished"}'

# Las seis transiciones no permitidas -> 409 cada una
# Integrante que no es propietario -> 403
```

| Resultado esperado | Requisito |
| --- | --- |
| `planned → in_progress` e `in_progress → finished` aceptadas | FR-037 |
| Al finalizar queda `actualEndDate` con la fecha del cambio | FR-039, SC-010 |
| Finalizar antes de la fecha prevista se acepta, sin ser un error | Escenario US6-2 |
| Finalizar el mismo día en que comenzó: `actualEndDate` igual a `startDate` | Escenario US6-4 |
| `planned → in_progress` sin exigir nada del backlog ni de los sprints | FR-041, escenario US6-5 |
| Las **seis** transiciones restantes rechazadas con `409` y sin cambiar el estado | FR-038, SC-009 |
| Con sprint activo, `in_progress → finished` rechazada y el proyecto sigue `in_progress` sin `actualEndDate` | FR-041, RN-16 |
| Proyecto `finished`: cualquier cambio de estado `409` y `actualEndDate` intacta | FR-018, FR-040 |
| Quien no es propietario: `403` | FR-047, SC-006 |

**Las seis transiciones a verificar (SC-009)**: `planned → planned`, `planned → finished`,
`in_progress → planned`, `in_progress → in_progress`, `finished → planned`,
`finished → in_progress`.

**Zona horaria (FR-039)**: finalizar un proyecto a las 23:30 de la zona del sistema y comprobar que
`actual_end_date` es **ese mismo día**, aunque en UTC ya sea el siguiente.

```powershell
docker exec smye-dev-postgres-1 psql -U smye -d smye_dev -c `
  "SELECT name, status, actual_end_date FROM projects WHERE status = 'finished';"
```

**Inmutabilidad (SC-010)**: ningún endpoint acepta `actualEndDate` como entrada. Verificable
enviándolo en el cuerpo de `PUT /api/v1/projects/{projectId}`: el campo se ignora y la fecha no
cambia.

### Flujo en la interfaz

1. Iniciar sesión y llegar a `/projects`: la lista está vacía con el mensaje de FR-034.
2. Crear un proyecto desde `/projects/new`. El formulario muestra **antes de enviar** el máximo de
   100 caracteres del nombre, los 1000 de la descripción y el rango del factor (0,01 a 40 con hasta
   2 decimales).
3. Abrir el proyecto en `/projects/{projectId}`: datos generales, estado, fechas, integrantes,
   ausencia de sprint activo y contadores en cero, todo en una pantalla (SC-003).
4. Agregar a Bruno por correo desde la vista de estado, en no más de 3 pasos (SC-002).
5. Iniciar sesión como Bruno: ve el proyecto en su listado y en la vista de estado **sin los
   correos** de los demás integrantes, y no tiene acciones de administración disponibles.
6. Como Ana, modificar el proyecto en `/projects/{projectId}/edit` y avanzar su estado hasta
   Finalizado. Con el proyecto Finalizado, la interfaz muestra la fecha de finalización real y
   **oculta** las acciones de administración.
7. Quitar a Bruno y confirmar que, al refrescar, su vista del proyecto pasa a "el proyecto no
   existe".

---

## 7. Puertas de calidad antes de dar por terminada la feature

De las puertas de calidad de la constitución, aplicadas a esta feature:

- [ ] Cada prueba se escribió antes de su código de producción y ahora pasa (Principio II).
- [ ] Las pruebas corrieron contra `smye_test` en el puerto 5433, no contra desarrollo.
- [ ] Hay una prueba nombrada por cada escenario de la spec; buscar `002/US1` a `002/US6` en el
      código de pruebas devuelve la trazabilidad completa (55 escenarios).
- [ ] Hay pruebas unitarias de las validaciones de entrada, de las reglas de negocio RN-01 a RN-16 y
      del cálculo de horas estimadas (SC-012, Principio II).
- [ ] Las respuestas `404` de proyecto inexistente y de proyecto ajeno son idénticas en **todas** las
      acciones de la feature (SC-005).
- [ ] Las cuatro acciones de administración se rechazan con `403` para un integrante que no es el
      propietario, sin producir ningún cambio (SC-006).
- [ ] El correo de los integrantes no aparece en ninguna respuesta dirigida a quien no es el
      propietario (FR-030).
- [ ] Ninguna respuesta de esta feature incluye el hash de contraseña ni ningún dato de `users`
      distinto de identificador, nombre completo y correo.
- [ ] No hay secretos fuera de los `.env`; `backend/.env.example` incluye `APP_TIMEZONE` y está
      sincronizado con `backend/.env`.
- [ ] No hay `any` en el frontend ni errores de Go sin manejar en el backend.
- [ ] Cada bloque nuevo tiene su comentario en español y los identificadores están en inglés.
- [ ] Swagger publicado coincide con [contracts/openapi.yaml](./contracts/openapi.yaml).
- [ ] El `README.md` de la raíz incluye la variable `APP_TIMEZONE` y el paso a paso sigue siendo
      suficiente para alguien que no conoce el proyecto.
- [ ] `govulncheck ./...` no reporta vulnerabilidades conocidas.
- [ ] Se ejecutaron los comandos de cobertura de backend y de frontend.

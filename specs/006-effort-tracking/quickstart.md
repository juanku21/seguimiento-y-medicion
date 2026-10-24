# Guía de Validación (Fase 1): Registro de Esfuerzo

**Feature**: `specs/006-effort-tracking` | **Fecha**: 2026-10-24

Pasos para comprobar de punta a punta que la feature funciona. Los detalles de los endpoints están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y los del modelo en
[data-model.md](./data-model.md); acá no se repiten.

---

## 1. Prerrequisitos

- Las features `specs/001-user-auth` a `specs/004-sprint-management` están integradas en `main`. La
  003 y la 004 son imprescindibles: esta feature registra esfuerzo sobre sus historias, se ancla a
  sus sprints y reemplaza sus puertos `EffortRecords` y `SprintEffort`.
- **Una variable de entorno nueva**: `EFFORT_GRACE_PERIOD_HOURS` (entero `≥ 1`, por defecto `48`).
  Hay que agregarla a `backend/.env` y `backend/.env.test` con el valor de `backend/.env.example`,
  para que cada `.env` tenga las mismas claves que su plantilla (Principio VI). Si falta, la
  aplicación usa 48; un valor inválido (`0`, `-1`, `48h`, `abc`) impide el arranque.
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

`AutoMigrate` crea la tabla `effort_entries` y la columna `user_stories.ever_had_effort` al arrancar
la aplicación o la primera prueba. Comprobación de las claves foráneas sin cascada:

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT conname, confdeltype FROM pg_constraint WHERE conrelid = 'effort_entries'::regclass AND contype = 'f';"
```

Las tres filas (`story_id`, `user_id`, `sprint_id`) deben tener `confdeltype = a` (`NO ACTION`).

---

## 3. Ejecutar las pruebas

### Backend

Los comandos que abarcan varios paquetes llevan `-p 1`: el advisory lock de
`backend/internal/platform/testsupport/database.go` ya serializa los paquetes.

```powershell
cd backend
swag init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test -p 1 ./...

# Solo el módulo de esta feature
go test -p 1 ./internal/effort/...

# Reglas puras: anclaje, plazo de gracia, fecha, tope diario, comparación y redondeo
go test ./internal/effort/domain/...

# Trazabilidad del Principio II: los escenarios de una historia
go test -p 1 ./internal/effort/... -run TestUS1

# Concurrencia del tope diario: 10 altas simultáneas que suman más de 24 h (SC-005)
go test -p 1 ./internal/effort/... -run Concurrency -count=5

# Puertos reemplazados en la 003 y la 004
go test -p 1 ./internal/backlog/... ./internal/sprints/...

# Cobertura
go test -p 1 ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Vulnerabilidades conocidas
govulncheck ./...
```

Resultado esperado: todas las pruebas pasan, `-count=5` de la prueba de concurrencia nunca deja una
fecha con más de 24 horas, y la búsqueda de trazabilidad encuentra los cinco prefijos:

```powershell
Select-String -Path internal\effort\**\*_test.go -Pattern "006/US[1-5]" | Measure-Object
```

El total debe ser al menos **74** (un comentario por escenario).

### Frontend

```powershell
cd frontend
npm run lint
npm run test
npm run test:coverage
npm run format:check
```

---

## 4. Validación manual de punta a punta

Con la aplicación levantada (`go run ./cmd/app` en `backend/` y `npm run dev` en `frontend/`), dos
cuentas registradas —**Ana**, propietaria, y **Bruno**, integrante— y un proyecto `in_progress` con
factor **6** horas por Story Point, fecha de inicio anterior a hoy y tres historias: **A** (5 Story
Points), **B** (sin estimar) y **C** (0 Story Points). Para probar el plazo de gracia rápido, usar
`EFFORT_GRACE_PERIOD_HOURS=1` en `backend/.env`.

| # | Paso | Resultado esperado | Requisitos |
| --- | --- | --- | --- |
| 1 | Ana compromete A en un sprint y lo inicia; abre A y registra 6,5 h de hoy | `201`; el registro muestra el sprint Activo; total de A 6,50 h | US1-1, FR-014 |
| 2 | Ana registra 2 h en B, que está solo en el backlog | Registro "Sin sprint" | US1-2 |
| 3 | Ana intenta 1,3 h, 0 h, 25 h, actividad de espacios, fecha de mañana | `400` con el campo y el motivo; nada se crea | US1-17 a US1-20 |
| 4 | Ana registra 15,5 h más hoy en otra historia (total del día 24) y luego 0,25 h | La primera se acepta; la segunda, `409 daily_cap_exceeded` con 0 h disponibles | US1-10, US1-16 |
| 5 | Bruno abre A | Ve el total y la línea de Ana; no ve botones de editar ni eliminar en los registros de Ana | US2-1, FR-058 |
| 6 | Bruno envía `PATCH` sobre un registro de Ana (con la API) | `403`, el registro no cambia | US3-11 |
| 7 | Ana corrige su registro de A a 4 h | `200`; total de A baja; el sprint del registro no cambia | US3-1 |
| 8 | Ana cierra el sprint con A sin completar y registra 1 h en A | Se acepta y se ancla al sprint Cerrado | US1-8, RN-24 |
| 9 | Pasado el plazo configurado, Ana intenta corregir ese registro y registrar otra hora en A | La corrección: `409 effort_finalized`; el alta: se acepta **sin sprint** | US3-12, RN-25 |
| 10 | Ana intenta eliminar el sprint Planificado que tiene registros (crear uno, comprometer, registrar) | `409` de la 004: el sprint tiene esfuerzo | RC-07 |
| 11 | Ana elimina todos los registros de B e intenta eliminar B | `409` de la 003 con el motivo `effort` | RC-05, RN-22 |
| 12 | En "Esfuerzo" del proyecto, filtrar por Bruno, por "Sin sprint" y por un rango de un día | Cada filtro devuelve solo lo suyo y su total; sin coincidencias, lista vacía con total 0 | US4-2, US4-5, US4-6, US4-7 |
| 13 | En la comparación del proyecto | A: estimada 30,00 h; B: "No calculable" en estimada y desvíos; C: estimada 0,00 h y desvío porcentual "No calculable" | US5-1, US5-5, US5-6 |
| 14 | Ana cambia el factor a 8 y vuelve a la comparación | A: estimada 40,00 h; reales sin cambio | US5-4 |
| 15 | Ana quita a Bruno del proyecto | Sus registros siguen visibles con su nombre y en los totales; Bruno recibe `404` sobre ellos | US2-3, US3-18 |
| 16 | Ana finaliza el proyecto e intenta registrar, corregir y eliminar | `409 read_only` en las tres; las consultas siguen funcionando | US1-23, US3-17, FR-060 |

---

## 5. Puertas de calidad antes de dar por terminado

- [ ] Las pruebas se escribieron antes del código y ahora pasan, contra `smye_test`.
- [ ] `EFFORT_GRACE_PERIOD_HOURS` está en `backend/.env.example` con el mismo nombre que lee
      `config.Load`.
- [ ] Sin `any` en el frontend; ningún `error` de Go ignorado.
- [ ] Cada bloque nuevo tiene su comentario en español; identificadores en inglés.
- [ ] Swagger generado coincide con [contracts/openapi.yaml](./contracts/openapi.yaml).
- [ ] `README.md` describe el registro de esfuerzo, la variable nueva y los siete endpoints.
- [ ] Sin dependencias nuevas.

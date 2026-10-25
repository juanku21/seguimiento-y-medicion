# Guía de Validación (Fase 1): Gestión de Defectos

**Feature**: `specs/007-defect-tracking` | **Fecha**: 2026-10-25

Pasos para comprobar de punta a punta que la feature funciona. Los detalles de los endpoints están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y los del modelo en
[data-model.md](./data-model.md); acá no se repiten.

---

## 1. Prerrequisitos

- Las features `specs/001-user-auth` a `specs/004-sprint-management` están integradas en `main`. La
  003 y la 004 son imprescindibles: los defectos se vinculan a sus historias, se anclan a sus sprints
  y esta feature reemplaza el puerto `DefectRecords` de la 003.
- La 005 y la 006 no son requisito. La fase fundacional de la 006 ya está en `main`; si su rama
  todavía no se integró, la validación de esta feature no depende de ella.
- **Sin variables de entorno nuevas** y **sin dependencias nuevas** de Go ni de npm.

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

`AutoMigrate` crea la tabla `defects` y la columna `user_stories.ever_had_defects` al arrancar la
aplicación o la primera prueba. Comprobación de las claves foráneas sin cascada:

```powershell
docker exec smye-test-postgres-1 psql -U smye -d smye_test -c "SELECT conname, confdeltype FROM pg_constraint WHERE conrelid = 'defects'::regclass AND contype = 'f';"
```

Las seis filas (`story_id`, `detected_sprint_id`, `resolved_sprint_id`, `created_by_id`,
`updated_by_id`, `status_changed_by_id`) deben tener `confdeltype = a` (`NO ACTION`), y una de ellas
debe llamarse `fk_defects_story`, que es el nombre que traduce el `Delete` de la 003.

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
go test -p 1 ./internal/defects/...

# Reglas puras: máquina de estados, sprints admitidos, ventana de corrección, solo lectura del descartado
go test ./internal/defects/domain/...

# Trazabilidad del Principio II: los escenarios de una historia
go test -p 1 ./internal/defects/... -run TestUS2

# Concurrencia: resolución y descarte simultáneos sobre el mismo defecto En progreso (SC-019)
go test -p 1 ./internal/defects/... -run Concurrency -count=10

# Puerto reemplazado y traducción de la clave foránea en la 003
go test -p 1 ./internal/backlog/...

# Cobertura
go test -p 1 ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Vulnerabilidades conocidas
govulncheck ./...
```

Resultado esperado: todas las pruebas pasan; en las 10 repeticiones de la prueba de concurrencia,
exactamente una de las dos acciones se aplica y la otra recibe `409 invalid_transition`; y la búsqueda
de trazabilidad encuentra los seis prefijos:

```powershell
Select-String -Path internal\defects\**\*_test.go -Pattern "007/US[1-6]" | Measure-Object
```

El total debe ser al menos **73** (un comentario por escenario).

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
dos historias, **A** y **B**, y tres sprints: **S1** (iniciado y cerrado), **S2** (iniciado, activo) y
**S3** (planificado), con fechas consecutivas.

| # | Paso | Resultado esperado | Requisitos |
| --- | --- | --- | --- |
| 0 | En un proyecto nuevo sin sprints iniciados, Ana abre "Registrar defecto" | Aviso de que no hay sprint Activo ni Cerrado; por API, `409 no_detection_sprint` | US1-16, FR-010 |
| 1 | Ana abre "Registrar defecto" desde la historia A | La historia viene elegida y S2 propuesto como sprint de detección; S1 aparece como alternativa y S3 no | US1-1, US1-2, FR-008 |
| 2 | Ana registra "El total del reporte no suma los descuentos", Alta | `201`; Abierto, sin sprint de resolución, autor Ana | US1-1, FR-011 |
| 3 | Ana intenta descripción de espacios, severidad `urgent`, sprint S3 | `400` con el campo y el motivo; nada se crea | US1-9, US1-10, US1-12 |
| 4 | Bruno pasa el defecto a En progreso e intenta resolverlo sin sprint | La primera se acepta; la segunda, `400` en `resolvedSprintId`, sigue En progreso | US2-4, US2-7 |
| 5 | Bruno lo resuelve indicando S1 | `400`: el sprint de resolución no puede empezar antes que el de detección | US2-8 |
| 6 | Bruno lo resuelve en S2 | Resuelto con S2 en ambos campos | US2-2, FR-025 |
| 7 | Ana lo reabre | Abierto, sin sprint de resolución, detección S2 | US5-1, FR-029 |
| 8 | Ana corrige su sprint de detección a S1 y su historia a B en la misma operación | `200`; la historia A ya no lo lista y la B sí | US4-6, US4-9 |
| 9 | Ana lo pasa a En progreso e intenta corregir su historia a A | `409 links_frozen` | US4-12 |
| 10 | Ana cambia su severidad a Crítica | `200`, sigue En progreso | US4-4 |
| 11 | En dos pestañas, Ana resuelve en S2 y Bruno descarta a la vez | Una se aplica; la otra recibe `409 invalid_transition` con el estado vigente | SC-019, FR-018 |
| 12 | Ana registra otro defecto sobre A y lo descarta | Descartado, conserva la detección, sin sprint de resolución; el detalle es de solo lectura | US6-1, US6-6 |
| 13 | Ana intenta modificar, reabrir y resolver el descartado | `409 defect_discarded` en la modificación; `409 invalid_transition` en las transiciones | US6-8, US4-14 |
| 14 | En "Defectos" del proyecto, filtrar por Crítica + En progreso, por S2, y por los tres estados no descartados | Cada filtro devuelve solo lo suyo; por S2 aparecen detectados y resueltos en S2, distinguibles | US3-4, US3-5, US6-4 |
| 15 | Ana intenta eliminar la historia A, cuyo único defecto está Descartado | `409` de la 003 con el motivo `defects` | US6-5, RN-19 |
| 16 | Ana crea una historia C, registra un defecto sobre ella, lo revincula a B mientras está Abierto e intenta eliminar C | `409` de la 003 con el motivo `defects`: C tuvo defectos aunque hoy no tenga ninguno | RC-05 |
| 17 | Ana quita a Bruno del proyecto | Los defectos que registró siguen visibles con su nombre; Bruno recibe `404` sobre ellos | FR-055 |
| 18 | Ana finaliza el proyecto e intenta registrar, modificar, cambiar de estado, reabrir y descartar | `409 read_only` en todas; las consultas siguen funcionando | US1-14, US2-13, US4-16, US5-7, US6-9 |

---

## 5. Puertas de calidad antes de dar por terminado

- [ ] Las pruebas se escribieron antes del código y ahora pasan, contra `smye_test`.
- [ ] Sin `any` en el frontend; ningún `error` de Go ignorado.
- [ ] Cada bloque nuevo tiene su comentario en español; identificadores en inglés.
- [ ] Swagger generado coincide con [contracts/openapi.yaml](./contracts/openapi.yaml).
- [ ] `README.md` describe la gestión de defectos y los seis endpoints.
- [ ] Sin dependencias nuevas ni variables de entorno nuevas.

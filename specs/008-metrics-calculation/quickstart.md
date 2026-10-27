# Guía de Validación (Fase 1): Cálculo de Métricas

**Feature**: `specs/008-metrics-calculation` | **Fecha**: 2026-10-26

Pasos para comprobar de punta a punta que la feature funciona. Los detalles de los endpoints están en
[contracts/openapi.yaml](./contracts/openapi.yaml) y los de los tipos y las fórmulas en
[data-model.md](./data-model.md); acá no se repiten.

---

## 1. Prerrequisitos

- Las features `specs/001-user-auth` a `specs/007-defect-tracking` están integradas en `main`. La 007
  es imprescindible: esta feature lee la tabla `defects`, que crea su `AutoMigrate`. La 005 no es
  requisito.
- **Sin variables de entorno nuevas** y **sin dependencias nuevas**. El plazo de gracia es el de
  `EFFORT_GRACE_PERIOD_HOURS` (48 horas por defecto), ya presente en `backend/.env.example` y
  `backend/.env.test`.
- **Sin tablas nuevas**: esta feature no migra nada.

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

---

## 3. Ejecutar las pruebas

Los comandos que abarcan varios paquetes llevan `-p 1`: el advisory lock de
`backend/internal/platform/testsupport/database.go` ya serializa los paquetes.

```powershell
cd backend
swag init -g cmd/app/main.go -o docs

# Todas las pruebas contra la base de TEST
go test -p 1 ./...

# Solo el módulo de esta feature
go test -p 1 ./internal/metrics/...

# Fórmulas, redondeo, estados de valor, ventana de velocidad, series, defectos y advertencias (sin base)
go test ./internal/metrics/domain/...

# Trazabilidad del Principio II: los escenarios de una historia
go test -p 1 ./internal/metrics/... -run TestUS1

# Coherencia interna bajo escritura concurrente (SC-019): 20 consultas de 20
go test -p 1 ./internal/metrics/... -run Consistency -count=5

# El tipo Hundredths movido a platform/httpserver no cambia la salida de la 006
go test -p 1 ./internal/effort/... ./internal/platform/...

# Cobertura del backend
go test -p 1 -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Vulnerabilidades conocidas
govulncheck ./...
```

Resultado esperado: todas las pruebas pasan, y la búsqueda de trazabilidad devuelve los 61 escenarios
de la spec.

```powershell
Select-String -Path internal\metrics\**\*_test.go -Pattern "\(008/US[1-5]\)" | Measure-Object
```

---

## 4. Validación manual de punta a punta

Con la aplicación levantada contra la base de **desarrollo** (`go run ./cmd/app` desde `backend/`),
y un token obtenido con `/api/v1/auth/login`. Los datos se preparan con los endpoints de las features
002 a 007; acá solo se nombran.

### 4.1 Sprint Cerrado: el caso normal de US1

1. Crear un proyecto con factor **6** (002) y cuatro historias de 8, 5, 5 y 3 Story Points (003).
2. Crear un sprint, comprometer las cuatro, iniciarlo, completar las de 8 y 5, y registrar
   **142,5 horas** de esfuerzo repartidas entre ellas (004, 006).
3. Registrar 7 defectos con ese sprint como detección y resolver 4 en el mismo sprint (007).
4. Cerrar el sprint (004) y pedir:

```powershell
curl -H "Authorization: Bearer $token" http://localhost:8080/api/v1/projects/$projectId/metrics/sprints/$sprintId
```

Esperado: planificados `21`, completados `13`, estimadas `126.00`, reales `142.50`, desviación
`16.50` y `13.10`, completadas `50.00`, detectados `7`, resueltos `4`. Los cuatro primeros con
`finality: definitive`; reales y desviaciones `provisional`; los dos conteos `provisional`.

5. Cambiar el factor del proyecto a **8** y reestimar la historia de 5 a 13 (002, 003). Repetir la
   consulta: **nada** cambia (US1-3, US1-4, SC-005).

### 4.2 Sprint Activo y Planificado: US4

6. Crear un segundo sprint con 18 Story Points, iniciarlo, completar 5 puntos y cargar 40 horas.
   `GET .../metrics/active-sprint`: estimadas `108.00` (18 × 6, o `144.00` si en el paso 5 se dejó el
   factor en 8), desviación negativa y las nueve métricas `partial`.
7. Crear un tercer sprint sin iniciarlo, con 15 puntos. `GET .../metrics/sprints/{id}`: solo
   planificados y estimadas con valor; las otras siete `not_applicable`.
8. En un proyecto sin sprint Activo, `GET .../metrics/active-sprint` responde `200` con
   `{"activeSprint": false, "metrics": null}`.

### 4.3 Proyecto: US2, US3 y US5

9. `GET .../metrics`: la velocidad promedia solo sprints Cerrados y no lleva `finality`; el bloque
   `cumulative` dice `partial` mientras haya Activo; `effortOutsideSprintsHours` y
   `effortInPlannedSprintsHours` aparecen aunque valgan 0; los tres conteos de defectos traen las
   cuatro severidades y su suma cuadra con el total.
10. Cerrar el sprint Activo y repetir: `cumulative.finality` pasa a `definitive` (US3-5, SC-018).
11. `GET .../metrics/series`: seis listas con los mismos sprints Cerrados y Activo, en orden de
    fecha de inicio, sin el Planificado.

### 4.4 Plazo de gracia vencido

12. Con `EFFORT_GRACE_PERIOD_HOURS=1` en `backend/.env`, reiniciar la aplicación, cerrar un sprint y
    esperar una hora. Las horas reales y las desviaciones de ese sprint pasan a `definitive`; los
    conteos de defectos siguen `provisional` (US1-6). Las pruebas de integración cubren el mismo caso
    sin esperar, con el reloj inyectado.

### 4.5 Acceso

13. Con un usuario que no es integrante: las cuatro rutas responden `404` con
    `{"error":"El proyecto no existe."}`, idéntico al de un proyecto inexistente.
14. Con un integrante y el identificador de un sprint de **otro** proyecto: `404` con
    `{"error":"El sprint no existe."}`, idéntico al de un sprint inexistente.
15. Sin cabecera `Authorization`: `401` en las cuatro.
16. En un proyecto Finalizado (002), las cuatro responden `200`.

---

## 5. Swagger

```powershell
cd backend
swag init -g cmd/app/main.go -o docs
go run ./cmd/app
```

En `http://localhost:8080/swagger/index.html`, la etiqueta **Métricas** debe listar las cuatro rutas
con sus descripciones en español y los esquemas de [contracts/openapi.yaml](./contracts/openapi.yaml).

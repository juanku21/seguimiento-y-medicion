# Plan de Implementación: Cálculo de Métricas

**Rama**: `008-metrics-calculation` | **Fecha**: 2026-10-26 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/008-metrics-calculation/spec.md`

---

## Resumen

Esta feature es la única fuente de las fórmulas del producto: convierte los puntos, sprints, horas y
defectos que producen las features 002 a 007 en las métricas de estimación, seguimiento y calidad de
un proyecto, por sprint y para el proyecto completo. No escribe nada: calcula en cada consulta, con
"no calculable" ante todo denominador cero, y con cada cifra marcada como definitiva, provisoria o
parcial según de dónde sale. El dashboard (009) y los reportes (010) consumen estos resultados sin
recalcular.

El enfoque técnico tiene siete decisiones que lo definen:

1. **Un módulo `metrics` sin tablas.** El repositorio lee y suma por sprint; el dominio calcula. Todas
   las fórmulas son funciones puras de `metrics/domain` que reciben los datos crudos y el instante de
   la consulta, así que se prueban de forma unitaria sin base de datos (RN-01, RC-11).
2. **Una transacción `REPEATABLE READ` de solo lectura por consulta.** PostgreSQL fija una instantánea
   en la primera lectura y todas las cifras de una respuesta corresponden a ese único instante, sin
   bloquear a nadie (FR-003, RN-29, SC-019). El instante `now` se toma una vez por consulta.
3. **Aritmética entera en centésimas y un único redondeo**, el de la 006
   (`effortdomain.RoundPercentHundredths`), como exige su RC-13. La velocidad usa la misma función a
   través de `AverageHundredths`. El tipo `Hundredths` que serializa con 2 decimales se mueve de
   `effort/delivery` a `platform/httpserver` para compartirlo.
4. **Cada métrica lleva un estado explícito**: `available`, `not_calculable` o `not_applicable`, más
   `finality` (`definitive`, `provisional`, `partial`) en las de sprint. Un 0 real nunca se confunde
   con los otros dos (FR-035), y "provisoria" nunca se confunde con "parcial" (RN-24).
5. **La instantánea de cierre manda en el sprint Cerrado** y los datos vigentes en el Activo y el
   Planificado. "Completada" en el Activo usa el mismo criterio con el que la 004 congela el
   `outcome` al cerrar, así que la cifra no salta en el instante del cierre.
6. **Cuatro endpoints `GET` bajo `/api/v1/projects/:projectId`**: métricas del proyecto, series,
   sprint Activo y sprint por identificador. Colgar todo del proyecto es lo que permite responder
   "proyecto inexistente" a quien no es integrante y "sprint inexistente" a un sprint de otro
   proyecto, como piden US1-14 y US1-15.
7. **Una segunda cifra de esfuerzo separada**, el esfuerzo en sprints Planificados, decidida por el
   desarrollador el 2026-10-26 para que SC-016 se cumpla (research, sección 11) e incorporada a la
   spec en FR-029.

Esta feature **no tiene variables de entorno nuevas, ni dependencias nuevas, ni tablas nuevas, ni
frontend**: agrega un módulo de backend con cuatro endpoints y mueve un tipo compartido.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.3 (backend), sin cambios. Esta feature no toca el frontend (RC-10)

**Dependencias principales**: sin cambios: Gin v1.12.0, GORM v1.31.2 con driver `postgres` v1.6.3,
`google/uuid` v1.6.0, `swaggo/swag` v1.16.6 con `gin-swagger` v1.6.1 y `swaggo/files` v1.0.1. **Esta
feature no agrega ninguna dependencia** ([research.md](./research.md), sección 1)

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en los mismos contenedores disjuntos
de desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`). **Sin tablas
ni columnas nuevas**: lee `projects`, `user_stories`, `sprints`, `sprint_stories`, `effort_entries` y
`defects` con los índices que ya existen

**Pruebas**: `testify` v1.12.1 y `net/http/httptest` con reloj inyectado en el backend

**Plataforma objetivo**: API HTTP en Linux (contenedor); desarrollo en Windows

**Tipo de proyecto**: aplicación web, con backend de API y frontend separados en directorios de primer
nivel; esta feature solo agrega backend

**Objetivos de rendimiento**: la spec no fija tiempos de respuesta. Cada consulta es una transacción
de solo lectura con a lo sumo seis sentencias indexadas por proyecto o por sprint, sin bloqueos de
fila y sin paginar; el cálculo en memoria es lineal en la cantidad de compromisos del proyecto.
SC-011 pide las métricas completas de un sprint en una sola consulta, y así se cumple

**Restricciones**: resultados exactos contra el cálculo manual (SC-001, SC-002); cero divisiones por
cero que den 0, infinito o error (SC-003); exactamente 2 decimales con redondeo solo al final
(SC-004); sprints Cerrados inmutables ante reestimaciones y cambios de factor (SC-005); estados de
valor correctos en el 100 % de las métricas (SC-007, SC-015, SC-018); cero Descartados contados
(SC-008); desgloses que suman el total (SC-010); sesión y membresía en el 100 % de las consultas
(SC-013); `404` indistinguible (SC-014); cero horas invisibles (SC-016, ampliada); cero respuestas
contradictorias en 20 consultas concurrentes de 20 (SC-019)

**Escala y alcance**: 4 endpoints HTTP, 0 entidades persistidas, 1 tipo compartido movido de la 006 a
`platform`, 5 historias de usuario con **61** escenarios de aceptación y 31 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las decisiones de producto se cerraron en las sesiones de
clarificación del 2026-10-03, y la única inconsistencia encontrada al planificar (las horas ancladas a
sprints Planificados) la resolvió el desarrollador el 2026-10-26. Las incógnitas **técnicas** se
resolvieron en [research.md](./research.md): módulo sin tablas, forma de las rutas, coherencia por
consulta, aritmética y redondeo, representación de los tres estados de valor, estado definitivo,
provisorio y parcial, origen de cada dato según el estado del sprint, velocidad y series, acumulados,
esfuerzo separado, defectos, advertencias, acceso y códigos HTTP, pruebas y frontend.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **Enmienda de la spec ya aplicada.** La decisión del 2026-10-26 sobre el esfuerzo en sprints
   Planificados se integró con `/speckit-clarify` el mismo día: FR-029, RN-25, SC-016, la tabla
   "Métricas del proyecto", Casos Límite y el escenario nuevo US3-12. La spec pasó de 60 a 61
   escenarios.
2. **Dependencia de la 007 sin implementar.** El módulo `defects` está planificado, pero no está en
   `main`. Solo US5 necesita su tabla, sus tipos de dominio y sus endpoints: los tipos de defectos
   del dominio de métricas se declaran recién en US5, así que las fases 1 a 5 pueden empezar con la
   001 a la 006 en `main`, y la 007 completa solo bloquea US5.
3. **Esta feature toca código de la 006**: mueve el tipo `Hundredths` de `effort/delivery/dto.go` a
   `platform/httpserver/hundredths.go` y deja un alias en su lugar. No cambia ningún uso ni ninguna
   salida de la 006, y sus pruebas actuales lo verifican.
4. **Sprint Planificado sin estado de valor.** Lo fijó la sesión de clarificación del 2026-10-26
   (FR-015, US4-4): sus dos cifras se devuelven con `finality: null` (research, sección 7).
5. **Advertencias de inconsistencia adicionales.** Además del caso de FR-037, el plan define
   `missing_frozen_factor` y `missing_close_time` para dos punteros que la 004 nunca deja nulos en un
   sprint Cerrado, pero que el código tiene que manejar sin un `panic` ni un 500 (Principio VII,
   RN-30; research, sección 13).
6. **Fórmulas que ya existen fuera de esta feature.** La 002 muestra horas estimadas del proyecto
   (`projects/service/estimation.go`), la 004 muestra los totales de la instantánea en el detalle del
   sprint (`sprintResultsSelect`) y la 006 compara estimado con real por historia. RN-01 dice que
   ninguna otra feature recalcula una métrica. Este plan no las toca: son vistas propias de esas
   features y la 006 ya comparte el redondeo con esta (su RC-13). Si el equipo quiere que todas
   deleguen en `metrics`, es un cambio aparte.
7. **Rama de trabajo**: el plan se generó en `main`, y `.specify/feature.json` (ignorado por Git)
   ahora apunta a `specs/008-metrics-calculation`. La spec indica la rama `008-metrics-calculation`
   para la implementación.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I. Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 4 endpoints cubren las 5 historias (US1 y US4 por sprint y sprint Activo; US2 y US3 en el proyecto y las series; US5 repartida entre el proyecto y el sprint). Sin predicciones, burndown, métricas por persona, comparación entre proyectos, alertas, almacenamiento de resultados ni métricas definidas por el usuario: todo eso está en "Fuera de Alcance". |
| II. TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 61 escenarios son pruebas nombradas; el entorno de test existe desde la 001. Todas las fórmulas, el redondeo, los estados de valor y las advertencias son pruebas unitarias obligatorias. |
| III. Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/metrics/` con sus cuatro capas. |
| IV. Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva. |
| V. Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001; esta feature no agrega servicios. |
| VI. Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. No hay configuración nueva; ninguna lectura toca `users`. |
| VII. Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Las únicas entradas son dos UUID de ruta: el de proyecto lo valida `RequireMembership` y el de sprint, `delivery`, que responde `404` si está mal formado. Las divisiones por cero y los datos rotos se resuelven con estados y advertencias, nunca con `panic`. |
| VIII. Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de dominio en inglés; comentarios, mensajes y Swagger en español. README actualizado en el mismo cambio. |
| IX. Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Ninguna tabla, columna, índice ni variable de entorno. Las tres lecturas de `MetricsRepository` se usan todas. Sin endpoint de defectos ni de velocidad aparte, porque viajan en la respuesta del proyecto. Sin frontend, que es de la 009. `Hundredths` se mueve en lugar de copiarse (DRY), y el redondeo se reutiliza de la 006. | **PASA** |
| II | Los 61 escenarios se mapean a pruebas nombradas (tabla más abajo). Las pruebas unitarias obligatorias cubren las nueve fórmulas de sprint, `Percent`, `AverageHundredths`, `SprintFinality` con todas sus combinaciones, la ventana de velocidad con 0 a 4 Cerrados, `BuildSeries`, los acumulados, `TallyDefects`, las cuatro advertencias y las reglas RN-01 a RN-31. | **PASA** |
| III | `metrics/domain` importa solo los dominios de `sprints`, `backlog`, `effort` (para `GraceOpen` y `RoundPercentHundredths`) y `defects`. `metrics/delivery` no importa la capa de entrega de ningún otro módulo: recibe el subgrupo de proyecto desde `cmd/app/`, y el tipo `Hundredths` viene de `platform`. No hay ciclos: ningún módulo importa `metrics`. | **PASA** |
| IV | Ninguna dependencia nueva. El aislamiento de la transacción usa `database/sql.TxOptions` de la librería estándar. | **PASA** |
| V | Sin servicios ni migraciones nuevas; `testsupport` no cambia, porque las tablas que se leen ya se vacían. | **PASA** |
| VI | Ninguna respuesta revela datos de otros proyectos: `404` con los cuerpos de la 002 y la 004, y el sprint se busca siempre con el `project_id` de la ruta. Ninguna lectura incluye `users`. | **PASA** |
| VII | Cuatro endpoints sin cuerpo ni query. Errores de dominio traducidos a `404`; cualquier otro a `500` genérico con `slog`. La transacción es `ReadOnly`, así que la base rechaza cualquier escritura accidental (FR-002). Los punteros nulos de un sprint Cerrado producen una advertencia y un supuesto, no un `panic`. | **PASA** |
| VIII | El contrato OpenAPI está en español. Valores `available`/`not_calculable`/`not_applicable`, `definitive`/`provisional`/`partial`, `planning`/`data_inconsistency` y los códigos de advertencia en inglés en la API; los mensajes en español. La descripción general de `main.go` y el `README.md` se amplían con la feature. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Dos decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **Esta feature modifica código de la 006**: mueve `Hundredths` a `platform/httpserver` para no
   duplicarlo (Principio I, DRY) sin que `metrics/delivery` importe `effort/delivery` (Principio III)
   (research, sección 5).
2. **`metrics/domain` importa `effort/domain`** para reutilizar `RoundPercentHundredths` y `GraceOpen`.
   Es una dependencia entre dominios, como las que ya tienen `effort` y `defects` hacia `backlog` y
   `sprints`, y es la que exige RC-13 de la 006 (research, secciones 5 y 7).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/008-metrics-calculation/
├── spec.md              # Especificación (entrada)
├── plan.md              # Este archivo (salida de /speckit-plan)
├── research.md          # Salida de la Fase 0
├── data-model.md        # Salida de la Fase 1
├── quickstart.md        # Salida de la Fase 1
├── contracts/
│   └── openapi.yaml     # Salida de la Fase 1
├── checklists/
│   └── requirements.md  # Checklist de calidad de la spec
└── tasks.md             # Salida de /speckit-tasks; NO lo crea /speckit-plan
```

### Código fuente (raíz del repositorio)

Las líneas marcadas con `NUEVO` las crea esta feature; las marcadas con `TOCA` son archivos de las
features 001 a 007 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                          # TOCA: compone el módulo metrics sobre projectGroup con el plazo de gracia y el reloj; amplía @description
├── internal/
│   ├── metrics/                             # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── values.go                    # ValueStatus, Finality, Metric, Available, NotCalculable, NotApplicable
│   │   │   ├── arithmetic.go                # Percent, AverageHundredths (sobre effortdomain.RoundPercentHundredths)
│   │   │   ├── sprint.go                    # SprintData, SprintStoryData, SprintMetrics, MetricKind, SprintFinality, CalculateSprint
│   │   │   ├── project.go                   # ProjectData, Velocity, Cumulative, ProjectMetrics, CalculateProject
│   │   │   ├── series.go                    # ProjectSeries, SeriesPoint, BuildSeries
│   │   │   ├── defects.go                   # DefectCount, DefectTally, ProjectDefects, TallyDefects
│   │   │   ├── warnings.go                  # WarningKind, WarningCode, Warning
│   │   │   ├── errors.go                    # ErrSprintNotFound
│   │   │   └── repository.go                # MetricsRepository
│   │   ├── repository/
│   │   │   └── metrics_repository.go        # GORM; transacción REPEATABLE READ de solo lectura; LoadProject, LoadSprint, LoadActiveSprint
│   │   ├── service/
│   │   │   └── metrics_service.go           # ProjectMetrics, ProjectSeries, ActiveSprintMetrics, SprintMetrics; reloj y plazo de gracia
│   │   └── delivery/
│   │       ├── handler.go                   # 4 manejadores con anotaciones swag en español
│   │       ├── dto.go                       # Respuestas, MetricDTO, WarningDTO y mensajes de advertencia
│   │       └── routes.go                    # Rutas bajo el subgrupo /api/v1/projects/:projectId
│   ├── effort/
│   │   └── delivery/
│   │       └── dto.go                       # TOCA: type Hundredths = httpserver.Hundredths
│   └── platform/
│       └── httpserver/
│           └── hundredths.go                # NUEVO: Hundredths y MarshalJSON, movidos sin cambios desde effort/delivery
├── docs/                                    # Regenerado con swag init
├── postgres/                                # Sin cambios
├── .env.example                             # Sin cambios
├── Dockerfile                               # Sin cambios
├── go.mod                                   # Sin cambios
└── go.sum                                   # Sin cambios

frontend/                                    # Sin cambios (RC-10)

README.md                                    # TOCA: describe el cálculo de métricas y sus endpoints
```

**Pruebas**: en el backend, junto al código del paquete que prueban: `values_test.go`,
`arithmetic_test.go`, `sprint_test.go`, `project_test.go`, `series_test.go`, `defects_test.go` y
`warnings_test.go` en `metrics/domain` (unitarias, sin base); `handler_test.go` y
`consistency_test.go` en `metrics/delivery` (integración); `hundredths_test.go` en
`platform/httpserver`. No se crea un árbol `tests/` separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/metrics/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. No tiene
middleware propio: sus rutas cuelgan del subgrupo de proyecto de la 002, que ya exige sesión y
membresía. El frontend no cambia: la presentación es de la 009.

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `008/US1` a `008/US5` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (008/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run` (por ejemplo, US1-5 y US1-6 con el reloj antes y
después del plazo; US2-2 a US2-4 con 1, 2, 3 y 4 sprints Cerrados; US5-6 antes y después de
descartar).

**Frontend (Vitest)**: no aplica; esta feature no tiene interfaz (RC-10).

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Métricas de un sprint cerrado | 15 | `TestUS1_SprintCerradoCasoNormal`, `TestUS1_Sobrestimacion`, `TestUS1_CambioDeFactorPosterior`, `TestUS1_ReestimacionPosterior`, `TestUS1_EstadosDentroDelPlazoDeGracia`, `TestUS1_EstadosConPlazoVencido`, `TestUS1_EsfuerzoDentroDelPlazo`, `TestUS1_HistoriasEnCeroPuntos`, `TestUS1_SprintSinHistorias`, `TestUS1_TodasCompletadas`, `TestUS1_DecimalPeriodico`, `TestUS1_HistoriaReplanificada`, `TestUS1_SprintInexistente`, `TestUS1_SprintDeOtroProyecto`, `TestUS1_NoIntegrante` |
| US2 Velocidad y su evolución | 15 | `TestUS2_VelocidadTresCerrados`, `TestUS2_VentanaDeTres`, `TestUS2_UnSprintCerrado`, `TestUS2_DosSprintsCerrados`, `TestUS2_SerieDeVelocidad`, `TestUS2_SeisSeries`, `TestUS2_SeriesConSprintActivo`, `TestUS2_SeriesSinPlanificados`, `TestUS2_ValorCeroEnSeries`, `TestUS2_SeriesVacias`, `TestUS2_VelocidadCero`, `TestUS2_SoloSprintActivo`, `TestUS2_ProyectoSinSprints`, `TestUS2_VentanaQueSeDesplaza`, `TestUS2_NoIntegrante` |
| US3 Métricas acumuladas del proyecto | 15 | `TestUS3_AcumuladosCasoNormal`, `TestUS3_PorcentajeAcumulado`, `TestUS3_SinPlanificados`, `TestUS3_AcumuladoParcial`, `TestUS3_AcumuladoDefinitivo`, `TestUS3_HistoriaEnDosSprints`, `TestUS3_SoloSprintsPlanificados`, `TestUS3_ProyectoSinSprints`, `TestUS3_EstimadasEnCero`, `TestUS3_EsfuerzoFueraDeSprint`, `TestUS3_EsfuerzoFueraDeSprintEnCero`, `TestUS3_EsfuerzoEnSprintsPlanificados`, `TestUS3_BacklogSinEstimar`, `TestUS3_BacklogEstimado`, `TestUS3_ProyectoInexistente` |
| US4 Sprint activo o planificado | 7 | `TestUS4_SprintActivoCasoNormal`, `TestUS4_HistoriaAgregada`, `TestUS4_FactorVigente`, `TestUS4_SprintPlanificado`, `TestUS4_ActivoSinHistorias`, `TestUS4_ActivoSinHoras`, `TestUS4_SinSprintActivo` |
| US5 Métricas de defectos | 9 | `TestUS5_DefectosDelSprint`, `TestUS5_DefectosDelProyecto`, `TestUS5_DeteccionYResolucionEnDistintosSprints`, `TestUS5_MismoSprint`, `TestUS5_DefectoReabierto`, `TestUS5_DefectosDescartados`, `TestUS5_ProyectoSinDefectos`, `TestUS5_DeteccionEnSprintCerrado`, `TestUS5_NoIntegrante` |

**Total: 61 escenarios.**

**Cómo se preparan los escenarios que dependen de otras features**:

- Los proyectos y el factor se crean y se cambian con los endpoints de la 002; las historias y sus
  Story Points, con los de la 003; los sprints se crean, se comprometen, se inician, avanzan y se
  cierran con los de la 004; las horas se cargan con los de la 006, y los defectos se registran,
  resuelven, reabren y descartan con los de la 007. El router de prueba compone esos módulos junto
  con el de métricas, así que US1-3, US1-4 (instantánea), US1-7 (esfuerzo dentro del plazo), US1-12 y
  US3-6 (replanificación) y US5-5 (reapertura) ejercitan la integración real.
- El reloj inyectado en los servicios de esfuerzo y de métricas mueve `now` 10 horas y 49 horas
  después del cierre para US1-5 y US1-6, sin esperar.
- Los datos rotos de SC-020 (una historia sin Story Points dentro de un sprint) se fuerzan con una
  sentencia directa sobre la base de test, porque ninguna API permite crearlos.

**Pruebas transversales sin etiqueta de escenario**: la prueba sin sesión de cada uno de los cuatro
endpoints (SC-013), la comparación byte a byte de los `404` de proyecto y de sprint, con identificador
inexistente, ajeno y mal formado (SC-014), la de coherencia bajo escritura concurrente
(`TestMetrics_Consistency`, SC-019: 20 consultas mientras se cargan horas y se completan historias),
la de horas no invisibles (`TestMetrics_NoInvisibleEffort`, SC-016 ampliada: acumulado + fuera de
sprint + en Planificados = total del proyecto, con esfuerzo cargado sobre un sprint Planificado), la
de datos rotos (`TestMetrics_DataInconsistency`, SC-020), la de repetibilidad
(`TestMetrics_Repeatable`, SC-012) y la del proyecto Finalizado (`TestMetrics_FinishedProject`,
FR-043) se nombran sin `008/USn`, porque no derivan de un escenario de la spec y no deben alterar el
conteo de 61.

Además, pruebas unitarias **obligatorias** por el Principio II (cálculo de métricas, estimación,
reglas de negocio y validaciones), que cubren SC-021:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| `TestMetrics_SingleSourceOfFormulas`: ningún módulo fuera de `metrics` y `cmd/app` importa `metrics`; `metrics/delivery` no importa otra capa `delivery` | RN-01 |
| Función privada `readOnly` del repositorio: un `UPDATE` dentro de su transacción falla con SQLSTATE `25006`; `TestMetrics_DoesNotWrite` | RN-02 |
| `CalculateSprint` de un Cerrado con factor y puntos vigentes distintos de los congelados | RN-03 |
| `CalculateSprint` de un Activo: datos vigentes y las nueve `partial` | RN-04 |
| `CalculateSprint` de un Planificado: dos `available` y siete `not_applicable` | RN-05 |
| `CalculateProject` y `BuildSeries` con un Planificado entre los sprints | RN-06 |
| Horas estimadas con factor 6, 8 y con dos decimales (por ejemplo 6,25) | RN-07 |
| Signo de la desviación absoluta y porcentual en subestimación y sobrestimación | RN-08 |
| `StoryCompletionPercent` con puntos desiguales entre historias completadas y no completadas | RN-09 |
| Ventana de velocidad con 0, 1, 2, 3 y 4 Cerrados, y con `closed_at` en orden distinto de `start_date` | RN-10 |
| Acumulado de dos sprints con porcentajes distintos: el cociente sale de los totales, no del promedio | RN-11 |
| Una historia en dos sprints: dos veces planificada, una vez completada | RN-12 |
| Defecto detectado y resuelto en sprints distintos y en el mismo | RN-13 |
| Defecto reabierto: fuera de resueltos, dentro de detectados | RN-14 |
| `TallyDefects` con una fila `discarded` | RN-15 |
| `TallyDefects`: abiertos = `open` + `in_progress` | RN-16 |
| `Percent` con denominador 0; desviación con estimadas 0; completadas sin historias; velocidad sin Cerrados | RN-17 |
| Un sprint con desviación porcentual `not_calculable` y el resto `available` | RN-18 |
| `Percent` y `AverageHundredths` con 1/3, 2/3, 49/3, empates en ,005 positivos y negativos | RN-19 |
| `Metric` serializado: `available` con 0, `not_calculable` y `not_applicable` producen tres JSON distintos | RN-20 |
| Las cuatro rutas sin membresía | RN-21 |
| Las cuatro rutas sobre un proyecto Finalizado | RN-22 |
| `SprintFinality` con las 9 métricas × gracia vigente y vencida en un Cerrado | RN-23 |
| `SprintFinality`: ningún `partial` en un Cerrado y ningún `provisional` en un Activo | RN-24 |
| `CalculateProject`: esfuerzo sin sprint fuera de `Cumulative.ActualHours` | RN-25 |
| `CalculateSprint` nunca emite `unestimated_backlog_stories`; `CalculateProject` sí | RN-26 |
| `Cumulative.Finality` con y sin Activo | RN-27 |
| `Velocity` no tiene `finality` aunque haya Activo (prueba de tipo y de serialización) | RN-28 |
| `TestMetrics_Consistency` | RN-29 |
| `CalculateSprint` con una historia sin puntos, sin factor congelado y sin `closed_at`: calcula y avisa | RN-30 |
| `CalculateProject` con advertencias de las dos clases: todas las métricas presentes y `kind` distinto | RN-31 |

Comparación con los criterios de éxito: SC-001 y SC-002 con las pruebas de US1 a US5, que usan los
datos exactos de la spec; SC-003 con la fila RN-17; SC-004 con `TestUS1_DecimalPeriodico`,
`TestUS2_VentanaDeTres` y la fila RN-19; SC-005 con `TestUS1_CambioDeFactorPosterior` y
`TestUS1_ReestimacionPosterior`; SC-006 con `TestUS2_*` sobre 0 a 4 Cerrados; SC-007 con
`TestUS4_SprintActivoCasoNormal` y la fila RN-24; SC-008 con `TestUS5_DefectosDescartados`; SC-009
con `TestUS5_DefectoReabierto`; SC-010 con `TestUS5_DefectosDelProyecto` y `TallyDefects`; SC-011 con
`TestUS1_SprintCerradoCasoNormal` (una sola consulta); SC-012 con `TestMetrics_Repeatable`; SC-013 y
SC-014 con las pruebas transversales; SC-015 con `TestUS1_EstadosDentroDelPlazoDeGracia` y
`TestUS1_EstadosConPlazoVencido`; SC-016 con `TestUS3_EsfuerzoEnSprintsPlanificados` y `TestMetrics_NoInvisibleEffort`; SC-017 con
`TestUS3_BacklogSinEstimar` y la fila RN-26; SC-018 con `TestUS3_AcumuladoParcial`,
`TestUS3_AcumuladoDefinitivo` y la fila RN-28; SC-019 con `TestMetrics_Consistency`; SC-020 con
`TestMetrics_DataInconsistency`; SC-021 con la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

0. **Prerrequisito**: la 001 a la 006 en `main` para las fases 1 a 5, y la 007 completa en `main`
   para US5 (punto 2).
1. **Fase técnica fundacional**: mover `Hundredths` a `platform/httpserver` con su prueba; tipos de
   valor (`ValueStatus`, `Finality`, `Metric`), `Percent` y `AverageHundredths`; `Warning` y sus
   códigos; `ErrSprintNotFound`; `MetricsRepository` (firmas); esqueleto del servicio, de las rutas y
   de la composición en `cmd/app/`. Es el bloque que bloquea a todas las historias.
2. **US1 Sprint cerrado** (P1): `CalculateSprint` y `SprintFinality` para el Cerrado; `LoadSprint` con
   la instantánea, el esfuerzo y la transacción de solo lectura; endpoint
   `metrics/sprints/{sprintId}` y sus `404`.
3. **US2 Velocidad y series** (P1): ventana de velocidad, `velocity.series`, `BuildSeries`;
   `LoadProject`; endpoints `metrics` (solo velocidad) y `metrics/series`.
4. **US3 Acumulados** (P2): `Cumulative`, esfuerzo fuera de sprint y en Planificados, contador del
   backlog sin estimar y su advertencia; se completa la respuesta de `metrics`; prueba de horas no
   invisibles y de coherencia concurrente.
5. **US4 Sprint activo o planificado** (P2): `CalculateSprint` para el Activo y el Planificado;
   `LoadActiveSprint`; endpoint `metrics/active-sprint`.
6. **US5 Defectos** (P3): conteos por sprint en `LoadSprint` y `LoadProject`, `TallyDefects`, bloque
   `defects` del proyecto.
7. **Cierre**: advertencias de inconsistencia forzadas (SC-020), Swagger comparado con el contrato,
   `README.md`, comandos de cobertura, `govulncheck` y la verificación transversal de trazabilidad y
   de las puertas de seguridad (SC-013, SC-014, SC-021).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las dos concesiones
documentadas, que son el movimiento de `Hundredths` desde la 006 y la importación de `effort/domain`
desde `metrics/domain`, son consecuencias de DRY y de RC-13 de la 006, no desvíos de la constitución;
están justificadas en la sección "Verificación de la Constitución" y en [research.md](./research.md).

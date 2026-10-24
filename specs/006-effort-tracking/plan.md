# Plan de Implementación: Registro de Esfuerzo

**Rama**: `006-effort-tracking` | **Fecha**: 2026-10-24 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/006-effort-tracking/spec.md`

---

## Resumen

Esta feature permite que cada integrante registre las horas que trabajó sobre una historia, con
fecha y actividad, las corrija o las elimine mientras sean suyas y no estén congeladas, consulte el
esfuerzo por historia y por proyecto con filtros, y compare el esfuerzo estimado (Story Points ×
factor del proyecto) con el real. Los registros quedan anclados para siempre al sprint en el que
estaba la historia al cargarlos; son la fuente de las horas reales que consumirán las métricas de la
008.

El enfoque técnico tiene seis decisiones que lo definen:

1. **Un solo modelo nuevo, `effort_entries`, más una marca en `user_stories`.** El sprint del
   registro es una columna anulable que se escribe una vez y nunca cambia. `ever_had_effort` hace que
   una historia con esfuerzo no vuelva a ser eliminable aunque se borren sus registros (RC-05), con
   el mismo patrón que `ever_committed` de la 004.
2. **Todas las horas son enteros de centésimas.** La misma unidad que el factor del proyecto: sumas,
   estimaciones y desvíos son exactos, y el porcentaje se redondea una sola vez, al final, con
   aritmética entera. La API serializa siempre 2 decimales (`6.50`, `20.00`); "no calculable" es
   `null`.
3. **El anclaje y el plazo de gracia son funciones puras.** `ResolveAnchor` decide el sprint del alta
   con el compromiso abierto y el último congelado de la 004; `EnsureEntryWritable` decide si un
   registro todavía se corrige. El plazo es `closed_at + EFFORT_GRACE_PERIOD_HOURS` (por defecto 48),
   evaluado en cada pedido, cumplido al alcanzarse.
4. **El tope de 24 horas es una invariante bajo concurrencia.** Cada alta o modificación bloquea la
   fila del autor en `users` con `FOR NO KEY UPDATE` antes de sumar sus horas del día en todos sus
   proyectos: los pedidos simultáneos de una persona se serializan y el que excede se rechaza.
5. **Un orden de bloqueos que no choca con la 004.** Proyecto (`FOR SHARE`) → autor → inserción →
   marca de la historia. Nunca se bloquea una historia antes que su sprint, que es el orden de
   `Close` y `AssignStories`.
6. **Esta feature cierra los dos puertos que la 003 y la 004 dejaron abiertos** (`EffortRecords` y
   `SprintEffort`), y la eliminación de un sprint traduce la violación de la clave foránea al mismo
   `409` de "sprint con esfuerzo".

Esta feature **no tiene fase de arranque**: agrega un módulo, una tabla, una columna, siete
endpoints, una variable de entorno opcional, una sección en el detalle de la historia y una página
del proyecto, sin dependencias nuevas.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.3 (backend); TypeScript 5.9.3 sobre Node.js (frontend), sin cambios

**Dependencias principales**: sin cambios — Gin v1.12.0, GORM v1.31.2 con driver `postgres` v1.6.3,
`google/uuid` v1.6.0, `gin-contrib/cors` v1.7.9, `swaggo/swag` v1.16.6 con `gin-swagger` v1.6.1 y
`swaggo/files` v1.0.1; Next.js 16.4.2 con React 19.3.0 y Tailwind CSS 4.3.3. **Esta feature no
agrega ninguna dependencia** ([research.md](./research.md), sección 1)

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en los mismos contenedores disjuntos
de desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`). Una tabla
nueva, `effort_entries`, y una columna nueva, `user_stories.ever_had_effort`, creadas con
`AutoMigrate`

**Pruebas**: `testify` v1.12.1 y `net/http/httptest` con reloj y zona horaria inyectados en el
backend; Vitest 5.0.3 con `@testing-library/react` 16.3.3 y jsdom 30.1.2 en el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor) y navegador moderno; desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: la spec no fija tiempos de respuesta. Cada escritura es una
transacción con a lo sumo seis sentencias y tres bloqueos de fila; cada consulta, una sola lectura
indexada (por historia, por autor y fecha, o por proyecto). SC-001 a SC-003 son tiempos de tarea de
la persona: registrar en 3 pasos con la fecha ya cargada, ver total y desglose en el detalle de la
historia, y responder "cuántas horas puso cada uno en este sprint" con dos filtros

**Restricciones**: cero fechas con más de 24 horas de una persona bajo 10 altas simultáneas (SC-005);
totales exactos al cuarto de hora (SC-015); porcentaje con exactamente 2 decimales y un único
redondeo (SC-022); cero escrituras sobre registros ajenos (SC-008), sobre sprints definitivos
(SC-009) o en proyectos Finalizados (SC-016); el sprint asociado nunca cambia (SC-010); cero
registros perdidos por quitar a una persona (SC-011) o por cerrar un sprint (SC-021); sesión y
membresía en el 100 % de las acciones (SC-017); `404` indistinguible y ningún mensaje del tope que
nombre otro proyecto (SC-018)

**Escala y alcance**: 7 endpoints HTTP, 1 entidad persistida nueva más 1 columna en una existente, 2
puertos ajenos reemplazados, 1 variable de entorno opcional, 1 sección nueva en el detalle de la
historia de la 003, 1 página nueva del proyecto, 5 historias de usuario con **74** escenarios de
aceptación y 26 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las decisiones de producto se cerraron en las sesiones de
clarificación del 2026-10-02 y del 2026-10-04. Las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): forma de las rutas y de los `404`, acceso a un registro, modelo y marca
permanente, aritmética exacta, validación de entrada, "hoy" en la zona del sistema, anclaje y plazo
de gracia, tope diario concurrente, orden de bloqueos, consultas y filtros, comparación, integración
con la 003 y la 004, frontend y códigos HTTP.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **Criterio de empate del redondeo**: la spec y la 008 (FR-033) fijan "2 decimales solo al final"
   pero no el empate. El plan adopta **mitad lejos de cero** y deja la función en `effort/domain`
   para que la 008 la reutilice, como exige RC-13 (research, sección 5).
2. **Variable nueva `EFFORT_GRACE_PERIOD_HOURS`**: opcional, con defecto 48; un valor inválido
   detiene el arranque, igual que `APP_TIMEZONE` (research, sección 8). Hay que agregarla a
   `backend/.env.example` y al README, y también a `backend/.env` y `backend/.env.test`
   (Principio VI).
3. **Esta feature toca código de la 003 y de la 004**: reemplaza el cuerpo de
   `backlog/repository/effort_records.go` y de `sprints/repository/sprint_effort.go` (sus
   constructores pasan a recibir la conexión), agrega `EverHadEffort` a `UserStory`, traduce la
   violación de clave foránea en `SprintRepository.Delete`, y suma una sección al detalle de la
   historia y un enlace en el resumen del proyecto en el frontend.
4. **Un punto para `/revisar-seguridad`**: la ventana de carrera entre `HasEffort` y el borrado de
   una historia en la 003. La clave foránea impide perder el registro, pero la respuesta sería `500`
   en lugar de `409`; es la misma ventana que la 005 dejó señalada (research, punto abierto 2).
5. **Conteo de escenarios del checklist**: `checklists/requirements.md` dice 73 escenarios y 22
   criterios de éxito; la spec vigente tiene **74** escenarios (US1 25, US2 8, US3 18, US4 12, US5 11)
   y **23** criterios (SC-001 a SC-023), y además RC-13 y RN-26 de la sesión del 2026-10-04. El plan
   usa los números de la spec; el checklist no se modificó.
6. **Rama de trabajo**: el plan se generó en la rama `repo/005-plan`, y `.specify/feature.json`
   (ignorado por Git) ahora apunta a `specs/006-effort-tracking`. La spec indica la rama
   `006-effort-tracking` para la implementación.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 7 endpoints se corresponden con las 5 historias (US1 registrar; US2 consultar la historia; US3 corregir y eliminar; US4 listar el proyecto con filtros; US5 comparar por historia y por proyecto). Sin temporizadores, hojas semanales, carga masiva, aprobaciones, costos, agregados por sprint, gráficos, capacidad, categorías, notificaciones, exportación ni auditoría: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 74 escenarios son pruebas nombradas; el entorno de test existe desde la 001. El plazo de gracia y "hoy" se prueban con reloj y zona inyectados. Anclaje, plazo, fecha, tope, horas, actividad, comparación y redondeo son pruebas unitarias obligatorias. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/effort/` con sus cuatro capas; el frontend usa las capas existentes. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001; esta feature no agrega servicios. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. La variable nueva no es un secreto y tiene su entrada en `.env.example`. Los nombres se leen de `users` sin `password_hash`. El mensaje del tope no nombra otros proyectos. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Cuerpo, campos inmutables, parámetros de ruta y filtros se validan en `delivery`; las reglas que dependen de datos, dentro de la transacción. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de dominio en inglés; comentarios, mensajes y Swagger en español. README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Una tabla con las columnas de la entidad de la spec y ninguna derivable: `project_id`, "definitivo", totales y desvíos se derivan (data-model, secciones 1 y 5). Una columna en `user_stories` porque RC-05 la exige. Sin paginación ni endpoint de "mis horas", que la spec no pide. Un bloqueo de fila por persona en lugar de advisory locks o tablas de totales. `EffortRepository` declara 9 operaciones, todas usadas. | **PASA** |
| II | Los 74 escenarios se mapean a pruebas nombradas (tabla más abajo). Las pruebas unitarias obligatorias cubren `ResolveAnchor`, `GraceOpen`, `EnsureEntryWritable`, `ValidateEntryDate`, `CheckDailyCap`, `Compare`, `RoundPercentHundredths`, `EnsureAuthor`, `ParseHours`, la validación de actividad, de campos inmutables y de filtros, y las reglas RN-01 a RN-26. Reloj y zona inyectados hacen reproducibles las fronteras del plazo y de la medianoche. | **PASA** |
| III | `effort/domain` importa solo `backlog/domain`, `sprints/domain` (estados y resultado) y `auth/domain`. `effort/delivery` no importa `backlog/delivery` ni `projects/delivery`: recibe `RequireStoryAccess` y el subgrupo de proyecto desde `cmd/app/`. Los puertos de la 003 y la 004 leen tablas de esta feature sin importar `effort`. No hay ciclos. | **PASA** |
| IV | Ninguna dependencia nueva de Go ni de npm. Bloqueos con `gorm.io/gorm/clause`; horas leídas con `json.RawMessage` sobre su texto literal. | **PASA** |
| V | Sin servicios nuevos. La tabla y la columna se crean con `AutoMigrate` en las mismas bases disjuntas; el ayudante de pruebas vacía `effort_entries`. | **PASA** |
| VI | La autoría selecciona solo `users.id` y `users.full_name`. Ninguna respuesta revela datos de proyectos ajenos: `404` único, filtro ajeno con el mismo mensaje que el inexistente, tope sin nombres de proyecto. El cuerpo sigue limitado a 1 MiB. | **PASA** |
| VII | Los siete endpoints validan antes del servicio. Errores de dominio traducidos a `400`, `403`, `404` y `409` con `code`. Toda escritura es transaccional y atómica. El orden de bloqueos evita interbloqueos con la 004; la violación de clave foránea al borrar un sprint se traduce a `409`. Un `EFFORT_GRACE_PERIOD_HOURS` inválido detiene el arranque. | **PASA** |
| VIII | El contrato OpenAPI está en español. Valores `read_only`/`effort_finalized`/`daily_cap_exceeded` y `no_effort`/`no_matches` en la API, con etiquetas en español en el frontend. `any` prohibido. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Tres decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **Esta feature escribe una columna de la 003** (`user_stories.ever_had_effort`). Es la
   consecuencia mínima de RC-05 y repite el precedente de `ever_committed` de la 004 (research,
   sección 4).
2. **Esta feature modifica código de la 003 y de la 004**: los dos puertos que esas features dejaron
   anotados para que esta los implemente y la traducción de la clave foránea que la 004 anticipó en
   su comentario (research, sección 13).
3. **Tres rutas de esta feature cuelgan de `/api/v1/stories/:storyId`**, el prefijo de la 003, y dos
   del subgrupo de proyecto de la 002. Es lo que permite responder con los mismos `404` de historia y
   de proyecto (research, sección 2).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/006-effort-tracking/
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
│       └── main.go                          # TOCA: compone el módulo effort, pasa RequireStoryAccess y el subgrupo de proyecto, el plazo de gracia, la zona y el reloj; pasa db a los puertos de la 003 y la 004
├── internal/
│   ├── effort/                              # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── entry.go                     # Entidad EffortEntry
│   │   │   ├── anchor.go                    # OpenCommitment, ClosedCommitment, ResolveAnchor
│   │   │   ├── grace.go                     # AnchoredSprint, GraceOpen, EnsureEntryWritable
│   │   │   ├── entry_date.go                # ValidateEntryDate
│   │   │   ├── daily_cap.go                 # MaxDailyHundredths, CheckDailyCap
│   │   │   ├── comparison.go                # Comparison, Compare, RoundPercentHundredths
│   │   │   ├── author.go                    # EnsureAuthor
│   │   │   ├── errors.go                    # Errores de dominio y errores con datos (tope, fecha mínima)
│   │   │   └── repository.go                # EffortRepository y tipos de lectura
│   │   ├── repository/
│   │   │   └── effort_repository.go         # GORM; transacciones, orden de bloqueos, anclaje, tope, filtros y comparación
│   │   ├── service/
│   │   │   └── effort_service.go            # Create, Update, Delete, StoryEffort, ProjectEffort, StoryComparison, ProjectComparison
│   │   └── delivery/
│   │       ├── handler.go                   # 7 manejadores con anotaciones swag en español
│   │       ├── dto.go                       # Peticiones, respuestas y el tipo de 2 decimales
│   │       ├── validation.go                # ParseHours, actividad, fecha, campos inmutables, filtros
│   │       ├── middleware.go                # RequireEffortEntryAccess (404 único)
│   │       └── routes.go                    # Rutas sobre la historia, sobre el proyecto y grupo plano del registro
│   ├── backlog/
│   │   ├── domain/
│   │   │   └── user_story.go                # TOCA: campo EverHadEffort (columna ever_had_effort)
│   │   └── repository/
│   │       └── effort_records.go            # TOCA: lee ever_had_effort en lugar de informar "sin esfuerzo"
│   ├── sprints/
│   │   └── repository/
│   │       ├── sprint_effort.go             # TOCA: EXISTS sobre effort_entries.sprint_id
│   │       └── sprint_repository.go         # TOCA: Delete traduce gorm.ErrForeignKeyViolated a ErrSprintHasEffort
│   └── platform/
│       ├── config/
│       │   └── config.go                    # TOCA: EFFORT_GRACE_PERIOD_HOURS, defecto 48, validación al arrancar
│       ├── database/
│       │   └── postgres.go                  # TOCA: AutoMigrate de EffortEntry, después de SprintStory
│       └── testsupport/
│           └── database.go                  # TOCA: vacía effort_entries
├── postgres/                                # Sin cambios
├── .env.example                             # TOCA: EFFORT_GRACE_PERIOD_HOURS=48 con su comentario
├── Dockerfile                               # Sin cambios
├── go.mod                                   # Sin cambios
└── go.sum                                   # Sin cambios

frontend/
├── src/
│   ├── app/
│   │   └── projects/
│   │       └── [projectId]/
│   │           ├── backlog/
│   │           │   └── [storyId]/
│   │           │       └── page.tsx         # TOCA: sección "Esfuerzo" (US1, US2, US3, US5)
│   │           └── effort/
│   │               └── page.tsx             # NUEVO: esfuerzo del proyecto con filtros y comparación (US4, US5)
│   ├── features/
│   │   ├── effort/                          # NUEVO
│   │   │   ├── components/                  # StoryEffortSection, EffortEntryForm, EffortEntryList,
│   │   │   │                                # EffortEntryRow (edición en línea y eliminación),
│   │   │   │                                # EffortBreakdown, EffortComparisonCard,
│   │   │   │                                # ProjectEffortView, EffortFilters, EffortComparisonTable
│   │   │   ├── api.ts                       # Llamadas a los 7 endpoints
│   │   │   └── validation.ts                # Reglas de forma de fecha, actividad y horas
│   │   └── projects/
│   │       └── components/
│   │           └── project-overview.tsx     # TOCA: enlace a "Esfuerzo"
│   ├── types/
│   │   └── effort.ts                        # NUEVO: tipos de peticiones y respuestas; sin `any`
│   └── utils/
│       └── effort-format.ts                 # NUEVO: horas con coma, porcentaje con signo, "No calculable"
├── Dockerfile                               # Sin cambios
└── package.json                             # Sin cambios: ninguna dependencia nueva

README.md                                    # TOCA: describe el registro de esfuerzo, la variable nueva y los endpoints
```

**Pruebas**: en el backend, junto al código del paquete que prueban (`anchor_test.go`,
`grace_test.go`, `entry_date_test.go`, `daily_cap_test.go`, `comparison_test.go`, `author_test.go`,
`validation_test.go`, `effort_service_test.go`, `handler_test.go`, `middleware_test.go`,
`effort_concurrency_test.go`), más las pruebas de integración de los puertos reemplazados en
`backlog/repository` y `sprints/repository`, y la de `EFFORT_GRACE_PERIOD_HOURS` en
`platform/config`. En el frontend, junto al componente o módulo (`validation.test.ts`,
`effort-format.test.ts`, `effort-entry-form.test.tsx`, `effort-entry-list.test.tsx`,
`effort-breakdown.test.tsx`, `effort-filters.test.tsx`, `effort-comparison-table.test.tsx`,
`api.test.ts`). No se crea un árbol `tests/` separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/effort/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. El frontend no
agrega capas: la sección vive en el detalle de la historia porque es donde se trabaja sobre ella, y
la vista de proyecto es una página hermana de `backlog/` y `sprints/`. Antes de escribir código de
Next.js se lee la documentación de la versión instalada en `frontend/node_modules/next/dist/docs/`
(CLAUDE.md).

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `006/US1` a `006/US5` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (006/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run` (por ejemplo, US1-17 con 0, negativo y 24,25; US1-9
con 17:59 y 18:01).

**Frontend (Vitest)**: `describe("006/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario que tenga comportamiento visible en la
interfaz (formulario y sus mensajes, botones solo en registros propios y no definitivos, desglose,
lista vacía, filtros y "No calculable").

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Registrar el esfuerzo trabajado | 25 | `TestUS1_RegistroEnSprintActivo`, `TestUS1_HistoriaSoloEnBacklog`, `TestUS1_SprintPlanificado`, `TestUS1_HistoriaCompletadaEnSprintActivo`, `TestUS1_SegundoRegistroDelDia`, `TestUS1_HistoriaSinEstimar`, `TestUS1_CompletadaDentroDelPlazo`, `TestUS1_DevueltaAlBacklogDentroDelPlazo`, `TestUS1_LimiteDelPlazo`, `TestUS1_VeinticuatroHorasExactas`, `TestUS1_CuartoDeHora`, `TestUS1_ActividadDeTrescientosCaracteres`, `TestUS1_DiaDeInicioDelProyecto`, `TestUS1_HoySegunZonaDelSistema`, `TestUS1_TopeEntreProyectos`, `TestUS1_TopeDiarioSuperado`, `TestUS1_HorasFueraDeRango`, `TestUS1_HorasNoMultiplo`, `TestUS1_ActividadVacia`, `TestUS1_FechaFutura`, `TestUS1_FechaAnteriorAlInicio`, `TestUS1_SprintDefinitivo`, `TestUS1_ProyectoFinalizado`, `TestUS1_NoIntegrante`, `TestUS1_AutorAjenoIgnorado` |
| US2 Consultar el esfuerzo de una historia | 8 | `TestUS2_TotalYDesglose`, `TestUS2_RegistrosDeVariosSprints`, `TestUS2_ExIntegrante`, `TestUS2_HistoriaSinRegistros`, `TestUS2_CuartoDeHora`, `TestUS2_UnaLineaPorPersona`, `TestUS2_NoIntegrante`, `TestUS2_HistoriaInexistente` |
| US3 Corregir o eliminar un registro propio | 18 | `TestUS3_CorreccionDeHoras`, `TestUS3_Eliminacion`, `TestUS3_CambioDeFecha`, `TestUS3_SprintPlanificadoAhoraActivo`, `TestUS3_HistoriaQuitadaDelSprint`, `TestUS3_ReduccionEnDiaCompleto`, `TestUS3_AumentoQueSuperaElTope`, `TestUS3_EliminacionDelUnicoRegistro`, `TestUS3_DentroDelPlazo`, `TestUS3_PlazoCumplido`, `TestUS3_RegistroAjeno`, `TestUS3_ModificacionDefinitiva`, `TestUS3_EliminacionDefinitiva`, `TestUS3_RegistroSinSprint`, `TestUS3_DatosInvalidos`, `TestUS3_CamposInmutables`, `TestUS3_ProyectoFinalizado`, `TestUS3_ExIntegrante` |
| US4 Consultar el esfuerzo del proyecto con filtros | 12 | `TestUS4_SinFiltros`, `TestUS4_FiltroPorIntegrante`, `TestUS4_FiltroPorSprint`, `TestUS4_RangoEHistoria`, `TestUS4_SinSprint`, `TestUS4_RangoDeUnDia`, `TestUS4_SinCoincidencias`, `TestUS4_ProyectoSinEsfuerzo`, `TestUS4_FiltroPorExIntegrante`, `TestUS4_RangoInvertido`, `TestUS4_FiltroDeOtroProyecto`, `TestUS4_NoIntegrante` |
| US5 Comparar el esfuerzo estimado con el real | 11 | `TestUS5_Exceso`, `TestUS5_Ahorro`, `TestUS5_Exacta`, `TestUS5_CambioDeFactor`, `TestUS5_SinEstimar`, `TestUS5_CeroPuntos`, `TestUS5_SinRegistros`, `TestUS5_CeroPuntosSinRegistros`, `TestUS5_TodasSinEstimar`, `TestUS5_NoIntegrante`, `TestUS5_HistoriaInexistente` |

**Total: 74 escenarios.**

**Cómo se preparan los escenarios que dependen de otras features o del tiempo**:

- Las historias se crean y se estiman con los endpoints de la 003; los sprints se crean, se
  comprometen, se inician, se cierran y se les quitan historias con los de la 004, que el router de
  prueba compone junto con el módulo de esfuerzo. Así US1-3, US1-7, US1-8, US3-4, US3-5, SC-010 y
  RC-07 ejercitan la integración real.
- El proyecto Finalizado (US1-23, US3-17), el cambio de factor (US5-4) y la baja y reincorporación de
  integrantes (US2-3, US3-18, US4-9) se preparan con los endpoints de la 002.
- El plazo de gracia (US1-7 a US1-9, US1-22, US3-9, US3-10, US3-12, US3-13) se reproduce cerrando el
  sprint con el reloj inyectado de la 004 en un instante fijo y avanzando el reloj inyectado del
  módulo de esfuerzo hasta cada frontera. "Hoy" (US1-14, US1-20) usa la zona inyectada y un reloj
  cerca de la medianoche.
- El tope entre proyectos (US1-15, US1-16) usa tres proyectos del mismo integrante.

**Pruebas transversales sin etiqueta de escenario**: la prueba sin sesión de cada uno de los siete
endpoints (SC-017), la comparación byte a byte de los `404` de historia, de proyecto y de registro
(SC-018), el intento de atribuir un registro a otra persona por cada forma de indicarlo (SC-004), la
suma de 100 registros de 0,25 (SC-015), la de concurrencia del tope (`TestEffortDailyCap_Concurrency`,
SC-005: 10 altas simultáneas, y una variante con altas y modificaciones mezcladas), la del sprint
asociado inmutable tras quitar, recomprometer y cerrar (`TestEffortAnchor_Immutable`, SC-010), y las
cuatro fronteras del plazo por acción (`TestEffortGrace_Boundaries`, SC-020) se nombran sin
`006/USn`, porque no derivan de un escenario de la spec y no deben alterar el conteo de 74.

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones, cálculos y reglas de
negocio), que cubren SC-023:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Alta con un `userId` ajeno en el cuerpo y en la query; autor igual a la sesión | RN-01 |
| Validación de actividad: vacía, solo espacios, 300 y 301 runas, recorte | RN-02 |
| `ParseHours`: 0, negativo, 0.25, 1.3, 6.5, 24, 24.25, `"6.5"` entre comillas (rechazado), exponente, 3 decimales | RN-03 |
| `CheckDailyCap`: 0 + 24, 20 + 4, 20 + 5, 24 + 0.25, disponible nunca negativo | RN-04 |
| `ValidateEntryDate`: hoy, mañana, inicio, día anterior al inicio, inicio futuro | RN-05 |
| Escrituras en proyecto Finalizado con plazo vigente | RN-06 |
| `ResolveAnchor`: la tabla completa de data-model.md, sección 6.1 | RN-07, RN-24, RN-25 |
| Modificación que no toca `sprint_id`; puertos de la 004 sin escritura sobre `effort_entries` | RN-08 |
| `GraceOpen` y `EnsureEntryWritable`: la tabla de data-model.md, sección 6.2, con `now = closedAt + grace` | RN-09, RN-10, RN-23 |
| `EnsureAuthor` con el autor, otro integrante y el propietario | RN-11 |
| Validación de campos inmutables en el `PATCH` | RN-12 |
| Alta sobre historias `pending`, `in_progress`, `completed`, sin estimar y de 0 puntos | RN-13 |
| Listados con un autor quitado del proyecto | RN-14, RN-15 |
| `Compare`: la tabla de research.md, sección 12, con factor 6, 6,1 y 8 | RN-16 a RN-20 |
| Borrado de historia, sprint y usuario con registros: la base lo rechaza | RN-21 |
| `EffortRecords.HasEffort` después de borrar todos los registros | RN-22 |
| `RoundPercentHundredths`: 1/3, 2/3, −1/3, empates positivo y negativo, 16,5/126 = 13,10 | RN-19, RN-26 |
| Validación de filtros: UUID mal formado, `none`, rango invertido, rango de un día | FR-038, FR-043 |
| `config.Load` con `EFFORT_GRACE_PERIOD_HOURS` ausente, `48`, `0`, `-1`, `48h`, `abc` | RC-11 |

Comparación con los criterios de éxito: SC-001 y SC-002 con las pruebas de componentes de
`StoryEffortSection`; SC-003 con `TestUS4_FiltroPorIntegrante` y `TestUS4_FiltroPorSprint`; SC-004
con la prueba transversal de autoría; SC-005 con `TestEffortDailyCap_Concurrency`,
`TestUS1_TopeEntreProyectos`, `TestUS1_TopeDiarioSuperado` y `TestUS3_AumentoQueSuperaElTope`;
SC-006 con `ParseHours`; SC-007 con `ValidateEntryDate`, `TestUS1_DiaDeInicioDelProyecto` y
`TestUS1_HoySegunZonaDelSistema`; SC-008 con `TestUS3_RegistroAjeno`; SC-009 con
`TestUS3_ModificacionDefinitiva` y `TestUS3_EliminacionDefinitiva`; SC-010 con
`TestEffortAnchor_Immutable`; SC-011 con `TestUS2_ExIntegrante`; SC-012 y SC-013 con `Compare`,
`TestUS5_SinEstimar` y `TestUS5_CeroPuntos`; SC-014 con `TestUS5_*` sobre la muestra completa;
SC-015 con la suma de 100 cuartos de hora; SC-016 con `TestUS1_ProyectoFinalizado` y
`TestUS3_ProyectoFinalizado`; SC-017 y SC-018 con las pruebas transversales; SC-019 con US4; SC-020
con `TestEffortGrace_Boundaries`; SC-021 con `TestUS1_CompletadaDentroDelPlazo` y
`TestUS1_DevueltaAlBacklogDentroDelPlazo`; SC-022 con `RoundPercentHundredths` y el formato de 2
decimales; SC-023 con la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

1. **Fase técnica fundacional**: la entidad `EffortEntry`, la columna `EverHadEffort`, los errores de
   dominio, `EffortRepository` (firmas), `AutoMigrate`, el vaciado en `testsupport`,
   `EFFORT_GRACE_PERIOD_HOURS` en `config` y en `.env.example`, `RequireEffortEntryAccess`, el tipo
   de 2 decimales y el esqueleto de rutas en `cmd/app/`. En el frontend, los tipos, `api.ts` y
   `effort-format.ts`. Es el bloque que bloquea a todas las historias.
2. **US1 Registrar** (P1): `ParseHours`, validación de actividad y fecha; `ValidateEntryDate`,
   `CheckDailyCap`, `ResolveAnchor`, `GraceOpen`; alta con el orden de bloqueos y la marca de la
   historia; prueba de concurrencia del tope; reemplazo de los dos puertos de la 003 y la 004 y la
   traducción de la clave foránea; formulario de registro.
3. **US2 Consultar la historia** (P1): lista, total y desglose; sección "Esfuerzo" del detalle de la
   historia con el desglose y la lista.
4. **US3 Corregir o eliminar** (P2): validación del `PATCH` y de los campos inmutables;
   `EnsureEntryWritable`, `EnsureAuthor`; modificación y eliminación; edición en línea y eliminación
   con confirmación en la lista.
5. **US4 Esfuerzo del proyecto** (P2): validación de filtros y su pertenencia al proyecto; listado
   con total y `emptyReason`; página de esfuerzo del proyecto con filtros y enlace desde el resumen.
6. **US5 Comparar** (P3): `Compare` y `RoundPercentHundredths`; comparación de una historia y del
   proyecto; tarjeta en el detalle de la historia y tabla en la página del proyecto.
7. **Cierre**: Swagger comparado con el contrato, `README.md`, comandos de cobertura, `govulncheck` y
   la verificación transversal de trazabilidad y de las puertas de seguridad (SC-004, SC-017,
   SC-018, SC-023).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las tres concesiones
documentadas —la columna escrita en una tabla de la 003, la modificación puntual de los puertos de la
003 y la 004, y las rutas bajo los prefijos de historia y de proyecto— son consecuencias necesarias
de requisitos de la spec (RC-05, RC-07, FR-059) y no desvíos de la constitución; están justificadas
en la sección "Verificación de la Constitución" y en [research.md](./research.md).

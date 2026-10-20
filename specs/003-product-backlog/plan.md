# Plan de Implementación: Product Backlog

**Rama**: `003-product-backlog` | **Fecha**: 2026-10-20 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/003-product-backlog/spec.md`

---

## Resumen

Esta feature permite a los integrantes de un proyecto administrar su Product Backlog: registrar
historias de usuario, listarlas en orden de prioridad con filtros, consultar su detalle, estimarlas
en Story Points, refinarlas y eliminar las que nunca se trabajaron. El backlog es la materia prima
de todo lo que sigue: sin historias estimadas no hay qué planificar en un sprint (004), qué estimar
por Planning Poker (005), contra qué registrar esfuerzo (006) ni de dónde calcular velocidad,
burndown y desvío (008).

El enfoque técnico tiene cinco decisiones que lo definen:

1. **Dos formas de ruta para dos respuestas de "no acceso".** La spec pide "proyecto inexistente"
   en la creación y el listado, y "historia inexistente, idéntica a la de un identificador que no
   existe" en el detalle, la modificación, la estimación y la eliminación. La colección cuelga del
   subgrupo `/api/v1/projects/:projectId` de la 002 y hereda su `RequireMembership`; el ítem es
   plano (`/api/v1/stories/:storyId`) con un middleware propio que resuelve historia, membresía y
   estado del proyecto **en una sola consulta** con **una sola salida `404`**.
2. **El 0 y "sin estimar" son distintos en todas las capas.** `story_points` es un `smallint`
   anulable y el valor llega como `json.Number`, validado sobre su texto contra la escala cerrada.
   Así `6.5`, `4` y `-1` se rechazan con el mismo mensaje, y nunca se confunde 0 con ausencia.
3. **Modificación parcial real con `PATCH`.** FR-020 define qué significa un campo ausente y uno
   vacío, y FR-025 exige que dos ediciones de campos distintos no se pisen. Se aplica un `UPDATE`
   solo de las columnas presentes y la lista de criterios se reemplaza completa bajo un bloqueo de
   fila, para que dos listas simultáneas nunca se fusionen.
4. **Cuatro puertos hacia features que todavía no existen.** Comprometida en sprint, historial de
   sprint, esfuerzo, defectos y sesiones de Planning Poker los producen las features 004 a 007. Igual
   que la 002, `domain/` declara un puerto por feature productora y las implementaciones actuales
   informan lo que hoy es cierto: ninguna historia tiene ese historial.
5. **La vista de estado de la 002 empieza a informar datos reales.** Se reemplaza el cuerpo de su
   proveedor del resumen del backlog, que es el punto de integración que la 002 dejó anotado, sin
   tocar `projects/service`.

Esta feature **no tiene fase de arranque**: reutiliza todo lo construido por la 001 y la 002. Agrega
un módulo, dos tablas, seis endpoints y cuatro pantallas, sin dependencias ni variables de entorno
nuevas.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.2 (backend); TypeScript sobre Node.js 24.21.0 (frontend)

**Dependencias principales**: sin cambios respecto de la 002 — Gin v1.12.0, GORM v1.31.2 con driver
`postgres` v1.6.3, `google/uuid` v1.6.0, `gin-contrib/cors` v1.7.9, `swaggo/swag` v1.16.6 con
`gin-swagger` v1.6.1 y `swaggo/files` v1.0.1; Next.js 16.3.8 con React 19.3.0 y Tailwind CSS 4.3.3.
**Esta feature no agrega ninguna dependencia** ([research.md](./research.md), sección 1)

**Almacenamiento**: PostgreSQL, imagen `postgres:18.6-alpine`, en los mismos contenedores disjuntos
de desarrollo (puerto 5432, base `smye_dev`) y de test (puerto 5433, base `smye_test`). Dos tablas
nuevas: `user_stories` y `acceptance_criteria`

**Pruebas**: `testify` v1.12.1 (incluido `testify/mock` para los dobles de los puertos) y
`net/http/httptest` en el backend; Vitest 5.0.3 con `@testing-library/react` 16.3.3 y jsdom 30.1.2 en
el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor) y navegador moderno; desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: la spec no fija tiempos de respuesta. SC-001 a SC-003 son tiempos de
tarea de la persona (registrar en menos de 2 minutos, estimar en menos de 30 segundos y en 3 pasos,
encontrar las Altas sin estimar en menos de 15 segundos), que dependen de la interfaz: el listado
muestra prioridad, estado y "sin estimar" en cada fila y ofrece los filtros a un clic. Del lado del
servidor, el listado se resuelve en **una consulta** con conteo de criterios por subconsulta y el
detalle en **dos** (historia con autor, y criterios), sin N+1. SC-004 se verifica con un backlog de 50
historias. **No se fija una meta de peticiones por segundo**: sería alcance no solicitado (Principio
I)

**Restricciones**: historia ajena indistinguible de inexistente en el 100 % de las acciones (FR-047,
SC-006); 100 % de los valores fuera de la escala rechazados (SC-007); cero títulos de más de 150 y
criterios de más de 500 (SC-008); lista para planificar exacta (SC-009); cero historias Completadas
modificadas (SC-010); cero historial perdido por eliminación (SC-011); proyecto Finalizado de solo
lectura (SC-012); dos ediciones simultáneas de campos distintos conservadas (SC-013); el backlog nunca
cambia el estado de una historia (FR-027)

**Escala y alcance**: 6 endpoints HTTP, 2 entidades persistidas, 4 puertos hacia otras features más
1 de lectura de la 002, 4 pantallas de frontend (listado, creación, detalle y edición), 6 historias
de usuario con **58** escenarios de aceptación y 18 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las decisiones de producto se habían cerrado en las sesiones
de clarificación del 2026-09-25 y del 2026-09-26. Las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): forma de las rutas y del `404`, representación del 0 frente a "sin
estimar", persistencia y orden de los criterios, semántica del `PATCH`, edición simultánea, vía de
estimación, orden y filtros del listado, derivación de "lista para planificar", puertos hacia
sprints, esfuerzo, defectos y Planning Poker, eliminación definitiva, integración con la vista de
estado de la 002, solo lectura, atribución y códigos HTTP.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **Rama de trabajo**: el plan se generó estando en `main`, y `.specify/feature.json` (ignorado
   por Git) ahora apunta a `specs/003-product-backlog`. Los artefactos de esta feature son archivos
   de la copia de trabajo; la rama en la que se registran la decide el equipo.
2. **Forma del `PATCH`**: se rechazan con `400` los campos `status` y `storyPoints` en lugar de
   ignorarlos, y un cuerpo sin campos editables también se rechaza. Afecta al contrato
   ([research.md](./research.md), sección 6).
3. **Lectura de FR-049**: se persiste el autor de cada historia; la identidad de quien modifica,
   estima o elimina sale siempre de la sesión pero no se guarda, porque el historial de auditoría
   está fuera de alcance. Si el equipo entiende que hay que guardar el último modificador, se agrega
   `updated_by_id` ([research.md](./research.md), sección 15).
4. **Esta feature toca código de la 002**: reemplaza el cuerpo de
   `projects/repository/backlog_summary.go` (su constructor pasa a recibir la conexión, así que
   cambian `cmd/app/main.go` y los ayudantes de prueba de `projects/delivery`) y agrega etiquetas en
   español a `ProjectStatusCard` ([research.md](./research.md), sección 13).
5. **Puntos de integración para anotar al planificar 004, 005, 006 y 007**: cada una reemplaza la
   implementación de su puerto en `backlog/repository/`; Planning Poker reutiliza
   `StoryService.Estimate` para registrar el valor acordado; sprints es la única que escribe
   `user_stories.status`; y las claves foráneas que esas features creen hacia `user_stories` deben
   traducirse al mismo `409` de eliminación bloqueada ([research.md](./research.md), secciones 8, 11
   y 12).
6. **Ventana de carrera aceptada**: las reglas que dependen de los puertos (historia comprometida,
   historial para eliminar) se verifican antes de escribir, fuera del bloqueo. Hoy no hay carrera
   posible porque nadie produce esos datos; cuando exista la 004, la clave foránea es el respaldo de
   la eliminación y conviene revisar el resto en `/revisar-seguridad`.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 6 endpoints se corresponden uno a uno con las 6 historias. Sin tareas, épicas, etiquetas, adjuntos, comentarios, reordenamiento manual, búsqueda, paginación, auditoría ni responsable asignado: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 58 escenarios son pruebas nombradas; el entorno de test existe desde la 001. Las validaciones, la escala de estimación y las 18 reglas de negocio son pruebas unitarias obligatorias. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/backlog/` con sus cuatro capas; el frontend usa las capas ya existentes. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001; esta feature no agrega servicios. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. Sin variables nuevas. La autoría se lee de `users` sin seleccionar `password_hash`. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Cuerpos, parámetros de ruta y filtros se validan en `delivery`; los Story Points sobre su texto literal. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de dominio en inglés, comentarios, mensajes y Swagger en español, README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Dos tablas y ninguna columna sin requisito que la pida: se descartaron `ready_to_plan` (derivable), `priority_rank` (derivable), `deleted_at` (la eliminación es definitiva) y `updated_by_id` (sin escenario que la verifique). La interfaz `StoryRepository` declara 8 operaciones, todas usadas; sin cambio de estado ni de proyecto. Los puertos tienen una operación cada uno. | **PASA** |
| II | Los 58 escenarios se mapean a pruebas nombradas (tabla más abajo). Las pruebas unitarias obligatorias cubren las validaciones, **la escala de estimación y el 0 frente a "sin estimar"**, `Readiness` y las reglas RN-01 a RN-18. Los puertos son lo que hace verificables con valores reales US3-3, US4-6, US5-8, US6-4, US6-6 y US6-7. | **PASA** |
| III | `backlog/domain` importa solo `projects/domain` y `auth/domain`, para las claves foráneas. `backlog/delivery` no importa `projects/delivery`: recibe el subgrupo del proyecto desde `cmd/app/`. `projects/repository` importa `backlog/domain` para leer `user_stories`; no hay ciclo entre paquetes. | **PASA** |
| IV | Ninguna dependencia nueva de Go ni de npm. El bloqueo de fila usa `gorm.io/gorm/clause`, que es parte de GORM. | **PASA** |
| V | Sin servicios nuevos. Las tablas se crean con `AutoMigrate` en las mismas bases disjuntas; el ayudante de pruebas las vacía. | **PASA** |
| VI | La lectura de la autoría selecciona solo `users.id` y `users.full_name`. Ninguna respuesta revela datos de historias o proyectos ajenos. El cuerpo sigue limitado a 1 MiB por el middleware global de la 002. | **PASA** |
| VII | Los seis endpoints validan antes del servicio, incluidos los filtros. Errores de dominio traducidos a `404` y `409`; sin `panic` en el camino de ninguna petición. Creación, modificación y eliminación son transaccionales. | **PASA** |
| VIII | El contrato OpenAPI está en español. Valores `high`/`medium`/`low` y `pending`/`in_progress`/`completed` en la API, con etiquetas en español en el frontend. `any` prohibido: los tipos del frontend son explícitos. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Tres decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **Cuatro puertos con implementaciones que informan "sin historial"** hasta que existan las
   features 004 a 007. No es infraestructura "por si acaso": FR-018, FR-022, FR-039 y FR-041 están
   en esta spec con escenarios que los prueban, y sin los puertos no se pueden implementar ni
   probar ([research.md](./research.md), sección 11).
2. **Esta feature modifica un archivo de la 002** (`projects/repository/backlog_summary.go`). Es el
   punto de integración que la 002 fijó en su plan para que su vista de estado informe el backlog
   real (RC-07), y no toca su servicio ([research.md](./research.md), sección 13).
3. **Dos formas de ruta** (colección anidada, ítem plano) en lugar de una. Es lo que permite
   devolver la respuesta de "no acceso" que la spec pide en cada acción con un solo `404` por
   middleware ([research.md](./research.md), sección 2).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/003-product-backlog/
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
features 001 y 002 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                          # TOCA: compone el módulo backlog y pasa la conexión al proveedor del resumen
├── internal/
│   ├── backlog/                             # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── user_story.go                # Entidad UserStory
│   │   │   ├── acceptance_criterion.go      # Entidad AcceptanceCriterion
│   │   │   ├── priority.go                  # StoryPriority y su rango de orden
│   │   │   ├── status.go                    # StoryStatus (solo lectura en esta feature)
│   │   │   ├── story_points.go              # Escala cerrada e IsValidStoryPoints
│   │   │   ├── readiness.go                 # Readiness: lista para planificar y qué falta
│   │   │   ├── errors.go                    # Errores de dominio y DeletionBlockedError
│   │   │   ├── repository.go                # StoryRepository y tipos de lectura
│   │   │   └── ports.go                     # SprintAssignments, EffortRecords, DefectRecords, EstimationSessions, ProjectDirectory
│   │   ├── repository/
│   │   │   ├── story_repository.go          # GORM; transacciones, bloqueo de fila, orden y filtros
│   │   │   ├── project_directory.go         # Lee solo projects.status
│   │   │   ├── sprint_assignments.go        # Puerto de sprints: informa "sin sprint" hasta la 004
│   │   │   ├── effort_records.go            # Puerto de esfuerzo: informa "sin esfuerzo" hasta la 006
│   │   │   ├── defect_records.go            # Puerto de defectos: informa "sin defectos" hasta la 007
│   │   │   └── estimation_sessions.go       # Puerto de Planning Poker: informa "sin sesiones" hasta la 005
│   │   ├── service/
│   │   │   └── story_service.go             # Create, List, Get, Estimate, Update, Delete
│   │   └── delivery/
│   │       ├── handler.go                   # 6 manejadores HTTP con anotaciones swag en español
│   │       ├── dto.go                       # Peticiones y respuestas; StoryPatch con json.RawMessage
│   │       ├── validation.go                # Título, descripción, prioridad, escala, criterios y filtros
│   │       ├── middleware.go                # RequireStoryAccess (404 único)
│   │       └── routes.go                    # Colección en el subgrupo del proyecto; ítem plano
│   ├── projects/
│   │   └── repository/
│   │       └── backlog_summary.go           # TOCA: lee user_stories en lugar de informar vacío
│   └── platform/
│       ├── database/
│       │   └── postgres.go                  # TOCA: AutoMigrate de UserStory y AcceptanceCriterion
│       └── testsupport/
│           └── database.go                  # TOCA: vacía user_stories y acceptance_criteria
├── postgres/                                # Sin cambios
├── .env.example                             # Sin cambios
├── Dockerfile                               # Sin cambios
├── go.mod                                   # Sin cambios
└── go.sum                                   # Sin cambios

frontend/
├── src/
│   ├── app/
│   │   └── projects/
│   │       └── [projectId]/
│   │           └── backlog/
│   │               ├── page.tsx             # NUEVO: US2 listado con filtros
│   │               ├── new/page.tsx         # NUEVO: US1 creación
│   │               └── [storyId]/
│   │                   ├── page.tsx         # NUEVO: US3 detalle, US4 estimación y US6 eliminación
│   │                   └── edit/page.tsx    # NUEVO: US5 refinamiento
│   ├── features/
│   │   ├── backlog/                         # NUEVO
│   │   │   ├── components/                  # StoryList, BacklogFilters, StoryForm, AcceptanceCriteriaEditor,
│   │   │   │                                # StoryDetailCard, EstimateControl, DeleteStoryButton
│   │   │   ├── api.ts                       # Llamadas a los 6 endpoints
│   │   │   └── validation.ts                # Reglas de título, descripción, prioridad y criterios en el formulario
│   │   └── projects/
│   │       └── components/
│   │           ├── project-overview.tsx     # TOCA: enlace al backlog
│   │           └── project-status-card.tsx  # TOCA: etiquetas en español de los estados del backlog
│   ├── types/
│   │   └── story.ts                         # NUEVO: tipos de peticiones y respuestas; sin `any`
│   └── utils/
│       └── story-labels.ts                  # NUEVO: etiquetas de prioridad y estado, y la escala de Story Points
├── Dockerfile                               # Sin cambios
└── package.json                             # Sin cambios: ninguna dependencia nueva

README.md                                    # TOCA: describe el backlog y sus endpoints
```

**Pruebas**: en el backend, junto al código del paquete que prueban (`validation_test.go`,
`story_points_test.go`, `readiness_test.go`, `story_service_test.go`, `handler_test.go`,
`middleware_test.go`, `story_concurrency_test.go`), y en `projects/repository` una prueba de
integración del proveedor del resumen. En el frontend, junto al componente o módulo
(`story-form.test.tsx`, `validation.test.ts`). No se crea un árbol `tests/` separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/backlog/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. Se eligió un
único módulo para historias y criterios porque un criterio no existe fuera de su historia
([research.md](./research.md), sección 2). El frontend no agrega capas: usa las diez que ya declara el
Principio III, con las pantallas del backlog anidadas bajo el proyecto para la navegación aunque las
rutas de ítem de la API sean planas.

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `003/US1` a `003/US6` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (003/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run`.

**Frontend (Vitest)**: `describe("003/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario.

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Registrar una historia | 12 | `TestUS1_CreacionCompleta`, `TestUS1_SoloTituloYPrioridad`, `TestUS1_ProyectoPlanificado`, `TestUS1_CeroStoryPoints`, `TestUS1_LimitesDelTitulo`, `TestUS1_LimitesDelCriterio`, `TestUS1_TituloRepetido`, `TestUS1_CamposObligatorios`, `TestUS1_StoryPointsFueraDeEscala`, `TestUS1_CriterioVacio`, `TestUS1_ProyectoFinalizado`, `TestUS1_NoIntegrante` |
| US2 Ver el backlog ordenado y filtrado | 8 | `TestUS2_OrdenPorPrioridadYFecha`, `TestUS2_FiltroPorEstado`, `TestUS2_FiltrosCombinados`, `TestUS2_BacklogVacio`, `TestUS2_FiltroSinCoincidencias`, `TestUS2_OrdenEstableAnteEmpate`, `TestUS2_FiltroInvalido`, `TestUS2_NoIntegrante` |
| US3 Consultar el detalle | 6 | `TestUS3_DetalleCompleto`, `TestUS3_SinEstimarNiCriterios`, `TestUS3_AsignadaASprint`, `TestUS3_CeroStoryPoints`, `TestUS3_HistoriaAjena`, `TestUS3_HistoriaInexistente` |
| US4 Estimar en Story Points | 9 | `TestUS4_EstimacionInicial`, `TestUS4_Reestimacion`, `TestUS4_VolverASinEstimar`, `TestUS4_SinCriterios`, `TestUS4_ExtremosDeLaEscala`, `TestUS4_ComprometidaNoVuelveASinEstimar`, `TestUS4_ValorFueraDeEscala`, `TestUS4_HistoriaCompletada`, `TestUS4_ProyectoFinalizado` |
| US5 Refinar una historia | 13 | `TestUS5_RefinamientoCompleto`, `TestUS5_HistoriaDeOtroIntegrante`, `TestUS5_ReordenDeCriterios`, `TestUS5_VaciarCriterios`, `TestUS5_HistoriaEnProgreso`, `TestUS5_EdicionSimultaneaCamposDistintos`, `TestUS5_EdicionSimultaneaDeCriterios`, `TestUS5_ComprometidaNoQuedaSinCriterios`, `TestUS5_EntradaInvalidaSinCambioParcial`, `TestUS5_HistoriaCompletada`, `TestUS5_ProyectoFinalizado`, `TestUS5_HistoriaAjena`, `TestUS5_CambioDeEstadoNoDisponible` |
| US6 Eliminar una historia | 10 | `TestUS6_EliminacionExitosa`, `TestUS6_ListaParaPlanificarSeElimina`, `TestUS6_IntegranteNoPropietario`, `TestUS6_QuitadaDeUnSprint`, `TestUS6_UnicaHistoria`, `TestUS6_EsfuerzoODefectos`, `TestUS6_SesionesDeEstimacion`, `TestUS6_ProyectoFinalizado`, `TestUS6_HistoriaAjena`, `TestUS6_YaEliminada` |

**Total: 58 escenarios.**

**Cómo se preparan los escenarios que dependen de otras features**: los estados `in_progress` y
`completed` (US5-5, US4-8, US5-10) se siembran escribiendo la columna `status` directamente en la
base de test, porque ningún endpoint de esta feature los produce. Las historias comprometidas, con
historial de sprint, con esfuerzo, con defectos o con sesiones de estimación (US3-3, US4-6, US5-8,
US6-4, US6-6, US6-7) se preparan con dobles de `testify/mock` de los cuatro puertos, inyectados al
componer el router de prueba.

**Pruebas transversales sin etiqueta de escenario**: la prueba sin sesión de cada uno de los seis
endpoints (SC-005) y la comparación byte a byte de los `404` (SC-006) se nombran sin `003/USn`
(por ejemplo `TestStoryEndpoints_SinSesion`), porque no derivan de un escenario de la spec y no
deben alterar el conteo de 58.

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones, estimación y reglas de
negocio), que cubren SC-014:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Acceso por membresía sin distinguir propietario: `RequireStoryAccess` y el servicio con un actor que no es propietario | RN-01, RN-02 |
| Validación y recorte del título (0, 1, 150 y 151 runas; solo espacios) | RN-03 |
| Validación y normalización de la descripción (2000 y 2001 runas; vacía equivale a ausente) | RN-04 |
| Validación de la prioridad y de los filtros | RN-05 |
| **Escala de Story Points**: los diez valores válidos y 4, 6, 7, 50, 101, un negativo, un decimal y un exponente | RN-06 |
| **0 frente a "sin estimar"** en validación, persistencia y respuesta | RN-07 |
| Validación de cada criterio (0, 1, 500 y 501 runas) y conservación del orden | RN-08 |
| Toda historia nace `pending` | RN-09 |
| El backlog nunca escribe `status`: `PATCH` con `status` rechazado y `Update` sin esa columna | RN-10, RN-11 |
| `Readiness`: las cinco combinaciones de la tabla de [data-model.md](./data-model.md), sección 5 | RN-12, RN-13 |
| Historia `completed`: rechazo de modificar y de reestimar | RN-14 |
| Proyecto Finalizado: rechazo de crear, modificar, estimar y eliminar | RN-15 |
| Eliminación: cada uno de los cuatro motivos por separado, combinaciones y la lista de motivos | RN-16 |
| Rango de prioridad y orden del listado, con 50 historias de las tres prioridades | RN-17 |
| Historia comprometida: rechazo de `null` y de `[]`, y aceptación de otro valor de la escala | RN-18 |

Comparación con los criterios de éxito: SC-004 con la prueba de orden de 50 historias; SC-005 con la
prueba sin sesión por endpoint; SC-006 comparando byte a byte los `404` de ajeno e inexistente en las
seis acciones; SC-007 con `TestUS1_StoryPointsFueraDeEscala` y `TestUS4_ValorFueraDeEscala`; SC-008
con `TestUS1_LimitesDelTitulo`, `TestUS1_LimitesDelCriterio` y `TestUS5_EntradaInvalidaSinCambioParcial`;
SC-009 con las pruebas de `Readiness`; SC-010 con `TestUS4_HistoriaCompletada` y
`TestUS5_HistoriaCompletada`; SC-011 con `TestUS6_QuitadaDeUnSprint`, `TestUS6_EsfuerzoODefectos` y
`TestUS6_SesionesDeEstimacion`; SC-012 con las cuatro pruebas de proyecto Finalizado; SC-013 con
`TestUS5_EdicionSimultaneaCamposDistintos`; SC-014 con la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

0. **Prerrequisito**: las features 001 y 002 integradas en `main`. Se cumple.
1. **Fase técnica fundacional**: entidades `UserStory` y `AcceptanceCriterion`, los tipos
   `StoryPriority` y `StoryStatus`, la escala, los errores de dominio, `StoryRepository`, los cinco
   puertos, las cuatro implementaciones que informan "sin historial", `ProjectDirectory`,
   `AutoMigrate` de las dos tablas y el vaciado en `testsupport`. Es el bloque que bloquea a todas
   las historias.
2. **US1 Registrar una historia** (P1): validaciones de título, descripción, prioridad, escala y
   criterios; `Readiness`; creación transaccional; `FindDetail` con el autor, porque la respuesta de
   la creación ya es el detalle; solo lectura con `ProjectDirectory`; endpoint en el subgrupo del
   proyecto; pantalla de creación.
3. **US2 Ver el backlog** (P1): consulta de listado con orden, filtros y conteo de criterios;
   `HasStories` y `emptyReason`; validación de filtros; endpoint; pantalla de listado con filtros.
4. **US3 Consultar el detalle** (P2): `RequireStoryAccess` con su `404` único; uso de
   `SprintAssignments.EverAssigned`; endpoint plano; pantalla de detalle.
5. **US4 Estimar** (P2): `Estimate` con la escala, la regla de historia Completada y la de historia
   comprometida; endpoint; selector de la escala en el detalle.
6. **US5 Refinar** (P3): decodificación del `PATCH` con presencia de campos; `Update` con bloqueo de
   fila y columnas presentes; pruebas de concurrencia; endpoint; pantalla de edición.
7. **US6 Eliminar** (P3): consulta de los cuatro puertos, `DeletionBlockedError`, borrado
   transaccional; endpoint; botón de eliminación con confirmación.
8. **Integración con la 002**: proveedor real del resumen del backlog con su prueba de integración,
   ajuste de los ayudantes de prueba de `projects/delivery` y de `cmd/app/main.go`, etiquetas en
   `ProjectStatusCard` y enlace al backlog desde la vista del proyecto. Puede ir en cualquier
   momento después de US1.
9. **Cierre**: Swagger comparado con el contrato, `README.md`, comandos de cobertura,
   `govulncheck` y la verificación transversal de trazabilidad y de las puertas de seguridad
   (SC-005, SC-006, SC-014).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las tres concesiones
documentadas —los cuatro puertos hacia features que todavía no existen, la modificación del
proveedor del resumen de la 002 y las dos formas de ruta— son consecuencias necesarias de requisitos
de la spec (FR-018, FR-022, FR-039, FR-041, FR-047, RC-07) y no desvíos de la constitución; están
justificadas en la sección "Verificación de la Constitución" y en [research.md](./research.md).

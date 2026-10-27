# Plan de Implementación: Gestión de Defectos

**Rama**: `007-defect-tracking` | **Fecha**: 2026-10-25 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/007-defect-tracking/spec.md`

---

## Resumen

Esta feature permite que cualquier integrante registre los defectos que encuentra en el producto,
vinculados a una historia y al sprint en que se detectaron; que los siga por una máquina de cuatro
estados (Abierto, En progreso, Resuelto, Descartado) hasta resolverlos en un sprint o descartarlos;
que corrija sus datos con una ventana acotada para los vínculos; y que los consulte con filtros por
proyecto y por historia. Los defectos, con sus dos anclas de sprint, son la fuente de las métricas de
calidad de la 008, que esta feature no calcula.

El enfoque técnico tiene seis decisiones que lo definen:

1. **Un solo modelo nuevo, `defects`, más una marca en `user_stories`.** El sprint de resolución es
   una columna anulable que existe si y solo si el defecto está Resuelto. `ever_had_defects` hace
   que una historia que tuvo defectos no vuelva a ser eliminable aunque su único defecto se
   revincule a otra historia (RC-05), con el mismo patrón que `ever_committed` de la 004 y
   `ever_had_effort` de la 006.
2. **La máquina de estados es una función pura.** `PlanTransition` decide, a partir del estado
   vigente, el destino pedido y los dos sprints, si la transición se admite y cuál es el sprint de
   resolución resultante. Un único mapa de transiciones es la fuente de verdad y su prueba unitaria
   recorre las 16 combinaciones.
3. **Un único endpoint de cambio de estado.** `PUT /api/v1/defects/{id}/status` con el estado destino
   en el cuerpo cubre pasar a En progreso, resolver, reabrir y descartar, como el estado de una
   historia en la 004.
4. **Cada transición se valida contra el estado vigente.** Toda escritura sobre un defecto bloquea
   su fila con `FOR UPDATE` y relee el estado: de dos acciones simultáneas incompatibles se aplica
   una y la otra recibe `409 invalid_transition` con el estado actual (FR-018, SC-019).
5. **Un orden de bloqueos que no choca con la 003 ni con la 004.** Proyecto (`FOR SHARE`) → defecto
   (`FOR UPDATE`) → sprint (`FOR KEY SHARE`) → historia. Nunca se bloquea una historia antes que un
   sprint, que es el orden de `Close` y `AssignStories`.
6. **Esta feature cierra el puerto que la 003 dejó abierto** (`DefectRecords`) y traduce la violación
   de la clave foránea `fk_defects_story` al mismo `409` de "historia con defectos" en su `Delete`,
   como ya hace con la de `estimation_sessions`.

Esta feature **no tiene variables de entorno nuevas ni dependencias nuevas**: agrega un módulo, una
tabla, una columna, seis endpoints, tres páginas del proyecto, una sección en el detalle de la
historia y un enlace en el resumen del proyecto.

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
nueva, `defects`, y una columna nueva, `user_stories.ever_had_defects`, creadas con `AutoMigrate`

**Pruebas**: `testify` v1.12.1 y `net/http/httptest` con reloj inyectado en el backend; Vitest 5.0.3
con `@testing-library/react` 16.3.3 y jsdom 30.1.2 en el frontend

**Plataforma objetivo**: API HTTP en Linux (contenedor) y navegador moderno; desarrollo en Windows

**Tipo de proyecto**: aplicación web — backend de API y frontend separados en directorios de primer
nivel

**Objetivos de rendimiento**: la spec no fija tiempos de respuesta. Cada escritura es una transacción
con a lo sumo seis sentencias y cuatro bloqueos de fila; cada consulta, una sola lectura indexada por
proyecto o por historia, sin paginar. SC-001 a SC-003 son tiempos de tarea de la persona: registrar
en 5 pasos con el sprint ya propuesto, encontrar los Críticos abiertos con dos selecciones de filtro,
y ver los defectos de una historia desde su propio detalle

**Restricciones**: 100 % de las transiciones no permitidas rechazadas (SC-005); cero Resueltos sin
sprint de resolución (SC-006), con sprint de resolución anterior al de detección (SC-007) o anclados
a un sprint Planificado (SC-008); la historia y el sprint de detección nunca cambian por una
transición (SC-010); vínculos corregibles solo en Abierto (SC-011); cero Descartados con sprint de
resolución o con cambios posteriores (SC-012); orden estable (SC-014); cero escrituras en proyectos
Finalizados (SC-015); sesión y membresía en el 100 % de las acciones (SC-016); `404` indistinguible
(SC-017); exactamente una de dos acciones concurrentes incompatibles, en 10 intentos de 10 (SC-019)

**Escala y alcance**: 6 endpoints HTTP, 1 entidad persistida nueva más 1 columna en una existente, 1
puerto ajeno reemplazado, 1 traducción de clave foránea en la 003, 3 páginas nuevas del proyecto, 1
sección nueva en el detalle de la historia, 6 historias de usuario con **73** escenarios de
aceptación y 24 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las decisiones de producto se cerraron en la sesión de
clarificación del 2026-10-03. Las incógnitas **técnicas** se resolvieron en
[research.md](./research.md): forma de las rutas y del endpoint de estado, acceso a un defecto y
`404`, modelo y marca permanente, atribución sin historial, valores de dominio y orden, validación de
entrada, respuesta ante referencias ajenas en el cuerpo, máquina de estados, concurrencia, orden de
bloqueos, ventana de corrección, consultas y filtros, integración con la 003 y la 004, sprint
propuesto por defecto, frontend y códigos HTTP.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **La marca `ever_had_defects` no estaba prevista por la 003**: su puerto `DefectRecords` decía
   "indica si la historia tiene defectos asociados", lo que sugería un `EXISTS`. Como un defecto puede
   revincularse a otra historia (US4-2), el `EXISTS` dejaría eliminable una historia que tuvo
   defectos, en contra de RC-05. El plan agrega la columna (research, sección 4).
2. **Esta feature toca código de la 003**: reemplaza el cuerpo de
   `backlog/repository/defect_records.go` (su constructor pasa a recibir la conexión), agrega
   `EverHadDefects` a `UserStory` y amplía `StoryRepository.Delete` para traducir la violación de
   `fk_defects_story`. En el frontend, suma una sección al detalle de la historia y un enlace en el
   resumen del proyecto.
3. **Conflicto previsible con la rama de la 006**: las dos features agregan líneas en
   `backlog/domain/user_story.go`, `platform/database/postgres.go`,
   `platform/testsupport/database.go`, `cmd/app/main.go`, el detalle de la historia, el resumen del
   proyecto y el `README.md`. Los cambios son aditivos; quien integre la segunda rama resuelve el
   orden de las líneas.
4. **Un vínculo reenviado con el mismo valor se acepta fuera de Abierto**: la spec dice que la
   corrección se rechaza fuera de la ventana; el plan interpreta "corrección" como cambio de valor,
   para que un formulario que reenvía todos los campos no falle por los que no tocó (research,
   sección 11). Si el equipo prefiere rechazar la mera presencia del campo, es un cambio de una línea
   en `validation.go` y de los escenarios de prueba US4-12.
5. **"Abiertos" en la 008**: la 008 cuenta defectos "abiertos" del proyecto. Esta feature deja los
   datos para que esa cuenta sea `open` + `in_progress` (los no resueltos ni descartados), pero la
   definición es de la 008 y no se fija acá.
6. **Rama de trabajo**: el plan se generó en la rama `repo/007-plan`, y `.specify/feature.json`
   (ignorado por Git) ahora apunta a `specs/007-defect-tracking`. La spec indica la rama
   `007-defect-tracking` para la implementación.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I — Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Los 6 endpoints se corresponden con las 6 historias (US1 registrar; US2, US5 y US6 cambiar de estado por un único endpoint; US3 listar por proyecto, por historia y ver el detalle; US4 modificar). Sin asignaciones, adjuntos, comentarios, prioridad aparte, notificaciones, agregados de calidad, gráficos, búsqueda por texto, paginación, relaciones entre defectos, historial ni esfuerzo sobre defectos: todo eso está en "Fuera de Alcance". |
| II — TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 73 escenarios son pruebas nombradas; el entorno de test existe desde la 001. La máquina de estados, los sprints admitidos, el orden entre sprints, la ventana de corrección, la solo lectura del descartado y las validaciones son pruebas unitarias obligatorias. |
| III — Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Un módulo `internal/defects/` con sus cuatro capas; el frontend usa las capas existentes. |
| IV — Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA**. Ninguna dependencia nueva. |
| V — Contenerización | ¿Dev y test son disjuntos? | **PASA**. Se reutilizan los contenedores de la 001; esta feature no agrega servicios. |
| VI — Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. No hay configuración nueva. Los nombres se leen de `users` sin `password_hash`. |
| VII — Robustez y validación | ¿Toda entrada se valida antes del servicio? | **PASA**. Cuerpo, campos prohibidos, parámetros de ruta y filtros se validan en `delivery`; las reglas que dependen de datos, dentro de la transacción. |
| VIII — Estándares y documentación viva | ¿Swagger, README e idiomas? | **PASA**. Identificadores y valores de dominio en inglés; comentarios, mensajes y Swagger en español. README actualizado en el mismo cambio. |
| IX — Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. Sin violaciones que justificar.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Una tabla con las columnas de la entidad de la spec más la atribución que exigen FR-022 y FR-056, y ninguna derivable: `project_id`, rango de severidad y "matcheado por" se derivan (data-model, secciones 1 y 5; research, sección 12). Una columna en `user_stories` porque RC-05 la exige. Sin historial de estados, sin `CHECK` duplicando la regla, sin paginación ni endpoint de "sprint propuesto". Un bloqueo de fila por defecto en lugar de versiones optimistas. `DefectRepository` declara 8 operaciones, todas usadas. | **PASA** |
| II | Los 73 escenarios se mapean a pruebas nombradas (tabla más abajo). Las pruebas unitarias obligatorias cubren `PlanTransition` con las 16 combinaciones, `AllowedTransitions`, `EnsureSprintUsableForDetection`, `EnsureDefectEditable`, `EnsureLinksCorrectable`, `SeverityRank`, la validación de descripción, severidad, estado, campos prohibidos y filtros, y las reglas RN-01 a RN-24. | **PASA** |
| III | `defects/domain` importa solo `backlog/domain`, `sprints/domain` (estados) y `auth/domain`. `defects/delivery` no importa `backlog/delivery` ni `projects/delivery`: recibe `RequireStoryAccess` y el subgrupo de proyecto desde `cmd/app/`. El puerto de la 003 lee su propia columna sin importar `defects`; la traducción de la clave foránea compara el nombre de la restricción, sin importar `defects`. No hay ciclos. | **PASA** |
| IV | Ninguna dependencia nueva de Go ni de npm. Bloqueos con `gorm.io/gorm/clause`. | **PASA** |
| V | Sin servicios nuevos. La tabla y la columna se crean con `AutoMigrate` en las mismas bases disjuntas; el ayudante de pruebas vacía `defects`. | **PASA** |
| VI | Los nombres seleccionan solo `users.id` y `users.full_name`. Ninguna respuesta revela datos de proyectos ajenos: `404` único con los cuerpos de la 002, la 003 y la 004; filtro ajeno con el mismo mensaje que el inexistente. El cuerpo sigue limitado a 1 MiB. | **PASA** |
| VII | Los seis endpoints validan antes del servicio. Errores de dominio traducidos a `400`, `404` y `409` con `code`. Toda escritura es transaccional y atómica. El orden de bloqueos evita interbloqueos con la 003 y la 004; la violación de clave foránea al borrar una historia se traduce a `409`. | **PASA** |
| VIII | El contrato OpenAPI está en español. Valores `read_only`/`no_detection_sprint`/`invalid_transition`/`defect_discarded`/`links_frozen` y `no_defects`/`no_matches` en la API, con etiquetas en español en el frontend. `any` prohibido. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Tres decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **Esta feature escribe una columna de la 003** (`user_stories.ever_had_defects`). Es la
   consecuencia mínima de RC-05 frente a la revinculación (US4-2) y repite el precedente de
   `ever_committed` y `ever_had_effort` (research, sección 4).
2. **Esta feature modifica código de la 003**: el puerto que esa feature dejó anotado para esta y la
   traducción de la clave foránea en `Delete`, con el mismo patrón que la 005 (research, sección 13).
3. **Una ruta de esta feature cuelga de `/api/v1/stories/:storyId`**, el prefijo de la 003, y dos del
   subgrupo de proyecto de la 002. Es lo que permite responder con los mismos `404` de historia y de
   proyecto (research, sección 2).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/007-defect-tracking/
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
features 001 a 006 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/
│   └── app/
│       └── main.go                          # TOCA: compone el módulo defects, pasa RequireStoryAccess y el subgrupo de proyecto y el reloj; pasa db al puerto DefectRecords de la 003
├── internal/
│   ├── defects/                             # NUEVO: módulo único de esta feature
│   │   ├── domain/
│   │   │   ├── defect.go                    # Entidad Defect
│   │   │   ├── severity.go                  # DefectSeverity, Valid, SeverityRank
│   │   │   ├── status.go                    # DefectStatus, mapa de transiciones, AllowedTransitions, PlanTransition
│   │   │   ├── rules.go                     # EnsureSprintUsableForDetection, EnsureDefectEditable, EnsureLinksCorrectable
│   │   │   ├── errors.go                    # Errores de dominio, InvalidTransitionError, InvalidFilterError
│   │   │   └── repository.go                # DefectRepository y tipos de lectura
│   │   ├── repository/
│   │   │   └── defect_repository.go         # GORM; transacciones, orden de bloqueos, marca de la historia, filtros y orden
│   │   ├── service/
│   │   │   └── defect_service.go            # Create, ChangeStatus, Update, Get, ListForProject, ListForStory
│   │   └── delivery/
│   │       ├── handler.go                   # 6 manejadores con anotaciones swag en español
│   │       ├── dto.go                       # Peticiones, respuestas y DefectConflictResponse
│   │       ├── validation.go                # Descripción, severidad, estado, campos prohibidos, filtros
│   │       ├── middleware.go                # RequireDefectAccess (404 único)
│   │       └── routes.go                    # Rutas sobre el proyecto, sobre la historia y grupo plano del defecto
│   ├── backlog/
│   │   ├── domain/
│   │   │   └── user_story.go                # TOCA: campo EverHadDefects (columna ever_had_defects)
│   │   └── repository/
│   │       ├── defect_records.go            # TOCA: lee ever_had_defects en lugar de informar "sin defectos"
│   │       └── story_repository.go          # TOCA: Delete traduce la violación de fk_defects_story a DeletionBlockedError{defects}
│   └── platform/
│       ├── database/
│       │   └── postgres.go                  # TOCA: AutoMigrate de Defect, después de SprintStory
│       └── testsupport/
│           └── database.go                  # TOCA: vacía defects
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
│   │           ├── backlog/
│   │           │   └── [storyId]/
│   │           │       └── page.tsx         # TOCA: sección "Defectos" (US3) con enlace a registrar
│   │           └── defects/
│   │               ├── page.tsx             # NUEVO: listado del proyecto con filtros (US3, US6)
│   │               ├── new/
│   │               │   └── page.tsx         # NUEVO: registro con sprint propuesto (US1)
│   │               └── [defectId]/
│   │                   └── page.tsx         # NUEVO: detalle, modificación y acciones de estado (US2, US4, US5, US6)
│   ├── features/
│   │   ├── defects/                         # NUEVO
│   │   │   ├── components/                  # DefectForm, DefectList, DefectRow, DefectFilters,
│   │   │   │                                # DefectDetail, DefectEditForm, DefectStatusActions,
│   │   │   │                                # ResolveDefectForm, StoryDefectsSection
│   │   │   ├── api.ts                       # Llamadas a los 6 endpoints y al listado de sprints de la 004
│   │   │   ├── transitions.ts               # allowedTransitions: copia del mapa de la máquina de estados
│   │   │   └── validation.ts                # Reglas de forma de descripción y severidad
│   │   └── projects/
│   │       └── components/
│   │           └── project-overview.tsx     # TOCA: enlace a "Defectos"
│   ├── types/
│   │   └── defect.ts                        # NUEVO: tipos de peticiones y respuestas; sin `any`
│   └── utils/
│       └── defect-labels.ts                 # NUEVO: etiquetas de severidad, estado, emptyReason y códigos de conflicto
├── Dockerfile                               # Sin cambios
└── package.json                             # Sin cambios: ninguna dependencia nueva

README.md                                    # TOCA: describe la gestión de defectos y sus endpoints
```

**Pruebas**: en el backend, junto al código del paquete que prueban (`status_test.go`,
`severity_test.go`, `rules_test.go`, `validation_test.go`, `defect_service_test.go`,
`handler_test.go`, `middleware_test.go`, `defect_concurrency_test.go`), más las pruebas de
integración del puerto reemplazado y de la traducción de la clave foránea en `backlog/repository`. En
el frontend, junto al componente o módulo (`validation.test.ts`, `transitions.test.ts`,
`defect-labels.test.ts`, `defect-form.test.tsx`, `defect-filters.test.tsx`,
`defect-status-actions.test.tsx`, `defect-list.test.tsx`, `story-defects-section.test.tsx`,
`api.test.ts`). No se crea un árbol `tests/` separado.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical,
`internal/defects/`, con sus cuatro capas y las dependencias apuntando hacia `domain/`. El frontend no
agrega capas: las páginas de defectos son hermanas de `backlog/` y `sprints/` bajo el proyecto, y la
sección vive en el detalle de la historia porque es donde se descubre el problema. Antes de escribir
código de Next.js se lee la documentación de la versión instalada en
`frontend/node_modules/next/dist/docs/` (CLAUDE.md).

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `007/US1` a `007/US6` en el código de pruebas devuelve
la trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en el paquete del módulo, precedida por el
comentario `// Escenario: <nombre del escenario en spec.md> (007/USn)`. Las variantes de un mismo
escenario se agrupan como subtests con `t.Run` (por ejemplo, US1-6 con 2000 y 2001 caracteres; US1-9
con vacía y solo espacios; US1-11 con historia y sprint faltantes; US6-8 con las cuatro acciones
sobre un descartado).

**Frontend (Vitest)**: `describe("007/USn - <título de la historia>")` con un
`it("<nombre del escenario en spec.md>")` por escenario que tenga comportamiento visible en la
interfaz (sprint propuesto, formulario y sus mensajes, botones de estado según el estado vigente,
selector de sprint de resolución, campos de vínculo solo en Abierto, detalle de solo lectura del
descartado, filtros y lista vacía).

| Historia | Escenarios | Pruebas de backend previstas |
| --- | --- | --- |
| US1 Registrar un defecto | 16 | `TestUS1_RegistroEnSprintActivo`, `TestUS1_SprintActivoPorDefecto`, `TestUS1_HistoriaCompletada`, `TestUS1_SinSprintActivo`, `TestUS1_VariosDefectosPorHistoria`, `TestUS1_DescripcionDeDosMilCaracteres`, `TestUS1_CuatroSeveridades`, `TestUS1_HistoriaConCincuentaDefectos`, `TestUS1_DescripcionVacia`, `TestUS1_SeveridadInvalida`, `TestUS1_CamposObligatorios`, `TestUS1_SprintPlanificado`, `TestUS1_VinculoDeOtroProyecto`, `TestUS1_ProyectoFinalizado`, `TestUS1_NoIntegrante`, `TestUS1_ProyectoSinSprintIniciado` |
| US2 Seguir y resolver un defecto | 13 | `TestUS2_CicloCompleto`, `TestUS2_MismoSprint`, `TestUS2_SprintPosterior`, `TestUS2_OtroIntegrante`, `TestUS2_ResolucionEnSprintCerrado`, `TestUS2_UnicoSprint`, `TestUS2_ResolucionSinSprint`, `TestUS2_ResolucionAnteriorALaDeteccion`, `TestUS2_ResolucionEnSprintPlanificado`, `TestUS2_SaltoAResuelto`, `TestUS2_VueltaAAbierto`, `TestUS2_SprintDeOtroProyecto`, `TestUS2_ProyectoFinalizado` |
| US3 Consultar y filtrar los defectos | 12 | `TestUS3_SinFiltros`, `TestUS3_DefectosDeUnaHistoria`, `TestUS3_FiltroPorEstado`, `TestUS3_SeveridadYEstado`, `TestUS3_FiltroPorSprint`, `TestUS3_ProyectoSinDefectos`, `TestUS3_HistoriaSinDefectos`, `TestUS3_SinCoincidencias`, `TestUS3_OrdenEstable`, `TestUS3_FiltroDeOtroProyecto`, `TestUS3_ValorDeFiltroInvalido`, `TestUS3_NoIntegrante` |
| US4 Corregir los datos de un defecto | 16 | `TestUS4_DescripcionYSeveridad`, `TestUS4_RevinculacionDeHistoria`, `TestUS4_DescripcionDeResuelto`, `TestUS4_SeveridadEnProgreso`, `TestUS4_CorreccionDeSprintDeDeteccion`, `TestUS4_AmbosVinculos`, `TestUS4_SinCambios`, `TestUS4_DescripcionDeDosMilCaracteres`, `TestUS4_VentanaReabierta`, `TestUS4_DatosInvalidos`, `TestUS4_HistoriaDeOtroProyecto`, `TestUS4_VinculosCongelados`, `TestUS4_SprintDeDeteccionInvalido`, `TestUS4_DefectoDescartado`, `TestUS4_EstadoDesdeModificacion`, `TestUS4_ProyectoFinalizado` |
| US5 Reabrir un defecto resuelto | 7 | `TestUS5_Reapertura`, `TestUS5_SegundaResolucion`, `TestUS5_VariasReaperturas`, `TestUS5_ResolucionEnElMismoSprint`, `TestUS5_SinSprintActivo`, `TestUS5_DefectoNoResuelto`, `TestUS5_ProyectoFinalizado` |
| US6 Descartar un defecto que no corresponde | 9 | `TestUS6_DescarteDesdeAbierto`, `TestUS6_DescarteDesdeEnProgreso`, `TestUS6_ListadoConDescartados`, `TestUS6_FiltroPorDescartado`, `TestUS6_HistoriaNoEliminable`, `TestUS6_DetalleDeDescartado`, `TestUS6_DescarteDeResuelto`, `TestUS6_DescartadoDefinitivo`, `TestUS6_ProyectoFinalizado` |

**Total: 73 escenarios.**

**Cómo se preparan los escenarios que dependen de otras features**:

- Las historias se crean con los endpoints de la 003; los sprints se crean, se comprometen, se inician
  y se cierran con los de la 004, que el router de prueba compone junto con el módulo de defectos.
  Así US1-3 (historia Completada), US1-4 y US5-5 (sin sprint Activo), US1-16 (sin sprints iniciados),
  US2-3, US2-5 y US2-8 (orden entre sprints) ejercitan la integración real.
- US6-5 elimina la historia con el endpoint de la 003 y verifica el `409` con el motivo `defects`; una
  variante revincula el único defecto a otra historia antes de intentar eliminar la original (RC-05).
- El proyecto Finalizado (US1-14, US2-13, US4-16, US5-7, US6-9) y la baja de integrantes (FR-055,
  SC-018) se preparan con los endpoints de la 002.

**Pruebas transversales sin etiqueta de escenario**: la prueba sin sesión de cada uno de los seis
endpoints (SC-016), la comparación byte a byte de los `404` de proyecto, de historia, de sprint y de
defecto, tanto en la ruta como en el cuerpo y en los filtros (SC-017), las de concurrencia,
con 10 intentos cada una (`TestDefectTransition_ConcurrentStart` en US2: dos pasos a En progreso
sobre un defecto Abierto; `TestDefectTransition_Concurrency` en US6, SC-019: resolución y descarte
simultáneos sobre un defecto En progreso; y `TestDefectTransition_ConcurrentUpdateAndDiscard` en US6:
una modificación junto a un descarte), la de vínculos inmutables a lo largo del ciclo completo (`TestDefectLinks_Immutable`,
SC-010), la de la carrera entre `HasDefects` y el borrado de la historia
(`TestStoryDelete_DefectRace`, en `backlog/repository`) y la de autoría tras quitar a un integrante
(`TestDefectAuthor_RemovedMember`, SC-018) se nombran sin `007/USn`, porque no derivan de un
escenario de la spec y no deben alterar el conteo de 73.

Además, pruebas unitarias **obligatorias** por el Principio II (validaciones, reglas de negocio y
máquina de estados), que cubren SC-020:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Alta y cambios de estado por un integrante que no es el autor; autoría de cada escritura igual a la sesión | RN-01 |
| Las cinco escrituras en un proyecto Finalizado; las tres lecturas permitidas | RN-02 |
| Validación de descripción: vacía, solo espacios, 1, 2000 y 2001 runas, recorte | RN-03 |
| `DefectSeverity.Valid` y `SeverityRank` con los cuatro valores y uno inexistente | RN-04 |
| Alta: estado `open` y `resolved_sprint_id` nulo con las cuatro severidades | RN-05 |
| Alta y revinculación con historia ausente, de otro proyecto e inexistente | RN-06 |
| Alta sobre historias `pending`, `in_progress` y `completed` | RN-07 |
| `EnsureSprintUsableForDetection` con `planned`, `active` y `closed`; sprint de otro proyecto | RN-08 |
| Propuesta del sprint en el formulario: con Activo, solo Cerrados, ninguno (Vitest) | RN-09 |
| Alta en un proyecto sin sprints iniciados, con uno `planned` y con uno inexistente en el cuerpo | RN-10 |
| `DefectStatus.Valid` con los cuatro valores y uno inexistente | RN-11 |
| `PlanTransition` con las 16 combinaciones de origen y destino | RN-12 |
| `PlanTransition` a `resolved` sin sprint, con `planned` y con un sprint de otro proyecto | RN-13 |
| `PlanTransition` con resolución anterior, igual y posterior a la detección | RN-14 |
| `PlanTransition` de `resolved` a `open`: sprint de resolución nulo | RN-15 |
| `EnsureLinksCorrectable` con los cuatro estados; transiciones que no cambian los vínculos | RN-16 |
| Dos reaperturas y resoluciones sucesivas: vale la última | RN-17 |
| Modificación que no toca `status`, `resolved_sprint_id` ni `created_by_id` | RN-18 |
| `DefectRecords.HasDefects` tras descartar y tras revincular el único defecto | RN-19 |
| Listados y detalle con un autor quitado del proyecto | RN-20 |
| `PlanTransition` a `discarded` desde los cuatro estados | RN-21 |
| `EnsureDefectEditable` con los cuatro estados; listado sin filtros con un descartado | RN-22 |
| `PlanTransition` a `discarded`: sprint de resolución nulo | RN-23 |
| `TestDefectTransition_ConcurrentStart`, `TestDefectTransition_Concurrency` y `TestDefectTransition_ConcurrentUpdateAndDiscard` | RN-24 |
| Validación de filtros: estado y severidad inexistentes, UUID mal formado, historia y sprint ajenos | FR-046, FR-047 |
| Validación de campos prohibidos en el `PATCH` y de `resolvedSprintId` fuera de la resolución | FR-035, FR-023 |

Comparación con los criterios de éxito: SC-001 con la prueba de componentes de `DefectForm` (sprint
propuesto y cinco controles); SC-002 con `TestUS3_SeveridadYEstado` y la de `DefectFilters`; SC-003
con la de `StoryDefectsSection`; SC-004 con `TestUS1_CuatroSeveridades`; SC-005 con `PlanTransition`
y sus 16 combinaciones; SC-006 con `TestUS2_ResolucionSinSprint`; SC-007 con
`TestUS2_ResolucionAnteriorALaDeteccion` y `TestUS2_MismoSprint`; SC-008 con
`TestUS1_SprintPlanificado` y `TestUS2_ResolucionEnSprintPlanificado`; SC-009 con
`TestUS5_Reapertura`; SC-010 con `TestDefectLinks_Immutable`; SC-011 con `EnsureLinksCorrectable`,
`TestUS4_VinculosCongelados` y `TestUS4_AmbosVinculos`; SC-012 con `TestUS6_DescartadoDefinitivo` y
`TestUS4_DefectoDescartado`; SC-013 con US3; SC-014 con `TestUS3_OrdenEstable`; SC-015 con las
pruebas `_ProyectoFinalizado` de cada historia; SC-016 y SC-017 con las pruebas transversales;
SC-018 con `TestDefectAuthor_RemovedMember`; SC-019 con `TestDefectTransition_Concurrency`; SC-020 con
la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

1. **Fase técnica fundacional**: la entidad `Defect`, los tipos `DefectSeverity` y `DefectStatus`, la
   columna `EverHadDefects`, los errores de dominio, `DefectRepository` (firmas), `AutoMigrate`, el
   vaciado en `testsupport`, `RequireDefectAccess`, `DefectConflictResponse` y el esqueleto de rutas
   en `cmd/app/`. En el frontend, los tipos, `api.ts` y `defect-labels.ts`. Es el bloque que bloquea a
   todas las historias.
2. **US1 Registrar** (P1): validación de descripción, severidad y referencias;
   `EnsureSprintUsableForDetection`; alta con el orden de bloqueos y la marca de la historia;
   reemplazo del puerto de la 003 y traducción de `fk_defects_story`; formulario con sprint propuesto.
3. **US2 Seguir y resolver** (P1): `AllowedTransitions` y `PlanTransition`; cambio de estado con
   bloqueo y relectura; prueba de concurrencia; acciones de estado y selector de sprint de resolución
   en el detalle.
4. **US3 Consultar y filtrar** (P2): validación de filtros y su pertenencia al proyecto; listado del
   proyecto y de la historia con `emptyReason`; detalle; página de defectos con filtros, sección en la
   historia y enlace desde el resumen.
5. **US4 Corregir** (P2): validación del `PATCH` y de los campos prohibidos; `EnsureDefectEditable` y
   `EnsureLinksCorrectable`; modificación con revinculación y marca de la historia nueva; formulario
   de edición en el detalle.
6. **US5 Reabrir** (P3): reapertura sobre `PlanTransition` y su botón en el detalle.
7. **US6 Descartar** (P3): descarte sobre `PlanTransition`, detalle de solo lectura y confirmación en
   la interfaz.
8. **Cierre**: Swagger comparado con el contrato, `README.md`, comandos de cobertura, `govulncheck` y
   la verificación transversal de trazabilidad y de las puertas de seguridad (SC-016, SC-017,
   SC-020).

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. Las tres concesiones
documentadas —la columna escrita en una tabla de la 003, la modificación puntual del puerto y del
`Delete` de la 003, y las rutas bajo los prefijos de historia y de proyecto— son consecuencias
necesarias de requisitos de la spec (RC-05, RN-19, FR-053) y no desvíos de la constitución; están
justificadas en la sección "Verificación de la Constitución" y en [research.md](./research.md).

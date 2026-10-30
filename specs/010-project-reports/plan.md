# Plan de Implementación: Reportes de Proyecto y de Sprint

**Rama**: `010-project-reports` | **Fecha**: 2026-10-30 | **Spec**: [spec.md](./spec.md)

**Entrada**: Especificación de feature en `specs/010-project-reports/spec.md`

---

## Resumen

El reporte junta en un documento lo que vive en seis features: historias, estimaciones, esfuerzo,
métricas y defectos de un sprint o del proyecto completo. Se revisa en pantalla y se exporta a PDF
con el mismo contenido. Tiene dos reglas centrales: **no calcula** (las cifras son de la 008) y **el
PDF dice exactamente lo mismo que la pantalla**.

El enfoque técnico tiene siete decisiones que lo definen:

1. **Un módulo `reports` en el backend con dos `POST`**, uno por alcance. Arma el reporte completo
   en una respuesta y escribe el registro de generación, que es lo único que persiste (research,
   secciones 1, 2 y 10).
2. **Las métricas se piden a `MetricsService` dentro del proceso** y viajan con la forma JSON exacta
   de la 008. Para eso se exportan sus constructores de DTO (research, sección 3).
3. **Un sprint Cerrado sale entero de su instantánea**, con un corte en el `closed_at` para las
   sesiones de Planning Poker. Así dos reportes del mismo sprint Cerrado son idénticos (research,
   secciones 4 y 5).
4. **Cada sección se lee por separado** y lleva su estado (`complete`, `empty` o `unavailable`), de
   modo que una falla no se lleve el resto (research, sección 8).
5. **El PDF se arma en el navegador desde la misma respuesta**, con un único modelo de presentación
   (`ReportView`) que consumen la pantalla y el PDF. La fidelidad queda garantizada por construcción
   (research, sección 13).
6. **pdfmake 0.3.13**, autorizada por el desarrollador el 2026-10-30 según el Principio IV: tablas que
   repiten su encabezado, pie "Página N de M" y Roboto embebida (research, sección 14).
7. **Nombre de archivo saneado y legible** con proyecto, alcance, fecha y hora hasta el segundo
   (research, sección 15).

Esta feature agrega **2 endpoints, 1 tabla, 1 pantalla, 1 módulo de backend, 1 módulo de frontend y
1 dependencia del frontend**. No agrega variables de entorno ni servicios.

Las decisiones y las alternativas descartadas están en [research.md](./research.md).

---

## Contexto Técnico

**Lenguaje y versión**: Go 1.27.3 (backend) y TypeScript 5.9.3 sobre Next.js 16.4.2 y React 19.3.0
(frontend)

**Dependencias principales**: Gin 1.12.0, GORM 1.31.2 con `gorm.io/driver/postgres` 1.6.3,
`google/uuid` 1.6.0 y swag 1.16.6, todas ya en `go.mod`. **Nueva**: `pdfmake` `0.3.13` en el
frontend, fijada sin rango y autorizada el 2026-10-30 ([research.md](./research.md), sección 14). Sus
tipos se declaran localmente en `types/pdfmake.d.ts`, sin `@types/pdfmake`

**Almacenamiento**: PostgreSQL, con la tabla nueva `report_generations` creada por `AutoMigrate`
([data-model.md](./data-model.md), sección 1). El documento no se guarda en ningún lado, tampoco en
el navegador

**Pruebas**: `go test` con testify contra la base de test (`smye-test`, puerto 5433) para el backend,
y Vitest 5.0.3 con jsdom 30.1.2 y Testing Library 16.3.3, con `fetch` sustituido por un backend de
prueba según el contrato y `export-pdf` sustituido con `vi.mock`, para el frontend

**Plataforma objetivo**: backend y frontend en contenedores; el PDF se genera en el navegador del
integrante; desarrollo en Windows

**Tipo de proyecto**: aplicación web, con backend de API y frontend separados en directorios de
primer nivel

**Objetivos de rendimiento**: SC-001 pide el reporte en pantalla en menos de 30 segundos y en 3
pasos. La generación hace un número fijo de lecturas por sección (no una por historia): 8 consultas
del repositorio más las de la 008. pdfmake se carga de forma diferida, solo al exportar

**Restricciones**: cero cifras distintas de la 008 (SC-004); cero "no calculable" como 0 (SC-005);
100 % de los reportes con Activo marcados como parciales en pantalla y en el PDF (SC-006); cero
reportes de Planificados (SC-007); seis secciones siempre (SC-008); `empty` distinguible de
`unavailable` (SC-009); 80 historias paginadas sin pérdida (SC-010); acentos y eñes (SC-011); nombre
válido y distinguible (SC-012); falla de PDF que conserva la pantalla (SC-013); sesión y membresía
(SC-014, SC-015); defectos que cuadran (SC-016); reproducibilidad del Cerrado (SC-017); 100 % de las
generaciones registradas (SC-018)

**Escala y alcance**: 2 endpoints, 1 entidad persistida, 1 pantalla, 6 secciones, 4 historias de
usuario con **35** escenarios de aceptación y 25 reglas de negocio

### Incógnitas resueltas en la Fase 0

No queda ningún `NEEDS CLARIFICATION`. Las tres decisiones de producto se cerraron en la sesión de
clarificación del 2026-10-04, y la librería de PDF la decidió el desarrollador el 2026-10-30. Las
incógnitas técnicas se resolvieron en [research.md](./research.md): ubicación del módulo, forma de los
endpoints, reutilización de las métricas, fuentes y orden de las historias, corte de las
estimaciones, anclaje del esfuerzo, listado y totales de defectos, estados de sección, parcialidad y
período, registro de generación, proyecto Finalizado, pantalla, fidelidad, PDF, nombre de archivo,
falla de exportación, reutilización de la 009 y pruebas.

### Puntos señalados al desarrollador (no bloquean el diseño)

1. **Dependencia autorizada.** `pdfmake@0.3.13` queda autorizada por la decisión del 2026-10-30.
   Conviene sumarla a la tabla de dependencias autorizadas de la constitución con una enmienda PATCH,
   como se propuso para `recharts` en la 009.
2. **Corrección sobre los tipos.** Al autorizar se indicó que pdfmake traía sus propios tipos de
   TypeScript, y no es así: el paquete no trae declaraciones. El plan **no** suma `@types/pdfmake`:
   declara localmente solo lo que usa (research, sección 14). Si se prefieren los tipos oficiales,
   `@types/pdfmake@0.3.3` necesita su propia autorización.
3. **Prerrequisito: la 009 integrada.** El frontend reutiliza `types/metrics.ts`, `formatDecimal`,
   `formatShare` y `explainNotCalculable` de la 009, que hoy tiene plan y tareas pero no código
   (research, sección 17). El backend de la 010 no depende de la 009 y puede empezar antes.
4. **Interpretaciones y precisiones de la spec.**
   - **Incorporadas a la spec el 2026-10-30** (Clarifications, sesión del 2026-10-30), a partir de
     `/speckit-analyze`: las historias comprometidas solo en sprints Planificados van al grupo de no
     planificadas (FR-036, research sección 4); el encabezado no admite el estado "no obtenible" y
     su falla hace fallar la generación sin registro (FR-024, FR-025, research sección 8); la
     membresía se verifica al generar, no al exportar (US2-8, FR-038, SC-014); y solo un proyecto sin
     sprints ni historias tiene sus cinco secciones de contenido sin datos (US3-10, US4-4, SC-008).
   - **Pendientes de llevar a la spec** con `/speckit-clarify` o a mano: el listado de defectos suma
     la descripción (FR-018, research sección 7), y en un sprint Cerrado solo cuentan las sesiones de
     Planning Poker hasta el cierre (research sección 5).
5. **Toca código de otras features, sin cambiarles el comportamiento**: `metrics/delivery` exporta
   `NewSprintMetrics` y `NewProjectMetrics`; `platform/database` migra la tabla nueva;
   `platform/testsupport` la vacía; `cmd/app/main.go` compone el módulo; `ProjectOverview` suma el
   enlace "Reportes". Las pruebas actuales siguen pasando.
6. **Rama de trabajo**: el plan se generó en `repo/010-plan`, y `.specify/feature.json` (ignorado por Git)
   ahora apunta a `specs/010-project-reports`. La spec indica la rama `010-project-reports` para la
   implementación.

---

## Verificación de la Constitución

*PUERTA: debe pasar antes de la investigación de la Fase 0. Reevaluada después del diseño de la
Fase 1.*

### Primera evaluación (antes de la Fase 0)

| Principio | Puerta | Resultado |
| --- | --- | --- |
| I. Simplicidad deliberada | ¿El alcance se limita a lo pedido? | **PASA**. Seis secciones fijas, solo PDF, sin envío por correo, programación, plantillas, comparación entre proyectos, gráficos, firma, edición ni lectura del historial de generaciones, todos en "Fuera de Alcance". |
| II. TDD | ¿Hay forma de escribir la prueba antes del código? | **PASA**. Los 35 escenarios son pruebas de integración en Go o de Vitest; las reglas de negocio y las validaciones tienen pruebas unitarias. |
| III. Arquitectura por capas | ¿La estructura cabe en el layout obligatorio? | **PASA**. Módulo `backend/internal/reports/` con sus cuatro capas; frontend en `app/`, `features/reports/`, `types/` y `utils/`. |
| IV. Stack fijo y versiones estables | ¿Hace falta algo fuera de la tabla? | **PASA CON AUTORIZACIÓN**. La librería de PDF no está en la tabla, pero el Principio IV prevé autorizarla al planificar la 010. El desarrollador autorizó `pdfmake@0.3.13`, estable, el 2026-10-30. |
| V. Contenerización | ¿Dev y test son disjuntos? | **PASA**. Sin servicios nuevos; la tabla nueva se crea en las dos bases con `AutoMigrate`. |
| VI. Seguridad y secretos | ¿Hay secretos fuera de `.env`? | **PASA**. Sin configuración nueva. El PDF no lleva nada que un integrante no vea ya (RC-11). |
| VII. Robustez y validación | ¿Toda entrada se valida? | **PASA**. Los identificadores de ruta se validan antes del servicio; cada falla de lectura se traduce a un estado de sección o a un `500` sin detalles; la respuesta se reconoce en el frontend antes de mostrarse. |
| VIII. Estándares y documentación viva | ¿Idiomas, `any`, Swagger, README? | **PASA**. Identificadores en inglés y textos en español; sin `any`, también en la declaración local de pdfmake. Swagger y README se actualizan en el mismo cambio. |
| IX. Gobierno del repositorio | ¿El plan pide operaciones prohibidas? | **PASA**. El plan no ejecuta ninguna escritura de Git. |

**Resultado: todas las puertas pasan. La única dependencia nueva cuenta con la autorización que
exige el Principio IV.**

### Reevaluación (después del diseño de la Fase 1)

| Principio | Qué se revisó en el diseño | Resultado |
| --- | --- | --- |
| I | Dos rutas sin cuerpo en lugar de una con discriminador. Sin endpoint de lectura de registros y sin copia del nombre del autor. Sin `@types/pdfmake`. Se reutilizan `MetricsService` y sus DTO, los formateadores de la 009 y las etiquetas de la 003, la 004 y la 007 (DRY). Las interfaces del dominio declaran solo las lecturas que se usan. | **PASA** |
| II | Los 35 escenarios se mapean a pruebas nombradas (tabla más abajo). Cada regla RN-01 a RN-25 tiene al menos una prueba que falla si se rompe. | **PASA** |
| III | `reports/domain` importa los dominios de `metrics`, `sprints`, `backlog`, `defects` y `poker`, como ya hace `metrics/domain`. `reports/delivery` importa `auth/delivery` y `metrics/delivery`. Ninguna feature importa `reports`. En el frontend, `features/reports` importa `features/sprints` (`listSprints`) y nadie importa `features/reports`. | **PASA** |
| IV | `pdfmake@0.3.13`, versión exacta y estable, autorizada. Ninguna otra dependencia. | **PASA** |
| V | La prueba de integración usa la base de test, y `testsupport` vacía también `report_generations`. | **PASA** |
| VI | Lecturas acotadas al `projectId` de la ruta, detrás de `RequireMembership`. Un sprint ajeno responde igual que uno inexistente y no deja registro. Los `500` no exponen detalles. | **PASA** |
| VII | `uuid.Parse` del `sprintId` antes del servicio; errores del repositorio envueltos con `%w` y traducidos con `errors.Is`; reconocedor `isReport` en el frontend; `try/catch` en la exportación. | **PASA** |
| VIII | Comentarios en español por bloque; Swagger con la etiqueta "Reportes", en español; `README.md` suma la sección de reportes, `pdfmake` en la tabla del stack y la tabla nueva. | **PASA** |
| IX | Los artefactos generados son archivos de la copia de trabajo. | **PASA** |

**Resultado: todas las puertas siguen pasando después del diseño. La sección de seguimiento de
complejidad queda vacía.**

#### Dos decisiones que la constitución obliga a documentar

Ninguna es una violación, pero son concesiones conscientes y quedan registradas:

1. **El reporte suma horas por historia y por integrante.** La 008 no define esas cifras. Son sumas
   de los registros de la 006 con el mismo criterio de anclaje que la 008, y su total coincide con
   las horas reales de la 008, lo que se verifica con una prueba de integración. La única cifra que la
   008 sí define, las horas reales, se muestra desde la 008 en la sección de métricas (research,
   sección 6).
2. **Las lecturas por sección no comparten una transacción**, para que una falla no se lleve el
   resto (FR-025). En un sprint Activo, una carga concurrente puede hacer que el total de esfuerzo y
   las horas reales difieran en ese registro. El reporte ya está marcado como parcial (research,
   secciones 6 y 8).

---

## Estructura del Proyecto

### Documentación de esta feature

```text
specs/010-project-reports/
├── spec.md               # Especificación (entrada)
├── plan.md               # Este archivo (salida de /speckit-plan)
├── research.md           # Salida de la Fase 0
├── data-model.md         # Salida de la Fase 1
├── quickstart.md         # Salida de la Fase 1
├── contracts/
│   ├── openapi.yaml      # Salida de la Fase 1: las dos operaciones y el esquema del reporte
│   └── ui-contract.md    # Salida de la Fase 1: ruta, nombres accesibles, textos fijos y PDF
├── checklists/
│   └── requirements.md   # Checklist de calidad de la spec
└── tasks.md              # Salida de /speckit-tasks; NO lo crea /speckit-plan
```

### Código fuente (raíz del repositorio)

Las líneas marcadas con `NUEVO` las crea esta feature; las marcadas con `TOCA` son archivos de las
features 001 a 009 que esta feature modifica de forma puntual. Todo lo demás existe y no se toca.

```text
backend/
├── cmd/app/main.go                               # TOCA: compone reports con MetricsService y registra sus rutas en projectGroup
├── docs/                                         # TOCA: regenerado con swag init
└── internal/
    ├── reports/                                  # NUEVO: módulo de esta feature
    │   ├── domain/
    │   │   ├── report.go                         # Report, Header, secciones, grupos y líneas
    │   │   ├── scope.go                          # ReportScope, SectionState, StoryResult, PokerState
    │   │   ├── generation.go                     # ReportGeneration (tabla report_generations)
    │   │   ├── rules.go                          # EnsureReportable, IsPartial, ReportableSprints, ReportPeriod, SortStories, StoryTitle, estados
    │   │   ├── estimation.go                     # SelectEstimation con corte
    │   │   ├── errors.go                         # ErrSprintNotFound, ErrSprintNotStarted
    │   │   └── ports.go                          # ReportRepository, GenerationRepository, MetricsSource
    │   ├── repository/
    │   │   ├── report_repository.go              # Las ocho lecturas, una por consulta
    │   │   └── generation_repository.go          # Save
    │   ├── service/
    │   │   └── report_service.go                 # SprintReport y ProjectReport: arma secciones, decide estados, registra
    │   └── delivery/
    │       ├── dto.go                            # Forma JSON del contrato; métricas con los constructores de la 008
    │       ├── handler.go                        # generateSprintReport y generateProjectReport con Swagger
    │       └── routes.go                         # RegisterRoutes(projectGroup, handler)
    ├── metrics/delivery/dto.go                   # TOCA: exporta NewSprintMetrics y NewProjectMetrics
    └── platform/
        ├── database/postgres.go                  # TOCA: AutoMigrate de ReportGeneration
        └── testsupport/database.go               # TOCA: report_generations en el TRUNCATE

frontend/
├── src/
│   ├── app/projects/[projectId]/reports/page.tsx # NUEVO: Server Component; SessionGuard + ReportsView
│   ├── features/
│   │   ├── reports/                              # NUEVO: módulo de esta feature
│   │   │   ├── api.ts                            # generateSprintReport, generateProjectReport, isReport
│   │   │   ├── view-model.ts                     # buildReportView, metricText
│   │   │   ├── pdf/
│   │   │   │   ├── pdf-document.ts               # buildPdfDocument(view)
│   │   │   │   └── export-pdf.ts                 # import() diferido de pdfmake, createPdf, download
│   │   │   └── components/
│   │   │       ├── reports-view.tsx              # Selector, generar, exportar y estados de la pantalla
│   │   │       ├── report-scope-select.tsx       # "Proyecto completo" y sprints reportables
│   │   │       ├── report-document.tsx           # Recorre ReportView: título, marca parcial y seis secciones
│   │   │       └── report-section.tsx            # Título, estado, bloques y tablas de una sección
│   │   └── projects/components/project-overview.tsx  # TOCA: enlace "Reportes"
│   ├── types/
│   │   ├── report.ts                             # NUEVO: tipos de la respuesta
│   │   └── pdfmake.d.ts                          # NUEVO: declaración mínima de lo que se usa de pdfmake
│   └── utils/
│       └── report-file-name.ts                   # NUEVO: reportFileName
├── package.json                                  # TOCA: pdfmake 0.3.13 sin rango
└── package-lock.json                             # TOCA: regenerado por npm install

README.md                                         # TOCA: reportes, pdfmake en el stack y report_generations
```

**Pruebas del backend**: integración en `reports/delivery/handler_test.go`, con su
`testenv_test.go` (router real, base de test y siembra), como la 008. Unitarias en
`reports/domain/rules_test.go` y `reports/domain/estimation_test.go`. La prueba de US4-6 compone el
servicio con una fuente que falla, en `reports/service/report_service_test.go`.

**Pruebas del frontend**: junto al código, como en el resto del frontend. Escenarios en
`features/reports/components/reports-view.test.tsx`, que monta `ReportsView` con el backend de
prueba compartido de `features/reports/components/reports-test-backend.ts`. Unitarias en
`features/reports/api.test.ts`, `features/reports/view-model.test.ts`,
`features/reports/pdf/pdf-document.test.ts` y `utils/report-file-name.test.ts`.

**Decisión de estructura**: se mantiene la aplicación web con `backend/` y `frontend/` como
directorios de primer nivel (Principio III). El backend agrega **un** módulo vertical, `reports`, con
sus cuatro capas. El frontend agrega **un** módulo, `features/reports/`, con su API, su modelo de
presentación, su PDF y sus componentes, una página en `app/` y archivos en `types/` y `utils/`.

---

## Trazabilidad de escenarios a pruebas (Principio II)

Cada escenario Given/When/Then de la spec se implementa como al menos una prueba cuyo nombre
identifica la historia y el escenario. Buscar `010/US` en el código de pruebas devuelve la
trazabilidad completa.

**Backend (Go)**: función `TestUSn_<Escenario>` en `reports/delivery` (US4-6 en `reports/service`),
precedida por `// Escenario: <nombre de la tabla> (010/USn)`. Cubre los escenarios de generación: US1,
US3 y US4 salvo US4-5, que es solo de exportación. Son **26**.

**Frontend (Vitest)**: un `describe("010/USn - <título de la historia>")` por historia, con un
`it("<nombre del escenario>")` por escenario, precedido por `// Escenario: USn-k (<caso>)`. Cubre los
**35**: la generación vista en pantalla (US1, US3 y US4) y la exportación (US2).

La spec no nombra sus escenarios; los nombres de esta tabla son la referencia para `TestUSn_` e
`it(...)`.

| Historia | Escenario | Backend (`TestUSn_…`) | Frontend (`it`) |
| --- | --- | --- | --- |
| **US1** `Generar y ver el reporte de un sprint` | 1 | `ReporteDeSprintCerrado` | "muestra las seis secciones del reporte del sprint cerrado" |
| | 2 | `SprintActivoParcial` | "marca como parcial el reporte del sprint activo" |
| | 3 | `HistoriaConPlanningPoker` | "muestra el resumen de Planning Poker con 3 rondas y valor acordado 8" |
| | 4 | `HistoriaSinPlanningPoker` | "indica que la historia no tuvo sesión de estimación" |
| | 5 | `SinHistoriasCompletadas` | "lista las historias planificadas sin ninguna completada" |
| | 6 | `SprintSinHistorias` | "muestra sin datos en las secciones que dependen de historias" |
| | 7 | `MetricaNoCalculable` | "muestra el valor no calculable con su explicación y nunca 0" |
| | 8 | `SprintPlanificado` | "informa que un sprint planificado no tiene nada que reportar" |
| | 9 | `SprintDeOtroProyecto` | "responde a un sprint de otro proyecto como inexistente" |
| | 10 | `NoIntegrante` | "responde a quien no es integrante como proyecto inexistente" |
| **US2** `Exportar el reporte a PDF` | 1 | — | "exporta un PDF con el contenido de la pantalla, páginas numeradas y nombre con proyecto, alcance y fecha" |
| | 2 | — | "lleva la marca de parcial al PDF" |
| | 3 | — | "distingue en el nombre el alcance proyecto del de sprint" |
| | 4 | — | "pagina 80 historias repitiendo encabezados sin perder filas" |
| | 5 | — | "conserva acentos y eñes en el contenido y sanea el nombre del archivo" |
| | 6 | — | "distingue dos reportes del mismo día por la hora" |
| | 7 | — | "informa la falla del PDF y conserva el reporte en pantalla" |
| | 8 | — | "no ofrece exportar a quien no es integrante" |
| **US3** `Generar y ver el reporte de un proyecto completo` | 1 | `ProyectoConTresSprintsCerrados` | "agrupa las historias por sprint en orden cronológico" |
| | 2 | `ProyectoConSprintActivo` | "marca como parcial el reporte del proyecto con sprint activo" |
| | 3 | `SprintPlanificadoExcluido` | "no muestra el sprint planificado" |
| | 4 | `ProyectoFinalizado` | "muestra el reporte del proyecto finalizado sin marca de parcial" |
| | 5 | `HistoriaEnDosSprints` | "muestra la historia replanificada en los dos grupos con su estado" |
| | 6 | `HistoriasNoPlanificadas` | "muestra el grupo final de historias no planificadas" |
| | 7 | `SinHistoriasNoPlanificadas` | "muestra el grupo de no planificadas con sin datos" |
| | 8 | `HistoriasCambiadasDespuesDelCierre` | "muestra el título, la prioridad y los Story Points del cierre" |
| | 9 | `ReporteReproducible` | "muestra dos reportes del mismo sprint cerrado iguales salvo la generación" |
| | 10 | `ProyectoSinSprints` | "muestra el encabezado y las secciones sin datos" |
| | 11 | `ProyectoInexistente` | "informa que el proyecto no existe" |
| **US4** `Leer un reporte con secciones sin datos` | 1 | `SprintSinEsfuerzo` | "muestra la sección de esfuerzo con sin datos" |
| | 2 | `SprintSinDefectos` | "muestra la sección de defectos con sin datos y totales en 0" |
| | 3 | `HistoriasSinEstimar` | "lista las historias sin estimar y las métricas de la fuente" |
| | 4 | `SeccionesSinDatos` | "muestra todas las secciones con su título y su mensaje" |
| | 5 | — | "conserva en el PDF las secciones sin datos" |
| | 6 | `SeccionNoObtenible` | "distingue la sección no obtenida de la sección sin datos" |

**Total: 35 escenarios** (26 con prueba de backend, 35 con prueba de frontend).

**Cómo se preparan los escenarios**:

- **Backend**: cada prueba siembra con SQL directo o con los servicios de las features 002 a 007 los
  datos exactos de la spec (por ejemplo, 4 historias y 3 defectos en US1-1; una sesión Finalizada de
  3 rondas con valor 8 en US1-3) y verifica el cuerpo, el código y la fila de `report_generations`.
  El reloj de `MetricsService` y del servicio de reportes se inyecta, para fijar `generatedAt` y el
  plazo de gracia.
- **US3-8 y US3-9** cierran el sprint, renombran, cambian la prioridad y reestiman sus historias
  (también con una sesión de Planning Poker nueva después del cierre), y comparan dos cuerpos
  ignorando solo `header.generatedAt` y `header.generatedBy`.
- **US1-7** fuerza un "no calculable" con un sprint de 0 horas estimadas (`deviationPercent`).
- **US4-6** compone el servicio con un `MetricsSource` que devuelve error y verifica `metrics` y
  `defects` en `unavailable`, y el resto `complete`. Repite con el repositorio fallando solo en
  `LoadEffort`.
- **Frontend**: el backend de prueba responde las dos operaciones del contrato y `listSprints` de la
  004. `export-pdf` se sustituye con `vi.mock`: captura el documento de pdfmake y el nombre, o
  rechaza la promesa en US2-7. La comparación de US2-1 extrae los textos del documento y los de la
  pantalla, y los compara sección por sección.
- **US2-4** usa un reporte de 80 historias: el documento tiene las 80 filas, y cada tabla tiene
  `headerRows: 1` y `dontBreakRows: true` y un pie numerado. El render real se revisa a mano
  ([quickstart.md](./quickstart.md), paso 3.5).

**Pruebas transversales sin etiqueta de escenario**: sin token, `401` en las dos operaciones y la
guardia redirige a `/login` (SC-014); exportar no hace ninguna consulta (RN-02 en el frontend); un
`sprintId` mal formado responde `404` sin consultar; el `500` al guardar el registro no entrega el
reporte (SC-018); el total de esfuerzo es igual a `actualHours` de la 008 (research, sección 6). Se
nombran sin `010/USn` para no alterar el conteo de 35.

Además, pruebas unitarias **obligatorias** por el Principio II (reglas de negocio y validaciones),
que cubren SC-019:

| Objeto de prueba | Reglas cubiertas |
| --- | --- |
| Datos de US1-1 elegidos para que un recálculo dé otro número; `metrics` igual al cuerpo de `GET …/metrics/sprints/{id}` | RN-01 |
| La única fila nueva tras generar está en `report_generations`; ningún `updated_at` de otras tablas cambia | RN-02 |
| `IsPartial` con un sprint Activo y con uno Cerrado | RN-03 |
| `EnsureReportable` con `planned`, `active` y `closed` | RN-04 |
| `ReportableSprints` con Planificados intercalados | RN-05 |
| `IsPartial` en el alcance de proyecto, con y sin Activo | RN-06 |
| `ReportableSprints` conserva `start_date ASC, id ASC` | RN-07 |
| Integración US3-5: una historia en dos grupos con su resultado en cada uno | RN-08 |
| `SelectEstimation`: dos Finalizadas, elige la de mayor `closed_at`; empate por `id` | RN-09 |
| `SelectEstimation`: solo Canceladas → `no_agreement`; sin sesiones → `none`; Finalizada después del corte → `no_agreement` | RN-10 |
| `StoriesState`, `EffortState`, `DefectsState` con listas vacías; `buildReportView` con `empty` | RN-11 |
| `buildReportView` con `unavailable` y con `empty`: textos y roles distintos | RN-12 |
| `metricText` con `not_calculable` en las tres métricas que lo admiten: nunca "0" | RN-13 |
| `metricText` con `partial`, `provisional`, `definitive` y `not_applicable`: cuatro textos distintos | RN-14 |
| `buildPdfDocument(view)`: sus textos, en orden, son los de `ReportDocument` | RN-15 |
| `buildPdfDocument`: `headerRows: 1` y `dontBreakRows: true` en cada tabla; `footer` con "Página N de M" | RN-16 |
| `reportFileName` con acentos, eñes, `\ / : * ? " < > \|`, espacios, nombre vacío y nombre de 200 caracteres | RN-17 |
| `ReportsView` con `export-pdf` rechazado: el `article` "Reporte" sigue igual y el botón, habilitado | RN-18 |
| Integración: no integrante y sprint ajeno → `404` idéntico al inexistente, sin registro | RN-19 |
| Integración US3-4: proyecto `finished` → `200` y registro | RN-20 |
| Integración: `generatedBy` es el titular de la sesión y la fila lo registra | RN-21 |
| `StoryTitle` y `SortStories`; integración US3-9 | RN-22 |
| Integración: con un sprint Activo, un cambio en el backlog cambia el reporte siguiente | RN-23 |
| Integración: una fila por `200` con `scope`, `sprint_id`, autor e instante; ninguna columna con el documento | RN-24 |
| Integración US3-6 y US3-7 | RN-25 |
| `isReport` con cuerpos válidos e inválidos | Principio VII |

Comparación con los criterios de éxito: SC-001 y SC-002 con el contrato de interfaz (tres pasos y un
paso); SC-003 con US2-1 y la fila RN-15; SC-004 con la fila RN-01 y la prueba transversal de
esfuerzo; SC-005 con US1-7 y la fila RN-13; SC-006 con US1-2, US2-2 y US3-2; SC-007 con US1-8;
SC-008 con US4-4; SC-009 con US4-6 y la fila RN-12; SC-010 con US2-4 y el paso 3.5 del quickstart;
SC-011 y SC-012 con US2-5, US2-6 y la fila RN-17; SC-013 con US2-7; SC-014 y SC-015 con US1-9,
US1-10 y las pruebas transversales; SC-016 con US1-1 sembrado con un defecto Descartado; SC-017 con
US3-9; SC-018 con la fila RN-24 y la prueba transversal del `500`; SC-019 con la tabla de arriba.

---

## Secuencia de trabajo prevista

El orden lo fija `/speckit-tasks`; esto es la dependencia entre bloques, no la lista de tareas.

0. **Prerrequisitos**: las features 001 a 008 en `main` para el backend; la 009 en `main` para el
   frontend.
1. **Fase técnica fundacional**: `ReportGeneration` con su `AutoMigrate` y su `TRUNCATE`; esqueleto
   del módulo `reports` y sus rutas en `main.go`; constructores exportados en `metrics/delivery`;
   `pdfmake@0.3.13` sin rango y `types/pdfmake.d.ts`; `types/report.ts`; página, `ReportsView` vacía y
   enlace desde `ProjectOverview`; backend de prueba compartido. Lo que tiene lógica y prueba
   unitaria obligatoria entra en la historia que lo usa primero, para que cada una tenga un RED real.
2. **US1 Reporte de sprint** (P1): `EnsureReportable`, `IsPartial`, `SortStories`, `StoryTitle`,
   `SelectEstimation`; las ocho lecturas para un sprint; registro de generación; `buildReportView`,
   `metricText`, `ReportDocument` y `ReportSection`.
3. **US2 Exportar a PDF** (P1): `reportFileName`, `buildPdfDocument`, `exportReportPdf` y el botón con
   su manejo de falla.
4. **US3 Reporte de proyecto** (P2): `ReportableSprints`, `ReportPeriod`, grupos por sprint, historias
   no planificadas, esfuerzo y defectos del proyecto, métricas del proyecto y la opción "Proyecto
   completo".
5. **US4 Secciones sin datos** (P3): estados `empty` y `unavailable` en el servicio, lecturas
   independientes, la composición con fuente fallida y sus textos en pantalla y en el PDF.
6. **Cierre**: Swagger (`swag init`), `README.md`, cobertura (`go test ./... -coverprofile` y
   `npm run test:coverage`), `lint`, `format:check`, `gofmt` y la verificación transversal de
   trazabilidad (`010/US`) y de las puertas de calidad.

En cada historia el ciclo es el del Principio II: escribir la prueba, **verificar que falla y
detenerse** para que el desarrollador registre el commit RED con `/redactar-commit`, implementar,
verificar que pasa, refactorizar. Un agente no avanza de RED a GREEN sin ese alto.

---

## Seguimiento de Complejidad

> Se completa SOLO si la verificación de la constitución tiene violaciones que justificar.

**Sin violaciones.** Las dos puertas de evaluación pasan en su totalidad. La dependencia nueva cuenta
con la autorización que prevé el Principio IV. Las dos concesiones documentadas, las sumas de
esfuerzo y las lecturas sin transacción común, están justificadas en "Verificación de la
Constitución" y en [research.md](./research.md).

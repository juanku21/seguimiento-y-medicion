---

description: "Lista de tareas de la feature 009 — Dashboard del Proyecto"
---

# Tareas: Dashboard del Proyecto

**Entrada**: documentos de diseño de `specs/009-project-dashboard/`

**Prerrequisitos**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/consumed-api.md](./contracts/consumed-api.md),
[contracts/ui-contract.md](./contracts/ui-contract.md), [quickstart.md](./quickstart.md)

**Base existente**: las features [`specs/001-user-auth`](../001-user-auth/spec.md) a
[`specs/008-metrics-calculation`](../008-metrics-calculation/spec.md) integradas en `main`. Esta
feature usa del frontend `lib/api-client.ts` (`getJson`, `isRecord`, `ApiResult`),
`hooks/use-authorized-query.ts`, `features/auth/components/session-guard.tsx`,
`providers/session-provider.tsx`, `components/alert.tsx` y `components/button.tsx`; de la 002
`getProjectDetail`, `MemberList`, `ProjectDetail` y `PROJECT_STATUS_LABELS`; de la 004 `getSprint`,
`listSprints`, `SprintSchedule` y sus tipos; de la 006 `formatHours`, `formatSignedHours` y
`formatPercent` (`utils/effort-format.ts`); de la 007 `DEFECT_SEVERITY_LABELS`; y `formatDate`
(`utils/format-date.ts`). Del backend usa, sin tocarlos, los seis `GET` de
[contracts/consumed-api.md](./contracts/consumed-api.md). **El backend no cambia.**

**Pruebas**: **incluidas y obligatorias**. El Principio II de la constitución es NO NEGOCIABLE: la
prueba se escribe antes del código de producción. Las pruebas unitarias son obligatorias para las
reglas de presentación RN-01 a RN-21, los formateadores, las explicaciones de "no calculable" y los
reconocedores de las respuestas (validación de entradas).

**Organización**: las tareas se agrupan por historia de usuario para que cada una se implemente y
se pruebe de forma independiente.

## Formato: `[ID] [P?] [Historia] Descripción`

- **[P]**: se puede ejecutar en paralelo (archivos distintos, sin dependencias pendientes)
- **[USn]**: historia de usuario a la que pertenece la tarea
- Cada descripción incluye la ruta exacta del archivo

## Convenciones de ruta

- **Frontend**: página nueva `frontend/src/app/projects/[projectId]/dashboard/page.tsx`; módulo nuevo
  `frontend/src/features/dashboard/` (`api.ts`, `series.ts`, `components/`); archivos nuevos
  `frontend/src/types/metrics.ts`, `frontend/src/utils/metric-format.ts` y
  `frontend/src/utils/metric-explanations.ts`; se tocan de forma puntual
  `frontend/src/hooks/use-authorized-query.ts`,
  `frontend/src/features/projects/components/project-overview.tsx`, `frontend/package.json` y
  `frontend/package-lock.json`
- **Backend**: sin cambios
- **Pruebas**: junto al código que prueban (`*.test.ts` y `*.test.tsx`), como en el resto del
  frontend

## Ciclo obligatorio en cada tarea

Según el Principio II y la skill `/speckit-implement`:

1. **Tarea de pruebas**: se escribe la prueba, se verifica que **falla** y se **detiene**. El
   desarrollador registra el commit con `/redactar-commit`, pie `TDD: red`.
2. **Tarea de implementación**: código mínimo que hace pasar la prueba (`TDD: green`), pausa y, si
   corresponde, refactor (`TDD: refactor`).
3. **Tarea técnica**: sin prueba asociada, sin pie TDD.

Ningún agente avanza de RED a GREEN sin ese alto.

## `DashboardView` crece por historia

Todas las pruebas de escenario montan la vista real, `DashboardView`, así que cada historia le suma
sus secciones en el orden fijo de [contracts/ui-contract.md](./contracts/ui-contract.md):

| Historia | Qué agrega a `DashboardView` |
| --- | --- |
| US1 | Consulta del resumen, que bloquea; `ProjectSummary` y `ActiveSprintPanel` |
| US2 | Consultas de métricas del proyecto y del sprint activo, que la vista guarda y reparte; `KeyIndicators` |
| US3 | Consulta de series, una sola para los tres gráficos; `VelocityChart` |
| US4 | `EffortChart` sobre la misma consulta de series |
| US5 | `QualityChart` y `OpenDefectsDistribution`, esta con la consulta de métricas de US2 |
| US6 | Consulta del listado de sprints y estado vacío; `DashboardWarnings`; reintento del resumen |

Las consultas viven en `DashboardView` y las secciones reciben su `QueryState` por propiedades. Así,
una consulta compartida, como la de series o la de métricas del proyecto, se hace una sola vez. La
excepción es `ActiveSprintPanel`, que hace su propia consulta porque depende del `activeSprint.id`
del resumen.

## Cómo se preparan los escenarios

- El backend de prueba compartido (T004) sustituye `fetch` y responde las seis operaciones de
  [contracts/consumed-api.md](./contracts/consumed-api.md). Por defecto responde con los datos de la
  spec, y cada prueba cambia solo la operación que su escenario necesita, con otro cuerpo o con otro
  código.
- La falla de un solo gráfico se fuerza con su porción de las series malformada: por ejemplo,
  `defectsDetected: "x"`. Así, los otros dos gráficos se dibujan con la misma respuesta.
- La recarga (US6-7, US6-8) se simula con `unmount()` y un `render` nuevo, después de cambiar la
  respuesta del backend de prueba.
- Los días restantes llegan resueltos por la 004, así que ninguna prueba usa reloj, salvo la de
  RN-17, que usa temporizadores falsos para comprobar que la vista no vuelve a consultar sola.
- Las pruebas sin etiqueta `009/USn` (sin sesión, solo `GET`, ninguna consulta después del `404`,
  sin actualización automática) no alteran el conteo de 45.

---

## Fase 1: Setup (Inicialización)

**Propósito**: instalar la dependencia de gráficos autorizada el 2026-10-28.

**Rama**: `fase/009-foundational`, la misma que la Fase 2: las dos forman un único pull request.

- [X] T001 Instalar en `frontend/` las dependencias autorizadas con versión exacta:
      `npm install --save-exact recharts@3.10.1 react-is@19.3.0`. Verificar que
      `frontend/package.json` las lista en `dependencies` **sin** `^` ni `~` y que
      `frontend/package-lock.json` quedó actualizado. Verificar también que no se agregó ninguna
      otra dependencia directa y que `npm run build`, `npm run lint` y `npm test` siguen pasando sin
      cambios. Si `npm` informa un conflicto de peers con React 19.3.0, detenerse y avisar al
      desarrollador antes de usar `--legacy-peer-deps` (Principio IV; research.md sección 2)

---

## Fase 2: Foundational (Prerrequisitos bloqueantes)

**Propósito**: los tipos de las respuestas de la 008, el backend de prueba compartido y el esqueleto
de la pantalla. Todas las tareas son **técnicas**: declaran tipos, ayudantes de prueba y rutas, sin
lógica de presentación.

**⚠️ CRÍTICO**: ninguna historia de usuario puede empezar hasta que esta fase esté completa.

**Rama**: `fase/009-foundational`

- [X] T002 [P] Declarar en `frontend/src/types/metrics.ts` los tipos de data-model.md sección 1, con
      un comentario de cabecera ("Tipos de las respuestas de cálculo de métricas, según
      specs/008-metrics-calculation/contracts/openapi.yaml; solo los campos que usa el dashboard"):
      `ValueStatus = "available" | "not_calculable" | "not_applicable"`;
      `Finality = "definitive" | "provisional" | "partial"`;
      `Metric = { status: ValueStatus; value: number | null }`, con el comentario "`value` es null si
      y solo si `status` no es `available`"; `SprintMetric = Metric & { finality: Finality | null }`;
      `ActiveSprintMetrics` con `plannedStoryPoints`, `completedStoryPoints`, `estimatedHours`,
      `actualHours`, `deviationHours`, `deviationPercent` y `storyCompletionPercent` de tipo
      `SprintMetric`; `ActiveSprintMetricsResponse = { activeSprint: boolean; metrics:
      ActiveSprintMetrics | null }`; `Cumulative` con `finality: "definitive" | "partial"` y las
      mismas siete métricas de tipo `Metric`; `SeverityBreakdown = { critical; high; medium; low:
      number }`; `DefectTally = { total: number; bySeverity: SeverityBreakdown }`;
      `WarningKind = "planning" | "data_inconsistency"`; `MetricsWarning = { kind: WarningKind;
      code: string; message: string }`, con el comentario de que `code` es `string` porque el
      dashboard no decide nada según el código; `ProjectMetricsResponse = { velocity: { value:
      Metric }; cumulative: Cumulative; defects: { open: DefectTally }; warnings: MetricsWarning[] }`;
      `SeriesPoint = { sprintId: string; sprintName: string; sprintStatus: "active" | "closed";
      status: ValueStatus; value: number | null; finality: Finality }`; y `ProjectSeriesResponse`
      con las seis listas `plannedStoryPoints`, `completedStoryPoints`, `estimatedHours`,
      `actualHours`, `defectsDetected` y `defectsResolved` de `SeriesPoint[]`. Sin `any` y sin
      campos que el dashboard no muestra
- [X] T003 [P] Leer antes `frontend/node_modules/next/dist/docs/01-app/01-getting-started/03-layouts-and-pages.md`
      y `05-server-and-client-components.md` (CLAUDE.md). Crear
      `frontend/src/app/projects/[projectId]/dashboard/page.tsx`, un Server Component con
      `metadata.title = "Dashboard | Software Metrics & Estimation"` que lee `params` como `Promise`,
      muestra el `h1` "Dashboard" y monta `<SessionGuard><DashboardView projectId={projectId} />
      </SessionGuard>`, con la misma estructura que `app/projects/[projectId]/page.tsx` y un ancho
      mayor (`max-w-5xl`) para los gráficos. Crear
      `frontend/src/features/dashboard/components/dashboard-view.tsx` con `"use client"`, la
      exportación `DashboardView({ projectId }: DashboardViewProps)` y un cuerpo que todavía no
      muestra nada (`return null`), con el comentario de qué va a componer
- [X] T004 Crear en `frontend/src/features/dashboard/components/dashboard-test-backend.ts` el
      ayudante de pruebas compartido, que importan **solo** archivos `*.test.tsx`:
      `installDashboardBackend(overrides?)` reemplaza `globalThis.fetch` con `vi.fn` y responde por
      método y ruta las seis operaciones de contracts/consumed-api.md
      (`GET /api/v1/projects/{id}`, `GET /api/v1/sprints/{sprintId}`,
      `GET /api/v1/projects/{id}/sprints`, `GET .../metrics`, `GET .../metrics/active-sprint` y
      `GET .../metrics/series`), con `404 {"error":"El proyecto no existe."}` para cualquier otro
      proyecto; cada operación se puede reemplazar con `{ status, body }`; devuelve la lista de
      peticiones recibidas (método y ruta) para verificar qué consultó la vista. Datos por defecto,
      con los valores de la spec: un proyecto `in_progress` con 4 integrantes y `activeSprint` con
      `remainingDays: 10` (US1-1); métricas del proyecto con velocidad `14.00`, defectos abiertos
      `7` repartidos en 2/3/1/1 y un acumulado `partial`; métricas del sprint activo con `18`, `5`,
      `27.78`, `108.00`, `40.00`, `-68.00` y `-62.96`, todas `partial` (US2-1); las seis series para 4
      sprints Cerrados, con 21/24/13/25 Story Points planificados y 13/21/8/20 completados (US3-1); listado de sprints con
      `emptyReason: null`. Exportar también `installResizeObserverStub()`, un `ResizeObserver`
      mínimo que informa 800 × 320 para que `ResponsiveContainer` de Recharts dibuje en jsdom
      (research.md sección 2), y `renderDashboard(projectId)`, que guarda un token con
      `writeToken` y monta `DashboardView` dentro de `SessionProvider`, como
      `project-effort-view.test.tsx`. El `vi.mock("next/navigation", …)` lo declara cada archivo de
      prueba, porque Vitest solo eleva los `vi.mock` escritos en el propio archivo — depende de T002, T003

**Checkpoint**: `npm run build` y `npm run lint` pasan, `npm test` pasa sin pruebas nuevas, y
`/projects/{id}/dashboard` existe y está protegida por la guardia de sesión; el enlace desde la
vista del proyecto llega en US1 (T009), con su prueba. Las historias pueden
empezar.

---

## Fase 3: Historia de Usuario 1 — Ver el estado del proyecto y del sprint en curso (Prioridad: P1) 🎯 MVP

**Objetivo**: un integrante abre el dashboard y ve el nombre, el estado, las fechas y los
integrantes del proyecto, y del sprint en curso su nombre, Sprint Goal, fechas y días restantes, tal
como los entrega la 004.

**Rama**: `hu/009-us1-ver-el-estado-del-proyecto-y-del-sprint-en-curso`

**Prueba independiente**: con el backend de prueba por defecto, el dashboard muestra el proyecto con
sus 4 integrantes y el sprint activo con "10 días restantes"; con un proyecto ajeno muestra "El
proyecto no existe." y no hace ninguna otra consulta.

### Pruebas de la Historia 1 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T005 [P] [US1] Pruebas unitarias **obligatorias** de `ActiveSprintPanel` en
      `frontend/src/features/dashboard/components/active-sprint-panel.test.tsx`, con `fetch`
      sustituido, cubriendo RN-12, RN-13 y RN-14: con `activeSprintId` `null` muestra "No hay ningún
      sprint en curso.", no usa `role="alert"` y **no** pide `GET /api/v1/sprints/{id}`; con un
      detalle con `schedule.remainingDays: 10` muestra nombre, Sprint Goal, "Inicio" y "Fin" con
      `formatDate`, y "10 días restantes"; con `remainingDays: 0`, "0 días restantes"; con
      `overdueDays: 3`, "Vencido hace 3 días", y el texto de la región no contiene "−3" ni "-3"; con
      un `500` muestra "No se pudo cargar el sprint activo." con `role="alert"` — depende de T004
- [X] T006 [US1] Pruebas de escenario en
      `frontend/src/features/dashboard/components/project-summary.test.tsx`, montando
      `DashboardView` con `renderDashboard`, con
      `describe("009/US1 - Ver el estado del proyecto y del sprint en curso")` y un `it` por escenario
      con los nombres de la tabla de plan.md, cada uno precedido por `// Escenario: US1-k (<caso>)`:
      (1) región "Resumen del proyecto" con nombre, estado con `PROJECT_STATUS_LABELS`, "Inicio",
      "Finalización prevista" y los 4 integrantes, y región "Sprint activo" con nombre, Sprint Goal,
      fechas y "10 días restantes"; (2) proyecto `finished` con `actualEndDate` y `activeSprint:
      null`: "Finalización real" y "No hay ningún sprint en curso."; (3) `in_progress` sin
      `activeSprint`: el resumen se muestra y no hay ningún `role="alert"`; (4) `remainingDays: 0` →
      "0 días restantes"; (5) `overdueDays: 3` → "Vencido hace 3 días"; (6) un detalle de sprint con
      `remainingDays: 7` y un `endDate` elegido para que la diferencia con hoy **no** sea 7: se muestra
      "7 días restantes", el valor de la 004, sin restar fechas; (7) un único integrante, sin ningún texto especial; (8) `404`
      del resumen: muestra "El proyecto no existe." y ninguna otra sección; (9) el `404` de un
      proyecto ajeno y el de uno inexistente producen exactamente el mismo `textContent`. Agregar,
      **sin** etiqueta `009/USn`, `it("con el 404 del resumen no hace ninguna otra consulta")`
      (SC-015, RN-18), que verifica con la lista de peticiones del backend de prueba que solo se pidió
      `GET /api/v1/projects/{id}`, e `it("sin sesión lleva al inicio de sesión")` (SC-014), sin
      token: la guardia llama a `router.replace("/login")` y no se pide nada. Agregar además en
      `frontend/src/features/projects/components/project-overview.test.tsx`, junto a las pruebas de
      los enlaces al backlog y a los sprints, `it("ofrece el enlace al dashboard")`: la navegación
      "Secciones del proyecto" tiene el enlace "Dashboard" con `href` `/projects/{id}/dashboard`
      como primer elemento — depende de T004, T003

### Implementación de la Historia 1

- [X] T007 [P] [US1] Implementar en
      `frontend/src/features/dashboard/components/active-sprint-panel.tsx`
      `ActiveSprintPanel({ activeSprintId }: { activeSprintId: string | null })`: una región "Sprint
      activo" (`<section aria-labelledby>` con `h2`); con `null` muestra "No hay ningún sprint en
      curso." sin consultar; si no, consulta `getSprint` con `useAuthorizedQuery` y una carga
      estabilizada con `useCallback([activeSprintId])`, muestra "Cargando sprint activo…" con
      `role="status"`, el error "No se pudo cargar el sprint activo." con `Alert` y, con datos, el
      nombre, el Sprint Goal, "Inicio"/"Fin" con `formatDate` y el componente `SprintSchedule` de
      `features/sprints/components/sprint-schedule.tsx` **sin cambios**, sin restar fechas (FR-009,
      FR-010, RN-13) — depende de T005
- [X] T008 [P] [US1] Implementar en `frontend/src/features/dashboard/components/project-summary.tsx`
      `ProjectSummary({ project }: { project: ProjectDetail })`: una región "Resumen del proyecto" con
      el nombre, el estado con `PROJECT_STATUS_LABELS`, "Inicio" y "Finalización prevista" con
      `formatDate`, "Finalización real" solo si `status === "finished"` y `actualEndDate` no es `null`
      (FR-006), y la lista "Integrantes" con `MemberList` de la 002, con `viewerIsOwner={false}` y
      `readOnly`, para que no aparezca ninguna acción (FR-003, FR-007) — depende de T006
- [X] T009 [US1] Implementar en `frontend/src/features/dashboard/components/dashboard-view.tsx` la
      consulta del resumen con `useAuthorizedQuery(getProjectDetail)` y una carga estabilizada con
      `useCallback([projectId])`: mientras carga, "Cargando dashboard…" con `role="status"` y nada
      más; con error, el `message` recibido dentro de `Alert` y **ninguna** otra sección ni consulta
      (FR-030, FR-035); con datos, `ProjectSummary` y `ActiveSprintPanel` con
      `activeSprintId={project.activeSprint?.id ?? null}`, en el orden de contracts/ui-contract.md.
      Las demás secciones se montan solo cuando el resumen llegó, para que un `404` no dispare otras
      consultas (research.md sección 3). Sumar el enlace "Dashboard" a
      `/projects/${project.id}/dashboard` como **primer** elemento de la navegación "Secciones del
      proyecto" de `frontend/src/features/projects/components/project-overview.tsx`, con las mismas
      clases que los demás enlaces, y actualizar el comentario del componente — depende de T007, T008

**Checkpoint**: US1 funciona sola: el resumen y el sprint en curso se muestran con los datos de la
002 y la 004, sin aritmética de fechas, y un proyecto ajeno no revela nada.

---

## Fase 4: Historia de Usuario 2 — Ver los indicadores clave del momento (Prioridad: P1)

**Objetivo**: el integrante ve los indicadores del sprint activo, todos parciales, y los del
proyecto (velocidad, defectos abiertos y acumulado con su marca), con exactamente los valores de la
008 y con los "no calculable" explicados, nunca como 0.

**Rama**: `hu/009-us2-ver-los-indicadores-clave-del-momento`

**Prueba independiente**: con el backend de prueba por defecto, el grupo "Indicadores del sprint activo" muestra
"18", "5", "27,78 %", "108 h", "40 h", "−68 h" y "−62,96 %", cada uno con "Parcial"; el grupo
"Indicadores del proyecto" muestra la velocidad "14,00" sin marca y "7" defectos abiertos.

### Pruebas de la Historia 2 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T010 [P] [US2] Pruebas unitarias **obligatorias** en `frontend/src/utils/metric-format.test.ts`
      y `frontend/src/utils/metric-explanations.test.ts`, cubriendo FR-013 y FR-014:
      `formatDecimal(14)` → "14,00", `formatDecimal(0)` → "0,00", `formatDecimal(13.1)` → "13,10";
      `formatShare(27.78)` → "27,78 %", `formatShare(0)` → "0,00 %", `formatShare(100)` →
      "100,00 %", sin signo "+"; para cada valor de la spec (14,00; 27,78; 13,10; 0), el texto sin la
      unidad y con la coma cambiada por punto vale **exactamente** el número recibido (ningún
      redondeo); `explainNotCalculable("velocity")` → "Todavía no hay sprints cerrados.",
      `("deviationPercent")` → "Las horas estimadas son 0.", `("storyCompletionPercent")` → "No hay
      historias planificadas." y cualquier otra clave → "No se puede calcular con los datos
      actuales." (research.md secciones 5 y 6)
- [X] T011 [P] [US2] Pruebas unitarias **obligatorias** de `MetricValue` y `FinalityBadge` en
      `frontend/src/features/dashboard/components/metric-value.test.tsx`, cubriendo RN-03 a RN-06:
      `{status: "available", value: 0}` muestra "0" con el formateador recibido;
      `{status: "not_calculable", value: null}` con `kind` `velocity`, `deviationPercent` y
      `storyCompletionPercent` muestra "No calculable" y su explicación, y el texto **no** es "0",
      "—", "-" ni vacío; `{status: "not_applicable", value: null}` muestra "No aplica"; los tres
      textos son distintos entre sí; `finality: "partial"` muestra "Parcial"; `"provisional"` muestra
      "Provisoria" y no "Parcial"; `"definitive"` y `null` no muestran marca; `FinalityBadge` con
      `partial` y con `provisional` produce textos distintos — depende de T010
- [X] T012 [P] [US2] Pruebas unitarias **obligatorias** de los reconocedores y las consultas en
      `frontend/src/features/dashboard/api.test.ts` (validación de entradas, Principio VII):
      `isMetric` acepta `available` con número y `not_calculable` y `not_applicable` con `null`, y
      rechaza `available` con `null`, `not_calculable` con número y un `status` desconocido;
      `isSprintMetric` exige `finality` válido o `null`; `isActiveSprintMetricsResponse` acepta
      `{activeSprint: false, metrics: null}` y la respuesta completa, y rechaza `activeSprint: true`
      con `metrics: null` y una métrica faltante; `isProjectMetricsResponse` rechaza un `defects.open`
      sin alguna de las cuatro severidades, una advertencia sin `message` y un `cumulative.finality`
      `provisional`; con `fetch` sustituido, `getProjectMetrics("a/b")` y
      `getActiveSprintMetrics("a/b")` piden con `GET` las rutas con el identificador codificado
      (`a%2Fb`) y un cuerpo que no cumple el reconocedor llega como error — depende de T002
- [X] T013 [US2] Pruebas de escenario en
      `frontend/src/features/dashboard/components/key-indicators.test.tsx`, montando `DashboardView`
      con `renderDashboard`, con `describe("009/US2 - Ver los indicadores clave del momento")` y un
      `it` por escenario según plan.md, cada uno precedido por `// Escenario: US2-k (<caso>)`, que
      buscan cada valor por su término (`<dt>`) dentro de la región "Indicadores clave": (1) los
      valores exactos de US2-1 en el grupo "Indicadores del sprint activo", la velocidad "14,00" y "7" defectos
      abiertos en "Indicadores del proyecto"; los datos se eligen para que un recálculo dé otro número (RN-01): la
      velocidad `14.00` **no** es el promedio de la serie de la misma respuesta; (2) cada valor del
      grupo "Indicadores del sprint activo" con "Parcial", y la velocidad sin marca; (3) con `cumulative.finality:
      "partial"`, el encabezado del grupo "Indicadores del proyecto" muestra "Parcial", y con `definitive` no;
      (4) velocidad `not_calculable` → "No calculable" y "Todavía no hay sprints cerrados.", nunca
      "0"; (5) estimadas `0`, desviación porcentual `not_calculable` y desviación absoluta `40.00`:
      "No calculable" con "Las horas estimadas son 0." y "+40 h"; (6) `defects.open.total: 0` → "0"
      como cifra, sin "No calculable"; (7) las dos consultas de métricas en `500`: los dos mensajes
      de falla de contracts/ui-contract.md con `role="alert"`, y el resumen y el sprint activo se
      muestran igual. Agregar, **sin** etiqueta `009/USn` para no alterar el conteo de 45,
      `it("muestra el porcentaje de historias no calculable con su explicación")` (SC-005): sprint
      activo con `storyCompletionPercent` en `not_calculable` muestra "No calculable" y "No hay
      historias planificadas.", nunca "0" — depende de T004, T009

### Implementación de la Historia 2

- [X] T014 [P] [US2] Implementar en `frontend/src/utils/metric-format.ts` `formatDecimal(value:
      number): string` (`value.toFixed(2)` con coma decimal) y `formatShare(value: number): string`
      (lo mismo más " %", sin signo), con el comentario de que el valor llega con a lo sumo 2
      decimales desde la 008 y `toFixed(2)` solo restituye los ceros que `JSON.parse` quitó; y en
      `frontend/src/utils/metric-explanations.ts` el tipo `MetricKind` (`"velocity" |
      "storyPoints" | "storyCompletionPercent" | "hours" | "deviationHours" | "deviationPercent" |
      "defects"`) y `explainNotCalculable(kind: MetricKind): string` con los textos de research.md
      sección 6 — depende de T010
- [X] T015 [P] [US2] Implementar en `frontend/src/features/dashboard/api.ts` los reconocedores
      `isMetric`, `isSprintMetric`, `isActiveSprintMetricsResponse` e `isProjectMetricsResponse` de
      data-model.md sección 3, sobre `isRecord` de `lib/api-client.ts` y sin `any`, y las consultas
      `getProjectMetrics(projectId)` y `getActiveSprintMetrics(projectId)` con `getJson` sobre
      `/api/v1/projects/${encodeURIComponent(projectId)}/metrics` y `.../metrics/active-sprint`.
      Comentario de cabecera: este módulo solo lee (RN-02) — depende de T012
- [X] T016 [US2] Implementar en `frontend/src/features/dashboard/components/finality-badge.tsx`
      `FinalityBadge({ finality }: { finality: Finality | null })`: "Parcial" para `partial`,
      "Provisoria" para `provisional`, con estilos distintos, y nada para `definitive` o `null`; y en
      `frontend/src/features/dashboard/components/metric-value.tsx` `MetricValue({ metric, kind,
      format })`, el **único** presentador de `Metric` y `SprintMetric` (research.md sección 5):
      `available` → `format(value)`, `not_calculable` → "No calculable" más
      `explainNotCalculable(kind)`, `not_applicable` → "No aplica", y `FinalityBadge` si la métrica
      tiene `finality` — depende de T011, T014
- [X] T017 [US2] Implementar en `frontend/src/features/dashboard/components/section-error.tsx`
      `SectionError({ message }: { message: string })`, que muestra el mensaje con `Alert`
      (`role="alert"`), y declarar en el mismo archivo las constantes de los ocho mensajes de falla
      de contracts/ui-contract.md, en un único lugar — depende de T013
- [X] T018 [US2] Implementar en `frontend/src/features/dashboard/components/key-indicators.tsx`
      `KeyIndicators({ projectMetrics, activeSprintMetrics })`, que recibe los dos `QueryState`:
      región "Indicadores clave" con los grupos "Indicadores del sprint activo" e "Indicadores del proyecto" como `<dl>`, cada uno
      dentro de un `<div>` con `h3` y `aria-labelledby`, sin `<section>`, para que no sea una región,, con los
      términos y el formato de contracts/ui-contract.md; el grupo "Indicadores del sprint activo" con
      `activeSprint: false` dice "No hay ningún sprint en curso." (FR-011); el grupo "Indicadores del proyecto"
      muestra la velocidad sin marca, `defects.open.total` con `String` y el acumulado con
      `FinalityBadge` de `cumulative.finality` en su encabezado; cada grupo con su propio "Cargando…"
      y su `SectionError`. Sumar en `dashboard-view.tsx` las dos consultas con `useAuthorizedQuery`
      y montar `KeyIndicators` después del resumen. Ningún componente hace aritmética con las cifras
      (RN-01) — depende de T015, T016, T017

**Checkpoint**: US1 y US2 juntas son el mínimo útil del dashboard (spec, US2): la pantalla ya sirve
sin gráficos.

---

## Fase 5: Historia de Usuario 3 — Ver la evolución de la velocidad (Prioridad: P2)

**Objetivo**: el integrante ve un gráfico de barras agrupadas con los Story Points planificados y
completados de cada sprint Cerrado, en el orden de la 008, con título, ejes rotulados y leyenda, y
una tabla con los valores exactos.

**Rama**: `hu/009-us3-ver-la-evolucion-de-la-velocidad`

**Prueba independiente**: con 4 sprints Cerrados de 21/24/13/25 y 13/21/8/20, la figura "Velocidad
por sprint" muestra la leyenda con las dos series, los ejes "Sprint" y "Story Points", y una tabla con
las 4 filas en el orden recibido; con un sprint Activo más, la tabla sigue teniendo 4 filas.

### Pruebas de la Historia 3 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T019 [P] [US3] Pruebas unitarias **obligatorias** en
      `frontend/src/features/dashboard/series.test.ts` y en `frontend/src/features/dashboard/api.test.ts`
      (casos nuevos), cubriendo RN-20 y la validación por porción: `closedSprintPoints` sobre
      `[closed A, active B, closed C]` devuelve `[A, C]`, con los mismos objetos y en el mismo
      orden, y sobre una lista sin `active` la devuelve igual; `readSeries(envelope, key)` devuelve
      los puntos de una serie válida, `null` si la serie no es lista, si un punto no tiene `sprintId`
      o tiene un `sprintStatus` o `finality` desconocido; con `defectsDetected` malformada,
      `readSeries` de `plannedStoryPoints` sigue devolviendo sus puntos; `getProjectSeries("a/b")`
      pide con `GET` `/api/v1/projects/a%2Fb/metrics/series` — depende de T015
- [X] T020 [P] [US3] Pruebas unitarias **obligatorias** de `SprintSeriesChart` en
      `frontend/src/features/dashboard/components/sprint-series-chart.test.tsx`, con
      `installResizeObserverStub()`, cubriendo RN-07 a RN-10 y FR-025: con dos series de 3 puntos,
      la `figure` tiene como nombre accesible el título de su `<figcaption>`, el SVG contiene los
      rótulos de los dos ejes y la leyenda muestra los dos nombres de serie (RN-08); la tabla "Datos
      de <título>" tiene una fila por sprint en el orden recibido, con puntos en orden no alfabético
      ni por valor (RN-07); un punto con `value: 0` aparece como "0" en su fila (FR-025); un punto
      `partial` lleva "Parcial" en la tabla y el sufijo "(parcial)" en el nombre del sprint del eje
      X, y uno `provisional` lleva "Provisoria"; con un punto se dibuja y muestra "Todavía no hay
      evolución que comparar." (RN-09); con cero puntos no hay SVG ni tabla y se muestra el
      `emptyMessage` (RN-10); dos series con sprints distintos o en distinto orden muestran el
      `errorMessage` en lugar del gráfico, sin reordenar — depende de T016
- [X] T021 [US3] Pruebas de escenario en
      `frontend/src/features/dashboard/components/velocity-chart.test.tsx`, montando `DashboardView`
      con `renderDashboard` e `installResizeObserverStub()`, con
      `describe("009/US3 - Ver la evolución de la velocidad")` y un `it` por escenario según plan.md,
      cada uno precedido por `// Escenario: US3-k (<caso>)`, que verifican los valores en la tabla de
      la figura "Velocidad por sprint": (1) los 4 pares 21/13, 24/21, 13/8 y 25/20 en el orden
      recibido, con leyenda "Story Points planificados" y "Story Points completados", título y ejes
      "Sprint" y "Story Points"; (2) 3 Cerrados y un Activo en las series: 3 filas y ningún
      "(parcial)"; (3) un único Cerrado: una fila y "Todavía no hay evolución que comparar.";
      (4) series vacías: "Todavía no hay sprints cerrados para mostrar la velocidad." y ningún SVG;
      (5) 20 Cerrados con nombres "Sprint 1" a "Sprint 20": 20 filas, cada nombre una sola vez en el
      eje X y en la tabla, cada fila con su par de valores (SC-009); (6) un Cerrado con 0 completados:
      su fila muestra "0"; (7) `plannedStoryPoints` malformada: "No se pudo cargar el gráfico de
      velocidad." y el resumen y los indicadores se muestran — depende de T004, T018

### Implementación de la Historia 3

- [X] T022 [P] [US3] Implementar en `frontend/src/features/dashboard/series.ts`
      `closedSprintPoints(points: SeriesPoint[]): SeriesPoint[]`, que solo omite los puntos con
      `sprintStatus === "active"`, sin ordenar, sumar ni completar, con el comentario de research.md
      sección 8 ("única selección que hace el dashboard; la velocidad se mide sobre sprints
      terminados, US3-2") — depende de T019
- [X] T023 [P] [US3] Implementar en `frontend/src/features/dashboard/api.ts` `isSeriesEnvelope`
      (solo verifica que el cuerpo es un objeto), el reconocedor de `SeriesPoint`,
      `readSeries(envelope, key): SeriesPoint[] | null` y `getProjectSeries(projectId)` con `getJson`
      sobre `.../metrics/series`, según data-model.md sección 3 — depende de T019
- [X] T024 [US3] Implementar en `frontend/src/features/dashboard/components/sprint-series-chart.tsx`
      el componente común de los tres gráficos (research.md sección 9): propiedades `title`,
      `yAxisLabel`, `kind` (para `MetricValue`), `format`, `emptyMessage`, `errorMessage` y
      `series: [SeriesInput, SeriesInput]` con `name` y `points`. Si las dos series no tienen los
      mismos `sprintId` en el mismo orden, muestra `SectionError` con `errorMessage`. Con cero
      puntos, muestra `emptyMessage` sin ejes. Si no, una `<figure>` con `<figcaption>` y nombre
      accesible igual al título; `ResponsiveContainer` con `initialDimension` de 800 × 320;
      `BarChart` de Recharts con dos `Bar` y los nombres de la leyenda; `XAxis` con `interval={0}`,
      etiquetas inclinadas, rótulo "Sprint" y el sufijo " (parcial)" en el sprint Activo; `YAxis` con
      el rótulo `yAxisLabel`; `Legend` y `Tooltip`; sin `minPointSize`. Las filas del gráfico se
      arman por posición, en el orden recibido. Debajo, un `<details>` "Ver datos" con la tabla "Datos
      de <título>", una fila por sprint y cada valor con `MetricValue`. Con un punto, agrega
      "Todavía no hay evolución que comparar." — depende de T020, T016
- [X] T025 [US3] Implementar en `frontend/src/features/dashboard/components/velocity-chart.tsx`
      `VelocityChart({ series })`, que recibe el `QueryState` de las series, lee `plannedStoryPoints`
      y `completedStoryPoints` con `readSeries`, aplica `closedSprintPoints` a las dos y monta
      `SprintSeriesChart` con el título "Velocidad por sprint", el eje "Story Points", las series
      "Story Points planificados" y "Story Points completados", y los mensajes de vacío y de falla de
      contracts/ui-contract.md; si una porción es `null` o la consulta falló, muestra
      `SectionError` con "No se pudo cargar el gráfico de velocidad.". Sumar en `dashboard-view.tsx`
      la **única** consulta de series y montar `VelocityChart` después de los indicadores — depende
      de T021, T022, T023, T024

**Checkpoint**: US3 funciona sola: la velocidad se lee como tendencia, solo con sprints Cerrados y en
el orden de la 008.

---

## Fase 6: Historia de Usuario 4 — Ver la evolución del esfuerzo (Prioridad: P2)

**Objetivo**: el integrante compara, sprint por sprint, las horas estimadas con las reales, con el
sprint Activo marcado como parcial.

**Rama**: `hu/009-us4-ver-la-evolucion-del-esfuerzo`

**Prueba independiente**: con 3 sprints de 126/144/108 horas estimadas y 142,5/130/95 reales, la
figura "Esfuerzo por sprint" muestra las dos series en orden, y con un sprint Activo en las series su
fila lleva "Parcial".

### Pruebas de la Historia 4 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T026 [US4] Pruebas de escenario en
      `frontend/src/features/dashboard/components/effort-chart.test.tsx`, montando `DashboardView`
      con `renderDashboard` e `installResizeObserverStub()`, con
      `describe("009/US4 - Ver la evolución del esfuerzo")` y un `it` por escenario según plan.md,
      cada uno precedido por `// Escenario: US4-k (<caso>)`, que verifican la tabla de la figura
      "Esfuerzo por sprint": (1) "126 h"/"142,5 h", "144 h"/"130 h" y "108 h"/"95 h" en orden, con
      leyenda "Horas estimadas" y "Horas reales", título y ejes "Sprint" y "Horas"; (2) con un Activo
      en las series, su fila lleva "Parcial" en los dos valores y su nombre lleva "(parcial)" en el
      eje X; (3) estimadas `0` con reales registradas: la fila muestra "0 h" y el valor real;
      (4) un sprint sin horas reales: "0 h" en la serie de reales; (5) series vacías: "Todavía no hay
      sprints iniciados para mostrar esfuerzo." y ningún SVG; (6) `actualHours` malformada: "No se
      pudo cargar el gráfico de esfuerzo." y el gráfico de velocidad se dibuja igual — depende de
      T004, T025

### Implementación de la Historia 4

- [X] T027 [US4] Implementar en `frontend/src/features/dashboard/components/effort-chart.tsx`
      `EffortChart({ series })` sobre `SprintSeriesChart`, con `estimatedHours` y `actualHours`
      leídas con `readSeries` y **sin** `closedSprintPoints` (US4-2), título "Esfuerzo por sprint",
      eje "Horas", series "Horas estimadas" y "Horas reales", formato `formatHours` y los mensajes de
      contracts/ui-contract.md; montarlo en `dashboard-view.tsx` después de `VelocityChart`, con la
      misma consulta de series — depende de T026

**Checkpoint**: US4 funciona sola: el sprint en curso no se lee como una mejora de estimación.

---

## Fase 7: Historia de Usuario 5 — Ver la calidad del producto (Prioridad: P2)

**Objetivo**: el integrante ve los defectos detectados y resueltos por sprint y la distribución de
los abiertos en las cuatro severidades, con su total.

**Rama**: `hu/009-us5-ver-la-calidad-del-producto`

**Prueba independiente**: con 7, 4 y 9 defectos detectados y 4, 6 y 2 resueltos, la figura "Defectos
por sprint" los muestra en orden; con 7 abiertos repartidos en 2/3/1/1, la región "Defectos abiertos
por severidad" muestra las cuatro cantidades y "Total" 7.

### Pruebas de la Historia 5 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T028 [US5] Pruebas de escenario en
      `frontend/src/features/dashboard/components/quality-chart.test.tsx`, montando `DashboardView`
      con `renderDashboard` e `installResizeObserverStub()`, con
      `describe("009/US5 - Ver la calidad del producto")` y un `it` por escenario según plan.md, cada
      uno precedido por `// Escenario: US5-k (<caso>)`: (1) la tabla de la figura "Defectos por
      sprint" con 7/4, 4/6 y 9/2 en orden, leyenda "Defectos detectados" y "Defectos resueltos",
      título y ejes "Sprint" y "Defectos"; (2) la región "Defectos abiertos por severidad" con
      Crítica 2, Alta 3, Media 1, Baja 1 y "Total" 7, con los nombres de `DEFECT_SEVERITY_LABELS` en
      ese orden; (3) un sprint con 2 detectados y 6 resueltos: los dos valores, sin ningún
      `role="alert"`; (4) `open.total: 0` y las cuatro en 0: las cuatro en "0", "Total" 0 y "No hay
      defectos abiertos.", sin `role="alert"`; (5) solo Críticos: 4 en Crítica y 0 en las otras tres;
      (6) la distribución muestra tal cual los conteos de la 008, que ya excluyen
      los Descartados (RC-07): no aparece ningún texto "Descartado" y el dashboard no vuelve a
      filtrar; además, el "Total" es `open.total` y no la suma de las cuatro severidades, lo que se
      verifica con un cuerpo de prueba donde difieren (RN-01); (7) `defectsDetected` malformada:
      "No se pudo cargar el gráfico de defectos." y los gráficos de velocidad y de esfuerzo, el
      resumen y los indicadores se muestran — depende de T004, T027

### Implementación de la Historia 5

- [X] T029 [P] [US5] Implementar en
      `frontend/src/features/dashboard/components/open-defects-distribution.tsx`
      `OpenDefectsDistribution({ projectMetrics })`, que recibe el `QueryState` de las métricas del
      proyecto: región "Defectos abiertos por severidad" con un `<dl>` de Crítica, Alta, Media y Baja
      según `DEFECT_SEVERITY_LABELS` y `bySeverity`, y "Total" con `open.total`, **sin sumar**; con
      total 0 agrega "No hay defectos abiertos." como texto informativo; su "Cargando…" y su
      `SectionError` con "No se pudo cargar la distribución de defectos abiertos." (research.md
      sección 10) — depende de T028
- [X] T030 [US5] Implementar en `frontend/src/features/dashboard/components/quality-chart.tsx`
      `QualityChart({ series })` sobre `SprintSeriesChart`, con `defectsDetected` y
      `defectsResolved`, sin `closedSprintPoints`, título "Defectos por sprint", eje "Defectos",
      formato `String` y los mensajes de contracts/ui-contract.md; montar en `dashboard-view.tsx`
      `QualityChart` después de `EffortChart` y `OpenDefectsDistribution` al final, con la consulta
      de métricas del proyecto de US2 — depende de T028, T029

**Checkpoint**: US5 funciona sola: el eje de calidad se ve independiente del de esfuerzo.

---

## Fase 8: Historia de Usuario 6 — Usar el dashboard cuando falta información o falla una sección (Prioridad: P3)

**Objetivo**: el dashboard de un proyecto sin sprints explica qué hacer, las advertencias de la 008
se muestran en sus dos clases, la falla de una sección no tumba a las demás y la del resumen invita a
reintentar.

**Rama**: `hu/009-us6-usar-el-dashboard-cuando-falta-informacion-o-falla-una-seccion`

**Prueba independiente**: con `emptyReason: no_sprints` el dashboard muestra el resumen y "Sin datos
todavía" en lugar de indicadores y gráficos; con un `500` en el resumen muestra "No se pudo mostrar
el dashboard." y, al pulsar "Reintentar" con el backend ya respondiendo, el dashboard completo.

### Pruebas de la Historia 6 ⚠️

> **Escribir estas pruebas PRIMERO y verificar que FALLAN antes de implementar.**

- [X] T031 [P] [US6] Pruebas unitarias en `frontend/src/hooks/use-authorized-query.test.tsx`: con
      una carga que devuelve un error con `status: 500`, el estado de error trae `httpStatus: 500` y el
      `message`; con `status: 0` (red caída), trae `httpStatus: 0`; con datos, `loaded` como hoy
      (research.md sección 4) — sin dependencias
- [X] T032 [P] [US6] Pruebas unitarias **obligatorias** de `DashboardWarnings` en
      `frontend/src/features/dashboard/components/dashboard-warnings.test.tsx`, cubriendo RN-21:
      con una advertencia `planning` y una `data_inconsistency`, la región "Advertencias" tiene la
      lista "Avisos de planificación" con `role="status"` y la lista "Datos que hay que revisar" con
      `role="alert"`, con el texto introductorio de que hay información rota que alguien debe
      revisar, y cada una muestra el `message` recibido sin cambios; con solo `planning` no hay
      `role="alert"`; con la lista vacía no se muestra la región — depende de T002
- [X] T033 [US6] Pruebas de escenario en
      `frontend/src/features/dashboard/components/dashboard-view.test.tsx`, montando `DashboardView`
      con `renderDashboard` e `installResizeObserverStub()`, con
      `describe("009/US6 - Usar el dashboard cuando falta información o falla una sección")` y un
      `it` por escenario según plan.md, cada uno precedido por `// Escenario: US6-k (<caso>)`:
      (1) `emptyReason: no_sprints` y backlog vacío: región "Sin datos todavía" con el texto de
      contracts/ui-contract.md, y ninguna región "Indicadores clave", ninguna figura ni distribución;
      (2) `no_sprints` con historias: resumen e integrantes completos y "Sin datos todavía";
      (3) una advertencia `planning` de 6 historias sin estimar: se muestra su `message`;
      (4) una `data_inconsistency`: en la lista con `role="alert"`, separada de la de planificación,
      y los indicadores se muestran igual; (5) `defectsDetected` malformada: solo la figura de
      calidad muestra su mensaje de falla, y el resumen, los indicadores, la velocidad y el esfuerzo
      están completos; (6) `500` en las métricas del proyecto y en las series: cada sección afectada
      muestra su propio mensaje (indicadores del proyecto, los tres gráficos, la distribución y las
      advertencias), y el resumen y el grupo "Indicadores del sprint activo" se muestran; (7) primero con las series
      en `500`, después `unmount`, respuesta corregida y `render`: los gráficos se dibujan;
      (8) primero con sprint activo, después `unmount`, el resumen con `activeSprint: null` y las
      series con el sprint ya Cerrado, y `render`: "No hay ningún sprint en curso." y el sprint en la
      tabla de velocidad (SC-013); (9) resumen en `500`: "No se pudo mostrar el dashboard." con
      `role="alert"` y el botón "Reintentar", ninguna otra sección; con el backend corregido, pulsar
      "Reintentar" muestra el dashboard; con un `404`, **no** hay botón. Agregar, **sin** etiqueta
      `009/USn`: un `it` por cada una de las cinco consultas degradables (sprint activo, listado de
      sprints, métricas del proyecto, métricas del sprint activo, series) en `500` de a una, que
      verifica que el resumen se muestra y que solo fallan las secciones de esa consulta (RN-15,
      SC-010); con el listado de sprints en `500`, las secciones se muestran normalmente; con el listado de
      sprints todavía sin responder, se ve "Cargando métricas…" y ninguna región "Indicadores clave"; y
      `it("no vuelve a consultar sin recargar")` (RN-17) con `vi.useFakeTimers()`: después de cargar,
      adelantar 10 minutos no agrega ninguna petición — depende de T004, T030

### Implementación de la Historia 6

- [X] T034 [P] [US6] Sumar `httpStatus: number` al caso de error de `QueryState` en
      `frontend/src/hooks/use-authorized-query.ts` (`{ status: "error"; httpStatus: number; message:
      string }`; no se llama `status` porque ese ya es el discriminante), tomado de `result.error.status`, y
      actualizar el comentario del tipo. El cambio es compatible con los usos actuales, que solo
      leen `message`: verificar que `npm test` sigue pasando — depende de T031
- [X] T035 [P] [US6] Implementar en
      `frontend/src/features/dashboard/components/dashboard-warnings.tsx`
      `DashboardWarnings({ warnings }: { warnings: MetricsWarning[] })` según research.md sección 11,
      sin cambiar ningún `message` — depende de T032
- [ ] T036 [P] [US6] Implementar en
      `frontend/src/features/dashboard/components/dashboard-empty-state.tsx` `DashboardEmptyState()`:
      región "Sin datos todavía" con el texto de contracts/ui-contract.md, sin enlaces (la
      navegación hacia los módulos está fuera de alcance) — depende de T033
- [ ] T037 [US6] Completar `frontend/src/features/dashboard/components/dashboard-view.tsx`: la
      consulta del listado de sprints con `listSprints(projectId)` sin filtros; con
      `emptyReason === "no_sprints"`, `DashboardEmptyState` en lugar de los indicadores, los tres
      gráficos y la distribución (FR-027); mientras esa consulta carga, en lugar de esas secciones se
      muestra un único "Cargando métricas…" con `role="status"`; si falla, las secciones se muestran
      normalmente, cada una con su propio estado vacío; `DashboardWarnings` antes de los indicadores con
      `projectMetrics.warnings`, también en el estado vacío, y su `SectionError` si falla la
      consulta; el reintento del resumen: con un error cuyo `httpStatus` no es `404`, "No se pudo
      mostrar el dashboard." más el `message`, y un `Button` "Reintentar" que incrementa un contador
      incluido en las dependencias del `useCallback` de la carga del resumen (research.md
      sección 4); con `404`, solo el mensaje — depende de T034, T035, T036

**Checkpoint**: las seis historias funcionan; la pantalla de un proyecto recién creado no se ve rota
y ninguna falla parcial tumba el dashboard.

---

## Fase 9: Polish (Cierre y asuntos transversales)

**Propósito**: pruebas transversales, documentación viva, cobertura, auditoría de dependencias y
verificación de las puertas de calidad.

**Rama**: `fase/009-polish`

- [ ] T038 Prueba de caracterización en
      `frontend/src/features/dashboard/components/dashboard-view.test.tsx`, **sin** etiqueta
      `009/USn`: `it("solo hace consultas GET")` (RN-02, FR-003), que monta el dashboard completo con
      sprint activo y recorre la lista de peticiones del backend de prueba verificando que todas son
      `GET` y que son exactamente las seis operaciones de contracts/consumed-api.md, cada una una
      sola vez, y que no hay ningún botón de edición en la pantalla. Caracterización: la regla ya se
      cumple, así que no tiene RED; el commit lo justifica — depende de T037
- [ ] T039 [P] Actualizar `README.md` de la raíz en español: la sección "Panel de Control y Reportes"
      describe el dashboard (secciones, estados de valor y degradación por sección), la tabla de
      "Arquitectura y Stack Tecnológico" suma Recharts 3.10.1 para los gráficos, y "Comprobar que
      todo funciona" suma abrir `/projects/{id}/dashboard`, manteniendo el criterio de que alguien
      que no conoce el proyecto pueda ejecutarlo siguiendo el documento — depende de T037
- [ ] T040 [P] Ejecutar en `frontend/` `npm run test:coverage`, `npm run lint`,
      `npm run format:check` y `npm run build`, y revisar que `src/features/dashboard/`,
      `src/utils/metric-format.ts` y `src/utils/metric-explanations.ts` tengan cubiertas todas sus
      ramas; ejecutar `npm audit --omit=dev` y resolver o justificar por escrito cada vulnerabilidad
      de `recharts`, `react-is` o sus dependencias — depende de T038
- [ ] T041 Verificar la trazabilidad y las puertas de calidad recorriendo `frontend/src/`: que buscar
      `009/US` en el código de pruebas devuelva los **seis** `describe` y los **45** `it` de
      escenario; que las **21** reglas RN-01 a RN-21 tengan prueba asociada según la tabla de
      plan.md (SC-016); que no haya ningún `any`, ni archivos `AGENTS.md`, ni cambios en `backend/`;
      que `package.json` solo sume `recharts` y `react-is` con versión exacta; que, revisado en el
      diff, ningún componente de `features/dashboard/` opere sobre valores de métricas o de series,
      salvo `closedSprintPoints` (RN-01); y que cada bloque nuevo tenga su
      comentario en español — depende de T038
- [ ] T042 Ejecutar de punta a punta la [guía de validación](./quickstart.md) completa, incluida la
      validación manual de la sección 2 contra el backend de desarrollo y la comparación de cada
      cifra con Swagger (SC-004) — depende de T039, T040, T041

---

## Phase 10: Convergence and Security

Propósito: ajustes transversales detectados por /speckit-converge y /revisar-seguridad.

- [ ] T043 Actualizar `undici` a 8.12.2 o superior en `frontend/package.json` y `frontend/package-lock.json` (con `npm audit fix` o subiendo `jsdom`) y confirmar que `npm test` sigue en verde per hallazgo de seguridad: undici vulnerable en dependencia de desarrollo (security) — origen: seguridad, detectado en 009/fase-foundational
- [ ] T044 Actualizar `zod` a 4.6.6 o superior en `frontend/package-lock.json` (con `npm audit fix` o subiendo `eslint-config-next`) y confirmar que `npm run lint` sigue funcionando per hallazgo de seguridad: zod vulnerable en dependencia de desarrollo (security) — origen: seguridad, detectado en 009/fase-foundational

---

## Dependencias y Orden de Ejecución

### Dependencias entre fases

- **Fases 1 y 2 Setup y Foundational**: parten de `main` con la 001 a la 008 integradas, en una
  única rama. **BLOQUEAN** todas las historias.
- **Fases 3 a 8 (historias)**: todas dependen de la Fase 2, en orden de prioridad
  (P1 → P1 → P2 → P2 → P2 → P3).
- **Fase 9 Polish**: depende de que estén completas las seis historias.

### Dependencias entre historias

Las historias **no son todas independientes en su implementación**, aunque cada una se prueba sola,
porque todas crecen sobre `DashboardView`:

- **US1 Resumen (P1)**: puede empezar después de la Fase 2. **Aporta** la consulta bloqueante del
  resumen, sobre la que se montan todas las demás.
- **US2 Indicadores (P1)**: necesita US1. **Aporta** `MetricValue`, `FinalityBadge`, `SectionError`,
  los formateadores, los reconocedores y la consulta de métricas del proyecto.
- **US3 Velocidad (P2)**: necesita US2 (`MetricValue` y `SectionError`). **Aporta**
  `SprintSeriesChart` y la consulta de series.
- **US4 Esfuerzo (P2)**: necesita US3.
- **US5 Calidad (P2)**: necesita US3 para el gráfico y US2 para la distribución. No depende de US4,
  pero sus pruebas montan el dashboard con US4 ya hecha.
- **US6 Fallas y estado vacío (P3)**: necesita las cinco anteriores, porque prueba la degradación
  cruzada de todas las secciones.

### Dentro de cada historia

- Las pruebas se escriben y **fallan** antes de la implementación (Principio II).
- Utilidades y reconocedores antes que componentes; componentes antes que su montaje en
  `DashboardView`.
- Una historia se termina antes de pasar a la siguiente prioridad.

### Oportunidades de paralelismo

- Fase 2: T002 y T003 en paralelo; T004 después, porque usa los tipos y monta `DashboardView`.
- US1: T005 en paralelo con T006; T007 y T008 en paralelo.
- US2: T010, T011 y T012 en paralelo; T014 y T015 en paralelo.
- US3: T019 y T020 en paralelo; T022 y T023 en paralelo.
- US5: T029 antes que T030, porque T030 monta los dos componentes en `dashboard-view.tsx`.
- US6: T031 y T032 en paralelo; T034, T035 y T036 en paralelo.
- Las tareas que tocan `dashboard-view.tsx` (T009, T018, T025, T027, T030, T037) **no** son
  paralelizables entre sí.
- Fase 9: T039 y T040 en paralelo.

---

## Ejemplo de paralelismo: Historia de Usuario 2

```text
# Pruebas en archivos distintos:
T010  frontend/src/utils/metric-format.test.ts + metric-explanations.test.ts
T011  frontend/src/features/dashboard/components/metric-value.test.tsx
T012  frontend/src/features/dashboard/api.test.ts

# Implementaciones independientes entre sí:
T014  frontend/src/utils/metric-format.ts + metric-explanations.ts
T015  frontend/src/features/dashboard/api.ts
```

## Ejemplo de paralelismo: Historia de Usuario 3

```text
# Pruebas en archivos distintos:
T019  frontend/src/features/dashboard/series.test.ts + api.test.ts
T020  frontend/src/features/dashboard/components/sprint-series-chart.test.tsx

# Implementaciones independientes entre sí:
T022  frontend/src/features/dashboard/series.ts
T023  frontend/src/features/dashboard/api.ts
```

## Ejemplo de paralelismo: Historia de Usuario 6

```text
# Pruebas en archivos distintos:
T031  frontend/src/hooks/use-authorized-query.test.tsx
T032  frontend/src/features/dashboard/components/dashboard-warnings.test.tsx

# Implementaciones independientes entre sí:
T034  frontend/src/hooks/use-authorized-query.ts
T035  frontend/src/features/dashboard/components/dashboard-warnings.tsx
T036  frontend/src/features/dashboard/components/dashboard-empty-state.tsx
```

---

## Estrategia de Implementación

### MVP primero (solo US1)

1. Completar las Fases 1 y 2 (Setup y Foundational). **CRÍTICO**: bloquean todas las historias.
2. Completar la Fase 3 (US1 Resumen y sprint en curso).
3. **DETENERSE y VALIDAR**: probar US1 de forma independiente con los pasos 1, 3, 7 y 9 de la
   sección 2 de la guía de validación.
4. Abrir el pull request de la historia con `/redactar-pr` y demostrar el incremento.

### Entrega incremental

1. Setup y Foundational → base lista (un pull request).
2. US1 Resumen → probar sola → entregar (**MVP**: el integrante sabe de qué proyecto se trata y qué
   se está haciendo ahora).
3. US2 Indicadores → probar sola → entregar (el mínimo útil del dashboard, sin gráficos).
4. US3 Velocidad → probar sola → entregar (la primera tendencia).
5. US4 Esfuerzo → probar sola → entregar.
6. US5 Calidad → probar sola → entregar.
7. US6 Fallas y estado vacío → probar sola → entregar (el primer día del proyecto no se ve roto).
8. Polish → cerrar la feature.

Cada historia añade valor sin romper las anteriores.

### Estrategia con varias personas

1. El equipo completa Setup y Foundational en conjunto.
2. US1 y US2 en secuencia: aportan la vista y los presentadores que usan todas.
3. Terminada US2, una persona puede avanzar `SprintSeriesChart` (US3) mientras otra prepara las
   pruebas de US5, coordinando `dashboard-view.tsx`, que todas tocan.
4. US6 al final, porque prueba la degradación de todas las secciones.

---

## Notas

- Las tareas `[P]` usan archivos distintos y no tienen dependencias pendientes.
- La etiqueta `[USn]` traza la tarea hasta su historia. Buscar `009/US1` a `009/US6` en el código de
  pruebas devuelve la trazabilidad completa de los 45 escenarios, que exige el Principio II.
- Cada tarea se ejecuta con `/speckit-implement Txxx`, una por invocación.
- Cada commit corresponde a una sola tarea y lleva el pie `Spec: 009/Txxx` más `Closes #N` o
  `Refs #N` (Principio IX, ámbito b), con el ámbito `dashboard`. Los commits los registra **una
  persona**, nunca un agente.
- Las pruebas de esta feature no usan la base de datos: sustituyen `fetch`. Nunca se apuntan al
  backend de desarrollo (Principio V).
- **Dependencias nuevas**: solo `recharts@3.10.1` y `react-is@19.3.0`, autorizadas el 2026-10-28
  (plan, punto 1). Si alguna tarea parece necesitar otra, detenerse y pedir autorización
  (Principio IV).
- **Pendiente fuera de esta lista, del responsable humano**: la enmienda PATCH de la constitución
  que suma Recharts a la tabla de dependencias autorizadas (plan, punto 1). No es una tarea de
  agente, porque la enmienda requiere la aprobación explícita del responsable del proyecto.
- **Código de otras features que esta feature modifica**, todo señalado en el plan: el enlace
  "Dashboard" de `ProjectOverview` (T009) y el código HTTP en el error de `useAuthorizedQuery`
  (T034). El campo se llama `httpStatus` porque `status` ya es el discriminante del estado.
- **Interpretaciones incorporadas a la spec el 2026-10-28** (plan, punto 2): el gráfico de
  velocidad omite el Activo (T022, T025), los indicadores van en dos grupos (T018), el proyecto
  Finalizado muestra la sección del sprint activo con "No hay ningún sprint en curso." (T006) y
  US4-5 es la serie vacía (T026).
- **Desviación justificada (Principio VIII, puerta "Swagger y `README.md` reflejan el estado
  actual")**: Swagger no cambia, porque no hay endpoints nuevos. La descripción del dashboard en el
  `README.md` se agrega una sola vez en la Fase 9 (T039). Entre el merge de una historia y la Fase 9,
  el README no menciona la pantalla ya integrada. Es el mismo criterio que aplicaron la 001 a la 008.

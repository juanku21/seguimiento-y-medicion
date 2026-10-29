# Modelo de Datos: Dashboard del Proyecto

**Feature**: `specs/009-project-dashboard` | **Fecha**: 2026-10-28 | **Plan**: [plan.md](./plan.md)

El dashboard es una **vista derivada sin estado propio** (spec, Entidades Clave). No crea tablas, ni
columnas, ni índices, ni migraciones, y no guarda nada en el navegador. Este documento describe:

1. Los tipos de TypeScript de las respuestas de la 008 que consume, que hoy no existen en el frontend.
2. Los tipos que reutiliza de las features 002, 004 y 007.
3. El estado de cada sección y las reglas de presentación que aplican.

Los nombres de los tipos y los campos son los de los contratos de origen. La forma exacta de cada
respuesta está en [contracts/consumed-api.md](./contracts/consumed-api.md).

---

## 1. Tipos nuevos: `frontend/src/types/metrics.ts`

Transcriben los esquemas de `specs/008-metrics-calculation/contracts/openapi.yaml`. Solo se declaran
los campos que el dashboard usa (Principio III: nada especulativo). Los que la 008 envía y el
dashboard no muestra, como `effortOutsideSprintsHours`, quedan fuera del tipo.

| Tipo | Campos que usa el dashboard | Origen en la 008 |
| --- | --- | --- |
| `ValueStatus` | `"available" \| "not_calculable" \| "not_applicable"` | `ValueStatus` |
| `Finality` | `"definitive" \| "provisional" \| "partial"` | `Finality` |
| `Metric` | `status: ValueStatus`, `value: number \| null` | `Metric` |
| `SprintMetric` | `Metric` más `finality: Finality \| null` | `SprintMetric` |
| `ActiveSprintMetrics` | `plannedStoryPoints`, `completedStoryPoints`, `estimatedHours`, `actualHours`, `deviationHours`, `deviationPercent`, `storyCompletionPercent`: `SprintMetric` | `SprintMetrics` |
| `ActiveSprintMetricsResponse` | `activeSprint: boolean`, `metrics: ActiveSprintMetrics \| null` | `ActiveSprintMetricsResponse` |
| `Cumulative` | `finality: "definitive" \| "partial"`, `plannedStoryPoints`, `completedStoryPoints`, `estimatedHours`, `actualHours`, `deviationHours`, `deviationPercent`, `storyCompletionPercent`: `Metric` | `Cumulative` |
| `SeverityBreakdown` | `critical`, `high`, `medium`, `low`: `number` | `SeverityBreakdown` |
| `DefectTally` | `total: number`, `bySeverity: SeverityBreakdown` | `DefectTally` |
| `WarningKind` | `"planning" \| "data_inconsistency"` | `Warning.kind` |
| `MetricsWarning` | `kind: WarningKind`, `code: string`, `message: string` | `Warning` |
| `ProjectMetricsResponse` | `velocity: { value: Metric }`, `cumulative: Cumulative`, `defects: { open: DefectTally }`, `warnings: MetricsWarning[]` | `ProjectMetricsResponse` |
| `SeriesPoint` | `sprintId`, `sprintName`, `sprintStatus: "active" \| "closed"`, `status: ValueStatus`, `value: number \| null`, `finality: Finality` | `SeriesPoint` |
| `ProjectSeriesResponse` | Las seis series: `plannedStoryPoints`, `completedStoryPoints`, `estimatedHours`, `actualHours`, `defectsDetected`, `defectsResolved`: `SeriesPoint[]` | `ProjectSeriesResponse` |

**Notas**:

- `MetricsWarning.code` se tipa como `string` y no como la unión de los cuatro códigos, porque el
  dashboard no decide nada según el código: muestra `message` y agrupa por `kind`. Un código nuevo de
  la 008 no rompe el dashboard.
- `SprintMetric.finality` admite `null` porque el contrato lo admite (sprint Planificado), aunque el
  endpoint del sprint activo siempre envía `partial`.
- `ProjectSeriesResponse.warnings` y `SprintMetrics.warnings` no se declaran: las advertencias salen
  solo de `ProjectMetricsResponse` ([research.md](./research.md), sección 3).

---

## 2. Tipos reutilizados de otras features

| Tipo | Archivo | Campos que usa el dashboard |
| --- | --- | --- |
| `ProjectDetail` (002) | `types/project.ts` | `id`, `name`, `status`, `startDate`, `plannedEndDate`, `actualEndDate`, `members`, `activeSprint` |
| `ActiveSprintRef` (002) | `types/project.ts` | `id` |
| `SprintDetail` (004) | `types/sprint.ts` | `name`, `goal`, `startDate`, `endDate`, `schedule` |
| `SprintSchedule` (004) | `types/sprint.ts` | `remainingDays`, `overdueDays` |
| `SprintListResponse` (004) | `types/sprint.ts` | `emptyReason` |
| `DefectSeverity` (007) | `types/defect.ts` | Las cuatro severidades, para los nombres de la distribución |

Las funciones de consulta también se reutilizan: `getProjectDetail` de `features/projects/api.ts` y
`getSprint` de `features/sprints/api.ts`. Para el listado, se reutiliza `listSprints` de `features/sprints/api.ts`, que pide
`GET /api/v1/projects/{projectId}/sprints`, llamada sin filtros.

---

## 3. Validación de las respuestas

El cliente HTTP (`lib/api-client.ts`) exige una función que reconozca el cuerpo antes de entregarlo,
sin `any` (Principio VIII). Las de la 008 viven en `features/dashboard/api.ts`:

| Función | Qué verifica |
| --- | --- |
| `isMetric(value)` | `status` es uno de los tres valores; `value` es número si `status` es `available` y `null` en los otros dos |
| `isSprintMetric(value)` | `isMetric` y `finality` es uno de los tres valores o `null` |
| `isActiveSprintMetricsResponse(value)` | `activeSprint` booleano; con `true`, las siete métricas cumplen `isSprintMetric`; con `false`, `metrics` es `null` |
| `isProjectMetricsResponse(value)` | `velocity.value`, el acumulado y su `finality`, `defects.open` con las cuatro severidades y `total`, y `warnings` como lista de advertencias con `kind`, `code` y `message` |
| `isSeriesEnvelope(value)` | Solo que el cuerpo es un objeto. La validación de cada serie la hace cada gráfico |
| `readSeries(envelope, key)` | Devuelve `SeriesPoint[]` si `envelope[key]` es una lista de puntos válidos, o `null` si no lo es |

**Regla de la validación por porción** ([research.md](./research.md), sección 3): un gráfico se
dibuja solo si **sus dos** series son válidas. Si una no lo es, ese gráfico informa que no pudo
cargarse y los otros dos se dibujan igual. Es lo que permite que el gráfico de calidad falle solo
(US6-5) aunque los tres gráficos salgan de la misma consulta (FR-026).

---

## 4. Estado de cada sección

Cada sección tiene su propia consulta con `useAuthorizedQuery` y, por lo tanto, su propio
`QueryState`:

```text
loading ──► loaded(data)
   │
   └──────► error(httpStatus, message)
```

`QueryState` ya existe en `hooks/use-authorized-query.ts`. Esta feature le suma `httpStatus: number` al
caso de error (el código HTTP, o `0` si falló la red), que hoy solo trae `message`. El cambio es
compatible con todos los usos actuales.

| Sección | Consulta | `loading` | `error` | `loaded` |
| --- | --- | --- | --- | --- |
| Resumen | `getProjectDetail` | "Cargando dashboard…" y nada más | Con `404`: el mensaje del backend y nada más. Con otro código: "No se pudo mostrar el dashboard." y el botón "Reintentar" (FR-030) | Se muestra el resumen y se montan las demás secciones |
| Sprint activo | `getSprint(activeSprint.id)`, solo si `activeSprint` no es `null` | "Cargando sprint activo…" | "No se pudo cargar el sprint activo." | Nombre, Sprint Goal, fechas y `SprintSchedule` |
| Hay sprints | listado de sprints sin filtros | Las secciones dependientes esperan: un único "Cargando métricas…" | Se muestran las secciones normalmente | Con `emptyReason: no_sprints`, el estado vacío del proyecto reemplaza indicadores, gráficos y distribución |
| Métricas del proyecto | `getProjectMetrics` | Cada sección que la usa muestra su propio "Cargando…" | Cada una de las tres secciones lo informa por separado | Indicadores del proyecto, distribución y advertencias |
| Sprint activo (métricas) | `getActiveSprintMetrics` | Ídem | "No se pudieron cargar los indicadores del sprint activo." | Indicadores del sprint activo, o "No hay ningún sprint en curso." |
| Series | `getProjectSeries` | Cada gráfico muestra su "Cargando…" | Cada uno de los tres gráficos lo informa por separado | Cada gráfico valida su porción (sección 3) |

**Proyecto sin sprint Activo**: si `ProjectDetail.activeSprint` es `null`, el bloque del sprint
activo dice "No hay ningún sprint en curso." sin pedir nada (FR-011). No es un error ni un estado
vacío del proyecto.

---

## 5. Reglas de presentación y dónde se aplican

Las reglas de negocio de la spec son todas de presentación: ninguna escribe ni calcula. Cada una
tiene un único lugar en el código, que es el que la prueba unitaria ejercita.

| Regla | Dónde se aplica |
| --- | --- |
| RN-01, FR-002, FR-004, FR-013: no calcular ni derivar | Ningún componente hace aritmética sobre las cifras. Los formateadores solo convierten el número recibido en texto (`utils/metric-format.ts`, `utils/effort-format.ts`) |
| RN-02, FR-003: solo lectura | `features/dashboard/api.ts` solo usa `getJson`. La lista de integrantes reutiliza `MemberList` con `viewerIsOwner={false}`, que no ofrece acciones |
| RN-03, FR-014: "no calculable" con explicación | `MetricValue` y `explainNotCalculable` |
| RN-04, RN-05, RN-06, FR-015 a FR-017: parcial, provisoria y los tres estados | `MetricValue`, y `FinalityBadge` para las marcas de bloque y de punto |
| RN-07, RN-20, FR-021, FR-026: orden cronológico de la 008 | `SprintSeriesChart` dibuja los puntos en el orden recibido; no ordena |
| RN-08, FR-021: título, ejes y leyenda | `SprintSeriesChart` |
| RN-09, FR-023: un único punto | `SprintSeriesChart` |
| RN-10, FR-024: serie vacía | `SprintSeriesChart` |
| RN-11, FR-027: proyecto sin sprints | `DashboardEmptyState`, según `emptyReason` |
| RN-12, FR-011: sin sprint activo | `ActiveSprintPanel` y `KeyIndicators` |
| RN-13, RN-14, FR-009, FR-010: días restantes y vencido | `SprintSchedule`, reutilizado de la 004 |
| RN-15, FR-028, FR-029: degradación por sección | `SectionError`, una instancia por sección |
| RN-16, FR-030: falla del resumen | `DashboardView` |
| RN-17, FR-031: sin actualización automática | `DashboardView` pide los datos al montarse y no programa nada |
| RN-18, FR-034, FR-035: solo integrantes | Lo aplica el backend; el `404` del resumen corta la vista |
| RN-19, FR-036: cualquier estado del proyecto | Ninguna sección depende del estado del proyecto |
| RN-21, FR-032: advertencias en dos clases | `DashboardWarnings` |
| US3-2: velocidad solo con Cerrados | `closedSprintPoints` |
